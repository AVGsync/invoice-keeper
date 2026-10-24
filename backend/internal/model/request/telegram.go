package request

import (
	"errors"
	"strings"
)

// MiniAppAuthRequest — тело входа в Mini App по уже существующей привязке.
type MiniAppAuthRequest struct {
	// Сырая строка initData из Telegram.WebApp (query-формат). Не логируется.
	InitData string `json:"init_data" example:"query_id=...&user=%7B...%7D&auth_date=...&hash=..."`
}

// MiniAppLinkRequest — тело привязки Telegram к аккаунту по логину и паролю из
// Mini App (пароль вводится один раз).
type MiniAppLinkRequest struct {
	// Сырая строка initData из Telegram.WebApp. Не логируется.
	InitData string `json:"init_data" example:"query_id=...&user=%7B...%7D&auth_date=...&hash=..."`
	// Логин аккаунта в Invoice Keeper.
	Login string `json:"login" example:"ivan.petrov"`
	// Пароль аккаунта. В логи не попадает.
	Password string `json:"password" example:"Str0ngPass!2026"`
}

// Validate проверяет, что initData передан.
func (r MiniAppAuthRequest) Validate() error {
	if strings.TrimSpace(r.InitData) == "" {
		return errors.New("init_data is required")
	}
	return nil
}

// Validate проверяет, что заданы initData, логин и пароль.
func (r MiniAppLinkRequest) Validate() error {
	if strings.TrimSpace(r.InitData) == "" {
		return errors.New("init_data is required")
	}
	if strings.TrimSpace(r.Login) == "" {
		return errors.New("login is required")
	}
	if r.Password == "" {
		return errors.New("password is required")
	}
	return nil
}
