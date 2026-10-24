package apiserver

import "strings"

// EmailConfig — письма-напоминания.
type EmailConfig struct {
	// Рассылка включена (работает, только если задан SMTP_HOST).
	EmailRemindersEnabled bool `envconfig:"EMAIL_REMINDERS_ENABLED" default:"true"`
	// Адрес приложения для ссылки в письме; по умолчанию https://<DOMAIN>.
	PublicURL string `envconfig:"PUBLIC_URL"`
	Domain    string `envconfig:"DOMAIN"`
}

func (c *Config) appURL() string {
	if u := strings.TrimSpace(c.PublicURL); u != "" {
		return u
	}
	d := strings.TrimSpace(c.Domain)
	if d == "" || d == "localhost" {
		return ""
	}
	return "https://" + d
}
