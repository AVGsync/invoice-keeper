package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PersonaUseCase interface {
	GetAllPersonas(ctx context.Context) (response.PersonaListResponse, error)
	CreatePersona(req request.CreatePersonaRequest, ctx context.Context) (model.Persona, error)
	UpdatePersona(id string, req request.UpdatePersonaRequest, ctx context.Context) (model.Persona, error)
	DeletePersona(id string, ctx context.Context) error
}

type PersonaHandler struct {
	useCase PersonaUseCase
}

func NewPersonaHandler(useCase PersonaUseCase) *PersonaHandler {
	return &PersonaHandler{useCase: useCase}
}

func personaIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid persona id", http.StatusBadRequest)
		return "", false
	}
	return id, true
}

func writePersonaError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, model.ErrPersonaNotFound):
		http.Error(w, "persona not found", http.StatusNotFound)
	case errors.Is(err, model.ErrPersonaNameTaken):
		http.Error(w, model.ErrPersonaNameTaken.Error(), http.StatusConflict)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

// GetAllPersonas godoc
//
// @Summary      Список персон
// @Description  Возвращает весь справочник персон без пагинации, отсортированный по названию
// @Description  (без учёта регистра). Используется в форме создания пользователя: администратор
// @Description  выбирает персону из этого списка и передаёт её `id` в `persona_id`
// @Description  (`POST /users`).
// @Description
// @Description  Персона — это должность или зона ответственности (Бухгалтер, Руководитель, DevOps).
// @Description  На права доступа она **не влияет**: права задаёт только роль (`admin` / `user`).
// @Description
// @Description  Поля элемента:
// @Description  - `id` — UUID персоны;
// @Description  - `name` — название;
// @Description  - `description` — описание или `null`;
// @Description  - `users_count` — сколько пользователей с этой персоной (удобно показать
// @Description    перед удалением);
// @Description  - `created_at` — когда создана.
// @Description
// @Description  Доступно только роли `admin`.
// @Description
// @Description  Коды ответов:
// @Description  - `200` — объект со списком в `items` и количеством в `total` (пустой справочник —
// @Description    `items: []`, `total: 0`);
// @Description  - `401 unauthorized` — нет или просрочен access-токен (middleware Auth);
// @Description  - `403 forbidden` — роль не `admin` (middleware IsAdmin);
// @Description  - `500 failed to get personas` — ошибка БД.
// @Tags         Personas
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  response.PersonaListResponse  "Список персон с total"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      403  {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      500  {string}  string  "Внутренняя ошибка сервера"
// @Router       /personas [get]
func (h *PersonaHandler) GetAllPersonas() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		personas, err := h.useCase.GetAllPersonas(r.Context())
		if err != nil {
			http.Error(w, "failed to get personas", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, personas)
	}
}

// CreatePersona godoc
//
// @Summary      Создание персоны
// @Description  Добавляет персону в справочник. После этого её можно назначать новым
// @Description  пользователям через `persona_id` в `POST /users`. Доступно только роли `admin`.
// @Description
// @Description  Требования к телу (иначе `400` с текстом ошибки):
// @Description  - `name`: обязательно, 1–64 символа; пробелы по краям обрезаются;
// @Description  - `description`: необязательно, до 255 символов; пустая строка сохраняется как `null`.
// @Description
// @Description  Название уникально **без учёта регистра**: «Бухгалтер» и « бухгалтер » —
// @Description  одна и та же персона, повтор вернёт `409`.
// @Description
// @Description  Коды ответов:
// @Description  - `201` — персона создана, в теле — созданная персона (`users_count: 0`);
// @Description  - `400 invalid request body` — тело не парсится как JSON;
// @Description  - `400 name must be 1-64 characters` / `400 description must be at most 255 characters`
// @Description    — не прошла валидация;
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - `409 persona with this name already exists` — такое название уже есть;
// @Description  - `500 failed to create persona` — прочая ошибка БД.
// @Tags         Personas
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        persona  body      request.CreatePersonaRequest  true  "Название и описание"
// @Success      201      {object}  model.Persona  "Созданная персона"
// @Failure      400      {string}  string  "Невалидное тело запроса или значения полей"
// @Failure      401      {string}  string  "Не авторизован"
// @Failure      403      {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      409      {string}  string  "Персона с таким названием уже есть"
// @Failure      500      {string}  string  "Внутренняя ошибка сервера"
// @Router       /personas [post]
func (h *PersonaHandler) CreatePersona() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request.CreatePersonaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		persona, err := h.useCase.CreatePersona(req, r.Context())
		if err != nil {
			writePersonaError(w, err, "failed to create persona")
			return
		}
		writeJSON(w, http.StatusCreated, persona)
	}
}

