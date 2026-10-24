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

// ErrNotificationSettingsNotFound возвращается, когда настройки уведомлений пользователя не найдены.
var ErrNotificationSettingsNotFound = errors.New("user notification settings not found")

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

type RecurrentPayment struct {
	ID          int64  `json:"id"`
	Amount      int64  `json:"amount"` // Сумма в минимальных денежных единицах
	Currency    string `json:"currency"`
	Frequency   string `json:"frequency"` // weekly, monthly
	Interval    int    `json:"interval"`  // Каждые N недель или месяцев
	Description string `json:"description"`

	// Для monthly: день месяца (1–31)
	// Для weekly: день недели (0–6)
	DayOffset int `json:"day_offset"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Payment struct {
	ID                 string  `json:"id"`
	UserID             string  `json:"user_id"`
	RecurrentPaymentID *string `json:"recurrent_payment_id,omitempty"`

	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`

	DueDate time.Time  `json:"due_date"`
	PaidAt  *time.Time `json:"paid_at,omitempty"`
	Status  string     `json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserNotificationSettings struct {
	UserID        string    `json:"user_id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	EmailEnabled  bool      `json:"email_enabled"`
	DaysBeforeDue int       `json:"days_before_due"`
	SendTime      string    `json:"send_time"`
	Timezone      string    `json:"timezone"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
