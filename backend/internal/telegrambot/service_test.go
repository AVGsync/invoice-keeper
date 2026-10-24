package telegrambot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

// updSeq выдаёт уникальные update_id в тестах (дедуп по update_id).
var updSeq int64

func nextUpd() int64 { updSeq++; return updSeq }

// reminderMemStore — пустой источник периодов для тестов, где важна лишь доступность команд, а не их содержимое.
func reminderMemStore() *reminder.MemoryStore {
	return reminder.NewMemoryStore(nil, nil)
}

// newReminderBot собирает минимальный reminder.BotService на заглушке.
func newReminderBot(store *reminder.MemoryStore, client *telegram.FakeClient) *reminder.BotService {
	return reminder.NewBotService(store, store, nil, client, clock.New(time.UTC), reminder.Config{LeadDays: 3})
}

type memCode struct {
	id        string
	userID    string
	hash      string
	claimedBy *int64
	used      bool
	expires   time.Time
}

type memRepo struct {
	mu        sync.Mutex
	seq       int
	linksByTG map[int64]model.TelegramLink
	linksByU  map[string]model.TelegramLink
	codes     map[string]*memCode // по id
	chat      *model.TelegramChat
	updates   map[int64]bool

	upsertCalls int
}

func newMemRepo() *memRepo {
	return &memRepo{
		linksByTG: map[int64]model.TelegramLink{},
		linksByU:  map[string]model.TelegramLink{},
		codes:     map[string]*memCode{},
		updates:   map[int64]bool{},
	}
}

func (m *memRepo) GetLinkByUserID(ctx context.Context, userID string) (model.TelegramLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.linksByU[userID]; ok {
		return l, nil
	}
	return model.TelegramLink{}, model.ErrTelegramNotLinked
}

func (m *memRepo) GetLinkByTGUserID(ctx context.Context, tg int64) (model.TelegramLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.linksByTG[tg]; ok {
		return l, nil
	}
	return model.TelegramLink{}, model.ErrTelegramNotLinked
}

func (m *memRepo) UpsertLink(ctx context.Context, userID string, tg int64, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls++
	if l, ok := m.linksByTG[tg]; ok && l.UserID != userID {
		return model.ErrTelegramAlreadyLinked
	}
	l := model.TelegramLink{UserID: userID, TGUserID: tg, Username: username, DMEnabled: true, LinkedAt: time.Now()}
	m.linksByU[userID] = l
	m.linksByTG[tg] = l
	return nil
}

func (m *memRepo) DeleteLink(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.linksByU[userID]
	if !ok {
		return model.ErrTelegramNotLinked
	}
	delete(m.linksByU, userID)
	delete(m.linksByTG, l.TGUserID)
	return nil
}

func (m *memRepo) SetBotBlocked(ctx context.Context, tg int64, blocked bool) error { return nil }

func (m *memRepo) CountActiveCodes(ctx context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.codes {
		if c.userID == userID && !c.used && c.expires.After(time.Now()) {
			n++
		}
	}
	return n, nil
}

func (m *memRepo) CreateLinkCode(ctx context.Context, userID, hash string, exp time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	id := "code-" + string(rune('A'+m.seq))
	m.codes[id] = &memCode{id: id, userID: userID, hash: hash, expires: exp}
	return id, nil
}

func (m *memRepo) ClaimLinkCode(ctx context.Context, hash string, tg int64) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.codes {
		if c.hash == hash && !c.used && c.expires.After(time.Now()) && (c.claimedBy == nil || *c.claimedBy == tg) {
			c.claimedBy = &tg
			return c.id, c.userID, nil
		}
	}
	return "", "", model.ErrTelegramCodeInvalid
}

func (m *memRepo) ConfirmLinkCode(ctx context.Context, id string, tg int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.codes[id]
	if !ok || c.used || c.claimedBy == nil || *c.claimedBy != tg || !c.expires.After(time.Now()) {
		return "", model.ErrTelegramCodeInvalid
	}
	c.used = true
	return c.userID, nil
}

func (m *memRepo) CancelLinkCode(ctx context.Context, id string, tg int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.codes[id]; ok {
		c.used = true
	}
	return nil
}

