package request

type CreateUserRequest struct {
	// Полное имя пользователя, отображается в профиле.
	FullName string `json:"full_name" validate:"required,min=3,max=64" example:"Иван Петров"`
	// Уникальный логин, используется для входа.
	Login string `json:"login" validate:"required,min=3,max=64" example:"ivan.petrov"`
	// Пароль, минимум 8 символов. Роль не передаётся: создаётся всегда обычный пользователь.
	Password string `json:"password" validate:"required,min=8" example:"Str0ngPass!2026"`
	// Персона из GET /personas. Необязательно: без неё пользователь создаётся без персоны.
	PersonaID *string `json:"persona_id,omitempty" format:"uuid" example:"7c9e6679-7425-40de-944b-e07fc1f90ae7"`
}

type CreatePersonaRequest struct {
	// Название, 1–64 символа, уникально без учёта регистра.
	Name string `json:"name" example:"Бухгалтер"`
	// Описание, до 255 символов.
	Description *string `json:"description,omitempty" example:"Оплачивает аренду и подписки"`
}

type UpdatePersonaRequest struct {
	// Новое название. Если поле не передано, текущее значение не меняется.
	Name *string `json:"name,omitempty" example:"Главный бухгалтер"`
	// Новое описание. Если поле не передано, текущее значение не меняется.
	Description *string `json:"description,omitempty" example:"Оплачивает аренду, подписки и налоги"`
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
