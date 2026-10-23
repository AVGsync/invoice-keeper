package request

type CreateUserRequest struct {
	// Полное имя пользователя, отображается в профиле.
	FullName string `json:"full_name" validate:"required,min=3,max=64" example:"Иван Петров"`
	// Уникальный логин, используется для входа.
	Login string `json:"login" validate:"required,min=3,max=64" example:"ivan.petrov"`
	// Пароль, минимум 8 символов. Роль не передаётся: создаётся всегда обычный пользователь.
	Password string `json:"password" validate:"required,min=8" example:"Str0ngPass!2026"`
}

type LoginRequest struct {
	// Логин зарегистрированного пользователя.
	Login string `json:"login" validate:"required,min=3,max=64" example:"ivan.petrov"`
	// Пароль пользователя.
	Password string `json:"password" validate:"required" example:"Str0ngPass!2026"`
}

type UserUpdateRequest struct {
	// Новый логин. Если поле не передано, текущее значение не меняется.
	Login *string `json:"login,omitempty" validate:"omitempty,min=3,max=64" example:"ivan.new"`
	// Новое полное имя. Если поле не передано, текущее значение не меняется.
	FullName *string `json:"full_name,omitempty" validate:"omitempty,min=3,max=64" example:"Иван Сергеевич Петров"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required" example:"Str0ngPass!2026"`
	NewPassword string `json:"new_password" validate:"required,min=8" example:"N3wStr0ngPass!2026"`
}
