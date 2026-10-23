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
//
// За nginx настоящий адрес приходит в заголовке X-Real-IP: nginx выставляет его
// как `proxy_set_header X-Real-IP $remote_addr`, то есть перезаписывает целиком —
// подделать его клиентом нельзя. X-Forwarded-For здесь НЕ используем: клиент может
// дослать свой XFF, а nginx лишь допишет к нему свой адрес.
// Без прокси (локальный запуск) падаем на RemoteAddr.
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
// Для запросов без клеймов падаем на IP.
func keyByUser(r *http.Request) (string, error) {
	if c, ok := ClaimsFromContext(r.Context()); ok {
		return c.UserID, nil
	}
	return clientIP(r), nil
}

// rateLimit — общий конструктор in-memory лимитера (token bucket) с ответом 429
// в едином со всем API формате. Хранилище — в памяти процесса; этого достаточно,
// пока сервис работает в один инстанс.
func (m *Middleware) rateLimit(requests int, window time.Duration, keyFunc httprate.KeyFunc) func(http.Handler) http.Handler {
	return httprate.LimitBy(
		requests, window, keyFunc,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("rate limit exceeded", "path", r.URL.Path, "ip", clientIP(r))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		}),
	)
}

// LimitByIP ограничивает по реальному IP клиента. Применяется к неаутентифицированным
// эндпоинтам (логин, регистрация, публичная заявка) для защиты от брутфорса и спама.
func (m *Middleware) LimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return m.rateLimit(requests, window, keyByClientIP)
}

// LimitByUser ограничивает по пользователю из клеймов, а для гостевого запроса — по IP.
// Ключ по userID точнее ключа по IP: каждый авторизованный пользователь получает
// независимый бюджет и не делит его с теми, кто оказался за тем же адресом.
func (m *Middleware) LimitByUser(requests int, window time.Duration) func(http.Handler) http.Handler {
	return m.rateLimit(requests, window, keyByUser)
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
