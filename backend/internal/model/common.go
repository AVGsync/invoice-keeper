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

type User struct {
	ID           string    `json:"id"`
	FullName     string    `json:"full_name"`
	Login        string    `json:"login"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
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
