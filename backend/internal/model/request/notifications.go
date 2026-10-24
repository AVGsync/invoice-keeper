package request

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var emailRegexp = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// UpdateUserNotificationSettingsRequest — полное создание или обновление
// настроек email-уведомлений пользователя.
type UpdateUserNotificationSettingsRequest struct {
	Email         string `json:"email"`
	EmailEnabled  bool   `json:"email_enabled"`
	DaysBeforeDue int    `json:"days_before_due"`
	SendTime      string `json:"send_time"`
	Timezone      string `json:"timezone"`
}

func (r UpdateUserNotificationSettingsRequest) Validate() error {
	email := strings.TrimSpace(r.Email)

	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > 320 || !emailRegexp.MatchString(email) {
		return errors.New("email must be a valid email address of at most 320 characters")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("email must be a valid email address")
	}
	if r.DaysBeforeDue < 0 || r.DaysBeforeDue > 365 {
		return errors.New("days_before_due must be between 0 and 365")
	}
	if _, err := time.Parse("15:04", r.SendTime); err != nil {
		return errors.New("send_time must be in HH:MM format")
	}
	if strings.TrimSpace(r.Timezone) == "" {
		return errors.New("timezone is required")
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return errors.New("timezone must be a valid IANA timezone")
	}

	return nil
}

// PatchUserNotificationSettingsRequest — частичное обновление настроек.
// Поле nil означает, что его не передали.
type PatchUserNotificationSettingsRequest struct {
	Email         *string `json:"email,omitempty"`
	EmailEnabled  *bool   `json:"email_enabled,omitempty"`
	DaysBeforeDue *int    `json:"days_before_due,omitempty"`
	SendTime      *string `json:"send_time,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
}

func (r PatchUserNotificationSettingsRequest) Validate() error {
	if r.Email == nil &&
		r.EmailEnabled == nil &&
		r.DaysBeforeDue == nil &&
		r.SendTime == nil &&
		r.Timezone == nil {
		return errors.New("at least one field must be provided")
	}

	if r.Email != nil {
		email := strings.TrimSpace(*r.Email)

		if email == "" {
			return errors.New("email must not be empty")
		}
		if utf8.RuneCountInString(email) > 320 ||
			!emailRegexp.MatchString(email) {
			return errors.New("email must be a valid email address of at most 320 characters")
		}
		if _, err := mail.ParseAddress(email); err != nil {
			return errors.New("email must be a valid email address")
		}
	}

	if r.DaysBeforeDue != nil &&
		(*r.DaysBeforeDue < 0 || *r.DaysBeforeDue > 365) {
		return errors.New("days_before_due must be between 0 and 365")
	}

	if r.SendTime != nil {
		if _, err := time.Parse("15:04", *r.SendTime); err != nil {
			return errors.New("send_time must be in HH:MM format")
		}
	}

	if r.Timezone != nil {
		if strings.TrimSpace(*r.Timezone) == "" {
			return errors.New("timezone is required")
		}
		if _, err := time.LoadLocation(*r.Timezone); err != nil {
			return errors.New("timezone must be a valid IANA timezone")
		}
	}

	return nil
}
