package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/telegrambot"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// TelegramUseCase — операции бота, нужные REST-слою. Реализует *telegrambot.Service.
type TelegramUseCase interface {
	CreateLinkCode(ctx context.Context, userID string) (deepLink string, expiresIn int, err error)
	GetLinkStatus(ctx context.Context, userID string) (telegrambot.LinkStatus, error)
	Unlink(ctx context.Context, userID string) error
	GetTeamChat(ctx context.Context) (model.TelegramChat, error)
	SendTeamChatTest(ctx context.Context) error
	HandleUpdate(ctx context.Context, upd telegram.Update)
	MiniAppAuth(ctx context.Context, initData string) (telegrambot.MiniAppResult, error)
	MiniAppLink(ctx context.Context, initData, login, password string) (telegrambot.MiniAppResult, error)
}

// TelegramHandler — HTTP-хендлеры привязки Telegram, командного чата и webhook.
type TelegramHandler struct {
	useCase       TelegramUseCase
	webhookSecret string
}

// NewTelegramHandler собирает хендлер. webhookSecret проверяется на вебхуке.
func NewTelegramHandler(useCase TelegramUseCase, webhookSecret string) *TelegramHandler {
	return &TelegramHandler{useCase: useCase, webhookSecret: webhookSecret}
}

func claimsUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		api.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return "", false
	}
	return claims.UserID, true
}

// CreateLinkCode godoc
//
// @Summary      Код привязки Telegram
// @Description  Создаёт одноразовый deep link для привязки Telegram к текущему пользователю.
// @Description  Ссылку нужно открыть в Telegram и подтвердить привязку кнопкой. Код живёт 10 минут,
// @Description  одновременно активных кодов не больше трёх.
// @Description  Коды ответов: `201` — ссылка создана; `429 too_many_link_codes` — слишком много
// @Description  активных кодов; `503 telegram_disabled` — бот выключен в конфигурации.
// @Tags         Telegram
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      201  {object}  response.TelegramLinkCodeResponse  "Deep link и TTL"
// @Failure      401  {object}  response.ErrorResponse
// @Failure      429  {object}  response.ErrorResponse  "Слишком много активных кодов"
// @Failure      503  {object}  response.ErrorResponse  "Бот выключен"
// @Router       /me/telegram/link-code [post]
func (h *TelegramHandler) CreateLinkCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := claimsUserID(w, r)
		if !ok {
			return
		}
		deepLink, expiresIn, err := h.useCase.CreateLinkCode(r.Context(), userID)
		if err != nil {
			switch {
			case errors.Is(err, telegrambot.ErrBotDisabled):
				api.WriteError(w, r, http.StatusServiceUnavailable, "telegram_disabled", "telegram bot is disabled", nil)
			case errors.Is(err, model.ErrTelegramTooManyCodes):
				api.WriteError(w, r, http.StatusTooManyRequests, "too_many_link_codes", "too many active link codes", nil)
			default:
				api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to create link code", nil)
			}
			return
		}
		api.WriteJSON(w, http.StatusCreated, response.TelegramLinkCodeResponse{DeepLink: deepLink, ExpiresIn: expiresIn})
	}
}

// GetLink godoc
//
// @Summary      Состояние привязки Telegram
// @Description  Возвращает, привязан ли Telegram у текущего пользователя, и настройки доставки.
// @Tags         Telegram
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  response.TelegramLinkResponse  "Состояние привязки"
// @Failure      401  {object}  response.ErrorResponse
// @Router       /me/telegram [get]
func (h *TelegramHandler) GetLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := claimsUserID(w, r)
		if !ok {
			return
		}
		status, err := h.useCase.GetLinkStatus(r.Context(), userID)
		if err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to get telegram link", nil)
			return
		}
		if !status.Linked {
			api.WriteJSON(w, http.StatusOK, response.TelegramLinkResponse{Linked: false})
			return
		}
		dm, blocked, linkedAt := status.DMEnabled, status.BotBlocked, status.LinkedAt
		api.WriteJSON(w, http.StatusOK, response.TelegramLinkResponse{
			Linked:     true,
			Username:   status.Username,
			DMEnabled:  &dm,
			BotBlocked: &blocked,
			LinkedAt:   &linkedAt,
		})
	}
}

