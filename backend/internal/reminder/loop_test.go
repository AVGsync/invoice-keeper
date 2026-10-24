package reminder

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

const (
	testOccID    = "11111111-1111-1111-1111-11111111aaaa"
	testAssignee = "22222222-2222-2222-2222-2222222222aa"
	testAdmin    = "22222222-2222-2222-2222-2222222222bb"
	testOther    = "22222222-2222-2222-2222-2222222222cc"
	teamChatID   = int64(-1001234567890)
)

// memChats — активный командный чат для тестов движка.
type memChats struct {
	mu     sync.Mutex
	chatID int64
	active bool
}

func (c *memChats) GetActiveChat(ctx context.Context) (model.TelegramChat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.active {
		return model.TelegramChat{}, model.ErrTelegramNoTeamChat
	}
	return model.TelegramChat{TGChatID: c.chatID, IsActive: true}, nil
}

func (c *memChats) DeactivateChat(ctx context.Context, tgChatID int64, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	return nil
}

func (c *memChats) UpdateChatID(ctx context.Context, oldID, newID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chatID = newID
	return nil
}

func (c *memChats) CleanupStale(ctx context.Context, updatesBefore, codesBefore time.Time) error {
	return nil
}

type harness struct {
	clk   clock.Fixed
	store *MemoryStore
	notif *memNotif
	chats *memChats
	fake  *telegram.FakeClient
	loop  *Loop
	bot   *BotService
	today clock.Date
}

func newHarness(t *testing.T, occs []model.ReminderOccurrence) *harness {
	t.Helper()
	tm := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(tm, time.UTC)
	store := NewMemoryStore(occs, StaticAdmins(testAdmin))
	notif := newMemNotif()
	chats := &memChats{chatID: teamChatID, active: true}
	fake := telegram.NewFakeClient()
	cfg := Config{LeadDays: 3, Tick: time.Second, PerChatGap: -1, GlobalGap: -1, BotUsername: "ik_dev_bot"}
	loop := NewLoop(store, notif, chats, fake, clk, nil, cfg)
	bot := NewBotService(store, store, notif, fake, clk, cfg)
	return &harness{clk: clk, store: store, notif: notif, chats: chats, fake: fake, loop: loop, bot: bot, today: clk.Today()}
}

func occ(id string, due clock.Date) model.ReminderOccurrence {
	a := testAssignee
	return model.ReminderOccurrence{
		ID: id, RecurringPaymentID: "rp-1", Title: "Аренда", Amount: "1500.00",
		Currency: "RUB", DueDate: due, AssigneeID: &a, AssigneeName: "Иван Петров",
	}
}

// due_soon уходит один раз; два тика подряд дают одну отправку (§9.2).
func TestLoop_DueSoonSentOnceAcrossTicks(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})

	h.loop.Tick(context.Background())
	h.loop.Tick(context.Background())

	if got := len(h.fake.Sent); got != 1 {
		t.Fatalf("ожидалась ровно одна отправка, got %d", got)
	}
	if h.notif.count(model.NotificationStateSent) != 1 {
		t.Errorf("ожидалась одна строка в состоянии sent")
	}
	if !strings.Contains(h.fake.Sent[0].Text, "Напоминание об оплате") {
		t.Errorf("не тот текст напоминания: %q", h.fake.Sent[0].Text)
	}
	// Кнопка «Оплачено» с корректным callback_data p:<hex>.
	mk := h.fake.Sent[0].ReplyMarkup
	if mk == nil || len(mk.InlineKeyboard) == 0 || mk.InlineKeyboard[0][0].CallbackData != "p:"+uuidToHex(testOccID) {
		t.Errorf("нет корректной кнопки «Оплачено»: %+v", mk)
	}
}

