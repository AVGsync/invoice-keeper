package middleware

import (
	"net/http"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
)

// DetectChannel определяет канал запроса (C.1): заголовок X-Client (miniapp|web)
// важнее всего; иначе Bearer-токен ⇒ miniapp (Bearer в продукте использует только
// Mini App); иначе (кука) ⇒ web.
func DetectChannel(r *http.Request) string {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("X-Client"))) {
	case "miniapp":
		return audit.ChannelMiniApp
	case "web":
		return audit.ChannelWeb
	}
	const prefix = "Bearer "
	if h := r.Header.Get("Authorization"); len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return audit.ChannelMiniApp
	}
	return audit.ChannelWeb
}

// AuditContext кладёт в контекст метаданные запроса для журнала действий (канал
// по DetectChannel). Ставится после Auth, чтобы увидеть клеймы.
func (m *Middleware) AuditContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := audit.WithMeta(r.Context(), m.auditMeta(r, DetectChannel(r)))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuditContextFor кладёт метаданные с фиксированным каналом (для Mini App-ручек,
// где канал всегда miniapp независимо от заголовков).
func (m *Middleware) AuditContextFor(channel string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := audit.WithMeta(r.Context(), m.auditMeta(r, channel))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// auditMeta собирает Meta из запроса: актор из клеймов (если есть), IP, UA,
// request_id.
func (m *Middleware) auditMeta(r *http.Request, channel string) audit.Meta {
	meta := audit.Meta{
		Channel:   channel,
		IP:        clientIP(r),
		UserAgent: r.Header.Get("User-Agent"),
		RequestID: api.RequestID(r.Context()),
	}
	if c, ok := ClaimsFromContext(r.Context()); ok {
		meta.ActorID = c.UserID
		meta.ActorRole = c.Role
	}
	return meta
}
