package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// TestMailHandler отправляет тестовое письмо, чтобы проверить SMTP-настройки.
type TestMailHandler struct {
	mailer Mailer // nil, если SMTP_HOST не задан
	audit  audit.Recorder
}

func NewTestMailHandler(mailer Mailer) *TestMailHandler {
	return &TestMailHandler{mailer: mailer, audit: audit.Nop{}}
}

func (h *TestMailHandler) SetAuditRecorder(r audit.Recorder) { h.audit = r }

type testEmailRequest struct {
	// Адрес получателя.
	To string `json:"to" example:"you@example.com"`
}

// SendTestEmail godoc
// @Summary      Отправить тестовое письмо
// @Description  Проверка SMTP: отправляет короткое письмо на указанный адрес через настроенный `SMTP_HOST`. Только администратор.
// @Description  Ответ `502` содержит текст ошибки SMTP-сервера (неверный ключ, неподтверждённый отправитель и т.п.).
// @Tags         Notifications
// @Security     BearerAuth
// @Accept       json
// @Param        request body testEmailRequest true "Получатель"
// @Success      202 "Письмо отправлено"
// @Failure      400 {string} string "Некорректный адрес"
// @Failure      403 {string} string "Только администратор"
// @Failure      502 {string} string "SMTP-сервер отклонил письмо"
// @Failure      503 {string} string "SMTP не настроен"
// @Router       /notifications/test-email [post]
func (h *TestMailHandler) Send() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.mailer == nil {
			http.Error(w, "smtp is not configured: set SMTP_HOST, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM", http.StatusServiceUnavailable)
			return
		}
		var req testEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		addr, err := mail.ParseAddress(strings.TrimSpace(req.To))
		if err != nil || addr.Name != "" {
			http.Error(w, "to must be a plain email address", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		sendErr := h.mailer.Send(ctx, addr.Address, "Invoice Keeper: тестовое письмо",
			"Это тестовое письмо Invoice Keeper. Если вы его читаете, SMTP настроен правильно.")

		entry := audit.Entry{
			Action:  audit.ActionNotificationTestEmail,
			Payload: map[string]any{"to_email": addr.Address},
		}
		if sendErr != nil {
			entry.Outcome = audit.OutcomeFailure
			h.audit.Record(r.Context(), entry)
			http.Error(w, "smtp send failed: "+sendErr.Error(), http.StatusBadGateway)
			return
		}
		h.audit.Record(r.Context(), entry)
		w.WriteHeader(http.StatusAccepted)
	}
}
