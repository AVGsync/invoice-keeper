package response

type UserResponse struct {
	ID       string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Login    string `json:"login" example:"ivan.petrov"`
	Fullname string `json:"full_name" example:"Иван Петров"`
	Role     string `json:"role" example:"user" enums:"user,admin"`
}
