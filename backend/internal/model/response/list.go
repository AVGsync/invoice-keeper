package response

import "github.com/AVGsync/invoice-keeper/backend/internal/model"

type UserListResponse struct {
	Items []model.User `json:"items"`
	Total int          `json:"total"`
}

type PersonaListResponse struct {
	Items []model.Persona `json:"items"`
	Total int             `json:"total"`
}
