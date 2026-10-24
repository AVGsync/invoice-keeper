package request

type CreateRecurrentPaymentRequest struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Frequency   string `json:"frequency"`
	Interval    int    `json:"interval"`
	DayOffset   int    `json:"day_offset"`
}

type UpdateRecurrentPaymentRequest struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Frequency   string `json:"frequency"`
	Interval    int    `json:"interval"`
	DayOffset   int    `json:"day_offset"`
}

type GetRecurrentPaymentRequest struct {
	ID string `json:"id"`
}

type DeleteRecurrentPaymentRequest struct {
	ID string `json:"id"`
}

type SubscribeUsersToRecurrentPaymentRequest struct {
	RecurrentPaymentID string   `json:"recurrent_payment_id"`
	UserIDs            []string `json:"user_ids"`
}

type UnsubscribeUsersFromRecurrentPaymentRequest struct {
	RecurrentPaymentID string   `json:"recurrent_payment_id"`
	UserIDs            []string `json:"user_ids"`
}
