package reminder

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// bumpAttempts/forceDue — тестовые хелперы правки строки outbox напрямую.
func (m *memNotif) bumpAttempts(id string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.Attempts = n
		r.NextAttemptAt = time.Time{}
	}
}

func (m *memNotif) first() model.TelegramNotification {
	all := m.all()
	if len(all) == 0 {
		return model.TelegramNotification{}
	}
	return all[0]
}

// 429 retry_after: строка failed, attempts НЕ растёт, next_attempt_at = now+N.
func TestLoop_SendError_RetryAfter(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	h.fake.QueueSendErr(&telegram.APIError{Code: 429, RetryAfter: 7})

	h.loop.Tick(context.Background())

	r := h.notif.first()
	if r.State != model.NotificationStateFailed {
		t.Fatalf("state = %s, want failed", r.State)
	}
	if r.Attempts != 0 {
		t.Errorf("attempts = %d, want 0 (429 не считается попыткой)", r.Attempts)
	}
	if want := h.clk.Now().Add(7 * time.Second); !r.NextAttemptAt.Equal(want) {
		t.Errorf("next_attempt_at = %v, want %v", r.NextAttemptAt, want)
	}
}

// chat not found: командный чат деактивируется, строка failed c backoff 15 мин.
func TestLoop_SendError_ChatNotFound(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	h.fake.QueueSendErr(&telegram.APIError{Code: 400, Description: "Bad Request: chat not found"})

	h.loop.Tick(context.Background())

	if _, err := h.chats.GetActiveChat(context.Background()); err == nil {
		t.Error("ожидалась деактивация командного чата")
	}
	r := h.notif.first()
	if r.State != model.NotificationStateFailed {
		t.Fatalf("state = %s, want failed", r.State)
	}
	if want := h.clk.Now().Add(15 * time.Minute); !r.NextAttemptAt.Equal(want) {
		t.Errorf("next_attempt_at = %v, want +15m %v", r.NextAttemptAt, want)
	}
}

// migrate_to_chat_id: меняем chat_id и один раз повторяем — строка уходит sent.
func TestLoop_SendError_MigrateThenRetry(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	const newID = int64(-1009999999999)
	h.fake.QueueSendErr(
		&telegram.APIError{Code: 400, Description: "group upgraded to supergroup", MigrateToChatID: newID},
		nil, // повтор успешен
	)

	h.loop.Tick(context.Background())

	if h.chats.chatID != newID {
		t.Errorf("chat_id = %d, want %d", h.chats.chatID, newID)
	}
	if h.notif.count(model.NotificationStateSent) != 1 {
		t.Errorf("ожидалась одна отправленная строка после повтора")
	}
	if len(h.fake.Sent) != 1 || h.fake.Sent[0].ChatID != newID {
		t.Errorf("повтор ушёл не в новый чат: %+v", h.fake.Sent)
	}
}

// Прочая 400 — строка dead сразу.
func TestLoop_SendError_Other400Dead(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	h.fake.QueueSendErr(&telegram.APIError{Code: 400, Description: "Bad Request: message text is too long"})

	h.loop.Tick(context.Background())

	if h.notif.count(model.NotificationStateDead) != 1 {
		t.Fatalf("ожидалась dead-строка, состояния: %+v", h.notif.all())
	}
}

// 5xx: backoff с ростом attempts, после maxSendAttempts — dead.
func TestLoop_SendError_5xxBackoffThenDead(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})

	h.fake.QueueSendErr(&telegram.APIError{Code: 500, Description: "Internal Server Error"})
	h.loop.Tick(context.Background())
	r := h.notif.first()
	if r.State != model.NotificationStateFailed || r.Attempts != 1 {
		t.Fatalf("после первого 5xx: state=%s attempts=%d, want failed/1", r.State, r.Attempts)
	}

	h.notif.bumpAttempts(r.ID, maxSendAttempts-1)
	h.fake.QueueSendErr(&telegram.APIError{Code: 503, Description: "Service Unavailable"})
	h.loop.Tick(context.Background())
	if h.notif.count(model.NotificationStateDead) != 1 {
		t.Fatalf("после %d попыток ожидалась dead, состояния: %+v", maxSendAttempts, h.notif.all())
	}
}

// Застрявшая overdue-строка после простоя: при отправке слишком старую просрочку отменяем, а не шлём пачкой.
func TestLoop_StaleOverdueCancelledOnSend(t *testing.T) {
	due := h2today(t).AddDays(-10) // срок 10 дней назад, вне окна enqueue
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, due)})

	if err := h.notif.Enqueue(context.Background(), []EnqueueItem{{
		OccurrenceID: testOccID, DueDate: due,
		Kind: model.NotificationKindOverdue, Recipient: model.NotificationRecipientTeam,
	}}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	h.loop.Tick(context.Background())

	if len(h.fake.Sent) != 0 {
		t.Fatalf("старую просрочку слать не должны, got %d", len(h.fake.Sent))
	}
	if h.notif.count(model.NotificationStateCancelled) != 1 {
		t.Fatalf("ожидалась отменённая строка, состояния: %+v", h.notif.all())
	}
}

func TestLoop_ReconcileUnrecoverableEditFixesStatus(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	ctx := context.Background()

	h.loop.Tick(ctx) // отправили, shown=open
	if err := h.store.MarkPaid(ctx, testOccID, h.today, testAdmin); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	h.fake.EditErr = &telegram.APIError{Code: 400, Description: "Bad Request: message to edit not found"}

	h.loop.Tick(ctx) // reconcile

	r := h.notif.first()
	if r.ShownStatus != model.ShownStatusPaid {
		t.Fatalf("shown_status = %s, want paid (строка иначе зациклит reconcile)", r.ShownStatus)
	}

	// Повторный тик не должен снова пытаться править (occ.Paid && shown=paid).
	before := len(h.fake.Edited)
	h.loop.Tick(ctx)
	if len(h.fake.Edited) != before {
		t.Errorf("reconcile повторяет правку уже оплаченной строки")
	}
}

func TestLoop_DeletedOccurrenceReconciled(t *testing.T) {
	h := newHarness(t, []model.ReminderOccurrence{occ(testOccID, h2today(t).AddDays(2))})
	ctx := context.Background()

	h.loop.Tick(ctx) // отправили напоминание
	if len(h.fake.Sent) != 1 {
		t.Fatalf("ожидалась отправка на первом тике, got %d", len(h.fake.Sent))
	}

	h.store.Delete(testOccID) // платёж остановлен, будущий период удалён

	h.loop.Tick(ctx) // reconcile должен отредактировать сообщение

	if len(h.fake.Edited) != 1 {
		t.Fatalf("ожидалась одна правка сообщения, got %d", len(h.fake.Edited))
	}
	if !strings.Contains(h.fake.Edited[0].Text, "отменён") {
		t.Errorf("правка не про отмену: %q", h.fake.Edited[0].Text)
	}
	for _, r := range h.notif.all() {
		if r.ShownStatus != model.ShownStatusEnded {
			t.Errorf("shown_status не ended: %s", r.ShownStatus)
		}
	}
}
