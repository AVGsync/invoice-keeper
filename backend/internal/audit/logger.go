package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"time"
)

// recordTimeout — таймаут синхронной записи строки журнала (§C.1).
const recordTimeout = 2 * time.Second

// maxPayloadBytes — сериализованный payload больше этого заменяется на
// {"truncated":true}, чтобы раздутый payload не попал в журнал.
const maxPayloadBytes = 8 << 10 // 8 КБ

// Logger — боевой рекордер: сливает Meta и Entry, редактирует payload/changes,
// применяет лимиты и синхронно пишет строку. Ошибка записи только логируется.
type Logger struct {
	repo Repository
}

// NewLogger создаёт рекордер поверх репозитория audit_log.
func NewLogger(repo Repository) *Logger {
	return &Logger{repo: repo}
}

// Record собирает и пишет строку журнала. Выполняется после успешной мутации,
// в том же запросе; использует context.WithoutCancel, чтобы запись не отменилась
// вместе с отменой запроса, и таймаут recordTimeout.
func (l *Logger) Record(ctx context.Context, e Entry) {
	if l == nil || l.repo == nil {
		return
	}

	meta, _ := MetaFrom(ctx)

	row := Row{
		Channel:    firstNonEmpty(meta.Channel, ChannelSystem),
		Action:     e.Action,
		Outcome:    firstNonEmpty(e.Outcome, OutcomeSuccess),
		ActorID:    ptr(firstNonEmpty(e.ActorID, meta.ActorID)),
		ActorRole:  ptr(firstNonEmpty(e.ActorRole, meta.ActorRole)),
		EntityType: ptr(e.EntityType),
		EntityID:   ptr(e.EntityID),
		UserAgent:  ptr(truncateRunes(meta.UserAgent, maxUserAgentRunes)),
		RequestID:  ptr(meta.RequestID),
	}

	// IP пишем только если он валиден (middleware уже проверяет, но страхуемся).
	if meta.IP != "" && net.ParseIP(meta.IP) != nil {
		ip := meta.IP
		row.IP = &ip
	}

	if changes := redactChanges(e.Changes); len(changes) > 0 {
		if b, err := json.Marshal(changes); err == nil {
			row.Changes = b
		} else {
			slog.Error("audit: marshal changes failed", "action", e.Action, "error", err)
		}
	}

	if payload := Redact(e.Payload); len(payload) > 0 {
		if b, err := json.Marshal(payload); err == nil {
			if len(b) > maxPayloadBytes {
				b, _ = json.Marshal(map[string]any{"truncated": true})
			}
			row.Payload = b
		} else {
			slog.Error("audit: marshal payload failed", "action", e.Action, "error", err)
		}
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if err := l.repo.Insert(writeCtx, row); err != nil {
		slog.Error("audit: record failed", "action", e.Action, "error", err)
	}
}

// ptr возвращает nil для пустой строки, иначе указатель на копию.
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
