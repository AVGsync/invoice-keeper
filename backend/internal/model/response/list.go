package response

import "github.com/AVGsync/invoice-keeper/backend/internal/model"

type UserListResponse struct {
	Items []model.User `json:"items"`
	Total int          `json:"total"`
}

type PersonaListResponse struct {
	// Персоны, отсортированные по названию (без учёта регистра).
	Items []model.Persona `json:"items"`
	// Сколько всего персон.
	Total int `json:"total" example:"1"`
}
