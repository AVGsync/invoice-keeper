package model

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrLoginTaken возвращается репозиторием при нарушении уникальности login.
var ErrLoginTaken = errors.New("login already taken")

// ErrUserNotFound возвращается, когда пользователь с указанным id не найден.
var ErrUserNotFound = errors.New("user not found")

// ErrUserInUse запрещает удалять пользователя, на которого ссылаются другие записи.
var ErrUserInUse = errors.New("user is referenced and cannot be deleted")

// ErrAdminImmutable запрещает менять и удалять администратора через API:
// его учётка задаётся только в .env.
var ErrAdminImmutable = errors.New("admin account is managed via .env")

// ErrPersonaNotFound — персоны с указанным id нет (в том числе при создании пользователя).
var ErrPersonaNotFound = errors.New("persona not found")

// ErrPersonaNameTaken — персона с таким названием уже есть (без учёта регистра).
var ErrPersonaNameTaken = errors.New("persona with this name already exists")

type User struct {
	// Идентификатор пользователя.
	ID string `json:"id" format:"uuid" example:"9a191dfa-5e71-47aa-bcb2-ed5e9c071830"`
	// Полное имя.
	FullName string `json:"full_name" example:"Ольга Иванова"`
	// Логин для входа.
	Login        string `json:"login" example:"olga"`
	PasswordHash string `json:"-"`
	// Роль: `admin` — администратор из .env, `user` — обычный пользователь.
	Role string `json:"role" enums:"admin,user" example:"user"`
	// Персона (должность) пользователя; `null`, если не назначена или была удалена.
	Persona *PersonaRef `json:"persona" extensions:"x-nullable"`
	// Когда создан.
	CreatedAt time.Time `json:"created_at" example:"2026-10-23T20:05:31Z"`
}

// Persona — должность или зона ответственности пользователя (Бухгалтер, DevOps).
// На права не влияет: права задаёт только Role.
type Persona struct {
	// Идентификатор персоны.
	ID string `json:"id" format:"uuid" example:"223f4e8e-98ea-4e50-9fb7-a9ea8984b2f1"`
	// Название, 1–64 символа, уникально без учёта регистра.
	Name string `json:"name" example:"Бухгалтер"`
	// Описание, до 255 символов; `null`, если не задано.
	Description *string `json:"description" extensions:"x-nullable" example:"Оплачивает аренду и подписки"`
	// Сколько пользователей с этой персоной.
	UsersCount int `json:"users_count" example:"1"`
	// Когда создана.
	CreatedAt time.Time `json:"created_at" example:"2026-10-23T20:05:31Z"`
}

// PersonaRef — персона в составе пользователя; null, если не назначена.
type PersonaRef struct {
	// Идентификатор персоны.
	ID string `json:"id" format:"uuid" example:"223f4e8e-98ea-4e50-9fb7-a9ea8984b2f1"`
	// Название персоны.
	Name string `json:"name" example:"Бухгалтер"`
}

type RefreshToken struct {
	ID        string     `json:"id"`
	TokenHash string     `json:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Type   string `json:"type"`

	jwt.RegisteredClaims
}
