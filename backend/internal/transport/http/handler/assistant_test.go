package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/middleware"
)

// mockUseCase — подмена сервиса ассистента для контрактных тестов хендлера.
type mockUseCase struct {
	enabled    bool
	proxyBody  string
	proxyCode  int
	proxyErr   error
	gotPath    string
	gotChannel string
	gotUserID  string
}

func (m *mockUseCase) Enabled() bool      { return m.enabled }
func (m *mockUseCase) Examples() []string { return []string{"Что просрочено?"} }
func (m *mockUseCase) Status(context.Context) (bool, string) {
	return true, "openrouter"
}
func (m *mockUseCase) Timeout() time.Duration { return 25 * time.Second }

func (m *mockUseCase) Proxy(_ context.Context, userID, channel, _, _, path string) (*http.Response, func(), error) {
	m.gotPath = path
	m.gotChannel = channel
	m.gotUserID = userID
	if m.proxyErr != nil {
		return nil, nil, m.proxyErr
	}
	code := m.proxyCode
	if code == 0 {
		code = http.StatusOK
	}
	resp := &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(m.proxyBody)),
		Header:     make(http.Header),
	}
	return resp, func() { _ = resp.Body.Close() }, nil
}

// withClaims кладёт клеймы, как это делает защищённая группа. Request-id пуст —
// хендлер корректно работает без него (api.RequestID вернёт "").
func withClaims(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, &model.Claims{UserID: "u1", Role: "user"})
	return r.WithContext(ctx)
}

func doAsk(t *testing.T, h *AssistantHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/ask", strings.NewReader(body))
	req = withClaims(req)
	rec := httptest.NewRecorder()
	h.Ask()(rec, req)
	return rec
}

func TestHandlerAsk_Disabled503(t *testing.T) {
	h := NewAssistantHandler(&mockUseCase{enabled: false}, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"что просрочено?"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "assistant_disabled") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandlerStatus_Disabled(t *testing.T) {
	h := NewAssistantHandler(&mockUseCase{enabled: false}, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assistant/status", nil)
	req = withClaims(req)
	rec := httptest.NewRecorder()
	h.Status()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	var out struct {
		Enabled  bool     `json:"enabled"`
		Healthy  bool     `json:"healthy"`
		Examples []string `json:"examples"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Enabled || out.Healthy {
		t.Fatalf("disabled status should be enabled=false healthy=false: %+v", out)
	}
	if len(out.Examples) == 0 {
		t.Fatalf("examples should be present even when disabled")
	}
}

func TestHandlerAsk_BadQuestionTooLong(t *testing.T) {
	h := NewAssistantHandler(&mockUseCase{enabled: true}, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	long := strings.Repeat("я", 501)
	rec := doAsk(t, h, `{"question":"`+long+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bad_request") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandlerAsk_BadChannelBot(t *testing.T) {
	h := NewAssistantHandler(&mockUseCase{enabled: true}, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"ок","channel":"bot"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (bot channel forbidden over HTTP)", rec.Code)
	}
}

func TestHandlerAsk_RateLimited429(t *testing.T) {
	m := &mockUseCase{enabled: true, proxyErr: &service.RateLimitedError{RetryAfter: 7 * time.Second}}
	h := NewAssistantHandler(m, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"ок"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra != "7" {
		t.Fatalf("Retry-After = %q, want 7", ra)
	}
	if !strings.Contains(rec.Body.String(), "assistant_rate_limited") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandlerAsk_Timeout504(t *testing.T) {
	m := &mockUseCase{enabled: true, proxyErr: service.ErrAssistantTimeout}
	h := NewAssistantHandler(m, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"ок"}`)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("code = %d, want 504", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "assistant_timeout") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandlerAsk_Busy503(t *testing.T) {
	m := &mockUseCase{enabled: true, proxyErr: service.ErrAssistantBusy}
	h := NewAssistantHandler(m, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"ок"}`)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "assistant_busy") {
		t.Fatalf("code = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerAsk_PassThroughEnvelope(t *testing.T) {
	envelope := `{"answer_id":"rid","status":"ok","mode":"llm","text":"ок","blocks":[],"followups":[],"notes":[],"meta":{"model":"x"}}`
	m := &mockUseCase{enabled: true, proxyBody: envelope, proxyCode: http.StatusOK}
	h := NewAssistantHandler(m, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	rec := doAsk(t, h, `{"question":"что просрочено?","channel":"web"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != envelope {
		t.Fatalf("envelope not proxied byte-for-byte:\n got %s\nwant %s", rec.Body.String(), envelope)
	}
	if m.gotPath != "/v1/ask" || m.gotChannel != "web" || m.gotUserID != "u1" {
		t.Fatalf("proxy args: path=%s channel=%s user=%s", m.gotPath, m.gotChannel, m.gotUserID)
	}
}

func TestHandlerAskStream_UsesStreamPath(t *testing.T) {
	m := &mockUseCase{enabled: true, proxyBody: "event: done\ndata: {}\n\n", proxyCode: http.StatusOK}
	h := NewAssistantHandler(m, response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assistant/ask/stream", strings.NewReader(`{"question":"ок"}`))
	req = withClaims(req)
	rec := httptest.NewRecorder()
	h.AskStream()(rec, req)
	if m.gotPath != "/v1/ask/stream" {
		t.Fatalf("path = %s, want /v1/ask/stream", m.gotPath)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("X-Accel-Buffering must be no")
	}
	if !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("stream body = %q", rec.Body.String())
	}
}
