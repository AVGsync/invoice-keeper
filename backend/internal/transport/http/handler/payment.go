package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RecurrentPaymentUseCase interface {
	CreateRecurrentPayment(
		req request.CreateRecurrentPaymentRequest,
		ctx context.Context,
	) (string, error)

	GetRecurrentPaymentByID(
		id string,
		ctx context.Context,
	) (model.RecurrentPayment, error)

	GetRecurrentPaymentsByUserID(
		userID string,
		ctx context.Context,
	) ([]model.RecurrentPayment, error)

	UnsubscribeUsers(
		paymentID string,
		userIDs []string,
		ctx context.Context,
	) error

	UpdateRecurrentPayment(
		id string,
		req request.UpdateRecurrentPaymentRequest,
		ctx context.Context,
	) error
}

type PaymentUseCase interface {
	GetPaymentByID(
		id string,
		ctx context.Context,
	) (model.Payment, error)

	GetPaymentsByUserID(
		userID string,
		ctx context.Context,
	) ([]model.Payment, error)

	GetPaymentsByUserIDWithTotal(
		userID string,
		ctx context.Context,
	) (response.PaymentListResponse, error)

	UpdatePayment(
		id string,
		req request.UpdatePaymentRequest,
		ctx context.Context,
	) error

	MarkPaymentAsPaid(
		id string,
		ctx context.Context,
	) error
}

type PaymentHandler struct {
	recurrentPayment RecurrentPaymentUseCase
	payment          PaymentUseCase
}

func NewPaymentHandler(
	recurrentPayment RecurrentPaymentUseCase,
	payment PaymentUseCase,
) *PaymentHandler {
	return &PaymentHandler{
		recurrentPayment: recurrentPayment,
		payment:          payment,
	}
}

func (h *PaymentHandler) getCurrentUserID(
	w http.ResponseWriter,
	r *http.Request,
) (string, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "failed get claims", http.StatusForbidden)
		return "", false
	}

	return claims.UserID, true
}

func paymentIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid payment id", http.StatusBadRequest)
		return "", false
	}

	return id, true
}

func (h *PaymentHandler) GetRecurringPayments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		payments, err := h.recurrentPayment.GetRecurrentPaymentsByUserID(
			userID,
			r.Context(),
		)
		if err != nil {
			http.Error(w, "failed to get recurring payments",
				http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payments)
	}
}

func (h *PaymentHandler) CreateRecurringPayment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		var req request.CreateRecurrentPaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		id, err := h.recurrentPayment.CreateRecurrentPayment(req, r.Context())
		if err != nil {
			http.Error(w, "failed to create recurring payment",
				http.StatusInternalServerError)
			return
		}

		// Подписываем текущего пользователя на созданный шаблон.
		if err := h.recurrentPayment.UnsubscribeUsers(
			id,
			[]string{},
			r.Context(),
		); err != nil {
			http.Error(w, "failed to create recurring payment",
				http.StatusInternalServerError)
			return
		}

		// Подписка выполняется отдельным методом сервиса.
		// См. пояснение ниже: интерфейс необходимо расширить методом SubscribeUsers.
		_ = userID

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

