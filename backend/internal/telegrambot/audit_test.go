package telegrambot

import (
	"context"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// botRecSpy — фейковый рекордер журнала для бота.
type botRecSpy struct{ entries []audit.Entry }

func (r *botRecSpy) Record(_ context.Context, e audit.Entry) { r.entries = append(r.entries, e) }

func (r *botRecSpy) find(action string) (audit.Entry, bool) {
	for _, e := range r.entries {
		if e.Action == action {
			return e, true
		}
	}
	return audit.Entry{}, false
}

func TestAuditBotUnlink(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", Role: "user", Login: "joe"}}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)
	rec := &botRecSpy{}
	svc.SetAuditRecorder(rec)

	repo.UpsertLink(context.Background(), "u1", 100, "joe")

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: nextUpd(),
		Message:  &telegram.Message{From: &telegram.User{ID: 100}, Chat: telegram.Chat{ID: 100, Type: "private"}, Text: "/unlink"},
	})

	e, ok := rec.find(audit.ActionTelegramUnlink)
	if !ok {
		t.Fatalf("telegram.unlink not recorded: %+v", rec.entries)
	}
	if e.EntityID != "u1" {
		t.Errorf("entity id = %q, want u1", e.EntityID)
	}
}

func TestAuditBotBind(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{"admin1": {ID: "admin1", Role: "admin", Login: "boss"}}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)
	rec := &botRecSpy{}
	svc.SetAuditRecorder(rec)

	repo.UpsertLink(context.Background(), "admin1", 100, "boss")

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: nextUpd(),
		Message:  &telegram.Message{From: &telegram.User{ID: 100}, Chat: telegram.Chat{ID: -100, Type: "supergroup", Title: "Fin"}, Text: "/bind"},
	})

	e, ok := rec.find(audit.ActionTelegramTeamChatBind)
	if !ok {
		t.Fatalf("telegram.team_chat_bind not recorded: %+v", rec.entries)
	}
	if e.EntityID != "-100" {
		t.Errorf("entity id = %q, want -100", e.EntityID)
	}
	if e.Payload["title"] != "Fin" {
		t.Errorf("payload title wrong: %+v", e.Payload)
	}
}

func TestOccurrenceIDFromHex(t *testing.T) {
	// Невалидный hex возвращается как есть (запись не теряется).
	if got := occurrenceIDFromHex("zzzz"); got != "zzzz" {
		t.Errorf("bad hex should pass through, got %q", got)
	}
}