// Оплата после отправки: reconcile правит сообщение на «Оплачено» (§5.6).
func TestLoop_PaidAfterSendEditsMessage(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	ctx := context.Background()

	h.loop.Tick(ctx) // отправили
	if len(h.fake.Sent) != 1 {
		t.Fatalf("ожидалась отправка на первом тике")
	}
	if err := h.store.MarkPaid(ctx, testOccID, h.today, testAdmin); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}

	h.loop.Tick(ctx) // reconcile должен отредактировать

	if len(h.fake.Edited) != 1 {
		t.Fatalf("ожидалась одна правка сообщения, got %d", len(h.fake.Edited))
	}
	if !strings.Contains(h.fake.Edited[0].Text, "Оплачено") {
		t.Errorf("правка не про оплату: %q", h.fake.Edited[0].Text)
	}
	for _, r := range h.notif.all() {
		if r.ShownStatus != model.ShownStatusPaid {
			t.Errorf("shown_status не paid: %s", r.ShownStatus)
		}
	}
}

// Напоминание о просрочке на следующий день после срока (§5.7, решение №3).
func TestLoop_OverdueSent(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(-1))})

	h.loop.Tick(context.Background())

	if len(h.fake.Sent) != 1 {
		t.Fatalf("ожидалось одно overdue-сообщение, got %d", len(h.fake.Sent))
	}
	if !strings.Contains(h.fake.Sent[0].Text, "Просрочен платёж") {
		t.Errorf("не тот текст просрочки: %q", h.fake.Sent[0].Text)
	}
	for _, r := range h.notif.all() {
		if r.Kind != model.NotificationKindOverdue {
			t.Errorf("ожидался kind=overdue, got %s", r.Kind)
		}
	}
}

// Старую просрочку (более 3 дней) не шлём (§5.7).
func TestLoop_OldOverdueNotSent(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(-6))})

	h.loop.Tick(context.Background())

	if len(h.fake.Sent) != 0 {
		t.Fatalf("старую просрочку слать не должны, got %d", len(h.fake.Sent))
	}
}

// Завершённый платёж: новых отправок нет (§1.5).
func TestLoop_EndedNotSent(t *testing.T) {
	o := occ(testOccID, h2today(t).AddDays(2))
	o.Ended = true
	h := newHarness(t, []model.ReminderOccurrence{o})

	h.loop.Tick(context.Background())

	if len(h.fake.Sent) != 0 {
		t.Fatalf("по завершённому платежу слать не должны, got %d", len(h.fake.Sent))
	}
}

// Нет командного чата: ничего не ставим и не шлём (решение №1 — только команда).
func TestLoop_NoTeamChatNoSend(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	h.chats.active = false

	h.loop.Tick(context.Background())

	if len(h.fake.Sent) != 0 {
		t.Fatalf("без командного чата слать некому, got %d", len(h.fake.Sent))
	}
}

// Кнопка «Оплачено»: ответственный — успех и правка; чужой — отказ алертом (§6.3).
func TestBot_MarkPaidCallback(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	ctx := context.Background()
	h.loop.Tick(ctx) // есть отправленное сообщение, которое будем править

	occHex := uuidToHex(testOccID)

	// Чужой пользователь — отказ, alert, без правок.
	text, alert := h.bot.MarkPaidCallback(ctx, occHex, testOther, "Чужой")
	if !alert || !strings.Contains(text, "ответственный или администратор") {
		t.Errorf("ожидался отказ чужому: %q alert=%v", text, alert)
	}
	if len(h.fake.Edited) != 0 {
		t.Errorf("при отказе правок быть не должно")
	}

	// Ответственный — успех и правка сообщения.
	text, alert = h.bot.MarkPaidCallback(ctx, occHex, testAssignee, "Иван Петров")
	if alert || text != "Отмечено" {
		t.Errorf("ожидался успех ответственному: %q alert=%v", text, alert)
	}
	if len(h.fake.Edited) != 1 || !strings.Contains(h.fake.Edited[0].Text, "Оплачено") {
		t.Errorf("ожидалась правка в «Оплачено», edited=%+v", h.fake.Edited)
	}
}

// h2today возвращает фиксированную «сегодня» тестов (2026-10-24).
func h2today(t *testing.T) clock.Date {
	t.Helper()
	return clock.Date{Y: 2026, M: 10, D: 24}
}
