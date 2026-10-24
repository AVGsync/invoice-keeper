package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// assistantMaxBody — лимит тела вопроса: вопрос ≤ 500 символов, 4 КБ с запасом.
const assistantMaxBody = 4096

// memoryConfig собирает конфигурацию памяти диалога из env.
func (s *APIServer) memoryConfig() service.MemoryConfig {
	return service.MemoryConfig{
		Enabled:  s.config.NLQMemoryEnabled,
		Turns:    s.config.NLQMemoryTurns,
		TTL:      s.config.NLQMemoryTTL,
		MaxTurns: s.config.NLQMemoryMaxTurns,
		Keep:     s.config.NLQHistoryKeep,
	}
}

// memoryInfo — сводка о памяти для GET /assistant/status и /history.
func (s *APIServer) memoryInfo() response.AssistantMemoryInfo {
	c := s.memoryConfig()
	return response.AssistantMemoryInfo{
		Enabled:      c.Enabled,
		ContextTurns: c.Turns,
		TTLSeconds:   int(c.TTL.Seconds()),
	}
}

// buildAssistant собирает сервис ассистента и сервис истории.
func (s *APIServer) buildAssistant() *service.AssistantService {
	s.assistantHistory = service.NewAssistantHistoryService(
		s.db.AssistantHistory(), s.memoryConfig(), clock.New(s.config.Location()))
	s.assistantHistory.SetAuditRecorder(s.audit)

	if !s.config.NLQEnabled {
		return nil
	}
	svc := service.NewAssistantService(
		s.db.User(),
		service.AssistantConfig{
			Enabled:       s.config.NLQEnabled,
			AgentURL:      s.config.NLQAgentURL,
			Timeout:       s.config.NLQTimeout,
			RatePerMin:    s.config.NLQRatePerMin,
			DailyLimit:    s.config.NLQDailyLimit,
			MaxConcurrent: s.config.NLQMaxConcurrent,
			ActorKey:      s.config.ActorKey(),
		},
		clock.New(s.config.Location()),
	)
	svc.SetHistory(s.assistantHistory)
	svc.SetAuditRecorder(s.audit)
	return svc
}

// mountAssistant монтирует ручки ассистента в защищённой группе (Auth, LimitByUser, IsUserOrAdmin уже применены).
func (s *APIServer) mountAssistant(r chi.Router, mw *middleware.Middleware) {
	h := handler.NewAssistantHandler(assistantUseCaseOrDisabled(s.assistant), s.memoryInfo())
	hh := handler.NewAssistantHistoryHandler(s.assistantHistory, s.memoryInfo())
	r.Group(func(r chi.Router) {
		r.Use(mw.MaxBytes(assistantMaxBody))
		r.Get("/assistant/status", h.Status())
		r.Post("/assistant/ask", h.Ask())
		r.Post("/assistant/ask/stream", h.AskStream())
		r.Get("/assistant/history", hh.Get())
		r.Delete("/assistant/history", hh.Delete())
	})
}

func assistantUseCaseOrDisabled(svc *service.AssistantService) handler.AssistantUseCase {
	if svc != nil {
		return svc
	}
	return disabledAssistant{}
}

// disabledAssistant — заглушка при NLQ_ENABLED=false: status отвечает enabled:false, ask — 503 assistant_disabled.
type disabledAssistant struct{}

func (disabledAssistant) Enabled() bool { return false }

func (disabledAssistant) Examples() []string {
	return []string{
		"Что просрочено?",
		"Сколько платим в долларах в этом месяце?",
		"Кто отвечает за аренду?",
	}
}

func (disabledAssistant) Status(context.Context) (bool, string) { return false, "" }

func (disabledAssistant) Proxy(context.Context, string, string, string, string, string) (*http.Response, func(), error) {
	return nil, nil, service.ErrAssistantDisabled
}

func (disabledAssistant) Timeout() time.Duration { return 0 }
