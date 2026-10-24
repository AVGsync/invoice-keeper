package service

import (
	"context"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// --- фейковое хранилище истории ---

type fakeHistoryRepo struct {
	turns    []model.AssistantTurn
	appended []model.AssistantTurn
	pruned   []int // keep при каждом Prune
	deleted  int
	delConv  int
	delAll   int
}

func (f *fakeHistoryRepo) Append(_ context.Context, t model.AssistantTurn) (model.AssistantTurn, error) {
	f.appended = append(f.appended, t)
	t.ID = "id"
	t.CreatedAt = time.Now()
	return t, nil
}

func (f *fakeHistoryRepo) ListRecent(_ context.Context, _, _ string, limit int, statuses []string) ([]model.AssistantTurn, error) {
	// Фильтр по статусам и срез DESC (новые первыми): turns хранятся DESC.
	var out []model.AssistantTurn
	for _, t := range f.turns {
		if len(statuses) > 0 && !contains(statuses, t.Status) {
			continue
		}
		out = append(out, t)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeHistoryRepo) Count(context.Context, string, string) (int, error) {
	return len(f.turns), nil
}

func (f *fakeHistoryRepo) DeleteConversation(context.Context, string, string) (int, error) {
	f.delConv++
	return f.deleted, nil
}

func (f *fakeHistoryRepo) DeleteAllForUser(context.Context, string) (int, error) {
	f.delAll++
	return f.deleted, nil
}

func (f *fakeHistoryRepo) Prune(_ context.Context, _, _ string, keep int) error {
	f.pruned = append(f.pruned, keep)
	return nil
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

type recordingAudit struct{ entries []audit.Entry }

func (r *recordingAudit) Record(_ context.Context, e audit.Entry) { r.entries = append(r.entries, e) }

func mkTurn(id string, ageMin int, status string) model.AssistantTurn {
	return model.AssistantTurn{
		ID:        id,
		Status:    status,
		CreatedAt: time.Now().Add(-time.Duration(ageMin) * time.Minute),
	}
}

// --- selectContext ---

func TestSelectContext(t *testing.T) {
	now := time.Now()
	ttl := 2 * time.Hour

	t.Run("last N within ttl", func(t *testing.T) {
		turns := []model.AssistantTurn{
			{CreatedAt: now.Add(-50 * time.Minute)},
			{CreatedAt: now.Add(-40 * time.Minute)},
			{CreatedAt: now.Add(-30 * time.Minute)},
			{CreatedAt: now.Add(-20 * time.Minute)},
			{CreatedAt: now.Add(-10 * time.Minute)},
		}
		n := selectContext(turns, now, 3, ttl)
		if n != 3 {
			t.Fatalf("marked = %d, want 3", n)
		}
		if turns[0].InContext || turns[1].InContext {
			t.Fatal("older turns must not be in context")
		}
		if !turns[2].InContext || !turns[3].InContext || !turns[4].InContext {
			t.Fatal("last 3 must be in context")
		}
	})

	t.Run("ttl breaks the chain", func(t *testing.T) {
		turns := []model.AssistantTurn{
			{CreatedAt: now.Add(-10 * time.Minute)}, // старый, но внутри ttl — не помечается, т.к. цепочка с конца
			{CreatedAt: now.Add(-3 * time.Hour)},    // старше ttl: обрыв
			{CreatedAt: now.Add(-5 * time.Minute)},
		}
		n := selectContext(turns, now, 4, ttl)
		if n != 1 {
			t.Fatalf("marked = %d, want 1 (chain breaks at old turn)", n)
		}
		if !turns[2].InContext || turns[1].InContext || turns[0].InContext {
			t.Fatalf("only last turn should be in context: %+v", turns)
		}
	})

	t.Run("turnsN zero marks nothing", func(t *testing.T) {
		turns := []model.AssistantTurn{{CreatedAt: now}}
		if n := selectContext(turns, now, 0, ttl); n != 0 {
			t.Fatalf("marked = %d, want 0", n)
		}
	})
}

// --- History: in_context выставляется по свежим ok|partial ---

func TestHistory_InContext(t *testing.T) {
	loc := time.UTC
	// turns DESC: новые первыми.
	repo := &fakeHistoryRepo{turns: []model.AssistantTurn{
		mkTurn("t3", 5, "ok"),
		mkTurn("t2", 15, "ok"),
		mkTurn("t1", 25, "error"),
	}}
	svc := NewAssistantHistoryService(repo, MemoryConfig{Enabled: true, Turns: 4, TTL: 2 * time.Hour, MaxTurns: 20, Keep: 100}, clock.New(loc))

	items, total, err := svc.History(context.Background(), "u1", "web", 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	// items по возрастанию: t1(error), t2, t3.
	if items[0].ID != "t1" || items[2].ID != "t3" {
		t.Fatalf("items order unexpected: %+v", items)
	}
	if items[0].InContext {
		t.Fatal("error turn must not be in context")
	}
	if !items[1].InContext || !items[2].InContext {
		t.Fatal("ok turns within ttl must be in context")
	}
}

// --- Clear: channel=all vs conversation + аудит ---

func TestClear(t *testing.T) {
	repo := &fakeHistoryRepo{deleted: 7}
	aud := &recordingAudit{}
	svc := NewAssistantHistoryService(repo, MemoryConfig{Enabled: true, Turns: 4, TTL: time.Hour, MaxTurns: 20, Keep: 100}, clock.New(time.UTC))
	svc.SetAuditRecorder(aud)

	if _, err := svc.Clear(context.Background(), "u1", "web", "web", false); err != nil {
		t.Fatal(err)
	}
	if repo.delConv != 1 || repo.delAll != 0 {
		t.Fatalf("conversation delete expected: conv=%d all=%d", repo.delConv, repo.delAll)
	}

	if _, err := svc.Clear(context.Background(), "u1", "all", "", true); err != nil {
		t.Fatal(err)
	}
	if repo.delAll != 1 {
		t.Fatalf("all delete expected: %d", repo.delAll)
	}
	if len(aud.entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(aud.entries))
	}
	for _, e := range aud.entries {
		if e.Action != audit.ActionAssistantHistoryClear {
			t.Fatalf("audit action = %q", e.Action)
		}
	}
}

// --- ContextFor: при выключенной памяти — пусто ---

func TestContextFor_DisabledEmpty(t *testing.T) {
	repo := &fakeHistoryRepo{turns: []model.AssistantTurn{mkTurn("t1", 1, "ok")}}
	svc := NewAssistantHistoryService(repo, MemoryConfig{Enabled: false, Turns: 4, TTL: time.Hour, MaxTurns: 20, Keep: 100}, clock.New(time.UTC))
	turns, n, err := svc.ContextFor(context.Background(), "u1", "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 0 || n != 0 {
		t.Fatalf("expected empty context when disabled, got %d turns", len(turns))
	}
}

// --- Append пишет ход и prune с Keep ---

func TestAppend_Prune(t *testing.T) {
	repo := &fakeHistoryRepo{}
	svc := NewAssistantHistoryService(repo, MemoryConfig{Enabled: true, Turns: 4, TTL: time.Hour, MaxTurns: 20, Keep: 42}, clock.New(time.UTC))
	err := svc.Append(context.Background(), AppendRecord{UserID: "u1", Conversation: "web", Question: "q", Status: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.appended) != 1 {
		t.Fatalf("appended = %d", len(repo.appended))
	}
	if len(repo.pruned) != 1 || repo.pruned[0] != 42 {
		t.Fatalf("prune keep = %v, want [42]", repo.pruned)
	}
}
