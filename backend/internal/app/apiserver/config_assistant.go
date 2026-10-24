package apiserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

// AssistantConfig — настройки NL-ассистента.
type AssistantConfig struct {
	// Включает ручки /api/v1/assistant/*.
	NLQEnabled bool `envconfig:"NLQ_ENABLED" default:"false"`
	// Адрес агента во внутренней сети.
	NLQAgentURL string `envconfig:"NLQ_AGENT_URL" default:"http://nlq-agent:8090"`
	// Общий таймаут одного вопроса.
	NLQTimeout time.Duration `envconfig:"NLQ_TIMEOUT" default:"25s"`
	// Лимит вопросов в минуту на пользователя.
	NLQRatePerMin int `envconfig:"NLQ_RATE_PER_MIN" default:"10"`
	// Лимит вопросов в сутки на пользователя.
	NLQDailyLimit int `envconfig:"NLQ_DAILY_LIMIT" default:"200"`
	// Одновременных вопросов на процесс (семафор).
	NLQMaxConcurrent int `envconfig:"NLQ_MAX_CONCURRENT" default:"4"`
	// base64 от 32-байтного seed Ed25519.
	NLQActorKey string `envconfig:"NLQ_ACTOR_PRIVATE_KEY"`

	// Читать/писать историю и слать history агенту.
	NLQMemoryEnabled bool `envconfig:"NLQ_MEMORY_ENABLED" default:"true"`
	// Сколько последних успешных ходов идёт в промпт (in_prompt), 0..8.
	NLQMemoryTurns int `envconfig:"NLQ_MEMORY_TURNS" default:"4"`
	// Максимальный возраст хода для попадания в промпт.
	NLQMemoryTTL time.Duration `envconfig:"NLQ_MEMORY_TTL" default:"2h"`
	// Сколько ходов передаётся агенту всего (in_prompt:false видит только recall_conversation), NLQMemoryTurns..50.
	NLQMemoryMaxTurns int `envconfig:"NLQ_MEMORY_MAX_TURNS" default:"20"`
	// Сколько ходов разговора хранить; прочее prune после вставки, ≥ MaxTurns.
	NLQHistoryKeep int `envconfig:"NLQ_HISTORY_KEEP" default:"100"`

	// actorKey — разобранный приватный ключ.
	actorKey ed25519.PrivateKey
}

// validateAssistant проверяет связность настроек ассистента.
func (c *Config) validateAssistant() error {
	if c.NLQMemoryTurns == 0 && c.NLQMemoryTTL == 0 && c.NLQMemoryMaxTurns == 0 && c.NLQHistoryKeep == 0 {
		c.NLQMemoryTurns = 4
		c.NLQMemoryTTL = 2 * time.Hour
		c.NLQMemoryMaxTurns = 20
		c.NLQHistoryKeep = 100
	}

	if c.NLQMemoryTurns < 0 || c.NLQMemoryTurns > 8 {
		return fmt.Errorf("NLQ_MEMORY_TURNS must be between 0 and 8")
	}
	if c.NLQMemoryTTL <= 0 {
		return fmt.Errorf("NLQ_MEMORY_TTL must be positive")
	}
	if c.NLQMemoryMaxTurns < c.NLQMemoryTurns || c.NLQMemoryMaxTurns > 50 {
		return fmt.Errorf("NLQ_MEMORY_MAX_TURNS must be between NLQ_MEMORY_TURNS and 50")
	}
	if c.NLQHistoryKeep < c.NLQMemoryMaxTurns {
		return fmt.Errorf("NLQ_HISTORY_KEEP must be >= NLQ_MEMORY_MAX_TURNS")
	}

	if !c.NLQEnabled {
		return nil
	}
	if c.NLQActorKey == "" {
		return fmt.Errorf("NLQ_ACTOR_PRIVATE_KEY is required when NLQ_ENABLED=true")
	}
	seed, err := base64.StdEncoding.DecodeString(c.NLQActorKey)
	if err != nil {
		return fmt.Errorf("NLQ_ACTOR_PRIVATE_KEY is not valid base64: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("NLQ_ACTOR_PRIVATE_KEY must decode to %d bytes (Ed25519 seed), got %d", ed25519.SeedSize, len(seed))
	}
	c.actorKey = ed25519.NewKeyFromSeed(seed)

	if c.NLQTimeout <= 0 {
		return fmt.Errorf("NLQ_TIMEOUT must be positive")
	}
	if c.NLQTimeout >= 30*time.Second {
		return fmt.Errorf("NLQ_TIMEOUT must be less than the 30s server WriteTimeout")
	}
	if c.NLQRatePerMin <= 0 || c.NLQDailyLimit <= 0 {
		return fmt.Errorf("NLQ_RATE_PER_MIN and NLQ_DAILY_LIMIT must be positive")
	}
	if c.NLQMaxConcurrent <= 0 {
		return fmt.Errorf("NLQ_MAX_CONCURRENT must be positive")
	}
	return nil
}

// ActorKey возвращает разобранный приватный ключ actor-JWT.
func (c *Config) ActorKey() ed25519.PrivateKey {
	return c.actorKey
}
