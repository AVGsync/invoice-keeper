package request

// UpdatePaymentRequest — правка неоплаченного периода (сумма/валюта/описание).
// due_date неизменяема и здесь отсутствует (D10).
type UpdatePaymentRequest struct {
	Amount      *int64  `json:"amount,omitempty"`
	Currency    *string `json:"currency,omitempty"`
	Description *string `json:"description,omitempty"`
}