func (m *memRepo) GetActiveChat(ctx context.Context) (model.TelegramChat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chat != nil && m.chat.IsActive {
		return *m.chat, nil
	}
	return model.TelegramChat{}, model.ErrTelegramNoTeamChat
}

func (m *memRepo) GetLastDeactivatedChat(ctx context.Context) (model.TelegramChat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chat != nil && !m.chat.IsActive {
		return *m.chat, nil
	}
	return model.TelegramChat{}, model.ErrTelegramNoTeamChat
}

func (m *memRepo) BindChat(ctx context.Context, tg int64, typ, title, boundBy string) (model.TelegramChat, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var prev model.TelegramChat
	hadPrev := false
	if m.chat != nil && m.chat.IsActive && m.chat.TGChatID != tg {
		prev = *m.chat
		hadPrev = true
	}
	t := title
	m.chat = &model.TelegramChat{TGChatID: tg, Type: typ, Title: &t, IsActive: true, BoundAt: time.Now()}
	return prev, hadPrev, nil
}

func (m *memRepo) DeactivateChat(ctx context.Context, tg int64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chat != nil && m.chat.IsActive && m.chat.TGChatID == tg {
		m.chat.IsActive = false
		return nil
	}
	return model.ErrTelegramNoTeamChat
}

func (m *memRepo) UpdateChatID(ctx context.Context, oldID, newID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chat != nil && m.chat.TGChatID == oldID {
		m.chat.TGChatID = newID
	}
	return nil
}

func (m *memRepo) SeedChat(ctx context.Context, tg int64, typ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chat == nil {
		m.chat = &model.TelegramChat{TGChatID: tg, Type: typ, IsActive: true, BoundAt: time.Now()}
	}
	return nil
}

func (m *memRepo) MarkUpdateSeen(ctx context.Context, id int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updates[id] {
		return false, nil
	}
	m.updates[id] = true
	return true, nil
}

func (m *memRepo) MaxUpdateID(ctx context.Context) (int64, error) { return 0, nil }

// memUsers — читатель пользователей.
type memUsers struct{ users map[string]model.User }

func (u memUsers) GetUserByID(id string, ctx context.Context) (model.User, error) {
	if usr, ok := u.users[id]; ok {
		return usr, nil
	}
	return model.User{}, errors.New("not found")
}

func (u memUsers) GetUserByLogin(login string, ctx context.Context) (model.User, error) {
	for _, usr := range u.users {
		if usr.Login == login {
			return usr, nil
		}
	}
	return model.User{}, errors.New("not found")
}

func newService(repo *memRepo, users memUsers, client BotAPI) *Service {
	return NewService(repo, users, client, nil, nil, Config{
		Enabled:     true,
		BotUsername: "ik_dev_bot",
		MiniAppURL:  "https://x.tld/tg",
		LinkCodeTTL: 10 * time.Minute,
	})
}

func TestCreateLinkCode(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo, memUsers{}, telegram.NewFakeClient())

	link, expires, err := svc.CreateLinkCode(context.Background(), "u1")
	if err != nil {
		t.Fatalf("CreateLinkCode: %v", err)
	}
	if expires != 600 {
		t.Errorf("expires = %d, want 600", expires)
	}
	if !strings.HasPrefix(link, "https://t.me/ik_dev_bot?start=link_") {
		t.Errorf("deep link = %q", link)
	}

	for i := 0; i < 2; i++ {
		if _, _, err := svc.CreateLinkCode(context.Background(), "u1"); err != nil {
			t.Fatalf("CreateLinkCode #%d: %v", i, err)
		}
	}
	if _, _, err := svc.CreateLinkCode(context.Background(), "u1"); !errors.Is(err, model.ErrTelegramTooManyCodes) {
		t.Fatalf("4th code err = %v, want ErrTelegramTooManyCodes", err)
	}
}

