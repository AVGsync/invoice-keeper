package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

func okEnvelopeWithMemory() []byte {
	b, _ := json.Marshal(map[string]any{
		"answer_id": "rid1", "status": "ok", "mode": "llm", "text": "ок",
		"blocks": []any{}, "followups": []string{}, "notes": []string{},
		"memory": map[string]any{"v": 1, "tools": []any{}, "facts": "f", "numbers": []string{"1"}},
	})
	return b
}

// agentCapture поднимает фейковый агент, записывает тело запроса и отдаёт конверт.
func agentCapture(t *testing.T, gotBody *[]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*gotBody = b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(okEnvelopeWithMemory())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func withMemory(t *testing.T, svc *AssistantService, repo *fakeHistoryRepo, enabled bool) (*AssistantHistoryService, *recordingAudit) {
	cfg := MemoryConfig{Enabled: enabled, Turns: 4, TTL: 2 * time.Hour, MaxTurns: 20, Keep: 100}
	hs := NewAssistantHistoryService(repo, cfg, clock.New(time.UTC))
	aud := &recordingAudit{}
	hs.SetAuditRecorder(aud)
	svc.SetHistory(hs)
	svc.SetAuditRecorder(aud)
	return hs, aud
}

func TestDispatch_SendsHistoryAndRecords(t *testing.T) {
	var gotBody []byte
	srv := agentCapture(t, &gotBody)
	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true})

	repo := &fakeHistoryRepo{turns: []model.AssistantTurn{
		mkTurn("t2", 5, "ok"),
		mkTurn("t1", 15, "ok"),
	}}
	_, aud := withMemory(t, svc, repo, true)

	if _, err := svc.Ask(context.Background(), "u1", "bot", "а в долларах?"); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var sent agentRequest
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("agent body: %v (%s)", err, gotBody)
	}
	if len(sent.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(sent.History))
	}
	for _, h := range sent.History {
		if !h.InPrompt {
			t.Fatalf("recent ok turns within ttl must be in_prompt: %+v", h)
		}
	}

	if len(repo.appended) != 1 {
		t.Fatalf("appended = %d, want 1", len(repo.appended))
	}
	if repo.appended[0].Question != "а в долларах?" || repo.appended[0].Status != "ok" {
		t.Fatalf("appended turn = %+v", repo.appended[0])
	}
	if string(repo.appended[0].Memory) == "" {
		t.Fatal("memory object must be stored")
	}

	var askRecorded bool
	for _, e := range aud.entries {
		if e.Action == audit.ActionAssistantAsk && e.Outcome == audit.OutcomeSuccess {
			askRecorded = true
		}
	}
	if !askRecorded {
		t.Fatalf("assistant.ask audit not recorded: %+v", aud.entries)
	}
}

func TestDispatch_MemoryDisabled_NoHistory(t *testing.T) {
	var gotBody []byte
	srv := agentCapture(t, &gotBody)
	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true})

	repo := &fakeHistoryRepo{turns: []model.AssistantTurn{mkTurn("t1", 1, "ok")}}
	withMemory(t, svc, repo, false)

	if _, err := svc.Ask(context.Background(), "u1", "bot", "что просрочено?"); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var sent agentRequest
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.History) != 0 {
		t.Fatalf("history must be empty when memory disabled, got %d", len(sent.History))
	}
	if len(repo.appended) != 0 {
		t.Fatal("no turn must be appended when memory disabled")
	}
}
