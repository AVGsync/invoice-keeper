package model

import "time"

// EmailVerifyState — состояние подтверждения адреса почты.
type EmailVerifyState struct {
	Email     string
	Verified  bool
	CodeHash  string
	ExpiresAt *time.Time
	Attempts  int
	SentAt    *time.Time
}

// EmailCandidate — неоплаченный платёж пользователя с включёнными и подтверждёнными письмами.
type EmailCandidate struct {
	UserID     string
	PaymentID  string
	DueDate    string
	Timezone   string
	SendTime   string
	DaysBefore int
}

type EmailEnqueue struct {
	UserID    string
	PaymentID string
	DueDate   string
	Kind      string
}

// EmailReminderRow — строка очереди писем вместе с актуальным состоянием платежа и настроек.
type EmailReminderRow struct {
	ID         string
	UserID     string
	Kind       string
	DueDate    string
	Attempts   int
	PaymentID  string
	Title      string
	Amount     int64
	Currency   string
	Paid       bool
	Stopped    bool
	PaymentDue string
	Email      string
	FullName   string
	Timezone   string
	Enabled    bool
	Verified   bool
}
