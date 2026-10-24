package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// AssistantHistoryUseCase — порт сервиса истории ассистента (реализует
// *service.AssistantHistoryService).
type AssistantHistoryUseCase interface {
	History(ctx context.Context, userID, conversation string, limit int) ([]model.AssistantTurn, int, error)
	Clear(ctx context.Context, userID, channel, conversation string, all bool) (int, error)
}

// AssistantHistoryHandler — HTTP-контракт истории диалогов (SPEC §D.5). Ошибки —
// JSON api.WriteError; работает и при NLQ_ENABLED=false.
type AssistantHistoryHandler struct {
	svc    AssistantHistoryUseCase
	memory response.AssistantMemoryInfo
}

func NewAssistantHistoryHandler(svc AssistantHistoryUseCase, memory response.AssistantMemoryInfo) *AssistantHistoryHandler {
	return &AssistantHistoryHandler{svc: svc, memory: memory}
}

// Get godoc
//
// @Summary      История диалога с ассистентом
// @Description  Последние ходы диалога (вопрос, короткий ответ, статус) по возрастанию
// @Description  времени. in_context — попадёт ли ход в контекст следующего вопроса.
// @Description  Работает и при выключенном ассистенте. Только свои ходы.
// @Tags         Ассистент
// @Produce      json
// @Param        channel       query     string  false  "Канал: web (по умолчанию), miniapp или bot"  Enums(web,miniapp,bot)
// @Param        conversation  query     string  false  "Область диалога (для bot: tg:dm или tg:<chat_id>)"
// @Param        limit         query     int     false  "Сколько ходов вернуть (1..50, по умолчанию 20)"
// @Success      200  {object}  response.AssistantHistoryResponse  "История диалога"
// @Failure      400  {object}  response.ErrorResponse  "Неверный параметр (bad_request)"
// @Failure      401  {object}  response.ErrorResponse  "Нет или просрочен токен"
// @Failure      500  {object}  response.ErrorResponse  "internal_error"
// @Security     BearerAuth
// @Router       /assistant/history [get]
func (h *AssistantHistoryHandler) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := request.ParseAssistantHistoryGet(r.URL.Query())
		if err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), nil)
			return
		}
		claims, _ := middleware.ClaimsFromContext(r.Context())

		turns, total, err := h.svc.History(r.Context(), claims.UserID, q.Conversation, q.Limit)
		if err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to read assistant history", nil)
			return
		}

		items := make([]response.AssistantHistoryItem, 0, len(turns))
		for _, t := range turns {
			items = append(items, response.AssistantHistoryItem{
				ID:         t.ID,
				AnswerID:   t.AnswerID,
				Question:   t.Question,
				AnswerText: t.AnswerText,
				Status:     t.Status,
				ErrorCode:  t.ErrorCode,
				Mode:       t.Mode,
				CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
				InContext:  t.InContext,
			})
		}
		api.WriteJSON(w, http.StatusOK, response.AssistantHistoryResponse{
			Channel:      q.Channel,
			Conversation: q.Conversation,
			Items:        items,
			Total:        total,
			Memory:       h.memory,
		})
	}
}

// Delete godoc
//
// @Summary      Очистить историю диалога с ассистентом
// @Description  Удаляет ходы одного разговора (или все разговоры пользователя при
// @Description  channel=all). Работает и при выключенном ассистенте. Только свои ходы.
// @Tags         Ассистент
// @Param        channel       query     string  false  "Канал: web (по умолчанию), miniapp, bot или all"  Enums(web,miniapp,bot,all)
// @Param        conversation  query     string  false  "Область диалога (для bot: tg:dm или tg:<chat_id>)"
// @Success      204  "История очищена"
// @Failure      400  {object}  response.ErrorResponse  "Неверный параметр (bad_request)"
// @Failure      401  {object}  response.ErrorResponse  "Нет или просрочен токен"
// @Failure      500  {object}  response.ErrorResponse  "internal_error"
// @Security     BearerAuth
// @Router       /assistant/history [delete]
func (h *AssistantHistoryHandler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := request.ParseAssistantHistoryDelete(r.URL.Query())
		if err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), nil)
			return
		}
		claims, _ := middleware.ClaimsFromContext(r.Context())

		if _, err := h.svc.Clear(r.Context(), claims.UserID, q.Channel, q.Conversation, q.All); err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to clear assistant history", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
