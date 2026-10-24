package apiserver

import (
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// TestEnvconfigEmbedding проверяет ключевое допущение дизайна [Д §2.6]:
// envconfig читает поля анонимно встроенной TelegramConfig по их собственным
// тегам, без префикса родителя. Если это сломается, бот молча получит дефолты.
func TestEnvconfigEmbedding(t *testing.T) {
	env := map[string]string{
		"JWT_ACCESS_SECRET":     "a",
		"JWT_REFRESH_SECRET":    "b",
		"TELEGRAM_MODE":         "polling",
		"TELEGRAM_BOT_TOKEN":    "123:ABC",
		"TELEGRAM_BOT_USERNAME": "dev_bot",
		"TELEGRAM_API_URL":      "http://localhost:9999",
		"TELEGRAM_TEAM_CHAT_ID": "-1001234567890",
		"MINIAPP_URL":           "https://example.tld/tg",
		"REMINDER_SOURCE":       "db",
		"REMINDER_LEAD_DAYS":    "5",
		"REMINDER_TICK":         "10s",
		"APP_TIMEZONE":          "Europe/Moscow",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatalf("envconfig.Process: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"TelegramMode", cfg.TelegramMode, "polling"},
		{"TelegramBotToken", cfg.TelegramBotToken, "123:ABC"},
		{"TelegramBotUsername", cfg.TelegramBotUsername, "dev_bot"},
		{"TelegramAPIURL", cfg.TelegramAPIURL, "http://localhost:9999"},
		{"TelegramTeamChatID", cfg.TelegramTeamChatID, "-1001234567890"},
		{"MiniAppURL", cfg.MiniAppURL, "https://example.tld/tg"},
		{"ReminderSource", cfg.ReminderSource, "db"},
		{"ReminderLeadDays", cfg.ReminderLeadDays, 5},
		{"ReminderTick", cfg.ReminderTick, 10 * time.Second},
		{"AppTimezone", cfg.AppTimezone, "Europe/Moscow"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v (envconfig не прочитал поле встроенной структуры без префикса)", c.name, c.got, c.want)
		}
	}
}

// TestEnvconfigDefaults проверяет дефолты встроенной структуры.
func TestEnvconfigDefaults(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "a")
	t.Setenv("JWT_REFRESH_SECRET", "b")

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatalf("envconfig.Process: %v", err)
	}
	if cfg.TelegramMode != "off" {
		t.Errorf("TelegramMode default = %q, want off", cfg.TelegramMode)
	}
	if cfg.TelegramAPIURL != "https://api.telegram.org" {
		t.Errorf("TelegramAPIURL default = %q", cfg.TelegramAPIURL)
	}
	if cfg.ReminderSource != "stub" {
		t.Errorf("ReminderSource default = %q, want stub", cfg.ReminderSource)
	}
	if cfg.ReminderLeadDays != 3 {
		t.Errorf("ReminderLeadDays default = %d, want 3", cfg.ReminderLeadDays)
	}
	if cfg.ReminderTick != 30*time.Second {
		t.Errorf("ReminderTick default = %v, want 30s", cfg.ReminderTick)
	}
	if cfg.AppTimezone != "Europe/Moscow" {
		t.Errorf("AppTimezone default = %q", cfg.AppTimezone)
	}
}

func TestValidateTimezone(t *testing.T) {
	cfg := Config{
		Environment:     "local",
		TTLAccessToken:  3600,
		TTLRefreshToken: 604800,
		AdminLogin:      "admin",
		AppTimezone:     "Nonexistent/Zone",
		TelegramConfig:  TelegramConfig{TelegramMode: "off", ReminderSource: "stub", ReminderLeadDays: 3, ReminderTick: 30 * time.Second},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate: expected error on invalid APP_TIMEZONE")
	}

	cfg.AppTimezone = "Europe/Moscow"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: unexpected error: %v", err)
	}
	if cfg.Location().String() != "Europe/Moscow" {
		t.Errorf("Location() = %q after validate", cfg.Location())
	}
}

func TestValidateTelegramModes(t *testing.T) {
	base := func() Config {
		return Config{
			Environment:     "local",
			TTLAccessToken:  3600,
			TTLRefreshToken: 604800,
			AdminLogin:      "admin",
			AppTimezone:     "Europe/Moscow",
			TelegramConfig: TelegramConfig{
				TelegramMode:     "off",
				ReminderSource:   "stub",
				ReminderLeadDays: 3,
				ReminderTick:     30 * time.Second,
			},
		}
	}

	t.Run("webhook requires url and secret", func(t *testing.T) {
		cfg := base()
		cfg.TelegramMode = "webhook"
		cfg.TelegramBotToken = "t"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error: webhook without url/secret")
		}
		cfg.TelegramWebhookURL = "https://x.tld/api/v1/telegram/webhook"
		cfg.TelegramWebhookSecret = "0123456789012345678901234567890123"
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("polling without token is allowed (bot disabled)", func(t *testing.T) {
		// Решение №7: режим polling без токена не ошибка — бот просто выключен.
		cfg := base()
		cfg.TelegramMode = "polling"
		if err := cfg.Validate(); err != nil {
			t.Errorf("polling без токена не должен быть ошибкой: %v", err)
		}
	})

	t.Run("webhook without token is allowed (bot disabled)", func(t *testing.T) {
		// Пустой токен => бот выключен, setWebhook не вызывается, url/secret не нужны.
		cfg := base()
		cfg.TelegramMode = "webhook"
		if err := cfg.Validate(); err != nil {
			t.Errorf("webhook без токена не должен быть ошибкой: %v", err)
		}
	})

	t.Run("production with all telegram vars unset passes", func(t *testing.T) {
		// Прод-compose c APP_ENV=production, но без токена (бот выключен) и
		// дефолтным REMINDER_SOURCE=stub должен стартовать, а не крашиться.
		cfg := base()
		cfg.Environment = "production"
		if err := cfg.Validate(); err != nil {
			t.Errorf("прод с выключенным ботом не должен падать: %v", err)
		}
	})

	t.Run("production forbids stub source when bot enabled", func(t *testing.T) {
		cfg := base()
		cfg.Environment = "production"
		cfg.TelegramMode = "polling"
		cfg.TelegramBotToken = "123:ABC" // бот включён
		cfg.ReminderSource = "stub"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error: stub source in production with bot enabled")
		}
		// С REMINDER_SOURCE=db прод с включённым ботом валиден.
		cfg.ReminderSource = "db"
		if err := cfg.Validate(); err != nil {
			t.Errorf("прод с db-источником не должен падать: %v", err)
		}
	})

	t.Run("lead days minimum 3", func(t *testing.T) {
		cfg := base()
		cfg.ReminderLeadDays = 2
		if err := cfg.Validate(); err == nil {
			t.Error("expected error: lead days < 3")
		}
	})
}
