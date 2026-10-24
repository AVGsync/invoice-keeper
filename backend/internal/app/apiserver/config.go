package apiserver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Тот же набор символов, что у ограничения login_check в базе.
var loginPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

type Config struct {
	Environment      string `envconfig:"APP_ENV" default:"local"`
	BindAddr         string `envconfig:"BIND_ADDR" default:"8080"`
	LogLevel         string `envconfig:"LOG_LEVEL" default:"info"`
	Debug            bool   `envconfig:"DEBUG" default:"false"`
	JWTAccessSecret  string `envconfig:"JWT_ACCESS_SECRET" required:"true"`
	JWTRefreshSecret string `envconfig:"JWT_REFRESH_SECRET" required:"true"`
	TTLAccessToken   int64  `envconfig:"TTL_ACCESS_TOKEN" default:"3600"`
	TTLRefreshToken  int64  `envconfig:"TTL_REFRESH_TOKEN" default:"604800"`

	PostgresUser     string `envconfig:"POSTGRES_USER" default:"postgres"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" default:"postgres"`
	PostgresHost     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort     string `envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresDB       string `envconfig:"POSTGRES_DB" default:"invoice_keeper"`

	SMTPHost     string `envconfig:"SMTP_HOST" default:""`
	SMTPPort     string `envconfig:"SMTP_PORT" default:"587"`
	SMTPUsername string `envconfig:"SMTP_USERNAME" default:""`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" default:""`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:""`

	AllowedOrigins []string `envconfig:"ALLOWED_ORIGINS" default:"http://localhost:3000,http://localhost:8080"`

	// Учётка администратора.
	AdminLogin    string `envconfig:"ADMIN_LOGIN" default:"admin"`
	AdminPassword string `envconfig:"ADMIN_PASSWORD" default:""`
	// Единая зона приложения.
	AppTimezone string `envconfig:"APP_TIMEZONE" default:"Europe/Moscow"`

	// Настройки Telegram-бота и напоминаний вынесены в отдельный файл и встроены анонимно: envconfig читает их поля без префикса родителя.
	TelegramConfig

	// Настройки NL-ассистента, также встроены анонимно.
	AssistantConfig

	// Хранилище чеков, встроено анонимно.
	ReceiptsConfig

	// Письма-напоминания, встроено анонимно.
	EmailConfig

	// location — разобранная AppTimezone.
	location *time.Location
}

func (c *Config) Validate() error {
	switch c.Environment {
	case "local", "development", "test", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of: local, development, test, production")
	}
	if c.TTLAccessToken <= 0 || c.TTLRefreshToken <= 0 {
		return fmt.Errorf("token TTL values must be positive")
	}
	if !loginPattern.MatchString(c.AdminLogin) {
		return fmt.Errorf("ADMIN_LOGIN must be 3-32 characters of [a-zA-Z0-9_.-]")
	}
	// Требование к длине то же, что у пароля пользователя (CreateUserRequest, min=8).
	if c.AdminPassword != "" && len(c.AdminPassword) < 8 {
		return fmt.Errorf("ADMIN_PASSWORD must be at least 8 characters")
	}
	// Зону грузим здесь, чтобы процесс упал на старте при неверном APP_TIMEZONE, а не при первом обращении к clock.
	loc, err := time.LoadLocation(c.AppTimezone)
	if err != nil {
		return fmt.Errorf("APP_TIMEZONE %q is invalid: %w", c.AppTimezone, err)
	}
	c.location = loc
	if err := c.validateTelegram(); err != nil {
		return err
	}
	if err := c.validateAssistant(); err != nil {
		return err
	}
	if err := c.validateReceipts(); err != nil {
		return err
	}
	return nil
}

// Location возвращает разобранную зону приложения.
func (c *Config) Location() *time.Location {
	if c.location == nil {
		return time.UTC
	}
	return c.location
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresDB,
	)
}

func NewConfig() (*Config, error) {
	if err := loadEnv(); err != nil && os.Getenv("APP_ENV") == "" {
		fmt.Fprintf(os.Stderr, "warn: .env not found: %v\n", err)
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("config: process env: %w", err)
	}

	origins := make([]string, 0, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	cfg.AllowedOrigins = origins
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}
	return &cfg, nil
}

func loadEnv() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			// .env.backend загружается первым: значения из него приоритетнее общих.
			if backendPath := filepath.Join(dir, ".env.backend"); fileExists(backendPath) {
				if err := godotenv.Load(backendPath); err != nil {
					return err
				}
			}
			return godotenv.Load(envPath)
		}
		dir = filepath.Dir(dir)
	}

	return fmt.Errorf(".env not found")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
