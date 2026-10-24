package response

import (
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// PaymentResponse — период платежа для API. due_date/paid_date — строки
// "YYYY-MM-DD"; status и paid_late вычисляются сервисом (D6).
type PaymentResponse struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	RecurrentPaymentID *string    `json:"recurrent_payment_id,omitempty"`
	Amount             int64      `json:"amount"`
	Currency           string     `json:"currency"`
	Description        string     `json:"description"`
	DueDate            string     `json:"due_date"`
	PaidDate           *string    `json:"paid_date,omitempty"`
	PaidLate           bool       `json:"paid_late"`
	Status             string     `json:"status"`
	AssigneeName       string     `json:"assignee_name,omitempty"`
	Ended              bool       `json:"ended"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type PaymentListResponse struct {
	Items []PaymentResponse `json:"items"`
	Total int               `json:"total"`
}

// NewPaymentResponse собирает ответ с вычисленными status и paid_late.
func NewPaymentResponse(payment model.Payment, status string, paidLate bool) PaymentResponse {
	return PaymentResponse{
		ID:                 payment.ID,
		UserID:             payment.UserID,
		RecurrentPaymentID: payment.RecurrentPaymentID,
		Amount:             payment.Amount,
		Currency:           payment.Currency,
		Description:        payment.Description,
		DueDate:            payment.DueDate,
		PaidDate:           payment.PaidDate,
		PaidLate:           paidLate,
		Status:             status,
		AssigneeName:       payment.AssigneeName,
		Ended:              payment.Ended,
		PaidAt:             payment.PaidAt,
		CreatedAt:          payment.CreatedAt,
		UpdatedAt:          payment.UpdatedAt,
	}
}