// DeleteLink godoc
//
// @Summary      Отвязать Telegram
// @Description  Удаляет привязку Telegram текущего пользователя.
// @Tags         Telegram
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      204  "Отвязано"
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse  "Привязки не было"
// @Router       /me/telegram [delete]
func (h *TelegramHandler) DeleteLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := claimsUserID(w, r)
		if !ok {
			return
		}
		if err := h.useCase.Unlink(r.Context(), userID); err != nil {
			if errors.Is(err, model.ErrTelegramNotLinked) {
				api.WriteError(w, r, http.StatusNotFound, "telegram_not_linked", "telegram account is not linked", nil)
				return
			}
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to unlink", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// GetTeamChat godoc
//
// @Summary      Командный чат (админ)
// @Description  Возвращает активный командный чат, куда уходят напоминания. Доступно роли `admin`.
// @Tags         Telegram
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  response.TelegramTeamChatResponse  "Состояние командного чата"
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Router       /telegram/team-chat [get]
func (h *TelegramHandler) GetTeamChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chat, err := h.useCase.GetTeamChat(r.Context())
		if err != nil {
			if errors.Is(err, model.ErrTelegramNoTeamChat) {
				api.WriteJSON(w, http.StatusOK, response.TelegramTeamChatResponse{Bound: false})
				return
			}
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to get team chat", nil)
			return
		}
		isActive := chat.IsActive
		boundAt := chat.BoundAt
		// bound=true только пока чат активен. Неактивный чат отдаём с last_error,
		// чтобы админ видел причину остановки напоминаний (§5.4).
		api.WriteJSON(w, http.StatusOK, response.TelegramTeamChatResponse{
			Bound:     chat.IsActive,
			Title:     chat.Title,
			TGChatID:  chat.TGChatID,
			IsActive:  &isActive,
			LastError: chat.LastError,
			BoundAt:   &boundAt,
		})
	}
}

// TestTeamChat godoc
//
// @Summary      Тест командного чата (админ)
// @Description  Отправляет тестовое сообщение в командный чат. Доступно роли `admin`.
// @Tags         Telegram
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      202  "Сообщение отправлено"
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      409  {object}  response.ErrorResponse  "Командный чат не привязан"
// @Failure      502  {object}  response.ErrorResponse  "Ошибка Telegram"
// @Router       /telegram/team-chat/test [post]
func (h *TelegramHandler) TestTeamChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h.useCase.SendTeamChatTest(r.Context())
		switch {
		case err == nil:
			w.WriteHeader(http.StatusAccepted)
		case errors.Is(err, model.ErrTelegramNoTeamChat):
			api.WriteError(w, r, http.StatusConflict, "team_chat_not_bound", "team chat is not bound", nil)
		case errors.Is(err, telegrambot.ErrBotDisabled):
			api.WriteError(w, r, http.StatusServiceUnavailable, "telegram_disabled", "telegram bot is disabled", nil)
		default:
			api.WriteError(w, r, http.StatusBadGateway, "telegram_error", "failed to send test message", nil)
		}
	}
}

// writeMiniAppResult пишет успешный ответ входа в Mini App (auth и link общие).
func writeMiniAppResult(w http.ResponseWriter, res telegrambot.MiniAppResult) {
	api.WriteJSON(w, http.StatusOK, response.MiniAppAuthResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   res.ExpiresIn,
		User:        response.NewMiniAppUser(res.User),
		DMAvailable: res.DMAvailable,
	})
}

// writeInitDataError маппит ошибки проверки initData в коды §7 (общее для обоих
// Mini App-потоков). Возвращает false, если ошибка не связана с initData.
func writeInitDataError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, telegram.ErrInitDataInvalid):
		api.WriteError(w, r, http.StatusUnauthorized, "invalid_init_data", "invalid init data", nil)
	case errors.Is(err, telegram.ErrInitDataStale):
		api.WriteError(w, r, http.StatusUnauthorized, "init_data_expired", "init data is expired", nil)
	case errors.Is(err, telegrambot.ErrBotDisabled):
		api.WriteError(w, r, http.StatusServiceUnavailable, "telegram_disabled", "telegram bot is disabled", nil)
	default:
		return false
	}
	return true
}

