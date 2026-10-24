package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// MemoryConfig — параметры памяти диалога.
type MemoryConfig struct {
	// Enabled — читать/писать историю и слать history агенту.
	Enabled bool
	// Turns — сколько свежих ходов идёт в промпт (in_prompt), 0..8.
	Turns int
	// TTL — максимальный возраст хода для попадания в промпт.
	TTL time.Duration
	// MaxTurns — сколько ходов передаётся агенту всего (in_prompt:false видит только recall_conversation), Turns..50.
	MaxTurns int
	// Keep — сколько ходов разговора хранить (прочее prune после вставки).
	Keep int
}

// HistoryTurn — компактный ход для передачи агенту.
type HistoryTurn struct {
	Q        string          `json:"q"`
	A        string          `json:"a"`
	At       string          `json:"at"`
	Status   string          `json:"status"`
	InPrompt bool            `json:"in_prompt"`
	Memory   json.RawMessage `json:"memory"`
}

// AppendRecord — данные одного хода для записи (из конверта ответа агента).
type AppendRecord struct {
	UserID       string
	Channel      string
	Conversation string
	AnswerID     string
	Question     string
	AnswerText   string
	Status       string
	ErrorCode    string
	Mode         string
	Memory       []byte // сырой объект memory (≤8 КБ, иначе {})
}

// AssistantHistoryRepo — хранилище ходов (реализует postgres.AssistantHistoryRepository).
type AssistantHistoryRepo interface {
	Append(ctx context.Context, t model.AssistantTurn) (model.AssistantTurn, error)
	ListRecent(ctx context.Context, userID, conversation string, limit int, statuses []string) ([]model.AssistantTurn, error)
	Count(ctx context.Context, userID, conversation string) (int, error)
	DeleteConversation(ctx context.Context, userID, conversation string) (int, error)
	DeleteAllForUser(ctx context.Context, userID string) (int, error)
	Prune(ctx context.Context, userID, conversation string, keep int) error
}

// AssistantHistoryService — история диалогов ассистента: контекст для уточняющих вопросов, чтение и очистка.
type AssistantHistoryService struct {
	repo  AssistantHistoryRepo
	cfg   MemoryConfig
	clock clock.Clock
	audit audit.Recorder
}

// NewAssistantHistoryService собирает сервис истории.
func NewAssistantHistoryService(repo AssistantHistoryRepo, cfg MemoryConfig, clk clock.Clock) *AssistantHistoryService {
	return &AssistantHistoryService{repo: repo, cfg: cfg, clock: clk, audit: audit.Nop{}}
}

// SetAuditRecorder подключает журнал действий.
func (s *AssistantHistoryService) SetAuditRecorder(r audit.Recorder) {
	if r != nil {
		s.audit = r
	}
}

// Enabled сообщает, включена ли память (NLQ_MEMORY_ENABLED).
func (s *AssistantHistoryService) Enabled() bool { return s.cfg.Enabled }

// Config возвращает конфигурацию памяти (для GET /status и /history).
func (s *AssistantHistoryService) Config() MemoryConfig { return s.cfg }

// selectContext помечает InContext у последних N подряд-идущих с конца ходов, пока их возраст укладывается в ttl.
func selectContext(turns []model.AssistantTurn, now time.Time, turnsN int, ttl time.Duration) int {
	if turnsN <= 0 {
		return 0
	}
	cutoff := now.Add(-ttl)
	marked := 0
	for i := len(turns) - 1; i >= 0; i-- {
		if marked >= turnsN {
			break
		}
		if turns[i].CreatedAt.Before(cutoff) {
			break
		}
		turns[i].InContext = true
		marked++
	}
	return marked
}

