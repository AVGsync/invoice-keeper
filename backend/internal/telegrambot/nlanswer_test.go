package telegrambot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
)

// fakeNL — подмена порта NLAnswerer: пишет вызовы и отдаёт заданный ответ/ошибку.
type fakeNL struct {
	mu     sync.Mutex
	calls  []nlCall
	answer model.AssistantAnswer
	err    error
	delay  time.Duration
}

type nlCall struct {
	userID, channel, question string
}

func (f *fakeNL) Ask(ctx context.Context, userID, channel, question string) (model.AssistantAnswer, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return model.AssistantAnswer{}, ctx.Err()
		}
	}
	f.mu.Lock()
	f.calls = append(f.calls, nlCall{userID, channel, question})
	f.mu.Unlock()
	return f.answer, f.err
}

func (f *fakeNL) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeNL) last() (nlCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nlCall{}, false
	}
	return f.calls[len(f.calls)-1], true
}

// nlAnswer — типичный ответ с текстом и блоком начислений.
func nlAnswer() model.AssistantAnswer {
	return model.AssistantAnswer{
		Text: "Просрочено 1 платёж: Хостинг AWS.",
		Blocks: []map[string]any{{
			"type":  "occurrence_list",
			"title": "Просроченные платежи",
			"total": float64(1),
			"items": []any{map[string]any{
				"title":    "Хостинг AWS",
				"display":  "120,00 $",
				"due_date": "20.10.2026",
			}},
		}},
		Followups: []string{"Кто отвечает за просроченные?"},
		Notes:     []string{"другие валюты не учитывались"},
	}
}

// newNLService собирает сервис с привязанным пользователем u1 (tg 555) и
// подключённым fakeNL. Возвращает wg для ожидания фоновой горутины ответа.
func newNLService(t *testing.T, nl *fakeNL) (*Service, *memRepo, *telegram.FakeClient, *sync.WaitGroup) {
	t.Helper()
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", Role: "user", FullName: "Иван Петров"}}}
	if err := repo.UpsertLink(context.Background(), "u1", 555, "ivan"); err != nil {
		t.Fatalf("UpsertLink: %v", err)
	}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)
	svc.AttachNLAnswerer(nl)
	var wg sync.WaitGroup
	svc.StartNL(context.Background(), &wg)
	return svc, repo, fake, &wg
}

func privateAsk(tgID, chatID int64, text string) telegram.Update {
	return telegram.Update{
		UpdateID: nextUpd(),
		Message: &telegram.Message{
			MessageID: nextUpd(),
			From:      &telegram.User{ID: tgID},
			Chat:      telegram.Chat{ID: chatID, Type: "private"},
			Text:      text,
		},
	}
}

func TestAsk_PrivateLinked_CallsAgentAndReplies(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask что просрочено?"))
	wg.Wait()

	call, ok := nl.last()
	if !ok {
		t.Fatal("agent was not called")
	}
	if call.userID != "u1" || call.channel != "bot" || call.question != "что просрочено?" {
		t.Fatalf("agent called with %+v", call)
	}
	sent, ok := fake.LastSent()
	if !ok {
		t.Fatal("no message sent")
	}
	if !strings.Contains(sent.Text, "Хостинг AWS") || !strings.Contains(sent.Text, "120,00 $") {
		t.Fatalf("reply missing block data: %q", sent.Text)
	}
	if !strings.Contains(sent.Text, "Можно спросить") {
		t.Fatalf("reply missing followups: %q", sent.Text)
	}
	if sent.ParseMode != "HTML" || !sent.DisableWebPagePreview {
		t.Fatalf("reply not HTML/preview-disabled: %+v", sent)
	}
	if len(fake.Actions) == 0 || fake.Actions[0].Action != "typing" {
		t.Fatalf("typing action not sent: %+v", fake.Actions)
	}
}

func TestFreeText_PrivateLinked_CallsAgent(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, _, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "кто отвечает за аренду?"))
	wg.Wait()

	if nl.callCount() != 1 {
		t.Fatalf("expected 1 agent call, got %d", nl.callCount())
	}
	call, _ := nl.last()
	if call.question != "кто отвечает за аренду?" {
		t.Fatalf("question = %q", call.question)
	}
}

func TestAsk_Unlinked_HintNoCall(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, wg := newNLService(t, nl)

	// tg 999 не привязан.
	svc.HandleUpdate(context.Background(), privateAsk(999, 100, "/ask что просрочено?"))
	wg.Wait()

	if nl.callCount() != 0 {
		t.Fatalf("agent must not be called for unlinked user")
	}
	sent, ok := fake.LastSent()
	if !ok || !strings.Contains(sent.Text, "Привяжите Telegram") {
		t.Fatalf("expected link hint, got %q", sent.Text)
	}
}

func TestAsk_FreeTextUnlinked_Hint(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(999, 100, "сколько платим в долларах?"))
	wg.Wait()

	if nl.callCount() != 0 {
		t.Fatalf("agent must not be called for unlinked free text")
	}
	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "Привяжите Telegram") {
		t.Fatalf("expected link hint, got %q", sent.Text)
	}
}

