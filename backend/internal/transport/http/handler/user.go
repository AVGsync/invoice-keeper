package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

type UserUseCase interface {
	AuthenticateUser(req request.LoginRequest, ctx context.Context) (string, string, error)
	ValidateRefreshToken(token string) (model.RefreshToken, string, error)
	GenerateAccessRefreshTokens(refreshToken model.RefreshToken, userID string) (string, string, error)
	RevokeRefreshToken(token string) error
	CreateUser(user request.CreateUserRequest, ctx context.Context) error
	GetAllUsers(ctx context.Context) ([]model.User, error)
	GetAllUsersWithTotal(ctx context.Context) (response.UserListResponse, error)
	DeleteUserByID(id string, ctx context.Context) error
	GetUserByID(id string, ctx context.Context) (model.User, error)
	UpdateUserByID(ctx context.Context, userID string, req request.UserUpdateRequest) error
}

type UserHandler struct {
	useCase UserUseCase
}

func NewUserHandler(useCase UserUseCase) *UserHandler {
	return &UserHandler{
		useCase: useCase,
	}
}

// Login godoc
//
// @Summary      Авторизация сотрудника CRM
// @Description  Проверяет логин и пароль сотрудника CRM. При успехе выставляет две
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
// @Tags         CRM Auth
// @Accept       json
// @Produce      plain
// @Param        login  body      request.LoginRequest  true  "Логин и пароль"
// @Success      200    "Вход выполнен, куки AccessToken и RefreshToken установлены"
// @Failure      400    {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401    {string}  string  "Неверные учётные данные"
// @Router       /crm/auth/login [post]
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

		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    tokenAccess,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   3600, // 1 hour
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    tokenRefresh,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   3600 * 24 * 7, // 7 days
		})

		w.WriteHeader(http.StatusOK)
	}
}

// Logout godoc
//
// @Summary      Выход сотрудника из CRM
// @Description  Отзывает refresh-токен, взятый из HttpOnly-куки `RefreshToken`, и удаляет
// @Description  обе куки `AccessToken` и `RefreshToken` (Set-Cookie с MaxAge=-1).
// @Description
// @Description  Операция идемпотентна и всегда возвращает `204`:
// @Description  - куки нет или она пустая → просто чистим куки, БД не трогаем;
// @Description  - токен уже отозван / не найден в БД → ошибка отзова логируется,
// @Description    но наружу не отдаётся (клиент всё равно считается вышедшим).
// @Description  Действующий access-токен для вызова не требуется. Access-токен stateless —
// @Description  сервер его не хранит, поэтому до истечения TTL он технически валиден;
// @Description  клиент обязан удалить его сам (здесь мы лишь стираем куку).
// @Tags         CRM Auth
// @Produce      plain
// @Success      204  "Выход выполнен, куки очищены (или выходить уже нечем)"
// @Router       /crm/auth/logout [post]
func (h *UserHandler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("RefreshToken"); err == nil && c.Value != "" {
			// ошибку "не найден" глотаем — logout идемпотентен
			if err := h.useCase.RevokeRefreshToken(c.Value); err != nil {
				slog.Warn("logout: revoke failed", "error", err)
			}
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    "",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   -1,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    "",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   -1,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

// RefreshToken godoc
//
// @Summary      Обновление токенов CRM (ротация)
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
// @Tags         CRM Auth
// @Produce      plain
// @Success      200  "Токены обновлены, новые куки установлены"
// @Failure      401  {string}  string  "Refresh-токен отсутствует, невалиден, истёк или отозван"
// @Failure      500  {string}  string  "Не удалось выпустить новые токены"
// @Router       /crm/auth/refresh [post]
func (h *UserHandler) RefreshToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Обновление не требует валидного access-токена (он как раз истёк):
		// идентичность подтверждается refresh-токеном из HttpOnly-куки.
		refreshToken, err := r.Cookie("RefreshToken")
		if err != nil {
			http.Error(w, "no refresh token", http.StatusUnauthorized)
			return
		}

		refreshTokenModel, userID, err := h.useCase.ValidateRefreshToken(refreshToken.Value)
		if err != nil {
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}

		if _, err := h.useCase.GetUserByID(userID, r.Context()); err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		access, refresh, err := h.useCase.GenerateAccessRefreshTokens(refreshTokenModel, userID)
		if err != nil {
			http.Error(w, "failed to generate new tokens", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    access,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   3600, // 1 hour
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    refresh,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   3600 * 24 * 7, // 7 days
		})
	}
}

