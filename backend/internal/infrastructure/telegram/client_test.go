package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("TTOKEN", srv.URL)
}

func TestClient_SendMessage_OK(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/botTTOKEN/sendMessage") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"ok":true,"result":{"message_id":55,"chat":{"id":10,"type":"private"},"text":"hi"}}`))
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg, err := c.SendMessage(ctx, SendMessageParams{ChatID: 10, Text: "hi"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.MessageID != 55 {
		t.Errorf("MessageID = %d, want 55", msg.MessageID)
	}
}

func TestClient_RetryAfter(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":7}}`))
	})

	_, err := c.SendMessage(context.Background(), SendMessageParams{ChatID: 1, Text: "x"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if !apiErr.IsRetryAfter() || apiErr.RetryAfter != 7 {
		t.Errorf("retry_after = %d, IsRetryAfter=%v", apiErr.RetryAfter, apiErr.IsRetryAfter())
	}
}

func TestClient_Migrate(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: group chat was upgraded to a supergroup chat","parameters":{"migrate_to_chat_id":-100999}}`))
	})
	_, err := c.SendMessage(context.Background(), SendMessageParams{ChatID: -1, Text: "x"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if !apiErr.IsMigrate() || apiErr.MigrateToChatID != -100999 {
		t.Errorf("migrate = %d, IsMigrate=%v", apiErr.MigrateToChatID, apiErr.IsMigrate())
	}
}

func TestClient_DoError_NoTokenLeak(t *testing.T) {
	const token = "123456:SECRET-TOKEN"
	c := NewClient(token, "http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.SendMessage(ctx, SendMessageParams{ChatID: 1, Text: "x"})
	if err == nil {
		t.Fatal("ожидалась ошибка сети")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("токен утёк в текст ошибки: %q", err.Error())
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("часть токена утекла в текст ошибки: %q", err.Error())
	}
}

func TestClient_Blocked(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
	})
	_, err := c.SendMessage(context.Background(), SendMessageParams{ChatID: 1, Text: "x"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if !apiErr.IsBotBlocked() {
		t.Error("IsBotBlocked = false, want true")
	}
}