// ContextFor грузит контекст разговора для промпта: до MaxTurns ходов ok|partial (старые → новые) с проставленным in_prompt.
func (s *AssistantHistoryService) ContextFor(ctx context.Context, userID, conversation string) ([]HistoryTurn, int, error) {
	if s == nil || !s.cfg.Enabled {
		return nil, 0, nil
	}
	turns, err := s.repo.ListRecent(ctx, userID, conversation, s.cfg.MaxTurns, []string{"ok", "partial"})
	if err != nil {
		return nil, 0, err
	}
	reverse(turns)
	inPrompt := selectContext(turns, s.clock.Now(), s.cfg.Turns, s.cfg.TTL)

	out := make([]HistoryTurn, 0, len(turns))
	for _, t := range turns {
		mem := json.RawMessage(t.Memory)
		if len(mem) == 0 {
			mem = json.RawMessage("{}")
		}
		out = append(out, HistoryTurn{
			Q:        t.Question,
			A:        t.AnswerText,
			At:       t.CreatedAt.UTC().Format(time.RFC3339),
			Status:   t.Status,
			InPrompt: t.InContext,
			Memory:   mem,
		})
	}
	return out, inPrompt, nil
}

// Append записывает ход (после успешного ответа агента) и подрезает разговор до Keep.
func (s *AssistantHistoryService) Append(ctx context.Context, rec AppendRecord) error {
	if s == nil || !s.cfg.Enabled {
		return nil
	}
	turn := model.AssistantTurn{
		UserID:       rec.UserID,
		Channel:      rec.Channel,
		Conversation: rec.Conversation,
		AnswerID:     rec.AnswerID,
		Question:     rec.Question,
		AnswerText:   rec.AnswerText,
		Status:       rec.Status,
		Mode:         rec.Mode,
		Memory:       rec.Memory,
	}
	if rec.ErrorCode != "" {
		turn.ErrorCode = &rec.ErrorCode
	}
	if _, err := s.repo.Append(ctx, turn); err != nil {
		return err
	}
	if err := s.repo.Prune(ctx, rec.UserID, rec.Conversation, s.cfg.Keep); err != nil {
		slog.Error("assistant history: prune failed", "error", err)
	}
	return nil
}

func (s *AssistantHistoryService) History(ctx context.Context, userID, conversation string, limit int) ([]model.AssistantTurn, int, error) {
	total, err := s.repo.Count(ctx, userID, conversation)
	if err != nil {
		return nil, 0, err
	}

	recentOK, err := s.repo.ListRecent(ctx, userID, conversation, s.cfg.MaxTurns, []string{"ok", "partial"})
	if err != nil {
		return nil, 0, err
	}
	reverse(recentOK)
	selectContext(recentOK, s.clock.Now(), s.cfg.Turns, s.cfg.TTL)
	inContext := make(map[string]bool, len(recentOK))
	for _, t := range recentOK {
		if t.InContext {
			inContext[t.ID] = true
		}
	}

	items, err := s.repo.ListRecent(ctx, userID, conversation, limit, nil)
	if err != nil {
		return nil, 0, err
	}
	reverse(items)
	for i := range items {
		items[i].InContext = inContext[items[i].ID]
	}
	return items, total, nil
}

// Clear удаляет историю: разговор или (all) все разговоры пользователя.
func (s *AssistantHistoryService) Clear(ctx context.Context, userID, channel, conversation string, all bool) (int, error) {
	var (
		deleted int
		err     error
	)
	if all {
		deleted, err = s.repo.DeleteAllForUser(ctx, userID)
	} else {
		deleted, err = s.repo.DeleteConversation(ctx, userID, conversation)
	}
	if err != nil {
		return 0, err
	}

	scope := conversation
	if all {
		scope = "all"
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionAssistantHistoryClear,
		EntityType: audit.EntityAssistantConversation,
		EntityID:   scope,
		ActorID:    userID,
		Payload:    map[string]any{"deleted": deleted, "channel": channel, "conversation": scope},
	})
	return deleted, nil
}

// reverse переворачивает срез ходов на месте (DESC → ASC).
func reverse(turns []model.AssistantTurn) {
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
}
