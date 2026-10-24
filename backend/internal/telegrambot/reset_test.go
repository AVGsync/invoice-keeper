package telegrambot

import (
	"context"
	"strings"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
)

// resetLast возвращает последний вызов ResetConversation.
func (f *fakeNL) resetLast() (resetCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.resets) == 0 {
		return resetCall{}, false
	}
	return f.resets[len(f.resets)-1], true
}

func TestReset_PrivateLinked_ClearsDM(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, _ := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/new"))

	call, ok := nl.resetLast()
	if !ok {
		t.Fatal("ResetConversation was not called")
	}
	if call.userID != "u1" || call.channel != "bot" || call.conversation != "tg:dm" {
		t.Fatalf("reset call = %+v", call)
	}
	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "новый диалог") {
		t.Fatalf("reply = %q", sent.Text)
	}
}

func TestReset_SynonymReset(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, _, _ := newNLService(t, nl)
	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/reset"))
	if _, ok := nl.resetLast(); !ok {
		t.Fatal("/reset must also reset")
	}
}

func TestReset_GroupTeamChat_ClearsChatConversation(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, repo, _, _ := newNLService(t, nl)
	if _, _, err := repo.BindChat(context.Background(), -200, "supergroup", "Команда", "u1"); err != nil {
		t.Fatalf("BindChat: %v", err)
	}

	upd := telegram.Update{
		UpdateID: nextUpd(),
		Message: &telegram.Message{
			MessageID: nextUpd(),
			From:      &telegram.User{ID: 555},
			Chat:      telegram.Chat{ID: -200, Type: "supergroup"},
			Text:      "/new",
		},
	}
	svc.HandleUpdate(context.Background(), upd)

	call, ok := nl.resetLast()
	if !ok {
		t.Fatal("ResetConversation was not called in team chat")
	}
	if call.conversation != "tg:-200" {
		t.Fatalf("conversation = %q, want tg:-200", call.conversation)
	}
}

func TestReset_Unlinked_Hint(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, _ := newNLService(t, nl)
	svc.HandleUpdate(context.Background(), privateAsk(999, 100, "/new"))
	if _, ok := nl.resetLast(); ok {
		t.Fatal("unlinked user must not reset")
	}
	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "Привяжите Telegram") {
		t.Fatalf("reply = %q", sent.Text)
	}
}
