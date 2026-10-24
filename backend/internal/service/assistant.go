package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// Ошибки ассистента.
var (
	ErrAssistantDisabled    = errors.New("assistant disabled")
	ErrAssistantRateLimited = errors.New("assistant rate limited")
	ErrAssistantBusy        = errors.New("assistant busy")
	ErrAssistantUnavailable = errors.New("assistant unavailable")
	ErrAssistantTimeout     = errors.New("assistant timeout")
	ErrAssistantBadQuestion = errors.New("assistant bad question")
)

// RateLimitedError несёт Retry-After для ответа 429.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return "assistant rate limited" }
func (e *RateLimitedError) Unwrap() error { return ErrAssistantRateLimited }

const (
	// Канал bot по HTTP запрещён: его ставит только Telegram-бот.
	channelWeb     = "web"
	channelMiniApp = "miniapp"
	channelBot     = "bot"

	actorIssuer   = "invoice-keeper-backend"
	actorAudience = "nlq"
	actorTTL      = 60 * time.Second

	assistantHealthTTL = 10 * time.Second
	questionMaxRunes   = 500
)

// AssistantUserReader читает пользователя, чтобы перечитать актуальную роль из БД (в app-JWT роль может устареть до TTL_ACCESS_TOKEN).
type AssistantUserReader interface {
	GetUserByID(id string, ctx context.Context) (model.User, error)
}

// AssistantConfig — параметры сервиса ассистента (подмножество apiserver.Config).
type AssistantConfig struct {
	Enabled       bool
	AgentURL      string
	Timeout       time.Duration
	RatePerMin    int
	DailyLimit    int
	MaxConcurrent int
	ActorKey      ed25519.PrivateKey
}

// AssistantService — публичная точка входа ассистента.
type AssistantService struct {
	users      AssistantUserReader
	cfg        AssistantConfig
	httpClient *http.Client
	clock      clock.Clock

	limiter *userLimiter
	sem     chan struct{}

	// history — память диалога.
	history *AssistantHistoryService
	// audit — журнал действий; по умолчанию Nop.
	audit audit.Recorder

	healthMu   sync.Mutex
	healthAt   time.Time
	healthResp agentHealth
	healthOK   bool
}

// agentHealth — разобранный ответ агента GET /healthz.
type agentHealth struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
}

// NewAssistantService собирает сервис.
func NewAssistantService(users AssistantUserReader, cfg AssistantConfig, clk clock.Clock) *AssistantService {
	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = 4
	}
	return &AssistantService{
		users:      users,
		cfg:        cfg,
		clock:      clk,
		httpClient: &http.Client{},
		limiter:    newUserLimiter(cfg.RatePerMin, cfg.DailyLimit, clk),
		sem:        make(chan struct{}, maxConc),
		audit:      audit.Nop{},
	}
}

// SetHistory подключает память диалога.
func (s *AssistantService) SetHistory(h *AssistantHistoryService) { s.history = h }

// SetAuditRecorder подключает журнал действий (события assistant.ask).
func (s *AssistantService) SetAuditRecorder(r audit.Recorder) {
	if r != nil {
		s.audit = r
	}
}

// Enabled сообщает, включена ли фича.
func (s *AssistantService) Enabled() bool { return s.cfg.Enabled }

// Timeout — общий бюджет одного вопроса.
func (s *AssistantService) Timeout() time.Duration { return s.cfg.Timeout }

// Examples — подсказки для UI.
func (s *AssistantService) Examples() []string {
	return []string{
		"Что просрочено?",
		"Сколько платим в долларах в этом месяце?",
		"Кто отвечает за аренду?",
	}
}

// ValidateQuestion приводит и проверяет вопрос: trim, 1–500 рун, без управляющих символов.
func ValidateQuestion(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", ErrAssistantBadQuestion
	}
	n := 0
	for _, r := range q {
		n++
		if n > questionMaxRunes {
			return "", ErrAssistantBadQuestion
		}
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.Is(unicode.C, r) {
			return "", ErrAssistantBadQuestion
		}
	}
	return q, nil
}

// ValidateChannel проверяет канал для HTTP: web (по умолчанию) или miniapp.
func ValidateChannel(ch string) (string, error) {
	switch ch {
	case "", channelWeb:
		return channelWeb, nil
	case channelMiniApp:
		return channelMiniApp, nil
	default:
		return "", ErrAssistantBadQuestion
	}
}

// Ask — непотоковый вопрос.
func (s *AssistantService) Ask(ctx context.Context, userID, channel, question string) (model.AssistantAnswer, error) {
	return s.askOnce(ctx, userID, channel, conversationForChannel(channel), question)
}

