package response

import "time"

type RecurrentPaymentResponse struct {
	ID          string    `json:"id"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Frequency   string    `json:"frequency"`
	Interval    int       `json:"interval"`
	DayOffset   int       `json:"day_offset"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RecurrentPaymentListResponse struct {
	Payments []RecurrentPaymentResponse `json:"payments"`
	Total    int                        `json:"total"`
}

type DeleteRecurrentPaymentResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type SubscribeUsersToRecurrentPaymentResponse struct {
	RecurrentPaymentID string   `json:"recurrent_payment_id"`
	UserIDs            []string `json:"user_ids"`
	Message            string   `json:"message"`
}

type UnsubscribeUsersFromRecurrentPaymentResponse struct {
	RecurrentPaymentID string   `json:"recurrent_payment_id"`
	UserIDs            []string `json:"user_ids"`
	Message            string   `json:"message"`
}

type RecurrentPaymentUsersResponse struct {
	RecurrentPaymentID string   `json:"recurrent_payment_id"`
	UserIDs            []string `json:"user_ids"`
	Total              int      `json:"total"`
}
