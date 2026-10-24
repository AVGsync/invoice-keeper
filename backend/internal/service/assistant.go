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

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// Ошибки ассистента. Хендлер и бот маппят их в HTTP-коды/короткие сообщения
// (docs/NL_QUERIES.md §4.3, §9.1). Пользовательский текст и SQL в них не попадают.
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

// AssistantUserReader читает пользователя, чтобы перечитать актуальную роль из
// БД (в app-JWT роль может устареть до TTL_ACCESS_TOKEN). Реализует
// postgres.UserRepository (метод GetUserByID).
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

// AssistantService — публичная точка входа ассистента. Один экземпляр на
// процесс: держит лимиты, семафор и кэш здоровья агента (docs/NL_QUERIES.md §2.2).
type AssistantService struct {
	users      AssistantUserReader
	cfg        AssistantConfig
	httpClient *http.Client
	clock      clock.Clock

	limiter *userLimiter
	sem     chan struct{}

	healthMu   sync.Mutex
	healthAt   time.Time
	healthResp agentHealth
	healthOK   bool
}

// agentHealth — разобранный ответ агента GET /healthz (§9.2).
type agentHealth struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
}

// NewAssistantService собирает сервис. Семафор и лимитер создаются всегда, но
// при Enabled=false любой вызов Ask возвращает ErrAssistantDisabled.
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
	}
}

// Enabled сообщает, включена ли фича.
func (s *AssistantService) Enabled() bool { return s.cfg.Enabled }

// Timeout — общий бюджет одного вопроса. Хендлер по нему продлевает write
// deadline при стриминге SSE.
func (s *AssistantService) Timeout() time.Duration { return s.cfg.Timeout }

// Examples — подсказки для UI (§9.1).
func (s *AssistantService) Examples() []string {
	return []string{
		"Что просрочено?",
		"Сколько платим в долларах в этом месяце?",
		"Кто отвечает за аренду?",
	}
}

// ValidateQuestion приводит и проверяет вопрос: trim, 1–500 рун, без
// управляющих символов (§9.1). Возвращает очищенный вопрос или ErrAssistantBadQuestion.
//
// Правила зеркалят nlq/src/nlq/agent/service.py validate_question, чтобы Go
// отклонял локально ровно то же, что иначе вернул бы агент (и не тратил на это
// лимит пользователя): длину считаем по ВСЕМ рунам (включая \n и \t), а из
// запрещённых отвергаем любую руну категории Unicode C (Cc/Cf/Co/Cs — в т.ч.
// ZWJ U+200D, BOM U+FEFF, soft hyphen), кроме \n и \t.
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
// Канал bot по HTTP запрещён (§9.1).
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

// Ask — непотоковый вопрос (путь бота, порт NLAnswerer, §4.4). Применяет те же
// лимиты, что HTTP, разбирает конверт в model.AssistantAnswer.
func (s *AssistantService) Ask(ctx context.Context, userID, channel, question string) (model.AssistantAnswer, error) {
	resp, release, err := s.dispatch(ctx, userID, channel, question, "", "/v1/ask")
	if err != nil {
		return model.AssistantAnswer{}, err
	}
	defer release()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return model.AssistantAnswer{}, ErrAssistantUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		// Логируем статус (без тела): неверная пара actor-ключей, перекос часов
		// сверх leeway или not_ready иначе видны лишь как общий 503.
		slog.Warn("assistant: agent answered non-200", "status", resp.StatusCode)
		return model.AssistantAnswer{}, mapUpstreamStatus(resp.StatusCode)
	}
	return parseEnvelope(body)
}

// Proxy выполняет вопрос и отдаёт сырой ответ агента вызывающему (HTTP-хендлеру):
// он проксирует конверт байт-в-байт (для точной схемы блоков и meta) или
// стримит SSE. release закрывает тело и освобождает семафор — вызвать после
// чтения/копирования тела. При ошибке лимитов/подготовки release == nil.
func (s *AssistantService) Proxy(ctx context.Context, userID, channel, question, requestID, path string) (*http.Response, func(), error) {
	return s.dispatch(ctx, userID, channel, question, requestID, path)
}

// dispatch — общий путь: фича включена, лимиты, семафор, роль из БД, actor-JWT,
// HTTP к агенту. Возвращает «живой» *http.Response и release.
func (s *AssistantService) dispatch(ctx context.Context, userID, channel, question, requestID, path string) (*http.Response, func(), error) {
	if !s.cfg.Enabled {
		return nil, nil, ErrAssistantDisabled
	}
	if allowed, retryAfter := s.limiter.allow(userID); !allowed {
		return nil, nil, &RateLimitedError{RetryAfter: retryAfter}
	}

	// Семафор процесса: не больше MaxConcurrent одновременных вопросов.
	select {
	case s.sem <- struct{}{}:
	default:
		return nil, nil, ErrAssistantBusy
	}
	released := false
	releaseSem := func() {
		if !released {
			released = true
			<-s.sem
		}
	}

	// Роль перечитывается из БД: в app-JWT она могла устареть (§8.2).
	user, err := s.users.GetUserByID(userID, ctx)
	if err != nil {
		releaseSem()
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, model.ErrUserNotFound) {
			return nil, nil, ErrAssistantBadQuestion
		}
		slog.Error("assistant: load user", "error", err)
		return nil, nil, ErrAssistantUnavailable
	}

	token, err := s.mintActor(user.ID, user.Role, channel, requestID)
	if err != nil {
		releaseSem()
		slog.Error("assistant: mint actor token", "error", err)
		return nil, nil, ErrAssistantUnavailable
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)

	payload, _ := json.Marshal(map[string]string{"question": question, "channel": channel})
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.cfg.AgentURL+path, bytes.NewReader(payload))
	if err != nil {
		cancel()
		releaseSem()
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
			return nil, nil, ErrAssistantTimeout
		}
		if errors.Is(err, context.Canceled) {
			// Клиент отключился: не наша неисправность.
			return nil, nil, context.Canceled
		}
		slog.Warn("assistant: agent request failed", "error", err)
		return nil, nil, ErrAssistantUnavailable
	}

	release := func() {
		_ = resp.Body.Close()
		cancel()
		releaseSem()
	}
	return resp, release, nil
}

// mintActor подписывает короткоживущий actor-JWT (EdDSA, §8.2). Формат обязан
// совпадать с nlq/src/nlq/common/actor.py (aud=nlq, iss=invoice-keeper-backend,
// role перечитана из БД, exp=iat+60s).
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

// Status возвращает состояние ассистента для GET /status. healthy —
// закэшированный на 10 с результат GET /healthz агента (§9.1).
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

// mapUpstreamStatus маппит не-200 ответ агента в ошибку сервиса (§4.3).
func mapUpstreamStatus(code int) error {
	switch code {
	case http.StatusBadRequest:
		return ErrAssistantBadQuestion
	default:
		// 401 (неверный ключ — наша ошибка конфигурации), 503 (not_ready), 5xx.
		return ErrAssistantUnavailable
	}
}

// --- in-memory лимитер на пользователя ---

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

// allow проверяет минутный и суточный лимиты. Инкремент происходит только если
// оба окна под лимитом (иначе отказ не «съедает» бюджет другого окна).
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
