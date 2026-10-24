package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/handler"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
)

// assistantMaxBody — лимит тела вопроса (§9.1): вопрос ≤ 500 символов, 4 КБ с запасом.
const assistantMaxBody = 4096

// buildAssistant собирает сервис ассистента. Один экземпляр на процесс: держит
// лимиты, семафор и кэш здоровья агента. Используется и HTTP-ручкой, и ботом
// (порт NLAnswerer), поэтому лимиты у них общие (docs/NL_QUERIES.md §8.6).
// Возвращает nil, когда фича выключена.
func (s *APIServer) buildAssistant() *service.AssistantService {
	if !s.config.NLQEnabled {
		return nil
	}
	return service.NewAssistantService(
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
}

// mountAssistant монтирует ручки ассистента в защищённой группе (Auth,
// LimitByUser, IsUserOrAdmin уже применены). При выключенной фиче ничего не
// монтирует: UI прячет ассистента по GET /status, которого тоже нет — фронт
// трактует отсутствие маршрута как enabled:false. Чтобы status всегда отвечал,
// при выключенной фиче монтируем только его на заглушке.
func (s *APIServer) mountAssistant(r chi.Router, mw *middleware.Middleware) {
	h := handler.NewAssistantHandler(assistantUseCaseOrDisabled(s.assistant))
	r.Group(func(r chi.Router) {
		r.Use(mw.MaxBytes(assistantMaxBody))
		r.Get("/assistant/status", h.Status())
		r.Post("/assistant/ask", h.Ask())
		r.Post("/assistant/ask/stream", h.AskStream())
	})
}

// assistantUseCaseOrDisabled возвращает рабочий сервис или безопасную заглушку
// «выключено», чтобы GET /status отвечал enabled:false, а ask — 503.
func assistantUseCaseOrDisabled(svc *service.AssistantService) handler.AssistantUseCase {
	if svc != nil {
		return svc
	}
	return disabledAssistant{}
}

// disabledAssistant — заглушка при NLQ_ENABLED=false: status отвечает
// enabled:false, ask — 503 assistant_disabled.
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
