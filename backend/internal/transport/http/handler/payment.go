package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RecurrentPaymentUseCase interface {
	CreateRecurrentPayment(req request.CreateRecurrentPaymentRequest, ctx context.Context) (string, error)
	GetRecurrentPaymentByID(id string, ctx context.Context) (model.RecurrentPayment, error)
	GetAllRecurrentPayments(ctx context.Context) ([]model.RecurrentPayment, error)
	UpdateRecurrentPayment(id string, req request.UpdateRecurrentPaymentRequest, ctx context.Context) error
	Stop(id string, ctx context.Context) error
}

type PaymentUseCase interface {
	GetPaymentByID(id string, ctx context.Context) (model.Payment, error)
	GetPaymentsByUserID(userID string, ctx context.Context) ([]model.Payment, error)
	GetAllPayments(ctx context.Context) ([]model.Payment, error)
	UpdatePayment(id string, req request.UpdatePaymentRequest, ctx context.Context) error
	MarkOccurrencePaid(ctx context.Context, id string, paidDate clock.Date, actor service.PaymentActor) error
	ChangeOccurrencePaidDate(ctx context.Context, id string, paidDate clock.Date, actor service.PaymentActor) error
	MarkOccurrenceUnpaid(ctx context.Context, id string, actor service.PaymentActor) error
}

// PaymentScheduler — генерация периодов при создании шаблона.
type PaymentScheduler interface {
	EnsureFor(ctx context.Context, rpID string) error
}

type PaymentHandler struct {
	recurrentPayment RecurrentPaymentUseCase
	payment          PaymentUseCase
	scheduler        PaymentScheduler
	clk              clock.Clock
}

func NewPaymentHandler(
	recurrentPayment RecurrentPaymentUseCase,
	payment PaymentUseCase,
	scheduler PaymentScheduler,
	clk clock.Clock,
) *PaymentHandler {
	return &PaymentHandler{
		recurrentPayment: recurrentPayment,
		payment:          payment,
		scheduler:        scheduler,
		clk:              clk,
	}
}

// writePaymentError раскладывает sentinel-ошибки домена по HTTP-статусам.
func writePaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrPaymentNotFound), errors.Is(err, model.ErrRecurrentPaymentNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, model.ErrPaymentForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, model.ErrPaymentAlreadyPaid):
		http.Error(w, "payment already paid", http.StatusConflict)
	case errors.Is(err, service.ErrInvalidPayment), errors.Is(err, service.ErrInvalidRecurrentPayment):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) actor(r *http.Request) (service.PaymentActor, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		return service.PaymentActor{}, false
	}
	return service.PaymentActor{UserID: claims.UserID, IsAdmin: claims.Role == "admin"}, true
}

func paymentIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid payment id", http.StatusBadRequest)
		return "", false
	}

	return id, true
}

// GetRecurringPayments godoc
// @Summary      Получить список регулярных платежей
// @Description  Возвращает все шаблоны регулярных платежей команды (включая остановленные).
// @Tags         Recurring Payments
// @Security     BearerAuth
// @Produce      json
// @Success      200 {array} model.RecurrentPayment
// @Failure      500 {string} string "Не удалось получить регулярные платежи"
// @Router       /recurring-payments [get]
func (h *PaymentHandler) GetRecurringPayments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payments, err := h.recurrentPayment.GetAllRecurrentPayments(r.Context())
		if err != nil {
			http.Error(w, "failed to get recurring payments", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payments)
	}
}

