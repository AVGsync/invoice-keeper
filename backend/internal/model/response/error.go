package response

// ErrorResponse is the uniform JSON error contract for cart/order/chat APIs.
type ErrorResponse struct {
	// Error — единый envelope новых cart/order/chat методов.
	Error APIError `json:"error"`
}

type APIError struct {
	// Code — стабильный машинный код; клиентская логика ветвится по нему.
	Code string `json:"code" example:"order_version_conflict"`
	// Message предназначен для диагностики; UI может локализовать сообщение по Code.
	Message string `json:"message" example:"order version conflict"`
	// RequestID совпадает с response header X-Request-ID и нужен для диагностики.
	RequestID string `json:"request_id,omitempty" example:"f16cc96e-aaf4-44fb-a462-836ed4d0b135"`
	// Details содержит предметные данные, например items при insufficient_stock.
	Details map[string]any `json:"details,omitempty" swaggertype:"object"`
}
