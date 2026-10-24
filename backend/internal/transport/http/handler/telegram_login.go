package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/telegrambot"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
)

// WebLoginUseCase — вход на сайт через бота.
type WebLoginUseCase interface {
	StartWebLogin(ctx context.Context) (code, deepLink string, expiresIn int, err error)
	PollWebLogin(ctx context.Context, code string) (status, userID string, err error)
}

// SessionIssuer открывает сессию (access + refresh) без пароля.
type SessionIssuer interface {
	IssueSession(ctx context.Context, userID string) (access, refresh string, err error)
}

// TelegramLoginHandler — вход на сайт через бота (Telegram deep link).
type TelegramLoginHandler struct {
	useCase    WebLoginUseCase
	sessions   SessionIssuer
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTelegramLoginHandler(useCase WebLoginUseCase, sessions SessionIssuer, accessTTL, refreshTTL time.Duration) *TelegramLoginHandler {
	return &TelegramLoginHandler{useCase: useCase, sessions: sessions, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

type telegramLoginStartResponse struct {
	Code      string `json:"code"`
	DeepLink  string `json:"deep_link"`
	ExpiresIn int    `json:"expires_in"`
}

type telegramLoginPollRequest struct {
	Code string `json:"code"`
}

type telegramLoginPollResponse struct {
	Status string `json:"status"`
}

// Start godoc
//
// @Summary      Вход через Telegram: начать
// @Description  Создаёт одноразовый запрос на вход и ссылку на бота. Пользователь открывает ссылку,
// @Description  подтверждает вход кнопкой в Telegram, сайт тем временем опрашивает `/auth/telegram/poll`.
// @Description  Работает для аккаунтов с уже привязанным Telegram. Запрос живёт 5 минут.
// @Tags         Auth
// @Produce      json
// @Success      201  {object}  telegramLoginStartResponse
// @Failure      503  {object}  response.ErrorResponse  "Бот выключен"
// @Router       /auth/telegram/start [post]
func (h *TelegramLoginHandler) Start() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, link, expiresIn, err := h.useCase.StartWebLogin(r.Context())
		if err != nil {
			if errors.Is(err, telegrambot.ErrBotDisabled) {
				api.WriteError(w, r, http.StatusServiceUnavailable, "telegram_disabled", "telegram bot is disabled", nil)
				return
			}
			slog.Error("telegram login: start", "error", err)
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to start login", nil)
			return
		}
		api.WriteJSON(w, http.StatusCreated, telegramLoginStartResponse{Code: code, DeepLink: link, ExpiresIn: expiresIn})
	}
}

// Poll godoc
//
// @Summary      Вход через Telegram: проверить подтверждение
// @Description  `200 {"status":"pending"}` — ждём подтверждения в боте; `200 {"status":"ok"}` — вход выполнен,
// @Description  куки `AccessToken` и `RefreshToken` установлены (запрос сжигается);
// @Description  `410 login_expired` — запрос истёк, отклонён или уже использован.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      telegramLoginPollRequest  true  "Код из start"
// @Success      200   {object}  telegramLoginPollResponse
// @Failure      410   {object}  response.ErrorResponse  "Запрос недействителен"
// @Router       /auth/telegram/poll [post]
func (h *TelegramLoginHandler) Poll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req telegramLoginPollRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || len(req.Code) > 128 {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", nil)
			return
		}
		status, userID, err := h.useCase.PollWebLogin(r.Context(), req.Code)
		if err != nil {
			slog.Error("telegram login: poll", "error", err)
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to check login", nil)
			return
		}
		switch status {
		case "pending":
			api.WriteJSON(w, http.StatusOK, telegramLoginPollResponse{Status: "pending"})
		case "confirmed":
			access, refresh, err := h.sessions.IssueSession(r.Context(), userID)
			if err != nil {
				slog.Error("telegram login: issue session", "error", err)
				api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to sign in", nil)
				return
			}
			setAuthCookie(w, "AccessToken", access, int(h.accessTTL.Seconds()))
			setAuthCookie(w, "RefreshToken", refresh, int(h.refreshTTL.Seconds()))
			api.WriteJSON(w, http.StatusOK, telegramLoginPollResponse{Status: "ok"})
		default:
			api.WriteError(w, r, http.StatusGone, "login_expired", "login request is expired", nil)
		}
	}
}