// CreateRecurringPayment godoc
// @Summary      Создать регулярный платеж
// @Description  Создает шаблон регулярного платежа и назначает ответственного (только администратор).
// @Tags         Recurring Payments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body request.CreateRecurrentPaymentRequest true "Данные регулярного платежа"
// @Success      201 {object} map[string]string "Идентификатор созданного платежа"
// @Failure      400 {string} string "Некорректное тело запроса"
// @Failure      500 {string} string "Не удалось создать регулярный платеж"
// @Router       /recurring-payments [post]
func (h *PaymentHandler) CreateRecurringPayment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request.CreateRecurrentPaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		id, err := h.recurrentPayment.CreateRecurrentPayment(req, r.Context())
		if err != nil {
			writePaymentError(w, err)
			return
		}

		// Генерация первых периодов — best effort: ошибка не отменяет создание.
		if err := h.scheduler.EnsureFor(r.Context(), id); err != nil {
			// Залогировано в планировщике; ответ всё равно 201.
			_ = err
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

// StopRecurringPayment godoc
// @Summary      Остановить регулярный платеж
// @Description  Мягко останавливает шаблон: новые периоды не создаются, напоминания прекращаются, история сохраняется.
// @Tags         Recurring Payments
// @Security     BearerAuth
// @Param        id path string true "UUID регулярного платежа"
// @Success      204 "Регулярный платеж остановлен"
// @Failure      400 {string} string "Некорректный UUID"
// @Failure      404 {string} string "Регулярный платеж не найден"
// @Failure      500 {string} string "Не удалось остановить регулярный платеж"
// @Router       /recurring-payments/{id} [delete]
func (h *PaymentHandler) StopRecurringPayment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		if err := h.recurrentPayment.Stop(id, r.Context()); err != nil {
			writePaymentError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// UpdateRecurringPayment godoc
// @Summary      Обновить регулярный платеж
// @Description  Частично обновляет сумму, валюту, описание и ответственного. Поля графика неизменяемы.
// @Tags         Recurring Payments
// @Security     BearerAuth
// @Accept       json
// @Param        id path string true "UUID регулярного платежа"
// @Param        request body request.UpdateRecurrentPaymentRequest true "Новые параметры регулярного платежа"
// @Success      204 "Регулярный платеж обновлен"
// @Failure      400 {string} string "Некорректный UUID или тело запроса"
// @Failure      404 {string} string "Регулярный платеж не найден"
// @Failure      500 {string} string "Не удалось обновить регулярный платеж"
// @Router       /recurring-payments/{id} [patch]
func (h *PaymentHandler) UpdateRecurringPayment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		var req request.UpdateRecurrentPaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.recurrentPayment.UpdateRecurrentPayment(id, req, r.Context()); err != nil {
			writePaymentError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GetPaymentOccurrences godoc
// @Summary      Получить список платежей
// @Description  Возвращает периоды платежей всей команды. Фильтр ?assignee=me — только свои.
// @Tags         Payments
// @Security     BearerAuth
// @Produce      json
// @Param        assignee query string false "me — только периоды текущего пользователя"
// @Success      200 {object} response.PaymentListResponse
// @Failure      403 {string} string "Не удалось получить данные авторизации"
// @Failure      500 {string} string "Не удалось получить платежи"
// @Router       /payment-occurrences [get]
func (h *PaymentHandler) GetPaymentOccurrences() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := h.actor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}

		var (
			payments []model.Payment
			err      error
		)
		if r.URL.Query().Get("assignee") == "me" {
			payments, err = h.payment.GetPaymentsByUserID(actor.UserID, r.Context())
		} else {
			payments, err = h.payment.GetAllPayments(r.Context())
		}
		if err != nil {
			http.Error(w, "failed to get payments", http.StatusInternalServerError)
			return
		}

		today := h.clk.Today()
		items := make([]response.PaymentResponse, 0, len(payments))
		for _, p := range payments {
			status, paidLate := service.EffectiveStatus(p, today)
			items = append(items, response.NewPaymentResponse(p, status, paidLate))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response.PaymentListResponse{Items: items, Total: len(items)})
	}
}

type PaymentCurrencySummary struct {
	Currency         string `json:"currency"`
	PaidTotal        int64  `json:"paid_total"`
	OutstandingTotal int64  `json:"outstanding_total"`
	OverdueTotal     int64  `json:"overdue_total"`
}

type PaymentSummaryResponse struct {
	Items []PaymentCurrencySummary `json:"items"`
}

// GetPaymentsSummary godoc
// @Summary      Получить сводку платежей
// @Description  Возвращает суммы оплаченных, неоплаченных и просроченных платежей команды по валютам (минимальные единицы).
// @Tags         Payments
// @Security     BearerAuth
// @Produce      json
// @Param        from query string false "Нижняя граница due_date (YYYY-MM-DD)"
// @Param        to query string false "Верхняя граница due_date (YYYY-MM-DD)"
// @Success      200 {object} PaymentSummaryResponse
// @Failure      400 {string} string "Некорректный формат from/to"
// @Failure      500 {string} string "Не удалось получить сводку платежей"
// @Router       /payments/summary [get]
func (h *PaymentHandler) GetPaymentsSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payments, err := h.payment.GetAllPayments(r.Context())
		if err != nil {
			http.Error(w, "failed to get payment summary", http.StatusInternalServerError)
			return
		}

		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		// Валидируем формат границ, иначе неверная дата молча даёт неверную сводку
		// (сравнение строк YYYY-MM-DD корректно только при валидном формате).
		if from != "" {
			if _, err := clock.ParseDate(from); err != nil {
				http.Error(w, "from must be YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		}
		if to != "" {
			if _, err := clock.ParseDate(to); err != nil {
				http.Error(w, "to must be YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		}
		today := h.clk.Today()

		summary := make(map[string]*PaymentCurrencySummary)
		for _, p := range payments {
			if from != "" && p.DueDate < from {
				continue
			}
			if to != "" && p.DueDate > to {
				continue
			}

			if summary[p.Currency] == nil {
				summary[p.Currency] = &PaymentCurrencySummary{Currency: p.Currency}
			}
			s := summary[p.Currency]

			status, _ := service.EffectiveStatus(p, today)
			switch status {
			case "paid":
				s.PaidTotal += p.Amount
			case "overdue":
				s.OutstandingTotal += p.Amount
				s.OverdueTotal += p.Amount
			case model.ShownStatusEnded:
				// Неоплаченный хвост остановленного/удалённого шаблона: не долг и
				// не просрочка (см. EffectiveStatus и NL is_ended). Не суммируем.
			default: // pending
				s.OutstandingTotal += p.Amount
			}
		}

		items := make([]PaymentCurrencySummary, 0, len(summary))
		for _, item := range summary {
			items = append(items, *item)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Currency < items[j].Currency })

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaymentSummaryResponse{Items: items})
	}
}

// GetPaymentOccurrence godoc
// @Summary      Получить платеж по ID
// @Description  Возвращает период платежа команды.
// @Tags         Payments
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "UUID платежа"
// @Success      200 {object} response.PaymentResponse
// @Failure      400 {string} string "Некорректный UUID"
// @Failure      404 {string} string "Платеж не найден"
// @Failure      500 {string} string "Не удалось получить платеж"
// @Router       /payment-occurrences/{id} [get]
func (h *PaymentHandler) GetPaymentOccurrence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		payment, err := h.payment.GetPaymentByID(id, r.Context())
		if err != nil {
			writePaymentError(w, err)
			return
		}

		status, paidLate := service.EffectiveStatus(payment, h.clk.Today())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response.NewPaymentResponse(payment, status, paidLate))
	}
}