// AskIn — непотоковый вопрос с явным разговором (бот в личке/группе).
func (s *AssistantService) AskIn(ctx context.Context, userID, conversation, question string) (model.AssistantAnswer, error) {
	return s.askOnce(ctx, userID, channelBot, conversation, question)
}

// askOnce выполняет непотоковый вопрос, разбирает конверт в model.AssistantAnswer.
func (s *AssistantService) askOnce(ctx context.Context, userID, channel, conversation, question string) (model.AssistantAnswer, error) {
	resp, release, err := s.dispatch(ctx, userID, channel, conversation, question, "", "/v1/ask")
	if err != nil {
		return model.AssistantAnswer{}, err
	}
	defer release()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return model.AssistantAnswer{}, ErrAssistantUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("assistant: agent answered non-200", "status", resp.StatusCode)
		return model.AssistantAnswer{}, mapUpstreamStatus(resp.StatusCode)
	}
	return parseEnvelope(body)
}

// ResetConversation очищает один разговор пользователя (бот /new, /reset).
func (s *AssistantService) ResetConversation(ctx context.Context, userID, channel, conversation string) (int, error) {
	if s.history == nil {
		return 0, nil
	}
	return s.history.Clear(ctx, userID, channel, conversation, false)
}

func (s *AssistantService) Proxy(ctx context.Context, userID, channel, question, requestID, path string) (*http.Response, func(), error) {
	return s.dispatch(ctx, userID, channel, channel, question, requestID, path)
}

// conversationForChannel выбирает область диалога по каналу.
func conversationForChannel(channel string) string {
	if channel == channelBot {
		return "tg:dm"
	}
	return channel
}

// agentRequest — тело запроса к агенту.
type agentRequest struct {
	Question string        `json:"question"`
	Channel  string        `json:"channel"`
	History  []HistoryTurn `json:"history,omitempty"`
}

// dispatch — общий путь: фича включена, лимиты, семафор, роль из БД, actor-JWT, память в промпт, HTTP к агенту.
func (s *AssistantService) dispatch(ctx context.Context, userID, channel, conversation, question, requestID, path string) (*http.Response, func(), error) {
	if !s.cfg.Enabled {
		return nil, nil, ErrAssistantDisabled
	}
	if allowed, retryAfter := s.limiter.allow(userID); !allowed {
		s.auditAskFailure(ctx, channel, userID, "", "rate_limited")
		return nil, nil, &RateLimitedError{RetryAfter: retryAfter}
	}

	// Семафор процесса: не больше MaxConcurrent одновременных вопросов.
	select {
	case s.sem <- struct{}{}:
	default:
		s.auditAskFailure(ctx, channel, userID, "", "busy")
		return nil, nil, ErrAssistantBusy
	}
	released := false
	releaseSem := func() {
		if !released {
			released = true
			<-s.sem
		}
	}

	user, err := s.users.GetUserByID(userID, ctx)
	if err != nil {
		releaseSem()
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, model.ErrUserNotFound) {
			s.auditAskFailure(ctx, channel, userID, "", "bad_question")
			return nil, nil, ErrAssistantBadQuestion
		}
		slog.Error("assistant: load user", "error", err)
		s.auditAskFailure(ctx, channel, userID, "", "unavailable")
		return nil, nil, ErrAssistantUnavailable
	}

	token, err := s.mintActor(user.ID, user.Role, channel, requestID)
	if err != nil {
		releaseSem()
		slog.Error("assistant: mint actor token", "error", err)
		s.auditAskFailure(ctx, channel, user.ID, user.Role, "unavailable")
		return nil, nil, ErrAssistantUnavailable
	}

	// История в промпт.
	var history []HistoryTurn
	if s.history != nil && s.history.Enabled() {
		if h, _, herr := s.history.ContextFor(ctx, user.ID, conversation); herr != nil {
			slog.Warn("assistant: load history", "error", herr)
		} else {
			history = h
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)

	payload, _ := json.Marshal(agentRequest{Question: question, Channel: channel, History: history})
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.cfg.AgentURL+path, bytes.NewReader(payload))
	if err != nil {
		cancel()
		releaseSem()
		s.auditAskFailure(ctx, channel, user.ID, user.Role, "unavailable")
		return nil, nil, ErrAssistantUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		cancel()
		releaseSem()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			s.auditAskFailure(ctx, channel, user.ID, user.Role, "timeout")
			return nil, nil, ErrAssistantTimeout
		}
		if errors.Is(err, context.Canceled) {
			// Клиент отключился: не наша неисправность, аудит не пишем.
			return nil, nil, context.Canceled
		}
		slog.Warn("assistant: agent request failed", "error", err)
		s.auditAskFailure(ctx, channel, user.ID, user.Role, "unavailable")
		return nil, nil, ErrAssistantUnavailable
	}

	// Захват конверта для памяти и аудита — только при 200 и включённой памяти.
	if resp.StatusCode == http.StatusOK && s.history != nil && s.history.Enabled() {
		stream := strings.HasSuffix(path, "/stream")
		base := auditContext(ctx, channel, user.ID, user.Role)
		resp.Body = newCaptureBody(resp.Body, stream, func(raw []byte) {
			s.onEnvelope(base, user.ID, channel, conversation, question, raw)
		})
	}

	release := func() {
		_ = resp.Body.Close()
		cancel()
		releaseSem()
	}
	return resp, release, nil
}

