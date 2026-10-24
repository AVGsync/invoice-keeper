package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// AssistantUseCase — порт сервиса ассистента для HTTP-хендлера. Реализует
// *service.AssistantService (docs/NL_QUERIES.md §9.1).
type AssistantUseCase interface {
	Enabled() bool
	Examples() []string
	Status(ctx context.Context) (healthy bool, mode string)
	// Proxy выполняет вопрос и возвращает «живой» ответ агента; release закрывает
	// тело и освобождает семафор. При ошибке release == nil.
	Proxy(ctx context.Context, userID, channel, question, requestID, path string) (*http.Response, func(), error)
	Timeout() time.Duration
}

// AssistantHandler — HTTP-контракт ассистента.
type AssistantHandler struct {
	useCase AssistantUseCase
}

func NewAssistantHandler(useCase AssistantUseCase) *AssistantHandler {
	return &AssistantHandler{useCase: useCase}
}

// parseAskRequest читает и валидирует тело вопроса. channel bot по HTTP запрещён.
func parseAskRequest(w http.ResponseWriter, r *http.Request) (question, channel string, ok bool) {
	var req request.AssistantAskRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid request body", nil)
		return "", "", false
	}
	q, err := service.ValidateQuestion(req.Question)
	if err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "bad_request", "question must be 1-500 characters without control symbols", nil)
		return "", "", false
	}
	ch, err := service.ValidateChannel(req.Channel)
	if err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "bad_request", "channel must be web or miniapp", nil)
		return "", "", false
	}
	return q, ch, true
}

// writeAssistantError маппит ошибку сервиса в HTTP-код и машинный код (§4.3, §9.1).
func writeAssistantError(w http.ResponseWriter, r *http.Request, err error) {
	var rl *service.RateLimitedError
	switch {
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(rl.RetryAfter.Seconds())))
		api.WriteError(w, r, http.StatusTooManyRequests, "assistant_rate_limited", "too many questions, slow down", nil)
	case errors.Is(err, service.ErrAssistantDisabled):
		api.WriteError(w, r, http.StatusServiceUnavailable, "assistant_disabled", "assistant is disabled", nil)
	case errors.Is(err, service.ErrAssistantBusy):
		api.WriteError(w, r, http.StatusServiceUnavailable, "assistant_busy", "assistant is busy, try again shortly", nil)
	case errors.Is(err, service.ErrAssistantTimeout):
		api.WriteError(w, r, http.StatusGatewayTimeout, "assistant_timeout", "assistant did not answer in time", nil)
	case errors.Is(err, service.ErrAssistantBadQuestion):
		api.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid question", nil)
	case errors.Is(err, context.Canceled):
		// Клиент отключился: ответ уже не нужен.
	default:
		api.WriteError(w, r, http.StatusServiceUnavailable, "assistant_unavailable", "assistant is unavailable", nil)
	}
}

// Ask godoc
//
// @Summary      Вопрос ассистенту (NL-запрос о платежах)
// @Description  Задаёт ассистенту вопрос на естественном языке о платежах команды
// @Description  («что просрочено?», «сколько платим в долларах в этом месяце?»). Go
// @Description  аутентифицирует пользователя, применяет лимиты, перечитывает роль из БД,
// @Description  подписывает короткоживущий actor-JWT и проксирует вопрос в nlq-agent.
// @Description  Ответ — конверт §7.9: короткий текст плюс блоки с данными. Числа и даты
// @Description  считает БД, текст проходит grounding-check.
// @Tags         Ассистент
// @Accept       json
// @Produce      json
// @Param        request  body      request.AssistantAskRequest  true  "Вопрос и канал (web|miniapp)"
// @Success      200  {object}  response.AssistantAnswerResponse  "Конверт ответа агента"
// @Failure      400  {object}  response.ErrorResponse  "Пустой/длинный вопрос или неверный channel (bad_request)"
// @Failure      401  {object}  response.ErrorResponse  "Нет или просрочен токен"
// @Failure      429  {object}  response.ErrorResponse  "Превышен лимит (assistant_rate_limited), заголовок Retry-After"
// @Failure      503  {object}  response.ErrorResponse  "Фича выключена/занята/агент недоступен (assistant_disabled|assistant_busy|assistant_unavailable)"
// @Failure      504  {object}  response.ErrorResponse  "Агент не уложился в таймаут (assistant_timeout)"
// @Security     BearerAuth
// @Router       /assistant/ask [post]
func (h *AssistantHandler) Ask() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.useCase.Enabled() {
			api.WriteError(w, r, http.StatusServiceUnavailable, "assistant_disabled", "assistant is disabled", nil)
			return
		}
		question, channel, ok := parseAskRequest(w, r)
		if !ok {
			return
		}
		claims, _ := middleware.ClaimsFromContext(r.Context())
		requestID := api.RequestID(r.Context())

		resp, release, err := h.useCase.Proxy(r.Context(), claims.UserID, channel, question, requestID, "/v1/ask")
		if err != nil {
			writeAssistantError(w, r, err)
			return
		}
		defer release()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			writeAssistantError(w, r, service.ErrAssistantUnavailable)
			return
		}
		if resp.StatusCode != http.StatusOK {
			// Агент ответил не-200 (bad_question, not_ready): маппим и не
			// протаскиваем сырое тело наружу. Логируем статус (без тела): иначе
			// неверный actor-ключ или перекос часов виден лишь как общий 503.
			slog.Warn("assistant: agent answered non-200", "status", resp.StatusCode, "request_id", requestID)
			switch resp.StatusCode {
			case http.StatusBadRequest:
				writeAssistantError(w, r, service.ErrAssistantBadQuestion)
			default:
				writeAssistantError(w, r, service.ErrAssistantUnavailable)
			}
			return
		}
		// Конверт проксируется байт-в-байт: фронт получает точную схему блоков и meta.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// AskStream godoc
