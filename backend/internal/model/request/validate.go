package request

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Регэкспы совпадают с CHECK-ограничениями в миграциях,
// чтобы не отдавать 500 там, где должен быть 400.
var loginRegexp = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

func runesBetween(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}

func (r LoginRequest) Validate() error {
	if strings.TrimSpace(r.Login) == "" {
		return errors.New("login is required")
	}
	if r.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (r CreateUserRequest) Validate() error {
	if !runesBetween(r.FullName, 3, 64) {
		return errors.New("full_name must be 3-64 characters")
	}
	if !loginRegexp.MatchString(r.Login) {
		return errors.New("login must be 3-32 chars: letters, digits, . _ -")
	}
	if utf8.RuneCountInString(r.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	// bcrypt молча обрезает вход до 72 байт — не даём хранить обрезанный пароль.
	if len(r.Password) > 72 {
		return errors.New("password must be at most 72 bytes")
	}
	if r.PersonaID != nil {
		if _, err := uuid.Parse(*r.PersonaID); err != nil {
			return errors.New("persona_id must be a UUID")
		}
	}
	return nil
}

func (r CreatePersonaRequest) Validate() error {
	if !runesBetween(r.Name, 1, 64) {
		return errors.New("name must be 1-64 characters")
	}
	if r.Description != nil && utf8.RuneCountInString(strings.TrimSpace(*r.Description)) > 255 {
		return errors.New("description must be at most 255 characters")
	}
	return nil
}

func (r UpdatePersonaRequest) Validate() error {
	if r.Name == nil && r.Description == nil {
		return errors.New("at least one field must be provided")
	}
	if r.Name != nil && !runesBetween(*r.Name, 1, 64) {
		return errors.New("name must be 1-64 characters")
	}
	if r.Description != nil && utf8.RuneCountInString(strings.TrimSpace(*r.Description)) > 255 {
		return errors.New("description must be at most 255 characters")
	}
	return nil
}

func (r UserUpdateRequest) Validate() error {
	if r.Login == nil && r.FullName == nil {
		return errors.New("at least one field must be provided")
	}
	if r.Login != nil && !loginRegexp.MatchString(*r.Login) {
		return errors.New("login must be 3-32 chars: letters, digits, . _ -")
	}
	if r.FullName != nil && !runesBetween(*r.FullName, 3, 64) {
		return errors.New("full_name must be 3-64 characters")
	}
	return nil
}
