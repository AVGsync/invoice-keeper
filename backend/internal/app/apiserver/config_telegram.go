package apiserver

import (
	"fmt"
	"log/slog"
	"time"
)

// TelegramConfig — настройки Telegram-бота и движка напоминаний. Встраивается
// в Config анонимно (см. config.go), поэтому envconfig читает её поля по их
// собственным тегам без префикса родителя. Вынесено в отдельный файл, чтобы не
// конфликтовать с общим config.go при параллельной работе (docs/TELEGRAM_BOT.md §2.6, §8).
type TelegramConfig struct {
	// Режим приёма апдейтов: off | polling | webhook.
	TelegramMode string `envconfig:"TELEGRAM_MODE" default:"off"`
	// Токен бота. Обязателен в режимах polling и webhook; пустой => бот выключен.
	TelegramBotToken string `envconfig:"TELEGRAM_BOT_TOKEN"`
	// Username бота для deep link (t.me/<username>?start=...). Необязателен.
	TelegramBotUsername string `envconfig:"TELEGRAM_BOT_USERNAME"`
	// URL webhook: https://<DOMAIN>/api/v1/telegram/webhook. Обязателен в режиме webhook.
	TelegramWebhookURL string `envconfig:"TELEGRAM_WEBHOOK_URL"`
	// Секрет webhook (заголовок X-Telegram-Bot-Api-Secret-Token). Обязателен в режиме webhook.
	TelegramWebhookSecret string `envconfig:"TELEGRAM_WEBHOOK_SECRET"`
	// Базовый URL Bot API. Переопределяется в тестах; по умолчанию официальный.
	TelegramAPIURL string `envconfig:"TELEGRAM_API_URL" default:"https://api.telegram.org"`
	// Начальное значение командного чата, если в БД нет активного. Пусто допустимо.
	TelegramTeamChatID string `envconfig:"TELEGRAM_TEAM_CHAT_ID"`
	// URL Mini App (https://<DOMAIN>/tg). Пусто => кнопка «Открыть в приложении» не показывается.
	MiniAppURL string `envconfig:"MINIAPP_URL"`
	// short_name Mini App из BotFather (/newapp). Нужен для Direct Link в кнопке
	// «Открыть» напоминания: t.me/<bot>/<short_name>?startapp=...; пусто => ссылка
	// на основное Mini App бота t.me/<bot>?startapp=... (§5.5, решение №6/№8).
	TelegramMiniAppShortName string `envconfig:"TELEGRAM_MINIAPP_SHORT_NAME"`
	// Максимальный возраст initData Mini App (auth_date). Старее => 401 init_data_expired (§3.3).
	TelegramInitDataMaxAge time.Duration `envconfig:"TELEGRAM_INITDATA_MAX_AGE" default:"10m"`

	// Источник данных о периодах: stub (in-memory фикстуры) | db (SQL по таблицам B).
	// Пока таблиц B нет, по умолчанию stub.
	ReminderSource string `envconfig:"REMINDER_SOURCE" default:"stub"`
	// За сколько дней до срока напоминать. Минимум 3 по требованию.
	ReminderLeadDays int `envconfig:"REMINDER_LEAD_DAYS" default:"3"`
	// Период тика движка напоминаний.
	ReminderTick time.Duration `envconfig:"REMINDER_TICK" default:"30s"`
}

// validateTelegram проверяет связность Telegram-настроек. Вызывается из Config.Validate.
func (c *Config) validateTelegram() error {
	switch c.TelegramMode {
	case "off", "polling", "webhook":
	default:
		return fmt.Errorf("TELEGRAM_MODE must be one of: off, polling, webhook")
	}

	// Решение №7: пустой TELEGRAM_BOT_TOKEN означает «бот выключен, приложение
	// работает». Непустой режим без токена не ошибка — buildTelegram поднимет
	// enabled=false, горутин и движка не будет. Предупреждаем, но не падаем.
	botEnabled := c.TelegramMode != "off" && c.TelegramBotToken != ""
	if c.TelegramMode != "off" && c.TelegramBotToken == "" {
		slog.Warn("TELEGRAM_MODE != off, но TELEGRAM_BOT_TOKEN пуст: бот выключен (решение №7)",
			"mode", c.TelegramMode)
	}

	// URL и secret webhook обязательны только когда бот действительно включён в
	// режиме webhook. При выключенном боте setWebhook не вызывается.
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
	// При выключенном боте REMINDER_SOURCE не используется и не должен ронять старт:
	// иначе прод-compose с APP_ENV=production и дефолтным REMINDER_SOURCE=stub, но
	// без токена, уходил бы в краш-луп (решение №7).
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