// onEnvelope разбирает захваченный конверт, пишет ход истории и аудит assistant.ask.
func (s *AssistantService) onEnvelope(base context.Context, userID, channel, conversation, question string, raw []byte) {
	env, memory, ok := parseCaptureEnvelope(raw)
	if !ok {
		slog.Error("assistant: capture parse failed")
		return
	}
	octx, cancel := context.WithTimeout(context.WithoutCancel(base), 3*time.Second)
	defer cancel()

	if s.history != nil {
		if err := s.history.Append(octx, AppendRecord{
			UserID:       userID,
			Channel:      channel,
			Conversation: conversation,
			AnswerID:     env.AnswerID,
			Question:     question,
			AnswerText:   truncateRunes(env.Text, 1200),
			Status:       env.Status,
			ErrorCode:    env.ErrorCode,
			Mode:         env.Mode,
			Memory:       memory,
		}); err != nil {
			slog.Error("assistant: append history", "error", err)
		}
	}

	s.audit.Record(octx, audit.Entry{
		Action:     audit.ActionAssistantAsk,
		EntityType: audit.EntityAssistantTurn,
		EntityID:   env.AnswerID,
		ActorID:    userID,
		Outcome:    outcomeFor(env.Status),
		Payload:    map[string]any{"channel": channel, "conversation": conversation, "mode": env.Mode, "status": env.Status},
	})
}

// auditAskFailure пишет аудит отказа ассистента до ответа агента.
func (s *AssistantService) auditAskFailure(ctx context.Context, channel, userID, role, code string) {
	octx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	s.audit.Record(auditContext(octx, channel, userID, role), audit.Entry{
		Action:     audit.ActionAssistantAsk,
		EntityType: audit.EntityAssistantTurn,
		ActorID:    userID,
		ActorRole:  role,
		Outcome:    audit.OutcomeFailure,
		Payload:    map[string]any{"channel": channel, "error_code": code},
	})
}

func auditContext(ctx context.Context, channel, userID, role string) context.Context {
	m, _ := audit.MetaFrom(ctx)
	if m.Channel == "" {
		m.Channel = channel
	}
	if m.ActorID == "" {
		m.ActorID = userID
	}
	if m.ActorRole == "" {
		m.ActorRole = role
	}
	return audit.WithMeta(ctx, m)
}

// captureEnvelope — поля конверта, нужные для памяти и аудита.
type captureEnvelope struct {
	AnswerID  string
	Status    string
	ErrorCode string
	Mode      string
	Text      string
}

// parseCaptureEnvelope разбирает захваченный конверт.
func parseCaptureEnvelope(raw []byte) (captureEnvelope, []byte, bool) {
	var e struct {
		AnswerID  string          `json:"answer_id"`
		Status    string          `json:"status"`
		ErrorCode string          `json:"error_code"`
		Mode      string          `json:"mode"`
		Text      string          `json:"text"`
		Memory    json.RawMessage `json:"memory"`
	}
	if err := json.Unmarshal(raw, &e); err != nil || e.Status == "" {
		return captureEnvelope{}, nil, false
	}
	var memory []byte
	if n := len(e.Memory); n > 0 && n <= 8<<10 {
		if trimmed := bytes.TrimSpace(e.Memory); len(trimmed) > 0 && trimmed[0] == '{' {
			memory = append([]byte(nil), e.Memory...)
		}
	}
	return captureEnvelope{e.AnswerID, e.Status, e.ErrorCode, e.Mode, e.Text}, memory, true
}

// outcomeFor маппит статус конверта в исход аудита.
func outcomeFor(status string) string {
	if status == "error" {
		return audit.OutcomeFailure
	}
	return audit.OutcomeSuccess
}

// truncateRunes обрезает строку до n рун (без порчи рун).
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}