// CreateUser godoc
//
// @Summary      Создание сотрудника CRM
// @Description  Создаёт нового сотрудника CRM. Пароль хэшируется на сервере (bcrypt),
// @Description  `login` должен быть уникальным. Доступно только роли `admin`.
// @Description
// @Description  Требования к телу (иначе `400` с текстом ошибки):
// @Description  - `full_name`: 3–64 символа;
// @Description  - `login`: 3–32 символа, только `[a-zA-Z0-9_.-]`;
// @Description  - `password`: 8–72 байта (верхняя граница — ограничение bcrypt);
// @Description  - `role`: строго `admin` или `user`.
// @Description
// @Description  Коды ответов:
// @Description  - `201` — пользователь создан (тело пустое);
// @Description  - `400 invalid request body` — тело не парсится; либо текст ошибки валидации;
// @Description  - `401 unauthorized` — нет/просрочен access-токен (middleware Auth);
// @Description  - `403 forbidden` — роль не `admin` (middleware IsAdmin);
// @Description  - `409 user with this login already exists` — нарушение уникальности login;
// @Description  - `500 failed to create user` — прочая ошибка БД.
// @Tags         CRM Admin
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        user  body      request.CreateUserRequest  true  "Данные создаваемого пользователя"
// @Success      201   "Пользователь создан"
// @Failure      400   {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401   {string}  string  "Не авторизован"
// @Failure      403   {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      409   {string}  string  "Пользователь с таким login уже существует"
// @Failure      500   {string}  string  "Внутренняя ошибка сервера"
// @Router       /crm/admin/user/create [post]
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
			if errors.Is(err, model.ErrLoginTaken) {
				http.Error(w, "user with this login already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// GetAllUsers godoc
//
// @Summary      Список сотрудников CRM
// @Description  Возвращает всех сотрудников CRM (менеджеров и администраторов) без пагинации.
// @Description  Доступно только роли `admin`. Поле `password_hash` в ответ не попадает
// @Description  (скрыто на уровне сериализации).
// @Description
// @Description  Коды ответов:
// @Description  - `200` — объект с массивом пользователей в `items` и общим количеством в `total`;
// @Description  - `401 unauthorized` — нет/просрочен access-токен;
// @Description  - `403 forbidden` — роль не `admin`;
// @Description  - `500 failed to get users` — ошибка БД.
// @Tags         CRM Admin
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  response.UserListResponse  "Список пользователей с total"
// @Failure      401  {string}  string      "Не авторизован"
// @Failure      403  {string}  string      "Недостаточно прав (требуется роль admin)"
// @Failure      500  {string}  string      "Внутренняя ошибка сервера"
// @Router       /crm/admin/user/all [get]
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
// @Summary      Удаление сотрудника CRM
// @Description  Удаляет сотрудника CRM по идентификатору (query-параметр `id`).
// @Description  Доступно только роли `admin`.
// @Description
// @Description  Защитные проверки и коды ответов:
// @Description  - `id` не передан → `400 missing user id`;
// @Description  - попытка удалить собственную учётку (`id` == id из токена) →
// @Description    `400 cannot delete your own account`;
// @Description  - пользователь с таким `id` не найден → `404 user not found`;
// @Description  - удаляемый — единственный оставшийся администратор →
// @Description    `409 cannot delete the last admin` (защита от потери доступа к CRM);
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - прочая ошибка БД → `500 failed to delete user`;
// @Description  - `200` — пользователь удалён.
// @Tags         CRM Admin
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   query     string  true  "ID пользователя (UUID)"
// @Success      200  "Пользователь удалён"
// @Failure      400  {string}  string  "Не передан id или попытка удалить самого себя"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      403  {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      404  {string}  string  "Пользователь не найден"
// @Failure      409  {string}  string  "Нельзя удалить последнего администратора или пользователя из истории заказов"
// @Failure      500  {string}  string  "Внутренняя ошибка сервера"
// @Router       /crm/admin/user/delete [delete]
func (h *UserHandler) DeleteUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing user id", http.StatusBadRequest)
			return
		}

		// Запрещаем удалять собственную учётку, чтобы админ не заблокировал сам себя.
		if claims, ok := middleware.ClaimsFromContext(r.Context()); ok && claims.UserID == id {
			http.Error(w, "cannot delete your own account", http.StatusBadRequest)
			return
		}

		err := h.useCase.DeleteUserByID(id, r.Context())
		if err != nil {
			switch {
			case errors.Is(err, model.ErrUserNotFound):
				http.Error(w, "user not found", http.StatusNotFound)
			case errors.Is(err, model.ErrLastAdmin):
				http.Error(w, "cannot delete the last admin", http.StatusConflict)
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
// @Summary      Обновление сотрудника CRM
// @Description  Частичное (PATCH) обновление данных сотрудника: ФИО и/или логин.
// @Description  Роль и пароль этим методом не меняются. Доступно только роли `admin`.
// @Description
// @Description  Тело — только изменяемые поля (оба опциональны, но хотя бы одно обязательно):
// @Description  - `full_name`: если передан — 3–64 символа;
// @Description  - `login`: если передан — 3–32 символа, только `[a-zA-Z0-9_.-]`.
// @Description
// @Description  Коды ответов:
// @Description  - `200` — пользователь обновлён;
// @Description  - `400 missing user id` — не передан query-параметр `id`;
// @Description  - `400 invalid request body` — тело не парсится; либо текст ошибки валидации
// @Description    (в т.ч. `at least one field must be provided`, если оба поля пусты);
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - `500 failed to update user` — пользователь не найден ИЛИ ошибка БД
// @Description    (отдельный 404 на несуществующий id сейчас не выделяется).
// @Tags         CRM Admin
// @Accept       json
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id    query     string                      true  "ID пользователя (UUID)"
// @Param        user  body      request.UserUpdateRequest   true  "Обновляемые поля"
// @Success      200   "Пользователь обновлён"
// @Failure      400   {string}  string  "Не передан id или невалидное тело запроса"
// @Failure      401   {string}  string  "Не авторизован"
// @Failure      403   {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      500   {string}  string  "Пользователь не найден или внутренняя ошибка сервера"
// @Router       /crm/admin/user/update [patch]
func (h *UserHandler) UpdateUserByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing user id", http.StatusBadRequest)
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
			http.Error(w, "failed to update user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetMe godoc
//
// @Summary      Профиль текущего сотрудника CRM
// @Description  Возвращает сотрудника, которому принадлежит текущий access-токен. Frontend использует `id` как actor/manager ID, `role` — для показа admin-only действий, `full_name` — для подписи интерфейса. Идентификатор пользователя из query/body не принимается и не может подменить JWT actor.
// @Description  Browser CRM обычно аутентифицируется HttpOnly cookie `AccessToken`; Bearer также поддерживается. Пароль и refresh token в ответ не входят. Если access token отсутствует/просрочен, middleware завершает запрос до handler. После изменения роли или удаления пользователя клиенту следует перечитать профиль либо выполнить повторный login.
// @ID           crmMe
// @Tags         CRM
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  model.User  "Пользователь"
// @Failure      401  {string}  string      "Не авторизован"
// @Failure      500  {string}  string      "Внутренняя ошибка сервера"
// @Router       /crm/me [get]
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
