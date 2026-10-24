package response

import (
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

type PaymentResponse struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	RecurrentPaymentID *string    `json:"recurrent_payment_id,omitempty"`
	Amount             int64      `json:"amount"`
	Currency           string     `json:"currency"`
	Description        string     `json:"description"`
	DueDate            time.Time  `json:"due_date"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type PaymentListResponse struct {
	Items []PaymentResponse `json:"items"`
	Total int               `json:"total"`
}

func NewPaymentResponse(payment model.Payment) PaymentResponse {
	return PaymentResponse{
		ID:                 payment.ID,
		UserID:             payment.UserID,
		RecurrentPaymentID: payment.RecurrentPaymentID,
		Amount:             payment.Amount,
		Currency:           payment.Currency,
		Description:        payment.Description,
		DueDate:            payment.DueDate,
		PaidAt:             payment.PaidAt,
		Status:             payment.Status,
		CreatedAt:          payment.CreatedAt,
		UpdatedAt:          payment.UpdatedAt,
	}
}

func NewPaymentListResponse(payments []model.Payment) PaymentListResponse {
	items := make([]PaymentResponse, 0, len(payments))

	for _, payment := range payments {
		items = append(items, NewPaymentResponse(payment))
	}

	return PaymentListResponse{
		Items: items,
		Total: len(items),
	}
}
