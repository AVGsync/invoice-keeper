package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

type EmailVerifyUseCase interface {
	SendCode(ctx context.Context, userID string) error
	Verify(ctx context.Context, userID, code string) error
}

type EmailVerifyHandler struct {
	useCase EmailVerifyUseCase
}

func NewEmailVerifyHandler(u EmailVerifyUseCase) *EmailVerifyHandler {
	return &EmailVerifyHandler{useCase: u}
}

func writeEmailVerifyError(w http.ResponseWriter, err error) {
	var tooSoon *service.EmailCodeTooSoonError
	switch {
	case errors.Is(err, model.ErrNotificationSettingsNotFound):
		http.Error(w, "notification settings not found: save the email first", http.StatusNotFound)
	case errors.Is(err, service.ErrEmailAlreadyVerified):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.As(err, &tooSoon):
		w.Header().Set("Retry-After", strconv.Itoa(int(tooSoon.RetryAfter.Seconds())+1))
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, service.ErrEmailTooManyAttempts):
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, service.ErrEmailCodeInvalid), errors.Is(err, service.ErrEmailCodeExpired):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrMailDisabled):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, "failed to send email: "+err.Error(), http.StatusBadGateway)
	}
}

// SendCode godoc
// @Summary      Отправить код подтверждения почты
// @Description  Шлёт шестизначный код на адрес из настроек уведомлений. Код действует 15 минут, повторно можно запросить через минуту.
// @Description  Сначала нужно сохранить адрес через `POST` или `PUT /notifications/settings`.
// @Tags         Notifications
// @Security     BearerAuth
// @Success      202 "Код отправлен"
// @Failure      404 {string} string "Настройки не созданы"
// @Failure      409 {string} string "Адрес уже подтверждён"
// @Failure      429 {string} string "Слишком часто (см. Retry-After)"
// @Failure      502 {string} string "Не удалось отправить письмо"
// @Failure      503 {string} string "SMTP не настроен"
// @Router       /notifications/email/code [post]
func (h *EmailVerifyHandler) SendCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		if err := h.useCase.SendCode(r.Context(), claims.UserID); err != nil {
			writeEmailVerifyError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

type verifyEmailRequest struct {
	// Код из письма.
	Code string `json:"code" example:"123456"`
}

// VerifyEmail godoc
// @Summary      Подтвердить почту кодом
// @Description  После пяти неверных попыток код сгорает: запросите новый. Подтверждённый адрес получает письма-напоминания.
// @Tags         Notifications
// @Security     BearerAuth
// @Accept       json
// @Param        request body verifyEmailRequest true "Код"
// @Success      204 "Почта подтверждена"
// @Failure      400 {string} string "Неверный или просроченный код"
// @Failure      404 {string} string "Настройки не созданы"
// @Failure      409 {string} string "Адрес уже подтверждён"
// @Failure      429 {string} string "Слишком много неверных кодов"
// @Router       /notifications/email/verify [post]
func (h *EmailVerifyHandler) Verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}
		var req verifyEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.useCase.Verify(r.Context(), claims.UserID, req.Code); err != nil {
			writeEmailVerifyError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
