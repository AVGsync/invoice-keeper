package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
	"github.com/go-chi/httprate"
)

// clientIP возвращает реальный IP клиента.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func keyByClientIP(r *http.Request) (string, error) {
	return clientIP(r), nil
}

// keyByUser ключует лимит по userID из клеймов (проставляет Auth-middleware).
func keyByUser(r *http.Request) (string, error) {
	if c, ok := ClaimsFromContext(r.Context()); ok {
		return c.UserID, nil
	}
	return clientIP(r), nil
}

// rateLimit — общий конструктор in-memory лимитера (token bucket) с ответом 429 в едином со всем API формате.
func (m *Middleware) rateLimit(requests int, window time.Duration, keyFunc httprate.KeyFunc) func(http.Handler) http.Handler {
	return httprate.LimitBy(
		requests, window, keyFunc,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("rate limit exceeded", "path", r.URL.Path, "ip", clientIP(r))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		}),
	)
}

// LimitByIP ограничивает по реальному IP клиента.
func (m *Middleware) LimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return m.rateLimit(requests, window, keyByClientIP)
}

// LimitByUser ограничивает по пользователю из клеймов, а для гостевого запроса — по IP.
func (m *Middleware) LimitByUser(requests int, window time.Duration) func(http.Handler) http.Handler {
	return m.rateLimit(requests, window, keyByUser)
}

// LimitByIPJSON — вариант LimitByIP с ответом 429 в едином JSON-формате.
func (m *Middleware) LimitByIPJSON(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitBy(
		requests,
		window,
		keyByClientIP,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("rate limit exceeded", "path", r.URL.Path, "ip", clientIP(r))
			api.WriteError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests", nil)
		}),
	)
}

func (m *Middleware) LimitByUserJSON(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitBy(
		requests,
		window,
		keyByUser,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("rate limit exceeded", "path", r.URL.Path, "ip", clientIP(r))
			api.WriteError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests", nil)
		}),
	)
}
