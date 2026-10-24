package request

// CreateRecurrentPaymentRequest — создание шаблона регулярного платежа.
// day_offset необязателен и вычисляется из start_date; если передан и отличается — 400.
type CreateRecurrentPaymentRequest struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Frequency   string `json:"frequency"`
	Interval    int    `json:"interval"`
	DayOffset   *int   `json:"day_offset,omitempty"`
	StartDate   string `json:"start_date"`
	AssigneeID  string `json:"assignee_id"`
}

// UpdateRecurrentPaymentRequest — частичное обновление. Поля графика (frequency,
// interval, day_offset, start_date) неизменяемы: присутствие с другим значением → 400.
type UpdateRecurrentPaymentRequest struct {
	Amount      *int64  `json:"amount,omitempty"`
	Currency    *string `json:"currency,omitempty"`
	Description *string `json:"description,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`

	Frequency *string `json:"frequency,omitempty"`
	Interval  *int    `json:"interval,omitempty"`
	DayOffset *int    `json:"day_offset,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
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