func TestLinkFlow_ClaimConfirm(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", FullName: "Иван Петров", Login: "ivan", Role: "user"}}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)

	link, _, err := svc.CreateLinkCode(context.Background(), "u1")
	if err != nil {
		t.Fatalf("CreateLinkCode: %v", err)
	}
	code := strings.TrimPrefix(link[strings.Index(link, "start=link_")+len("start=link_"):], "")

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			MessageID: 1,
			From:      &telegram.User{ID: 777, Username: "ivan"},
			Chat:      telegram.Chat{ID: 777, Type: "private"},
			Text:      "/start link_" + code,
		},
	})
	last, ok := fake.LastSent()
	if !ok || !strings.Contains(last.Text, "Иван Петров") {
		t.Fatalf("confirm message not sent, last=%q", last.Text)
	}
	if len(last.ReplyMarkup.InlineKeyboard) == 0 {
		t.Fatal("confirm keyboard missing")
	}
	confirmData := last.ReplyMarkup.InlineKeyboard[0][0].CallbackData
	if !strings.HasPrefix(confirmData, "lc:") {
		t.Fatalf("confirm button data = %q", confirmData)
	}

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 2,
		CallbackQuery: &telegram.CallbackQuery{
			ID:      "cb1",
			From:    telegram.User{ID: 777, Username: "ivan"},
			Message: &telegram.Message{MessageID: 1, Chat: telegram.Chat{ID: 777, Type: "private"}},
			Data:    confirmData,
		},
	})

	if repo.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", repo.upsertCalls)
	}
	if _, err := repo.GetLinkByTGUserID(context.Background(), 777); err != nil {
		t.Fatalf("link not created: %v", err)
	}
	if len(fake.Answers) == 0 {
		t.Error("answerCallbackQuery not called")
	}
	if len(fake.Edited) == 0 || !strings.Contains(fake.Edited[len(fake.Edited)-1].Text, "Готово") {
		t.Errorf("confirm message not edited to done")
	}
}

func TestStartLinkInGroupRejected(t *testing.T) {
	repo := newMemRepo()
	fake := telegram.NewFakeClient()
	svc := newService(repo, memUsers{}, fake)

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			From: &telegram.User{ID: 5}, Chat: telegram.Chat{ID: -100, Type: "group"},
			Text: "/start link_whatever",
		},
	})
	last, ok := fake.LastSent()
	if !ok || !strings.Contains(strings.ToLower(last.Text), "в личном чате") {
		t.Fatalf("expected private-chat hint, got %q", last.Text)
	}
}

func TestDedupUpdate(t *testing.T) {
	repo := newMemRepo()
	fake := telegram.NewFakeClient()
	svc := newService(repo, memUsers{}, fake)

	upd := telegram.Update{
		UpdateID: 42,
		Message:  &telegram.Message{From: &telegram.User{ID: 5}, Chat: telegram.Chat{ID: 5, Type: "private"}, Text: "/help"},
	}
	svc.HandleUpdate(context.Background(), upd)
	svc.HandleUpdate(context.Background(), upd) // повтор с тем же update_id

	if len(fake.Sent) != 1 {
		t.Fatalf("sent %d messages, want 1 (dedup by update_id)", len(fake.Sent))
	}
}

func TestBind_AdminOnly(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{
		"admin1": {ID: "admin1", Role: "admin", Login: "boss"},
		"user1":  {ID: "user1", Role: "user", Login: "joe"},
	}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)

	repo.UpsertLink(context.Background(), "admin1", 100, "boss")
	repo.UpsertLink(context.Background(), "user1", 200, "joe")

	// Не админ — отказ.
	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		Message:  &telegram.Message{From: &telegram.User{ID: 200}, Chat: telegram.Chat{ID: -100, Type: "supergroup", Title: "Fin"}, Text: "/bind"},
	})
	if _, err := repo.GetActiveChat(context.Background()); !errors.Is(err, model.ErrTelegramNoTeamChat) {
		t.Fatal("non-admin should not bind chat")
	}

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 2,
		Message:  &telegram.Message{From: &telegram.User{ID: 100}, Chat: telegram.Chat{ID: -100, Type: "supergroup", Title: "Fin"}, Text: "/bind"},
	})
	chat, err := repo.GetActiveChat(context.Background())
	if err != nil {
		t.Fatalf("admin bind failed: %v", err)
	}
	if chat.TGChatID != -100 {
		t.Errorf("bound chat id = %d, want -100", chat.TGChatID)
	}
}