// mintActor подписывает короткоживущий actor-JWT.
func (s *AssistantService) mintActor(sub, role, channel, requestID string) (string, error) {
	now := s.clock.Now()
	claims := jwt.MapClaims{
		"iss":  actorIssuer,
		"aud":  actorAudience,
		"sub":  sub,
		"role": role,
		"ch":   channel,
		"iat":  now.Unix(),
		"nbf":  now.Unix(),
		"exp":  now.Add(actorTTL).Unix(),
	}
	if requestID != "" {
		claims["rid"] = requestID
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.cfg.ActorKey)
}

// Status возвращает состояние ассистента для GET /status.
func (s *AssistantService) Status(ctx context.Context) (healthy bool, mode string) {
	if !s.cfg.Enabled {
		return false, ""
	}
	s.healthMu.Lock()
	fresh := time.Since(s.healthAt) < assistantHealthTTL && !s.healthAt.IsZero()
	cachedOK, cached := s.healthOK, s.healthResp
	s.healthMu.Unlock()
	if fresh {
		return cachedOK, cached.Mode
	}

	ok, h := s.probeHealth(ctx)
	s.healthMu.Lock()
	s.healthAt = time.Now()
	s.healthOK = ok
	s.healthResp = h
	s.healthMu.Unlock()
	return ok, h.Mode
}

func (s *AssistantService) probeHealth(ctx context.Context) (bool, agentHealth) {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, s.cfg.AgentURL+"/healthz", nil)
	if err != nil {
		return false, agentHealth{}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, agentHealth{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, agentHealth{}
	}
	var h agentHealth
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&h); err != nil {
		return false, agentHealth{}
	}
	return h.Status == "ok", h
}

// parseEnvelope разбирает конверт агента в domain-модель для бота.
func parseEnvelope(body []byte) (model.AssistantAnswer, error) {
	var env struct {
		AnswerID  string           `json:"answer_id"`
		Status    string           `json:"status"`
		ErrorCode string           `json:"error_code"`
		Mode      string           `json:"mode"`
		Text      string           `json:"text"`
		Blocks    []map[string]any `json:"blocks"`
		Followups []string         `json:"followups"`
		Notes     []string         `json:"notes"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return model.AssistantAnswer{}, ErrAssistantUnavailable
	}
	return model.AssistantAnswer{
		AnswerID:  env.AnswerID,
		Status:    env.Status,
		ErrorCode: env.ErrorCode,
		Mode:      env.Mode,
		Text:      env.Text,
		Blocks:    env.Blocks,
		Followups: env.Followups,
		Notes:     env.Notes,
	}, nil
}

// mapUpstreamStatus маппит не-200 ответ агента в ошибку сервиса.
func mapUpstreamStatus(code int) error {
	switch code {
	case http.StatusBadRequest:
		return ErrAssistantBadQuestion
	default:
		return ErrAssistantUnavailable
	}
}

type limWindow struct {
	count   int
	resetAt time.Time
}

type userLimiter struct {
	mu         sync.Mutex
	perMin     map[string]*limWindow
	perDay     map[string]*limWindow
	ratePerMin int
	dailyLimit int
	clock      clock.Clock
}

func newUserLimiter(ratePerMin, dailyLimit int, clk clock.Clock) *userLimiter {
	return &userLimiter{
		perMin:     make(map[string]*limWindow),
		perDay:     make(map[string]*limWindow),
		ratePerMin: ratePerMin,
		dailyLimit: dailyLimit,
		clock:      clk,
	}
}

// allow проверяет минутный и суточный лимиты.
func (l *userLimiter) allow(userID string) (bool, time.Duration) {
	now := l.clock.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	minW := l.windowLocked(l.perMin, userID, now, time.Minute)
	dayW := l.windowLocked(l.perDay, userID, now, 24*time.Hour)

	if l.ratePerMin > 0 && minW.count >= l.ratePerMin {
		return false, retryAfter(now, minW.resetAt)
	}
	if l.dailyLimit > 0 && dayW.count >= l.dailyLimit {
		return false, retryAfter(now, dayW.resetAt)
	}
	minW.count++
	dayW.count++
	return true, 0
}

func (l *userLimiter) windowLocked(m map[string]*limWindow, userID string, now time.Time, span time.Duration) *limWindow {
	w := m[userID]
	if w == nil || !now.Before(w.resetAt) {
		w = &limWindow{count: 0, resetAt: now.Add(span)}
		m[userID] = w
	}
	return w
}

func retryAfter(now, resetAt time.Time) time.Duration {
	d := resetAt.Sub(now)
	if d < time.Second {
		d = time.Second
	}
	return d
}
