package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserUseCase interface {
	AuthenticateUser(req request.LoginRequest, ctx context.Context) (string, string, error)
	ValidateRefreshToken(token string) (model.RefreshToken, string, error)
	GenerateAccessRefreshTokens(refreshToken model.RefreshToken, userID string) (string, string, error)
	RevokeRefreshToken(ctx context.Context, token string) error
	CreateUser(user request.CreateUserRequest, ctx context.Context) error
	GetAllUsers(ctx context.Context) ([]model.User, error)
	GetAllUsersWithTotal(ctx context.Context) (response.UserListResponse, error)
	DeleteUserByID(id string, ctx context.Context) error
	GetUserByID(id string, ctx context.Context) (model.User, error)
	UpdateUserByID(ctx context.Context, userID string, req request.UserUpdateRequest) error
}

type UserHandler struct {
	useCase    UserUseCase
	accessTTL  time.Duration
	refreshTTL time.Duration
	// audit — журнал действий; по умолчанию Nop. Используется для событий,
	// которых не видит сервис (неуспешный refresh). Боевой рекордер ставит
	// apiserver через SetAuditRecorder.
	audit audit.Recorder
}

func NewUserHandler(useCase UserUseCase, accessTTL, refreshTTL time.Duration) *UserHandler {
	return &UserHandler{
		useCase:    useCase,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		audit:      audit.Nop{},
	}
}

// SetAuditRecorder подключает журнал действий к хендлеру.
func (h *UserHandler) SetAuditRecorder(r audit.Recorder) { h.audit = r }

// setAuthCookies выставляет пару токенов HttpOnly-куками; срок жизни кук
// совпадает с TTL токенов из конфига (TTL_ACCESS_TOKEN / TTL_REFRESH_TOKEN).
func (h *UserHandler) setAuthCookies(w http.ResponseWriter, access, refresh string) {
	setAuthCookie(w, "AccessToken", access, int(h.accessTTL.Seconds()))
	setAuthCookie(w, "RefreshToken", refresh, int(h.refreshTTL.Seconds()))
}

func clearAuthCookies(w http.ResponseWriter) {
	setAuthCookie(w, "AccessToken", "", -1)
	setAuthCookie(w, "RefreshToken", "", -1)
}

func setAuthCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   maxAge,
	})
}

// userIDParam достаёт id пользователя из пути. Не-UUID отсекаем сразу:
// иначе Postgres ответит ошибкой приведения типа и клиент получит 500.
func userIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return "", false
	}
	return id, true
}

// Login godoc
//
// @Summary      Авторизация пользователя
// @Description  Проверяет логин и пароль пользователя. При успехе выставляет две
// @Description  HttpOnly-куки: `AccessToken` (TTL 1 час) и `RefreshToken` (TTL 7 дней),
// @Description  обе с флагами `Secure` и `SameSite=Strict`. Тело успешного ответа пустое —
// @Description  токены передаются только через Set-Cookie.
// @Description
// @Description  Порядок проверок и коды ответов:
// @Description  1. тело не парсится как JSON → `400 invalid request body`;
// @Description  2. не прошла валидация (пустой login или password) → `400` с текстом ошибки;
// @Description  3. пользователь не найден ИЛИ неверный пароль → `401 invalid credentials`.
// @Description  В целях защиты от перебора все ошибки аутентификации отдаются одинаково (401),
// @Description  без различия «нет такого логина» / «неверный пароль».
// @Tags         Auth
// @Accept       json
// @Produce      plain
// @Param        login  body      request.LoginRequest  true  "Логин и пароль"
// @Success      200    "Вход выполнен, куки AccessToken и RefreshToken установлены"
// @Failure      400    {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401    {string}  string  "Неверные учётные данные"
// @Router       /auth/login [post]
func (h *UserHandler) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tokenAccess, tokenRefresh, err := h.useCase.AuthenticateUser(req, r.Context())
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		h.setAuthCookies(w, tokenAccess, tokenRefresh)
		w.WriteHeader(http.StatusOK)
	}
}