type PatchPaymentOccurrenceRequest struct {
	Action      string  `json:"action"`
	PaidDate    *string `json:"paid_date"`
	Amount      *int64  `json:"amount"`
	Currency    *string `json:"currency"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
}

// PatchPaymentOccurrence godoc
// @Summary      Изменить платеж
// @Description  action=confirm — отметить оплату (paid_date опционально, по умолчанию сегодня; повторно — меняет дату).
// @Description  action=cancel — снять оплату и очистить дату.
// @Description  Без action — правка суммы/валюты/описания (только администратор); due_date менять нельзя.
// @Tags         Payments
// @Security     BearerAuth
// @Accept       json
// @Param        id path string true "UUID платежа"
// @Param        request body PatchPaymentOccurrenceRequest true "Действие или поля для обновления"
// @Success      204 "Платеж обновлен"
// @Failure      400 {string} string "Некорректный запрос"
// @Failure      403 {string} string "Недостаточно прав"
// @Failure      404 {string} string "Платеж не найден"
// @Failure      409 {string} string "Платеж уже оплачен"
// @Failure      500 {string} string "Не удалось изменить платеж"
// @Router       /payment-occurrences/{id} [patch]
func (h *PaymentHandler) PatchPaymentOccurrence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := h.actor(r)
		if !ok {
			http.Error(w, "failed get claims", http.StatusForbidden)
			return
		}

		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		var req PatchPaymentOccurrenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "confirm":
			h.handleConfirm(w, r, id, req, actor)
		case "cancel":
			if err := h.payment.MarkOccurrenceUnpaid(r.Context(), id, actor); err != nil {
				writePaymentError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "":
			h.handleEdit(w, r, id, req, actor)
		default:
			http.Error(w, "action must be confirm or cancel", http.StatusBadRequest)
		}
	}
}

// handleConfirm обрабатывает подтверждение оплаты и исправление даты.
func (h *PaymentHandler) handleConfirm(
	w http.ResponseWriter,
	r *http.Request,
	id string,
	req PatchPaymentOccurrenceRequest,
	actor service.PaymentActor,
) {
	paidDate := h.clk.Today()
	if req.PaidDate != nil {
		d, err := clock.ParseDate(*req.PaidDate)
		if err != nil {
			http.Error(w, "paid_date must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		paidDate = d
	}

	err := h.payment.MarkOccurrencePaid(r.Context(), id, paidDate, actor)
	if errors.Is(err, model.ErrPaymentAlreadyPaid) {
		// Платёж уже оплачен: с датой — исправляем её, без даты — 409.
		if req.PaidDate == nil {
			http.Error(w, "payment already paid", http.StatusConflict)
			return
		}
		if err := h.payment.ChangeOccurrencePaidDate(r.Context(), id, paidDate, actor); err != nil {
			writePaymentError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writePaymentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleEdit правит сумму/валюту/описание неоплаченного периода. Только админ;
// due_date менять нельзя.
func (h *PaymentHandler) handleEdit(
	w http.ResponseWriter,
	r *http.Request,
	id string,
	req PatchPaymentOccurrenceRequest,
	actor service.PaymentActor,
) {
	if req.PaidDate != nil {
		http.Error(w, "action confirm is required to set paid_date", http.StatusBadRequest)
		return
	}
	if req.DueDate != nil {
		http.Error(w, "due_date cannot be changed", http.StatusBadRequest)
		return
	}
	if !actor.IsAdmin {
		http.Error(w, "only admin can edit a payment", http.StatusForbidden)
		return
	}
	if req.Amount == nil && req.Currency == nil && req.Description == nil {
		http.Error(w, "at least one field must be provided", http.StatusBadRequest)
		return
	}

	updateReq := request.UpdatePaymentRequest{
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
	}
	if err := h.payment.UpdatePayment(id, updateReq, r.Context()); err != nil {
		writePaymentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