// MiniAppAuth godoc
//
// @Summary      Вход в Mini App по привязке
// @Description  Проверяет подпись initData Telegram Mini App и, если этот Telegram уже
// @Description  привязан к аккаунту, выпускает access-токен (только access, без refresh и cookie).
// @Description  Роль берётся из БД, поэтому админ получает полные права и в Mini App.
// @Description  Коды ответов: `200` — вошли; `401 invalid_init_data` / `init_data_expired` —
// @Description  подпись неверна или данные старше 10 минут; `409 telegram_not_linked` — этот
// @Description  Telegram ещё не привязан (нужно вызвать `/telegram/miniapp/link`).
// @Description  Сырые initData не логируются. Лимит `30/min` по IP.
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        body  body      request.MiniAppAuthRequest  true  "initData из Telegram.WebApp"
// @Success      200   {object}  response.MiniAppAuthResponse  "Access-токен и профиль"
// @Failure      400   {object}  response.ErrorResponse  "Невалидное тело запроса"
// @Failure      401   {object}  response.ErrorResponse  "Неверная или устаревшая initData"
// @Failure      409   {object}  response.ErrorResponse  "Telegram не привязан"
// @Failure      503   {object}  response.ErrorResponse  "Бот выключен"
// @Router       /telegram/miniapp/auth [post]
func (h *TelegramHandler) MiniAppAuth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request.MiniAppAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", nil)
			return
		}
		if err := req.Validate(); err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}

		res, err := h.useCase.MiniAppAuth(r.Context(), req.InitData)
		if err != nil {
			if writeInitDataError(w, r, err) {
				return
			}
			if errors.Is(err, model.ErrTelegramNotLinked) {
				api.WriteError(w, r, http.StatusConflict, "telegram_not_linked", "telegram account is not linked", nil)
				return
			}
			api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to authenticate", nil)
			return
		}
		writeMiniAppResult(w, res)
	}
}

// MiniAppLink godoc
//
// @Summary      Привязка и вход в Mini App по логину/паролю
// @Description  Проверяет подпись initData, затем логин и пароль аккаунта. При успехе
// @Description  привязывает текущий Telegram к аккаунту и выпускает access-токен (пароль
// @Description  вводится один раз). Коды ответов: `200` — привязано и вошли;
// @Description  `401 invalid_init_data` / `init_data_expired` — проблема с initData;
// @Description  `401 invalid_credentials` — неверный логин или пароль (не различаются);
// @Description  `409 telegram_already_linked` — этот Telegram уже за другим аккаунтом.
// @Description  Сырые initData и пароль не логируются. Лимит `10/min` по IP, как у `/auth/login`.
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        body  body      request.MiniAppLinkRequest  true  "initData, логин и пароль"
// @Success      200   {object}  response.MiniAppAuthResponse  "Access-токен и профиль"
// @Failure      400   {object}  response.ErrorResponse  "Невалидное тело запроса"
// @Failure      401   {object}  response.ErrorResponse  "Неверная initData или учётные данные"
// @Failure      409   {object}  response.ErrorResponse  "Telegram уже привязан к другому аккаунту"
// @Failure      503   {object}  response.ErrorResponse  "Бот выключен"
// @Router       /telegram/miniapp/link [post]
func (h *TelegramHandler) MiniAppLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request.MiniAppLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", nil)
			return
		}
		if err := req.Validate(); err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}

		res, err := h.useCase.MiniAppLink(r.Context(), req.InitData, req.Login, req.Password)
		if err != nil {
			if writeInitDataError(w, r, err) {
				return
			}
			switch {
			case errors.Is(err, telegrambot.ErrMiniAppInvalidCredentials):
				api.WriteError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid login or password", nil)
			case errors.Is(err, model.ErrTelegramAlreadyLinked):
				api.WriteError(w, r, http.StatusConflict, "telegram_already_linked", "telegram is already linked to another account", nil)
			default:
				api.WriteError(w, r, http.StatusInternalServerError, "internal_error", "failed to link", nil)
			}
			return
		}
		writeMiniAppResult(w, res)
	}
}

// Webhook godoc
//
// @Summary      Webhook Telegram
// @Description  Приём апдейтов Telegram. Проверяется заголовок X-Telegram-Bot-Api-Secret-Token,
// @Description  тело разбирается как Update. Ответ всегда 200 (кроме неверного секрета — 401).
// @Tags         Telegram
// @Accept       json
// @Produce      plain
// @Success      200  "OK"
// @Failure      401  "Неверный секрет"
// @Router       /telegram/webhook [post]
func (h *TelegramHandler) Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.webhookSecret)) != 1 {
			// Неверный секрет: тело не разбираем.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var upd telegram.Update
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			// Телеграм ждёт 2xx, иначе повторит. Битый апдейт просто логируем.
			slog.Debug("telegram webhook: decode update", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		h.useCase.HandleUpdate(r.Context(), upd)
		w.WriteHeader(http.StatusOK)
	}
}
