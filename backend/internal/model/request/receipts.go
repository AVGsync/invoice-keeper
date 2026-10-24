package request

// AddReceiptLinkRequest — тело POST /payment-occurrences/{id}/receipts/link.
type AddReceiptLinkRequest struct {
	// Адрес чека: http или https, до 2048 символов. Без схемы подставляется https.
	URL string `json:"url" example:"https://check.example.com/r/123"`
	// Подпись, до 120 символов. Необязательно.
	Title string `json:"title" example:"Чек за октябрь"`
}