func TestBind_AnonymousRejected(t *testing.T) {
	repo := newMemRepo()
	fake := telegram.NewFakeClient()
	svc := newService(repo, memUsers{}, fake)

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			SenderChat: &telegram.Chat{ID: -100, Type: "supergroup"},
			Chat:       telegram.Chat{ID: -100, Type: "supergroup"},
			Text:       "/bind",
		},
	})
	last, _ := fake.LastSent()
	if !strings.Contains(strings.ToLower(last.Text), "от своего имени") {
		t.Errorf("anonymous bind not rejected, got %q", last.Text)
	}
}

// Сводка /today доступна только привязанному в личке или в активном командном чате; в чужой группе и в личке без привязки — отказ.
func TestSummary_Authorization(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", Role: "user", Login: "joe"}}}
	fake := telegram.NewFakeClient()
	svc := newService(repo, users, fake)
	store := reminderMemStore()
	svc.AttachPayments(newReminderBot(store, fake))

	deny := func(msg *telegram.Message) string {
		fake.Sent = nil
		svc.HandleUpdate(context.Background(), telegram.Update{UpdateID: nextUpd(), Message: msg})
		last, _ := fake.LastSent()
		return last.Text
	}

	// Личка без привязки — отказ.
	if txt := deny(&telegram.Message{From: &telegram.User{ID: 900}, Chat: telegram.Chat{ID: 900, Type: "private"}, Text: "/today"}); !strings.Contains(txt, "Привяжите Telegram") {
		t.Errorf("непривязанной личке ожидался отказ, got %q", txt)
	}

	// Чужая группа (не командный чат) — отказ.
	if txt := deny(&telegram.Message{From: &telegram.User{ID: 901}, Chat: telegram.Chat{ID: -555, Type: "supergroup"}, Text: "/today"}); !strings.Contains(txt, "командном чате") {
		t.Errorf("чужой группе ожидался отказ, got %q", txt)
	}

	repo.UpsertLink(context.Background(), "u1", 902, "joe")
	txt := deny(&telegram.Message{From: &telegram.User{ID: 902}, Chat: telegram.Chat{ID: 902, Type: "private"}, Text: "/today"})
	if strings.Contains(txt, "Привяжите") || strings.Contains(txt, "командном чате") {
		t.Errorf("привязанному в личке сводка должна быть доступна, got %q", txt)
	}

	repo.BindChat(context.Background(), -777, "supergroup", "Fin", "u1")
	txt = deny(&telegram.Message{From: &telegram.User{ID: 999}, Chat: telegram.Chat{ID: -777, Type: "supergroup"}, Text: "/today"})
	if strings.Contains(txt, "Привяжите") || strings.Contains(txt, "командном чате") {
		t.Errorf("в командном чате сводка должна быть доступна, got %q", txt)
	}
}

// Кнопка «Оплачено» от непривязанного Telegram — alert «Привяжите Telegram».
func TestPaidCallback_Unlinked(t *testing.T) {
	repo := newMemRepo()
	fake := telegram.NewFakeClient()
	svc := newService(repo, memUsers{}, fake)
	svc.AttachPayments(newReminderBot(reminderMemStore(), fake))

	svc.HandleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		CallbackQuery: &telegram.CallbackQuery{
			ID:   "cb-unlinked",
			From: telegram.User{ID: 4242},
			Data: "p:11111111222233334444555555555555",
		},
	})
	if len(fake.Answers) != 1 {
		t.Fatalf("ожидался ровно один answerCallbackQuery, got %d", len(fake.Answers))
	}
	a := fake.Answers[0]
	if !a.ShowAlert || !strings.Contains(a.Text, "Привяжите Telegram") {
		t.Errorf("ожидался alert «Привяжите Telegram», got %+v", a)
	}
}

func TestBotDisabled_LinkCode(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, memUsers{}, nil, nil, nil, Config{Enabled: false, BotUsername: "ik_dev_bot"})
	if _, _, err := svc.CreateLinkCode(context.Background(), "u1"); !errors.Is(err, ErrBotDisabled) {
		t.Fatalf("err = %v, want ErrBotDisabled", err)
	}
}
