package middleware

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
)

type ctxKey string

const ClaimsKey ctxKey = "claims"

// ClaimsFromContext достаёт клеймы, положенные Auth-middleware.
func ClaimsFromContext(ctx context.Context) (*model.Claims, bool) {
	c, ok := ctx.Value(ClaimsKey).(*model.Claims)
	return c, ok
}

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap нужен, чтобы http.ResponseController добрался до Flush/Hijack.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Hijack keeps WebSocket upgrades working through the tracing wrapper. Gorilla
// WebSocket requires the concrete response writer to implement http.Hijacker.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	rw.statusCode = http.StatusSwitchingProtocols
	rw.wroteHeader = true
	return hijacker.Hijack()
}

type JWTManager interface {
	ValidateAccess(tokenStr string) (*model.Claims, error)
	ValidateRefresh(tokenStr string) (*model.Claims, error)
}

type Middleware struct {
	jwtManager     JWTManager
	allowedOrigins map[string]struct{}
}

func NewMiddleware(jwtManager JWTManager, allowedOrigins []string) *Middleware {
	set := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		set[o] = struct{}{}
	}
	return &Middleware{
		jwtManager:     jwtManager,
		allowedOrigins: set,
	}
}

func (m *Middleware) Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := newResponseWriter(w)
		start := time.Now()

		slog.Debug(">>> incoming request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(rw, r)

		slog.Info("<<< done request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

// MaxBytes возвращает middleware, ограничивающий размер тела запроса значением limit
// (в байтах), чтобы клиент не смог прислать сколь угодно большое тело и исчерпать память.
// Разные маршруты используют разные лимиты: JSON — небольшой, загрузка файлов — больше.
func (m *Middleware) MaxBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// extractToken: сначала кука, затем Authorization: Bearer <token>.
func extractToken(r *http.Request, cookieName string) (string, bool) {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value, true
	}

	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		if tok := strings.TrimSpace(h[len(prefix):]); tok != "" {
			return tok, true
		}
	}
	return "", false
}

func (m *Middleware) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := extractToken(r, "AccessToken")
		if !ok {
			slog.Debug("no access token", "path", r.URL.Path)
			unauthorized(w)
			return
		}

		claims, err := m.jwtManager.ValidateAccess(token)
		if err != nil {
			slog.Debug("invalid access token", "error", err, "path", r.URL.Path)
			ClearCookie(w, "AccessToken", "/")
			unauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthJSON is the order/cart/chat counterpart of Auth. It preserves the
// existing authentication behavior but uses the module's JSON error contract.
func (m *Middleware) AuthJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := extractToken(r, "AccessToken")
		if !ok {
			api.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return
		}
		claims, err := m.jwtManager.ValidateAccess(token)
		if err != nil {
			slog.Debug("invalid access token", "error", err, "path", r.URL.Path)
			ClearCookie(w, "AccessToken", "/")
			api.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return
		}
		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth добавляет клеймы в контекст, если запрос содержит валидный access-токен.
// Без токена или с невалидным токеном запрос продолжает обрабатываться как гостевой.
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		w.Header().Add("Vary", "Cookie")

		token, ok := extractToken(r, "AccessToken")
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := m.jwtManager.ValidateAccess(token)
		if err != nil {
			slog.Debug("invalid optional access token", "error", err, "path", r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) IsAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			slog.Debug("not admin", "path", r.URL.Path)
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) IsUserCRM(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || (claims.Role != "admin" && claims.Role != "user") {
			slog.Debug("not a CRM user", "path", r.URL.Path)
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) IsUserCRMJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || (claims.Role != "admin" && claims.Role != "user") {
			api.WriteError(w, r, http.StatusForbidden, "forbidden", "forbidden", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Vary обязателен: ответ зависит от Origin, иначе кэш отдаст чужой заголовок.
		w.Header().Add("Vary", "Origin")

		if _, ok := m.allowedOrigins[origin]; ok && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func forbidden(w http.ResponseWriter) {
	http.Error(w, "forbidden", http.StatusForbidden)
}

func ClearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
