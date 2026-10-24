package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

type NotificationsUseCase interface {
	GetByUserID(userID string, ctx context.Context) (model.UserNotificationSettings, error)
	Create(userID string, req request.UpdateUserNotificationSettingsRequest, ctx context.Context) error
	Update(userID string, req request.UpdateUserNotificationSettingsRequest, ctx context.Context) error
	Patch(userID string, req request.PatchUserNotificationSettingsRequest, ctx context.Context) error
	Delete(userID string, ctx context.Context) error
}

type NotificationsHandler struct {
	useCase NotificationsUseCase
}

func NewNotificationsHandler(useCase NotificationsUseCase) *NotificationsHandler {
	return &NotificationsHandler{
		useCase: useCase,
	}
}

// GetSettings godoc
//
// @Summary      Получение настроек email-уведомлений
// @Description  Возвращает настройки email-уведомлений текущего авторизованного пользователя.
// @Description  Идентификатор пользователя берётся из access-токена, передавать его в запросе не нужно.
// @Description  Возвращаемые настройки включают email, статус подтверждения email, включение уведомлений,
// @Description  количество дней до платежа, время отправки и часовой пояс.
// @Description
// @Description  Коды ответов:
// @Description  - 200 — настройки получены;
// @Description  - 401 — пользователь не авторизован;
// @Description  - 404 — настройки ещё не созданы;
// @Description  - 500 — внутренняя ошибка.
// @ID           getNotificationSettings
// @Tags         Notifications
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  model.UserNotificationSettings  "Настройки уведомлений"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      404  {string}  string  "Настройки уведомлений не найдены"
// @Failure      500  {string}  string  "Внутренняя ошибка сервера"
// @Router       /notifications/settings [get]
func (h *NotificationsHandler) GetSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		settings, err := h.useCase.GetByUserID(claims.UserID, r.Context())
		if err != nil {
			if errors.Is(err, model.ErrNotificationSettingsNotFound) {
				http.Error(w, "notification settings not found", http.StatusNotFound)
				return
			}

			slog.Error(
				"notifications handler: get settings failed",
				"user_id", claims.UserID,
				"error", err,
			)
			http.Error(w, "failed to get notification settings", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(settings); err != nil {
			slog.Error("notifications handler: encode settings failed", "error", err)
		}
	}
}

