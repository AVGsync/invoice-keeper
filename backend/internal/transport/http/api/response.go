package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/google/uuid"
)

type requestIDKey struct{}

// RequestIDMiddleware gives every new JSON API response a correlation ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	requestID := RequestID(r.Context())
	if requestID == "" {
		requestID = uuid.NewString()
		w.Header().Set("X-Request-ID", requestID)
	}
	WriteJSON(w, status, response.ErrorResponse{Error: response.APIError{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}})
}

func validRequestID(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > 128 {
		return false
	}
	for _, r := range value {
		if !unicode.IsPrint(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