// Logout godoc
//
// @Summary      Выход пользователя из системы
// @Description  Отзывает refresh-токен, взятый из HttpOnly-куки `RefreshToken`, и удаляет
// @Description  обе куки `AccessToken` и `RefreshToken` (Set-Cookie с MaxAge=-1).
// @Description
// @Description  Операция идемпотентна и всегда возвращает `204`:
// @Description  - куки нет или она пустая → просто чистим куки, БД не трогаем;
// @Description  - токен уже отозван / не найден в БД → ошибка отзыва логируется,
// @Description    но наружу не отдаётся (клиент всё равно считается вышедшим).
// @Description  Действующий access-токен для вызова не требуется. Access-токен stateless —
// @Description  сервер его не хранит, поэтому до истечения TTL он технически валиден;
// @Description  клиент обязан удалить его сам (здесь мы лишь стираем куку).
// @Tags         Auth
// @Produce      plain
// @Success      204  "Выход выполнен, куки очищены (или выходить уже нечем)"
// @Router       /auth/logout [post]
func (h *UserHandler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("RefreshToken"); err == nil && c.Value != "" {
			// ошибку "не найден" глотаем — logout идемпотентен
			if err := h.useCase.RevokeRefreshToken(r.Context(), c.Value); err != nil {
				slog.Warn("logout: revoke failed", "error", err)
			}
		}
		clearAuthCookies(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

// RefreshToken godoc
//
// @Summary      Обновление токенов (ротация)
// @Description  Обновляет пару токенов по refresh-токену из HttpOnly-куки `RefreshToken`.
// @Description  Действующий access-токен НЕ требуется (он как раз истёк) — личность
// @Description  подтверждается самим refresh-токеном.
// @Description
// @Description  Порядок проверок и коды ответов:
// @Description  1. куки `RefreshToken` нет → `401 no refresh token`;
// @Description  2. токен невалиден: не найден в БД, отозван (`revoked_at`), истёк
// @Description     (`expires_at`) или не проходит проверку подписи → `401 invalid refresh token`;
// @Description  3. пользователь из токена не найден в БД → `401 user not found`;
// @Description  4. не удалось выпустить новую пару токенов → `500 failed to generate new tokens`.
// @Description
// @Description  При успехе выполняется ротация: выпускается новая пара, СНАЧАЛА сохраняется
// @Description  новый refresh-токен, затем отзывается старый (одноразовость refresh-токена).
// @Description  Обновлённые `AccessToken` и `RefreshToken` возвращаются через Set-Cookie,
// @Description  тело ответа пустое.
// @Tags         Auth
// @Produce      plain
// @Success      200  "Токены обновлены, новые куки установлены"
// @Failure      401  {string}  string  "Refresh-токен отсутствует, невалиден, истёк или отозван"
// @Failure      500  {string}  string  "Не удалось выпустить новые токены"
// @Router       /auth/refresh [post]
func (h *UserHandler) RefreshToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Обновление не требует валидного access-токена (он как раз истёк):
		// идентичность подтверждается refresh-токеном из HttpOnly-куки.
		refreshToken, err := r.Cookie("RefreshToken")
		if err != nil {
			h.recordRefreshFailure(r, "no_token")
			http.Error(w, "no refresh token", http.StatusUnauthorized)
			return
		}

		refreshTokenModel, userID, err := h.useCase.ValidateRefreshToken(refreshToken.Value)
		if err != nil {
			h.recordRefreshFailure(r, "invalid_token")
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}

		if _, err := h.useCase.GetUserByID(userID, r.Context()); err != nil {
			h.recordRefreshFailure(r, "user_not_found")
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		access, refresh, err := h.useCase.GenerateAccessRefreshTokens(refreshTokenModel, userID)
		if err != nil {
			h.recordRefreshFailure(r, "token_error")
			http.Error(w, "failed to generate new tokens", http.StatusInternalServerError)
			return
		}

		h.setAuthCookies(w, access, refresh)
	}
}

// CreateUser godoc
//
// @Summary      Создание обычного пользователя
// @Description  Создаёт нового пользователя с ролью `user`. Роль не передаётся: администратор
// @Description  задаётся только в .env и через API не создаётся. Пароль хэшируется на сервере
// @Description  (bcrypt), `login` должен быть уникальным. Доступно только роли `admin`.
// @Description
// @Description  Требования к телу (иначе `400` с текстом ошибки):
// @Description  - `full_name`: 3–64 символа;
// @Description  - `login`: 3–32 символа, только `[a-zA-Z0-9_.-]`;
// @Description  - `password`: 8–72 байта (верхняя граница — ограничение bcrypt);
// @Description  - `persona_id`: необязательно, UUID персоны из `GET /personas`.
// @Description
// @Description  Коды ответов:
// @Description  - `201` — пользователь создан (тело пустое);
// @Description  - `400 invalid request body` — тело не парсится; либо текст ошибки валидации;
// @Description  - `400 persona not found` — персоны с таким `persona_id` нет;
// @Description  - `401 unauthorized` — нет/просрочен access-токен (middleware Auth);
// @Description  - `403 forbidden` — роль не `admin` (middleware IsAdmin);
// @Description  - `409 user with this login already exists` — нарушение уникальности login;
// @Description  - `500 failed to create user` — прочая ошибка БД.
// @Tags         Admin
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        user  body      request.CreateUserRequest  true  "Данные создаваемого пользователя"
// @Success      201   "Пользователь создан"
// @Failure      400   {string}  string  "Невалидное тело запроса, значения полей или несуществующая персона"
// @Failure      401   {string}  string  "Не авторизован"
// @Failure      403   {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      409   {string}  string  "Пользователь с таким login уже существует"
// @Failure      500   {string}  string  "Внутренняя ошибка сервера"
// @Router       /users [post]
func (h *UserHandler) CreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user request.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := user.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := h.useCase.CreateUser(user, r.Context())
		if err != nil {
			switch {
			case errors.Is(err, model.ErrLoginTaken):
				http.Error(w, "user with this login already exists", http.StatusConflict)
			case errors.Is(err, model.ErrPersonaNotFound):
				http.Error(w, "persona not found", http.StatusBadRequest)
			default:
				http.Error(w, "failed to create user", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// GetAllUsers godoc
//
// @Summary      Список пользователей
// @Description  Возвращает всех пользователей (обычных пользователей и администраторов) без пагинации.
// @Description  Доступно только роли `admin`. Поле `password_hash` в ответ не попадает
// @Description  (скрыто на уровне сериализации).
// @Description  У каждого пользователя есть поле `persona` — назначенная персона `{id, name}`
// @Description  или `null`, если персона не назначена или была удалена.
// @Description
// @Description  Коды ответов:
// @Description  - `200` — объект с массивом пользователей в `items` и общим количеством в `total`;
// @Description  - `401 unauthorized` — нет/просрочен access-токен;
// @Description  - `403 forbidden` — роль не `admin`;
// @Description  - `500 failed to get users` — ошибка БД.
// @Tags         Admin
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  response.UserListResponse  "Список пользователей с total"
// @Failure      401  {string}  string      "Не авторизован"
// @Failure      403  {string}  string      "Недостаточно прав (требуется роль admin)"
// @Failure      500  {string}  string      "Внутренняя ошибка сервера"
// @Router       /users [get]
func (h *UserHandler) GetAllUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := h.useCase.GetAllUsersWithTotal(r.Context())
		if err != nil {
			http.Error(w, "failed to get users", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}
}

// DeleteUserByID godoc
//
// @Summary      Удаление пользователя
// @Description  Удаляет обычного пользователя по идентификатору (path-параметр `id`).
// @Description  Доступно только роли `admin`.
// @Description
// @Description  Защитные проверки и коды ответов:
// @Description  - `id` не UUID → `400 invalid user id`;
// @Description  - пользователь с таким `id` не найден → `404 user not found`;
// @Description  - удаляемый — администратор (в т.ч. собственная учётка) →
// @Description    `403 admin account is managed via .env`;
// @Description  - на пользователя ссылаются другие записи → `409`;
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - прочая ошибка БД → `500 failed to delete user`;
// @Description  - `200` — пользователь удалён.
// @Tags         Admin
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      string  true  "ID пользователя (UUID)"
// @Success      200  "Пользователь удалён"
// @Failure      400  {string}  string  "id не является UUID"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      403  {string}  string  "Недостаточно прав или попытка удалить администратора"
// @Failure      404  {string}  string  "Пользователь не найден"
// @Failure      409  {string}  string  "На пользователя ссылаются другие записи"
// @Failure      500  {string}  string  "Внутренняя ошибка сервера"
// @Router       /users/{id} [delete]
func (h *UserHandler) DeleteUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDParam(w, r)
		if !ok {
			return
		}

		err := h.useCase.DeleteUserByID(id, r.Context())
		if err != nil {
			switch {
			case errors.Is(err, model.ErrUserNotFound):
				http.Error(w, "user not found", http.StatusNotFound)
			case errors.Is(err, model.ErrAdminImmutable):
				http.Error(w, model.ErrAdminImmutable.Error(), http.StatusForbidden)
			case errors.Is(err, model.ErrUserInUse):
				http.Error(w, "user is referenced and cannot be deleted", http.StatusConflict)
			default:
				http.Error(w, "failed to delete user", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// UpdateUserByID godoc
//
// @Summary      Обновление пользователя
// @Description  Частичное (PATCH) обновление данных обычного пользователя: ФИО и/или логин.
// @Description  Роль и пароль этим методом не меняются. Учётку администратора менять нельзя —
// @Description  она задаётся в .env. Доступно только роли `admin`.
// @Description
// @Description  Тело — только изменяемые поля (оба опциональны, но хотя бы одно обязательно):
// @Description  - `full_name`: если передан — 3–64 символа;
// @Description  - `login`: если передан — 3–32 символа, только `[a-zA-Z0-9_.-]`.
// @Description
// @Description  Коды ответов:
// @Description  - `200` — пользователь обновлён;
// @Description  - `400 invalid user id` — `id` не UUID;
// @Description  - `400 invalid request body` — тело не парсится; либо текст ошибки валидации
// @Description    (в т.ч. `at least one field must be provided`, если оба поля пусты);
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - `403 admin account is managed via .env` — попытка изменить администратора;
// @Description  - `404 user not found` — пользователя с таким `id` нет;
// @Description  - `409 user with this login already exists` — логин занят;
// @Description  - `500 failed to update user` — ошибка БД.
// @Tags         Admin
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id    path      string                      true  "ID пользователя (UUID)"
// @Param        user  body      request.UserUpdateRequest   true  "Обновляемые поля"
// @Success      200   "Пользователь обновлён"
// @Failure      400   {string}  string  "id не UUID или невалидное тело запроса"
// @Failure      401   {string}  string  "Не авторизован"
// @Failure      403   {string}  string  "Недостаточно прав или попытка изменить администратора"
// @Failure      404   {string}  string  "Пользователь не найден"
// @Failure      409   {string}  string  "Логин уже занят"
// @Failure      500   {string}  string  "Внутренняя ошибка сервера"
// @Router       /users/{id} [patch]
func (h *UserHandler) UpdateUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDParam(w, r)
		if !ok {
			return
		}

		var req request.UserUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := h.useCase.UpdateUserByID(r.Context(), id, req)
		if err != nil {
			switch {
			case errors.Is(err, model.ErrUserNotFound):
				http.Error(w, "user not found", http.StatusNotFound)
			case errors.Is(err, model.ErrAdminImmutable):
				http.Error(w, model.ErrAdminImmutable.Error(), http.StatusForbidden)
			case errors.Is(err, model.ErrLoginTaken):
				http.Error(w, "user with this login already exists", http.StatusConflict)
			default:
				http.Error(w, "failed to update user", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// recordRefreshFailure пишет неуспешное обновление сессии в журнал (auth.refresh,
// failure). Актор неизвестен (токен не прошёл), сущность не указывается.
func (h *UserHandler) recordRefreshFailure(r *http.Request, reason string) {
	h.audit.Record(r.Context(), audit.Entry{
		Action:  audit.ActionAuthRefresh,
		Outcome: audit.OutcomeFailure,
		Payload: map[string]any{"reason": reason},
	})
}

// Me godoc
//
// @Summary      Профиль текущего пользователя
// @Description  Возвращает пользователя, которому принадлежит текущий access-токен. Frontend использует `id` как идентификатор текущего пользователя, `role` — для показа admin-only действий, `full_name` — для подписи интерфейса. Идентификатор пользователя из query/body не принимается и не может подменить пользователя из JWT.
// @Description  Веб-панель обычно аутентифицируется HttpOnly cookie `AccessToken`; Bearer также поддерживается. Пароль и refresh token в ответ не входят. Если access token отсутствует/просрочен, middleware завершает запрос до handler. После изменения роли или удаления пользователя клиенту следует перечитать профиль либо выполнить повторный login.
// @Description  Поле `persona` — персона (должность) пользователя `{id, name}` или `null`; на права не влияет.
// @ID           authMe
// @Tags         Auth
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  model.User  "Пользователь"
// @Failure      401  {string}  string      "Не авторизован"
// @Failure      500  {string}  string      "Внутренняя ошибка сервера"
// @Router       /auth/me [get]
func (h *UserHandler) Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}

		user, err := h.useCase.GetUserByID(claims.UserID, r.Context())
		if err != nil {
			http.Error(w, "failed to get user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}
