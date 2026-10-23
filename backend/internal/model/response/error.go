package response

// ErrorResponse is the uniform JSON error contract for the API.
type ErrorResponse struct {
	// Error — единый envelope JSON-ошибок.
	Error APIError `json:"error"`
}

type APIError struct {
	// Code — стабильный машинный код; клиентская логика ветвится по нему.
	Code string `json:"code" example:"payment_not_found"`
	// Message предназначен для диагностики; UI может локализовать сообщение по Code.
	Message string `json:"message" example:"payment not found"`
	// RequestID совпадает с response header X-Request-ID и нужен для диагностики.
	RequestID string `json:"request_id,omitempty" example:"f16cc96e-aaf4-44fb-a462-836ed4d0b135"`
	// Details содержит предметные данные, например currency или period при ошибке валидации платежа.
	Details map[string]any `json:"details,omitempty" swaggertype:"object"`
}
