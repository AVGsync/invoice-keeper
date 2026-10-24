package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// AuditUseCase — чтение журнала действий.
type AuditUseCase interface {
	List(ctx context.Context, f model.AuditFilter) ([]model.AuditEntry, int, error)
}

// AuditHandler обслуживает ленты журнала действий. Ошибки — JSON (api.WriteError).
type AuditHandler struct {
	svc AuditUseCase
	clk clock.Clock
}

func NewAuditHandler(svc AuditUseCase, clk clock.Clock) *AuditHandler {
	return &AuditHandler{svc: svc, clk: clk}
}

// AuditLog godoc
//
// @Summary      Журнал действий (админ)
// @Description  Полный журнал действий пользователей с фильтрами, сортировкой и
// @Description  пагинацией. Доступно только роли `admin`. Фильтры: actor_id, action
// @Description  (точный код или префикс с `*`, напр. `recurring.*`), entity_type,
// @Description  entity_id, channel, outcome, from/to (YYYY-MM-DD в зоне приложения
// @Description  либо RFC3339), q (подстрока по действию, сущности, payload/changes,
// @Description  ФИО актора). Мульти-значения — через запятую или повтором.
// @Tags         Audit
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        actor_id    query []string false "ID акторов (UUID)" collectionFormat(csv)
// @Param        action      query []string false "Действия (код или префикс с *)" collectionFormat(csv)
// @Param        entity_type query []string false "Типы сущностей" collectionFormat(csv)
// @Param        entity_id   query string   false "ID сущности"
// @Param        channel     query []string false "Каналы" collectionFormat(csv) Enums(web,miniapp,bot,system)
// @Param        outcome     query string   false "Исход" Enums(success,failure)
// @Param        from        query string   false "С (YYYY-MM-DD или RFC3339)"
// @Param        to          query string   false "По (YYYY-MM-DD или RFC3339)"
// @Param        q           query string   false "Поиск (≤100 символов)"
// @Param        order       query string   false "Порядок" Enums(asc,desc)
// @Param        limit       query int      false "1..200 (дефолт 50)"
// @Param        offset      query int      false "0..100000 (дефолт 0)"
// @Success      200 {object} response.AuditLogResponse
// @Failure      400 {object} response.ErrorResponse "Некорректный параметр"
// @Failure      401 {object} response.ErrorResponse "Не авторизован"
// @Failure      403 {object} response.ErrorResponse "Недостаточно прав (требуется admin)"
// @Failure      500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /audit-log [get]
func (h *AuditHandler) AuditLog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := request.ParseAuditFilter(r.URL.Query(), h.clk.Location())
		if err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}
		h.respond(w, r, f)
	}
}

// MyActivity godoc
//
// @Summary      Мои действия
// @Description  Записи журнала только текущего пользователя (actor_id берётся из
// @Description  JWT; параметр actor_id игнорируется). Остальные фильтры — как у
// @Description  GET /audit-log. Доступно любому авторизованному.
// @Tags         Audit
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        action      query []string false "Действия (код или префикс с *)" collectionFormat(csv)
// @Param        entity_type query []string false "Типы сущностей" collectionFormat(csv)
// @Param        entity_id   query string   false "ID сущности"
// @Param        channel     query []string false "Каналы" collectionFormat(csv) Enums(web,miniapp,bot,system)
// @Param        outcome     query string   false "Исход" Enums(success,failure)
// @Param        from        query string   false "С (YYYY-MM-DD или RFC3339)"
// @Param        to          query string   false "По (YYYY-MM-DD или RFC3339)"
// @Param        q           query string   false "Поиск (≤100 символов)"
// @Param        order       query string   false "Порядок" Enums(asc,desc)
// @Param        limit       query int      false "1..200 (дефолт 50)"
// @Param        offset      query int      false "0..100000 (дефолт 0)"
// @Success      200 {object} response.AuditLogResponse
// @Failure      400 {object} response.ErrorResponse "Некорректный параметр"
// @Failure      401 {object} response.ErrorResponse "Не авторизован"
// @Failure      500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /me/activity [get]
func (h *AuditHandler) MyActivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			api.WriteError(w, r, http.StatusForbidden, "forbidden", "failed get claims", nil)
			return
		}
		f, err := request.ParseAuditFilter(r.URL.Query(), h.clk.Location())
		if err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}
		// Только свои записи: actor_id из запроса игнорируется.
		f.ActorIDs = []string{claims.UserID}
		h.respond(w, r, f)
	}
}

// Actions godoc
//
// @Summary      Каталог действий журнала
// @Description  Список всех кодов действий с русскими метками и типом сущности —
// @Description  для построения фильтров в UI. Доступно любому авторизованному.
// @Tags         Audit
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200 {object} response.AuditActionsResponse
// @Router       /audit-log/actions [get]
func (h *AuditHandler) Actions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, response.AuditActionsResponse{Items: audit.Catalogue})
	}
}

// respond выполняет запрос и сериализует ленту журнала.
func (h *AuditHandler) respond(w http.ResponseWriter, r *http.Request, f model.AuditFilter) {
	entries, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to read audit log", nil)
		return
	}

	items := make([]response.AuditLogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, toAuditItem(e))
	}
	api.WriteJSON(w, http.StatusOK, response.AuditLogResponse{
		Items:  items,
		Total:  total,
		Limit:  f.Limit,
		Offset: f.Offset,
	})
}

// toAuditItem переводит строку журнала в элемент ответа.
func toAuditItem(e model.AuditEntry) response.AuditLogItem {
	var actor *response.AuditActor
	if e.ActorID != nil {
		actor = &response.AuditActor{
			ID:       *e.ActorID,
			FullName: e.ActorName,
			Role:     e.ActorRole,
		}
	}
	item := response.AuditLogItem{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		Actor:       actor,
		Channel:     e.Channel,
		Action:      e.Action,
		ActionLabel: audit.Label(e.Action),
		Outcome:     e.Outcome,
		EntityType:  e.EntityType,
		EntityID:    e.EntityID,
		IP:          e.IP,
		UserAgent:   e.UserAgent,
		RequestID:   e.RequestID,
	}
	if e.Changes != nil {
		item.Changes = json.RawMessage(e.Changes)
	}
	if e.Payload != nil {
		item.Payload = json.RawMessage(e.Payload)
	}
	return item
}