func TestAsk_GroupNonTeam_Hint(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, wg := newNLService(t, nl)

	upd := telegram.Update{
		UpdateID: nextUpd(),
		Message: &telegram.Message{
			MessageID: nextUpd(),
			From:      &telegram.User{ID: 555},
			Chat:      telegram.Chat{ID: -200, Type: "supergroup"},
			Text:      "/ask что просрочено?",
		},
	}
	svc.HandleUpdate(context.Background(), upd)
	wg.Wait()

	if nl.callCount() != 0 {
		t.Fatalf("agent must not be called in a non-team group")
	}
	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "командном чате") {
		t.Fatalf("expected team-chat hint, got %q", sent.Text)
	}
}

func TestAsk_GroupTeamChat_CallsAgent(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, repo, _, wg := newNLService(t, nl)
	// Делаем чат -200 активным командным.
	if _, _, err := repo.BindChat(context.Background(), -200, "supergroup", "Команда", "u1"); err != nil {
		t.Fatalf("BindChat: %v", err)
	}

	upd := telegram.Update{
		UpdateID: nextUpd(),
		Message: &telegram.Message{
			MessageID: nextUpd(),
			From:      &telegram.User{ID: 555},
			Chat:      telegram.Chat{ID: -200, Type: "supergroup"},
			Text:      "/ask что просрочено?",
		},
	}
	svc.HandleUpdate(context.Background(), upd)
	wg.Wait()

	if nl.callCount() != 1 {
		t.Fatalf("agent must be called in the team chat, calls=%d", nl.callCount())
	}
}

func TestAsk_EmptyQuestion_Usage(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, fake, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask"))
	wg.Wait()

	if nl.callCount() != 0 {
		t.Fatalf("empty /ask must not call the agent")
	}
	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "Напишите вопрос") {
		t.Fatalf("expected usage hint, got %q", sent.Text)
	}
}

func TestAsk_AgentError_ShortMessage(t *testing.T) {
	nl := &fakeNL{err: service.ErrAssistantUnavailable}
	svc, _, fake, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask что просрочено?"))
	wg.Wait()

	sent, ok := fake.LastSent()
	if !ok || !strings.Contains(sent.Text, "недоступен") {
		t.Fatalf("expected unavailable message, got %q", sent.Text)
	}
}

func TestAsk_TimeoutError_Message(t *testing.T) {
	nl := &fakeNL{err: service.ErrAssistantTimeout}
	svc, _, fake, wg := newNLService(t, nl)

	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask что просрочено?"))
	wg.Wait()

	sent, _ := fake.LastSent()
	if !strings.Contains(sent.Text, "Долго думаю") {
		t.Fatalf("expected timeout message, got %q", sent.Text)
	}
}

func TestAsk_Disabled_NoNLAnswerer(t *testing.T) {
	repo := newMemRepo()
	_ = repo.UpsertLink(context.Background(), "u1", 555, "ivan")
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", Role: "user"}}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake) // без AttachNLAnswerer

	// Свободный текст в личке при выключенной фиче — молчим.
	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "что просрочено?"))
	if _, ok := fake.LastSent(); ok {
		t.Fatalf("disabled assistant must stay silent on free text")
	}
	// /ask в личке — вежливый отказ.
	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask что просрочено?"))
	sent, ok := fake.LastSent()
	if !ok || !strings.Contains(sent.Text, "недоступен") {
		t.Fatalf("expected unavailable reply, got %q / ok=%v", sent.Text, ok)
	}
}

func TestAsk_SingleflightBusy(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer(), delay: 150 * time.Millisecond}
	svc, _, fake, wg := newNLService(t, nl)

	// Первый вопрос занимает слот; второй приходит, пока первый в работе.
	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask первый вопрос"))
	svc.HandleUpdate(context.Background(), privateAsk(555, 100, "/ask второй вопрос"))
	wg.Wait()

	// Один из двух получил «ещё думаю».
	busy := false
	for _, m := range fake.Sent {
		if strings.Contains(m.Text, "Ещё думаю") {
			busy = true
		}
	}
	if !busy {
		t.Fatalf("expected a singleflight busy reply; sent=%d", len(fake.Sent))
	}
	if nl.callCount() != 1 {
		t.Fatalf("only one question should reach the agent, got %d", nl.callCount())
	}
}

// TestHelpText_NLEnabled_NoRawAngleBrackets проверяет, что при подключённом
// ассистенте справка не содержит сырых '<'/'>' (их отверг бы Telegram с
// ParseMode HTML: "Unsupported start tag"), а плейсхолдер экранирован.
func TestHelpText_NLEnabled_NoRawAngleBrackets(t *testing.T) {
	nl := &fakeNL{answer: nlAnswer()}
	svc, _, _, wg := newNLService(t, nl)
	defer wg.Wait()

	help := svc.helpText()
	if svc.nl == nil {
		t.Fatal("precondition: NL answerer must be attached")
	}
	if strings.ContainsRune(help, '<') || strings.ContainsRune(help, '>') {
		t.Fatalf("helpText must not contain raw angle brackets under ParseMode HTML: %q", help)
	}
	if !strings.Contains(help, "/ask &lt;вопрос&gt;") {
		t.Fatalf("expected escaped /ask placeholder in helpText: %q", help)
	}
}
