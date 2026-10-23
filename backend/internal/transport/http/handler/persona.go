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
// @Description  Справочник персон (должностей), которые администратор назначает пользователям
// @Description  при создании. Персона на права не влияет — права задаёт только роль.
// @Description  `users_count` — сколько пользователей с этой персоной. Доступно только роли `admin`.
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
// @Description  Добавляет персону в справочник. Название 1–64 символа, уникально без учёта
// @Description  регистра; описание до 255 символов. Доступно только роли `admin`.
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
// @Description  Частичное обновление: название и/или описание (хотя бы одно поле).
// @Description  Пустое описание удаляет его. Доступно только роли `admin`.
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
// @Description  Удаляет персону из справочника. Пользователи с этой персоной не удаляются —
// @Description  у них персона становится пустой (`persona: null`). Доступно только роли `admin`.
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