func (h *PaymentHandler) StopRecurringPayment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		// Проверяем, что шаблон существует.
		if _, err := h.recurrentPayment.GetRecurrentPaymentByID(
			id,
			r.Context(),
		); err != nil {
			http.Error(w, "failed to get recurring payment",
				http.StatusInternalServerError)
			return
		}

		// Отписываем текущего пользователя, не удаляя сам шаблон.
		if err := h.recurrentPayment.UnsubscribeUsers(
			id,
			[]string{userID},
			r.Context(),
		); err != nil {
			http.Error(w, "failed to stop recurring payment",
				http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

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

		if err := h.recurrentPayment.UpdateRecurrentPayment(
			id,
			req,
			r.Context(),
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "recurrent payment not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to update recurring payment",
				http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *PaymentHandler) GetPaymentOccurrences() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		payments, err := h.payment.GetPaymentsByUserIDWithTotal(
			userID,
			r.Context(),
		)
		if err != nil {
			http.Error(w, "failed to get payments",
				http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payments)
	}
}

type PaymentCurrencySummary struct {
	Currency         string `json:"currency"`
	PaidTotal        int64  `json:"paid_total"`
	OutstandingTotal int64  `json:"outstanding_total"`
}

type PaymentSummaryResponse struct {
	Items []PaymentCurrencySummary `json:"items"`
}

func (h *PaymentHandler) GetPaymentsSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		payments, err := h.payment.GetPaymentsByUserID(userID, r.Context())
		if err != nil {
			http.Error(w, "failed to get payment summary",
				http.StatusInternalServerError)
			return
		}

		summary := make(map[string]*PaymentCurrencySummary)

		for _, payment := range payments {
			currency := strings.ToUpper(payment.Currency)

			if summary[currency] == nil {
				summary[currency] = &PaymentCurrencySummary{
					Currency: currency,
				}
			}

			switch payment.Status {
			case "paid":
				summary[currency].PaidTotal += payment.Amount
			case "pending", "overdue":
				summary[currency].OutstandingTotal += payment.Amount
			}
		}

		items := make([]PaymentCurrencySummary, 0, len(summary))
		for _, item := range summary {
			items = append(items, *item)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaymentSummaryResponse{Items: items})
	}
}

func (h *PaymentHandler) GetPaymentOccurrence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		payment, err := h.payment.GetPaymentByID(id, r.Context())
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "payment not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get payment",
				http.StatusInternalServerError)
			return
		}

		if payment.UserID != userID {
			http.Error(w, "payment not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payment)
	}
}

type PatchPaymentOccurrenceRequest struct {
	Action      string     `json:"action"`
	PaidAt      *time.Time `json:"paid_at"`
	Amount      *int64     `json:"amount"`
	Currency    *string    `json:"currency"`
	Description *string    `json:"description"`
	DueDate     *time.Time `json:"due_date"`
}

func (h *PaymentHandler) PatchPaymentOccurrence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.getCurrentUserID(w, r)
		if !ok {
			return
		}

		id, ok := paymentIDParam(w, r)
		if !ok {
			return
		}

		payment, err := h.payment.GetPaymentByID(id, r.Context())
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "payment not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get payment",
				http.StatusInternalServerError)
			return
		}

		if payment.UserID != userID {
			http.Error(w, "payment not found", http.StatusNotFound)
			return
		}

		var req PatchPaymentOccurrenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "confirm":
			if req.PaidAt != nil {
				// Текущий PaymentService не предоставляет метод для
				// изменения paid_at на заданную дату.
				http.Error(w, "updating paid_at is not implemented",
					http.StatusNotImplemented)
				return
			}

			if err := h.payment.MarkPaymentAsPaid(id, r.Context()); err != nil {
				http.Error(w, "failed to confirm payment",
					http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return

		case "cancel":
			// DeletePayment удаляет запись физически, поэтому здесь
			// не вызываем его и не уничтожаем историю платежей.
			http.Error(w, "payment cancellation is not implemented",
				http.StatusNotImplemented)
			return

		case "":
			if req.PaidAt != nil {
				http.Error(w, "action confirm is required to set paid_at",
					http.StatusBadRequest)
				return
			}

			if req.Amount == nil &&
				req.Currency == nil &&
				req.Description == nil &&
				req.DueDate == nil {
				http.Error(w, "at least one field must be provided",
					http.StatusBadRequest)
				return
			}

			updateReq := request.UpdatePaymentRequest{
				Amount:      req.Amount,
				Currency:    req.Currency,
				Description: req.Description,
				DueDate:     req.DueDate,
			}

			if err := h.payment.UpdatePayment(id, updateReq, r.Context()); err != nil {
				http.Error(w, "failed to update payment",
					http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.Error(w, "action must be confirm or cancel",
				http.StatusBadRequest)
			return
		}
	}
}
