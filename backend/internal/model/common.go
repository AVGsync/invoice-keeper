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

// ErrAdminImmutable запрещает менять и удалять администратора через API: его учётка задаётся только в .env.
var ErrAdminImmutable = errors.New("admin account is managed via .env")

// ErrPersonaNotFound — персоны с указанным id нет (в том числе при создании пользователя).
var ErrPersonaNotFound = errors.New("persona not found")

// ErrPersonaNameTaken — персона с таким названием уже есть (без учёта регистра).
var ErrPersonaNameTaken = errors.New("persona with this name already exists")

// Единые sentinel-ошибки модуля платежей.
var (
	// ErrPaymentNotFound — периода (payments) с таким id нет.
	ErrPaymentNotFound = errors.New("payment not found")
	// ErrRecurrentPaymentNotFound — шаблона (recurrent_payments) с таким id нет.
	ErrRecurrentPaymentNotFound = errors.New("recurrent payment not found")
	// ErrPaymentForbidden — пользователь не ответственный и не админ.
	ErrPaymentForbidden = errors.New("payment action forbidden")
	// ErrPaymentAlreadyPaid — период уже оплачен (повторное подтверждение).
	ErrPaymentAlreadyPaid = errors.New("payment already paid")
	// ErrPaymentEnded — платёж остановлен/удалён, отметить нельзя (использует бот).
	ErrPaymentEnded = errors.New("payment ended")
	// ErrNotificationSettingsExists — настройки уведомлений уже созданы (uq email).
	ErrNotificationSettingsExists = errors.New("notification settings already exist")
)

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
	// Привязан ли Telegram-аккаунт (таблица telegram_links).
	TelegramLinked bool `json:"telegram_linked" example:"true"`
	// Username в Telegram без @; пусто, если не привязан или username не задан.
	TelegramUsername string `json:"telegram_username,omitempty" example:"olga_iv"`
	// Когда привязан Telegram; не задано, если не привязан.
	TelegramLinkedAt *time.Time `json:"telegram_linked_at,omitempty" example:"2026-10-23T20:05:31Z"`
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

type RecurrentPayment struct {
	ID          string `json:"id" format:"uuid"`
	Amount      int64  `json:"amount"` // Сумма в минимальных денежных единицах
	Currency    string `json:"currency"`
	Frequency   string `json:"frequency"` // weekly, monthly
	Interval    int    `json:"interval"`  // Каждые N недель или месяцев
	Description string `json:"description"`

	// Для monthly: день месяца (1–31)
	// Для weekly: день недели (0–6)
	DayOffset int `json:"day_offset"`

	// StartDate — ближайшая дата оплаты (якорь графика), "YYYY-MM-DD".
	StartDate string `json:"start_date"`
	// StoppedAt — момент мягкой остановки; nil у активного шаблона.
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	// AssigneeID — ответственный (user_recurrent_payments, ровно один); nil, если не назначен.
	AssigneeID *string `json:"assignee_id,omitempty"`
	// AssigneeName — полное имя ответственного (для списков и бота).
	AssigneeName string `json:"assignee_name,omitempty"`

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

	// DueDate и PaidDate — календарные даты "YYYY-MM-DD" без зоны.
	DueDate  string     `json:"due_date"`
	PaidDate *string    `json:"paid_date,omitempty"`
	PaidAt   *time.Time `json:"paid_at,omitempty"`
	Status   string     `json:"status"`

	// AssigneeName — полное имя ответственного (payments.user_id → users.full_name).
	AssigneeName string `json:"assignee_name,omitempty"`
	// Ended — платёж остановлен/удалён (recurrent_payment_id IS NULL OR stopped_at IS NOT NULL).
	Ended bool `json:"ended"`
	// ReceiptsCount — сколько чеков прикреплено к периоду.
	ReceiptsCount int `json:"receipts_count"`

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