//
// @Summary      Вопрос ассистенту со стримингом (SSE)
// @Description  То же, что POST /assistant/ask, но ответ приходит потоком Server-Sent
// @Description  Events: кадры step (идёт инструмент), block (блок данных), text (текст
// @Description  после grounding) и done (полный конверт). Go проксирует поток от агента с
// @Description  немедленным flush и отключённой буферизацией (X-Accel-Buffering: no); при
// @Description  разрыве соединения с клиентом фоновая работа агента отменяется.
// @Tags         Ассистент
// @Accept       json
// @Produce      text/event-stream
// @Param        request  body      request.AssistantAskRequest  true  "Вопрос и канал (web|miniapp)"
// @Success      200  {string}  string  "Поток SSE (event: step|block|text|done|error)"
// @Failure      400  {object}  response.ErrorResponse  "bad_request"
// @Failure      401  {object}  response.ErrorResponse  "Нет или просрочен токен"
// @Failure      429  {object}  response.ErrorResponse  "assistant_rate_limited"
// @Failure      503  {object}  response.ErrorResponse  "assistant_disabled|assistant_busy|assistant_unavailable"
// @Failure      504  {object}  response.ErrorResponse  "assistant_timeout"
// @Security     BearerAuth
// @Router       /assistant/ask/stream [post]
func (h *AssistantHandler) AskStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.useCase.Enabled() {
			api.WriteError(w, r, http.StatusServiceUnavailable, "assistant_disabled", "assistant is disabled", nil)
			return
		}
		question, channel, ok := parseAskRequest(w, r)
		if !ok {
			return
		}
		claims, _ := middleware.ClaimsFromContext(r.Context())
		requestID := api.RequestID(r.Context())

		resp, release, err := h.useCase.Proxy(r.Context(), claims.UserID, channel, question, requestID, "/v1/ask/stream")
		if err != nil {
			writeAssistantError(w, r, err)
			return
		}
		defer release()

		if resp.StatusCode != http.StatusOK {
			slog.Warn("assistant: agent answered non-200", "status", resp.StatusCode, "request_id", requestID)
			switch resp.StatusCode {
			case http.StatusBadRequest:
				writeAssistantError(w, r, service.ErrAssistantBadQuestion)
			default:
				writeAssistantError(w, r, service.ErrAssistantUnavailable)
			}
			return
		}

		// SSE: отключаем буферизацию nginx и продлеваем write deadline за пределы
		// WriteTimeout сервера, иначе длинный поток оборвётся (§9.4).
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Now().Add(h.useCase.Timeout() + 10*time.Second))
		w.WriteHeader(http.StatusOK)
		_ = rc.Flush()

		// Копируем поток чанками и флашим каждый, чтобы события доходили сразу.
		// Разрыв с клиентом отменяет r.Context() → чтение resp.Body завершится
		// ошибкой, и цикл закончится (фоновая работа агента тоже отменится).
		buf := make([]byte, 4096)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
				_ = rc.Flush()
			}
			if rerr != nil {
				// Нормальный конец потока — io.EOF (кадр done уже ушёл). Если же
				// поток оборвался не по инициативе клиента (reqCtx-таймаут Go или
				// падение связи с агентом), отдаём кадр error (§9.4), чтобы фронт
				// отличил обрыв от нормального завершения.
				if rerr != io.EOF && r.Context().Err() == nil {
					code := "llm_unavailable"
					if errors.Is(rerr, context.DeadlineExceeded) {
						code = "timeout"
					}
					_, _ = w.Write([]byte("event: error\ndata: {\"code\":\"" + code + "\"}\n\n"))
					_ = rc.Flush()
				}
				return
			}
		}
	}
}

// Status godoc
//
// @Summary      Состояние ассистента
// @Description  Включена ли фича и отвечает ли агент (кэш 10 с). UI прячет ассистента при
// @Description  enabled=false. При выключенной фиче healthy=false и mode пустой.
// @Tags         Ассистент
// @Produce      json
// @Success      200  {object}  response.AssistantStatusResponse  "Статус ассистента"
// @Failure      401  {object}  response.ErrorResponse  "Нет или просрочен токен"
// @Security     BearerAuth
// @Router       /assistant/status [get]
func (h *AssistantHandler) Status() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := response.AssistantStatusResponse{
			Enabled:  h.useCase.Enabled(),
			Examples: h.useCase.Examples(),
		}
		if out.Enabled {
			out.Healthy, out.Mode = h.useCase.Status(r.Context())
		}
		api.WriteJSON(w, http.StatusOK, out)
	}
}
