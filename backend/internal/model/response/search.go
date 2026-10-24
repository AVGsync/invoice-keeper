package response

import "github.com/AVGsync/invoice-keeper/backend/internal/model"

// PaymentSearchResponse — конверт серверного поиска периодов. Элемент — тот же
// PaymentResponse, что и в GET /payment-occurrences. Items всегда массив.
type PaymentSearchResponse struct {
	Items  []PaymentResponse `json:"items"`
	Total  int               `json:"total"` // всего по фильтру, без учёта limit/offset
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// RecurringPaymentItem — шаблон для поиска: все поля GET /recurring-payments плюс
// ближайший неоплаченный срок и эффективный статус.
type RecurringPaymentItem struct {
	model.RecurrentPayment
	// NextDueDate — ближайший неоплаченный срок ≥ сегодня; null, если его нет.
	NextDueDate *string `json:"next_due_date" extensions:"x-nullable"`
	// Status — active (stopped_at IS NULL) или stopped.
	Status string `json:"status" enums:"active,stopped"`
}

// RecurringPaymentSearchResponse — конверт серверного поиска шаблонов.
type RecurringPaymentSearchResponse struct {
	Items  []RecurringPaymentItem `json:"items"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// NewRecurringPaymentItem собирает элемент ответа, вычисляя status из stopped_at.
func NewRecurringPaymentItem(row model.RecurringSearchRow) RecurringPaymentItem {
	status := "active"
	if row.StoppedAt != nil {
		status = "stopped"
	}
	return RecurringPaymentItem{
		RecurrentPayment: row.RecurrentPayment,
		NextDueDate:      row.NextDueDate,
		Status:           status,
	}
}
