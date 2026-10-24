package apiserver

import (
	"fmt"
	"log/slog"
	"time"
)

// TelegramConfig — настройки Telegram-бота и движка напоминаний.
type TelegramConfig struct {
	// Режим приёма апдейтов: off | polling | webhook.
	TelegramMode string `envconfig:"TELEGRAM_MODE" default:"off"`
	// Токен бота.
	TelegramBotToken string `envconfig:"TELEGRAM_BOT_TOKEN"`
	// Username бота для deep link (t.me/<username>?start=...).
	TelegramBotUsername string `envconfig:"TELEGRAM_BOT_USERNAME"`
	// URL webhook: https://<DOMAIN>/api/v1/telegram/webhook.
	TelegramWebhookURL string `envconfig:"TELEGRAM_WEBHOOK_URL"`
	// Секрет webhook (заголовок X-Telegram-Bot-Api-Secret-Token).
	TelegramWebhookSecret string `envconfig:"TELEGRAM_WEBHOOK_SECRET"`
	// Базовый URL Bot API.
	TelegramAPIURL string `envconfig:"TELEGRAM_API_URL" default:"https://api.telegram.org"`
	// Начальное значение командного чата, если в БД нет активного.
	TelegramTeamChatID string `envconfig:"TELEGRAM_TEAM_CHAT_ID"`
	// URL Mini App (https://<DOMAIN>/tg).
	MiniAppURL string `envconfig:"MINIAPP_URL"`
	// short_name Mini App из BotFather (/newapp).
	TelegramMiniAppShortName string `envconfig:"TELEGRAM_MINIAPP_SHORT_NAME"`
	// Максимальный возраст initData Mini App (auth_date).
	TelegramInitDataMaxAge time.Duration `envconfig:"TELEGRAM_INITDATA_MAX_AGE" default:"10m"`

	// Источник данных о периодах: db (SQL по таблицам B, по умолчанию) | stub (in-memory демо-фикстуры, только для демонстрации без базы).
	ReminderSource string `envconfig:"REMINDER_SOURCE" default:"db"`
	// За сколько дней до срока напоминать.
	ReminderLeadDays int `envconfig:"REMINDER_LEAD_DAYS" default:"3"`
	// Период тика движка напоминаний.
	ReminderTick time.Duration `envconfig:"REMINDER_TICK" default:"30s"`
}

// validateTelegram проверяет связность Telegram-настроек.
func (c *Config) validateTelegram() error {
	switch c.TelegramMode {
	case "off", "polling", "webhook":
	default:
		return fmt.Errorf("TELEGRAM_MODE must be one of: off, polling, webhook")
	}

	botEnabled := c.TelegramMode != "off" && c.TelegramBotToken != ""
	if c.TelegramMode != "off" && c.TelegramBotToken == "" {
		slog.Warn("TELEGRAM_MODE != off, но TELEGRAM_BOT_TOKEN пуст: бот выключен",
			"mode", c.TelegramMode)
	}

	// URL и secret webhook обязательны только когда бот действительно включён в режиме webhook.
	if botEnabled && c.TelegramMode == "webhook" {
		if c.TelegramWebhookURL == "" {
			return fmt.Errorf("TELEGRAM_WEBHOOK_URL is required in webhook mode")
		}
		if len(c.TelegramWebhookSecret) < 32 {
			return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must be at least 32 characters in webhook mode")
		}
	}

	switch c.ReminderSource {
	case "stub", "db":
	default:
		return fmt.Errorf("REMINDER_SOURCE must be one of: stub, db")
	}
	// Заглушку в проде запрещаем только когда движок реально работает (бот включён).
	if botEnabled && c.Environment == "production" && c.ReminderSource == "stub" {
		return fmt.Errorf("REMINDER_SOURCE=stub is not allowed in production when the bot is enabled")
	}

	if c.ReminderLeadDays < 3 {
		return fmt.Errorf("REMINDER_LEAD_DAYS must be at least 3")
	}
	if c.ReminderTick <= 0 {
		return fmt.Errorf("REMINDER_TICK must be positive")
	}
	return nil
}
