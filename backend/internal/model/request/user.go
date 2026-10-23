package request

type CreateUserRequest struct {
	// Полное имя пользователя, отображается в профиле.
	FullName string `json:"full_name" validate:"required,min=3,max=64" example:"Иван Петров"`
	// Уникальный логин, используется для входа.
	Login string `json:"login" validate:"required,min=3,max=64" example:"ivan.petrov"`
	// Пароль, минимум 8 символов. Роль не передаётся: создаётся всегда обычный пользователь.
	Password string `json:"password" validate:"required,min=8" example:"Str0ngPass!2026"`
	// Персона (должность) из `GET /personas`. Необязательно: без поля пользователь
	// создаётся без персоны. Несуществующий id → `400 persona not found`.
	PersonaID *string `json:"persona_id,omitempty" format:"uuid" example:"223f4e8e-98ea-4e50-9fb7-a9ea8984b2f1"`
}

type CreatePersonaRequest struct {
	// Название, 1–64 символа (пробелы по краям обрезаются), уникально без учёта регистра.
	Name string `json:"name" validate:"required" minLength:"1" maxLength:"64" example:"Бухгалтер"`
	// Описание, до 255 символов. Необязательно; пустая строка сохраняется как `null`.
	Description *string `json:"description,omitempty" maxLength:"255" example:"Оплачивает аренду и подписки"`
}

type UpdatePersonaRequest struct {
	// Новое название, 1–64 символа. Если поле не передано, текущее значение не меняется.
	Name *string `json:"name,omitempty" minLength:"1" maxLength:"64" example:"Главный бухгалтер"`
	// Новое описание, до 255 символов. Если поле не передано, не меняется;
	// пустая строка `""` удаляет описание.
	Description *string `json:"description,omitempty" maxLength:"255" example:"Оплачивает аренду, подписки и налоги"`
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
