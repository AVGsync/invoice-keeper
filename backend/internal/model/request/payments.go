package request

import "time"

type CreatePaymentRequest struct {
	UserID             string    `json:"user_id"`
	RecurrentPaymentID *string   `json:"recurrent_payment_id,omitempty"`
	Amount             int64     `json:"amount"`
	Currency           string    `json:"currency"`
	Description        string    `json:"description"`
	DueDate            time.Time `json:"due_date"`
}

type GetPaymentRequest struct {
	ID string `json:"id"`
}

type GetPaymentsByUserIDRequest struct {
	UserID string `json:"user_id"`
}

type UpdatePaymentRequest struct {
	Amount      *int64     `json:"amount,omitempty"`
	Currency    *string    `json:"currency,omitempty"`
	Description *string    `json:"description,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

type MarkPaymentAsPaidRequest struct {
	ID string `json:"id"`
}

type MarkPaymentAsOverdueRequest struct {
	ID string `json:"id"`
}

type DeletePaymentRequest struct {
	ID string `json:"id"`
}
