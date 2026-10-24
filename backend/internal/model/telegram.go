package model

import (
	"errors"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
)

// Доменные ошибки Telegram-бота.
var (
	// ErrTelegramNotLinked — у пользователя нет привязки Telegram.
	ErrTelegramNotLinked = errors.New("telegram account is not linked")
	// ErrTelegramAlreadyLinked — этот Telegram уже привязан к другому аккаунту.
	ErrTelegramAlreadyLinked = errors.New("telegram account is already linked to another user")
	// ErrTelegramTooManyCodes — у пользователя уже максимум активных кодов привязки.
	ErrTelegramTooManyCodes = errors.New("too many active link codes")
	// ErrTelegramCodeInvalid — код привязки не найден, истёк или уже использован.
	ErrTelegramCodeInvalid = errors.New("link code is invalid or expired")
	// ErrTelegramNoTeamChat — активный командный чат не привязан.
	ErrTelegramNoTeamChat = errors.New("no active team chat")
)

// TelegramLink — привязка аккаунта пользователя к Telegram. DM chat_id равен TGUserID.
type TelegramLink struct {
	UserID     string
	TGUserID   int64
	Username   string
	DMEnabled  bool
	BotBlocked bool
	LinkedAt   time.Time
	UpdatedAt  time.Time
}

// TelegramLinkCode — одноразовый код привязки из веба. Сам код не хранится,
// только его хеш; наружу отдаётся лишь deep link.
type TelegramLinkCode struct {
	ID          string
	UserID      string
	ExpiresAt   time.Time
	ClaimedByTG *int64
	UsedAt      *time.Time
	CreatedAt   time.Time
}

// TelegramChat — командный чат, куда уходят напоминания. Активным может быть
// только один (частичный уникальный индекс в миграции).
type TelegramChat struct {
	ID            string
	TGChatID      int64
	Type          string
	Title         *string
	IsActive      bool
	BoundBy       *string
	BoundAt       time.Time
	DeactivatedAt *time.Time
	LastError     *string
}

// ReminderOccurrence — снимок периода платежа, который читает движок напоминаний.
type ReminderOccurrence struct {
	ID                 string
	RecurringPaymentID string
	Title              string
	Amount             string // десятичная строка из минимальных единиц (amount/100), без float
	Currency           string // RUB|USD|EUR
	DueDate            clock.Date
	Paid               bool
	PaidDate           *clock.Date
	Ended              bool    // платёж завершён или удалён
	AssigneeID         *string // users.id, может быть nil
	AssigneeName       string
}

// Константы outbox-напоминаний.
const (
	// Виды напоминаний.
	NotificationKindDueSoon = "due_soon"
	NotificationKindOverdue = "overdue"

	// Логический адресат.
	NotificationRecipientTeam = "team"

	// Состояния строки outbox.
	NotificationStatePending   = "pending"
	NotificationStateFailed    = "failed"
	NotificationStateSent      = "sent"
	NotificationStateDead      = "dead"
	NotificationStateCancelled = "cancelled"

	// Что сейчас показано в отправленном сообщении.
	ShownStatusOpen  = "open"
	ShownStatusPaid  = "paid"
	ShownStatusEnded = "ended"
)

// TelegramNotification — строка outbox напоминаний. История at-least-once
// доставки; внешних ключей на платежи нет.
type TelegramNotification struct {
	ID            string
	OccurrenceID  string
	DueDate       clock.Date
	Kind          string
	Recipient     string
	State         string
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	ChatID        *int64
	MessageID     *int64
	ShownStatus   string
	CreatedAt     time.Time
	SentAt        *time.Time
	UpdatedAt     time.Time
}