// UpdatePersona godoc
//
// @Summary      Изменение персоны
// @Description  Частичное (PATCH) обновление персоны: название и/или описание. Пользователи,
// @Description  которым назначена персона, сразу видят новое название (`GET /users`,
// @Description  `GET /auth/me`) — персона хранится ссылкой, а не копией. Доступно только роли `admin`.
// @Description
// @Description  Тело — только изменяемые поля (оба необязательны, но хотя бы одно обязательно):
// @Description  - `name`: если передано — 1–64 символа, пробелы по краям обрезаются;
// @Description  - `description`: если передано — до 255 символов; пустая строка `""` удаляет описание
// @Description    (станет `null`); если не передано — не меняется.
// @Description
// @Description  Коды ответов:
// @Description  - `200` — персона обновлена, в теле — актуальная персона;
// @Description  - `400 invalid persona id` — `id` в пути не UUID;
// @Description  - `400 invalid request body` — тело не парсится; либо текст ошибки валидации
// @Description    (в т.ч. `at least one field must be provided`, если не передано ни одно поле);
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - `404 persona not found` — персоны с таким `id` нет;
// @Description  - `409 persona with this name already exists` — новое название занято
// @Description    другой персоной (без учёта регистра);
// @Description  - `500 failed to update persona` — прочая ошибка БД.
// @Tags         Personas
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id       path      string                        true  "ID персоны (UUID)"
// @Param        persona  body      request.UpdatePersonaRequest  true  "Изменяемые поля"
// @Success      200      {object}  model.Persona  "Обновлённая персона"
// @Failure      400      {string}  string  "id не UUID или невалидное тело запроса"
// @Failure      401      {string}  string  "Не авторизован"
// @Failure      403      {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      404      {string}  string  "Персона не найдена"
// @Failure      409      {string}  string  "Персона с таким названием уже есть"
// @Failure      500      {string}  string  "Внутренняя ошибка сервера"
// @Router       /personas/{id} [patch]
func (h *PersonaHandler) UpdatePersona() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := personaIDParam(w, r)
		if !ok {
			return
		}

		var req request.UpdatePersonaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := req.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		persona, err := h.useCase.UpdatePersona(id, req, r.Context())
		if err != nil {
			writePersonaError(w, err, "failed to update persona")
			return
		}
		writeJSON(w, http.StatusOK, persona)
	}
}

// DeletePersona godoc
//
// @Summary      Удаление персоны
// @Description  Удаляет персону из справочника. Доступно только роли `admin`.
// @Description
// @Description  Пользователи с этой персоной **не удаляются** и не блокируются: у них персона
// @Description  становится пустой (`persona: null` в `GET /users` и `GET /auth/me`). Права
// @Description  пользователей не меняются — персона на них не влияет. Сколько пользователей
// @Description  затронет удаление, видно в `users_count` из `GET /personas`.
// @Description
// @Description  Коды ответов:
// @Description  - `204` — персона удалена (тело пустое);
// @Description  - `400 invalid persona id` — `id` в пути не UUID;
// @Description  - `401 unauthorized` / `403 forbidden` — из middleware Auth/IsAdmin;
// @Description  - `404 persona not found` — персоны с таким `id` нет (в т.ч. уже удалена);
// @Description  - `500 failed to delete persona` — ошибка БД.
// @Tags         Personas
// @Produce      plain
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      string  true  "ID персоны (UUID)"
// @Success      204  "Персона удалена"
// @Failure      400  {string}  string  "id не UUID"
// @Failure      401  {string}  string  "Не авторизован"
// @Failure      403  {string}  string  "Недостаточно прав (требуется роль admin)"
// @Failure      404  {string}  string  "Персона не найдена"
// @Failure      500  {string}  string  "Внутренняя ошибка сервера"
// @Router       /personas/{id} [delete]
func (h *PersonaHandler) DeletePersona() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := personaIDParam(w, r)
		if !ok {
			return
		}

		if err := h.useCase.DeletePersona(id, r.Context()); err != nil {
			writePersonaError(w, err, "failed to delete persona")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