// CreateSettings godoc
//
// @Summary      Создание настроек email-уведомлений
// @Description  Создаёт настройки уведомлений для текущего авторизованного пользователя.
// @Description  Email должен быть корректным адресом; days_before_due должен находиться в диапазоне 0–365;
// @Description  send_time задаётся в формате HH:MM; timezone должен быть корректным часовым поясом IANA.
// @Description  При создании email считается неподтверждённым (email_verified=false).
// @Description  Идентификатор пользователя берётся из access-токена.
// @Description
// @Description  Коды ответов:
// @Description  - 201 — настройки созданы;
// @Description  - 400 — невалидное тело или значения полей;
// @Description  - 401 — пользователь не авторизован;
// @Description  - 500 — ошибка создания настроек.
// @ID           createNotificationSettings
// @Tags         Notifications
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        settings  body  request.UpdateUserNotificationSettingsRequest  true  "Настройки уведомлений"
// @Success      201  "Настройки созданы"
// @Failure      400  {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      500  {string}  string  "Не удалось создать настройки"
// @Router       /notifications/settings [post]
func (h *NotificationsHandler) CreateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req request.UpdateUserNotificationSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.useCase.Create(claims.UserID, req, r.Context()); err != nil {
			if errors.Is(err, model.ErrNotificationSettingsExists) {
				http.Error(w, "notification settings already exist", http.StatusConflict)
				return
			}
			slog.Error(
				"notifications handler: create settings failed",
				"user_id", claims.UserID,
				"error", err,
			)
			http.Error(w, "failed to create notification settings", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// UpdateSettings godoc
//
// @Summary      Полное обновление настроек email-уведомлений
// @Description  Полностью обновляет настройки уведомлений текущего пользователя.
// @Description  Все обязательные поля должны присутствовать в запросе.
// @Description  При изменении email он снова становится неподтверждённым.
// @Description  Идентификатор пользователя берётся из access-токена.
// @Description
// @Description  Коды ответов:
// @Description  - 204 — настройки обновлены;
// @Description  - 400 — невалидное тело или значения полей;
// @Description  - 401 — пользователь не авторизован;
// @Description  - 404 — настройки не найдены;
// @Description  - 500 — ошибка обновления.
// @ID           updateNotificationSettings
// @Tags         Notifications
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        settings  body  request.UpdateUserNotificationSettingsRequest  true  "Полный набор настроек"
// @Success      204  "Настройки обновлены"
// @Failure      400  {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      404  {string}  string  "Настройки не найдены"
// @Failure      500  {string}  string  "Не удалось обновить настройки"
// @Router       /notifications/settings [put]
func (h *NotificationsHandler) UpdateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req request.UpdateUserNotificationSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.useCase.Update(claims.UserID, req, r.Context()); err != nil {
			if errors.Is(err, model.ErrNotificationSettingsNotFound) {
				http.Error(w, "notification settings not found", http.StatusNotFound)
				return
			}

			slog.Error(
				"notifications handler: update settings failed",
				"user_id", claims.UserID,
				"error", err,
			)
			http.Error(w, "failed to update notification settings", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// PatchSettings godoc
//
// @Summary      Частичное обновление настроек email-уведомлений
// @Description  Обновляет только переданные поля настроек текущего пользователя.
// @Description  Можно передать одно или несколько полей: email, email_enabled,
// @Description  days_before_due, send_time, timezone. Хотя бы одно поле обязательно.
// @Description  При изменении email он снова становится неподтверждённым.
// @Description  Идентификатор пользователя берётся из access-токена.
// @Description
// @Description  Коды ответов:
// @Description  - 204 — настройки обновлены;
// @Description  - 400 — невалидное тело или значения полей;
// @Description  - 401 — пользователь не авторизован;
// @Description  - 404 — настройки не найдены;
// @Description  - 500 — ошибка обновления.
// @ID           patchNotificationSettings
// @Tags         Notifications
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        settings  body  request.PatchUserNotificationSettingsRequest  true  "Изменяемые поля настроек"
// @Success      204  "Настройки обновлены"
// @Failure      400  {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      404  {string}  string  "Настройки не найдены"
// @Failure      500  {string}  string  "Не удалось обновить настройки"
// @Router       /notifications/settings [patch]
func (h *NotificationsHandler) PatchSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req request.PatchUserNotificationSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.useCase.Patch(claims.UserID, req, r.Context()); err != nil {
			if errors.Is(err, model.ErrNotificationSettingsNotFound) {
				http.Error(w, "notification settings not found", http.StatusNotFound)
				return
			}

			slog.Error(
				"notifications handler: patch settings failed",
				"user_id", claims.UserID,
				"error", err,
			)
			http.Error(w, "failed to update notification settings", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteSettings godoc
//
// @Summary      Удаление настроек email-уведомлений
// @Description  Удаляет настройки email-уведомлений текущего пользователя.
// @Description  Сам пользователь при этом не удаляется.
// @Description  Идентификатор пользователя берётся из access-токена.
// @Description
// @Description  Коды ответов:
// @Description  - 204 — настройки удалены;
// @Description  - 401 — пользователь не авторизован;
// @Description  - 404 — настройки не найдены;
// @Description  - 500 — ошибка удаления.
// @ID           deleteNotificationSettings
// @Tags         Notifications
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      204  "Настройки удалены"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      404  {string}  string  "Настройки не найдены"
// @Failure      500  {string}  string  "Не удалось удалить настройки"
// @Router       /notifications/settings [delete]
func (h *NotificationsHandler) DeleteSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if err := h.useCase.Delete(claims.UserID, r.Context()); err != nil {
			if errors.Is(err, model.ErrNotificationSettingsNotFound) {
				http.Error(w, "notification settings not found", http.StatusNotFound)
				return
			}

			slog.Error(
				"notifications handler: delete settings failed",
				"user_id", claims.UserID,
				"error", err,
			)
			http.Error(w, "failed to delete notification settings", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
