package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
)

// mockHistory — подмена сервиса истории.
type mockHistory struct {
	turns    []model.AssistantTurn
	total    int
	gotUser  string
	gotConv  string
	gotLimit int

	clearUser string
	clearCh   string
	clearConv string
	clearAll  bool
}

func (m *mockHistory) History(_ context.Context, userID, conversation string, limit int) ([]model.AssistantTurn, int, error) {
	m.gotUser, m.gotConv, m.gotLimit = userID, conversation, limit
	return m.turns, m.total, nil
}

func (m *mockHistory) Clear(_ context.Context, userID, channel, conversation string, all bool) (int, error) {
	m.clearUser, m.clearCh, m.clearConv, m.clearAll = userID, channel, conversation, all
	return len(m.turns), nil
}

func memInfo() response.AssistantMemoryInfo {
	return response.AssistantMemoryInfo{Enabled: true, ContextTurns: 4, TTLSeconds: 7200}
}

func TestHistoryGet_OK(t *testing.T) {
	m := &mockHistory{
		total: 2,
		turns: []model.AssistantTurn{
			{ID: "t1", Question: "q1", AnswerText: "a1", Status: "ok", Mode: "llm", CreatedAt: time.Unix(1700000000, 0), InContext: true},
		},
	}
	h := NewAssistantHistoryHandler(m, memInfo())
	req := withClaims(httptest.NewRequest(http.MethodGet, "/api/v1/assistant/history?channel=miniapp", nil))
	rec := httptest.NewRecorder()
	h.Get()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	var out response.AssistantHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Channel != "miniapp" || out.Conversation != "miniapp" || out.Total != 2 {
		t.Fatalf("envelope = %+v", out)
	}
	if len(out.Items) != 1 || out.Items[0].ID != "t1" || !out.Items[0].InContext {
		t.Fatalf("items = %+v", out.Items)
	}
	if !out.Memory.Enabled || out.Memory.ContextTurns != 4 {
		t.Fatalf("memory = %+v", out.Memory)
	}
	if m.gotUser != "u1" || m.gotConv != "miniapp" || m.gotLimit != 20 {
		t.Fatalf("service args: user=%q conv=%q limit=%d", m.gotUser, m.gotConv, m.gotLimit)
	}
}

func TestHistoryGet_BadParams(t *testing.T) {
	cases := []struct{ query, want string }{
		{"channel=all", "channel must be web, miniapp or bot"},
		{"channel=web&conversation=tg:dm", "conversation is invalid"},
		{"limit=99", "limit must be between 1 and 50"},
	}
	for _, c := range cases {
		h := NewAssistantHistoryHandler(&mockHistory{}, memInfo())
		req := withClaims(httptest.NewRequest(http.MethodGet, "/api/v1/assistant/history?"+c.query, nil))
		rec := httptest.NewRecorder()
		h.Get()(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: code = %d", c.query, rec.Code)
		}
		var out response.ErrorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Error.Message != c.want {
			t.Fatalf("%s: message = %q, want %q", c.query, out.Error.Message, c.want)
		}
	}
}

func TestHistoryDelete_All(t *testing.T) {
	m := &mockHistory{}
	h := NewAssistantHistoryHandler(m, memInfo())
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/api/v1/assistant/history?channel=all", nil))
	rec := httptest.NewRecorder()
	h.Delete()(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if !m.clearAll || m.clearUser != "u1" || m.clearCh != "all" {
		t.Fatalf("clear args: all=%v user=%q ch=%q", m.clearAll, m.clearUser, m.clearCh)
	}
}

func TestHistoryDelete_Conversation(t *testing.T) {
	m := &mockHistory{}
	h := NewAssistantHistoryHandler(m, memInfo())
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/api/v1/assistant/history?channel=miniapp", nil))
	rec := httptest.NewRecorder()
	h.Delete()(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d", rec.Code)
	}
	if m.clearAll || m.clearConv != "miniapp" {
		t.Fatalf("clear args: all=%v conv=%q", m.clearAll, m.clearConv)
	}
}
