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
	ID           string      `json:"id"`
	FullName     string      `json:"full_name"`
	Login        string      `json:"login"`
	PasswordHash string      `json:"-"`
	Role         string      `json:"role"`
	Persona      *PersonaRef `json:"persona"`
	CreatedAt    time.Time   `json:"created_at"`
}

// Persona — должность или зона ответственности пользователя (Бухгалтер, DevOps).
// На права не влияет: права задаёт только Role.
type Persona struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	UsersCount  int       `json:"users_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// PersonaRef — персона в составе пользователя; null, если не назначена.
type PersonaRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
