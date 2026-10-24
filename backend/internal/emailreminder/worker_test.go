package emailreminder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

func TestKindFor(t *testing.T) {
	cases := []struct {
		diff, before int
		kind         string
		ok           bool
	}{
		{0, 3, KindDueSoon, true}, {3, 3, KindDueSoon, true}, {4, 3, "", false},
		{-1, 3, KindOverdue, true}, {-3, 3, KindOverdue, true}, {-4, 3, "", false},
		{1, 0, "", false}, {0, 0, KindDueSoon, true},
	}
	for _, c := range cases {
		k, ok := KindFor(c.diff, c.before)
		if k != c.kind || ok != c.ok {
			t.Errorf("diff=%d before=%d: got %q %v", c.diff, c.before, k, ok)
		}
	}
}

func TestBuildDigestKeepsCurrenciesApart(t *testing.T) {
	subject, body := BuildDigest("Ольга", []Item{
		{Title: "Аренда", Amount: 15000000, Currency: "RUB", Due: "2026-10-27", Days: 3},
		{Title: "GitHub", Amount: 4800, Currency: "USD", Due: "2026-10-25", Days: 1},
		{Title: "Хостинг", Amount: 8990, Currency: "EUR", Due: "2026-10-20", Days: -4},
	}, "https://x.test")
	if subject != "Invoice Keeper: к оплате 2, просрочено 1" {
		t.Errorf("subject: %q", subject)
	}
	for _, want := range []string{"150 000,00 ₽", "$48.00", "89,90 €", "через 3 дня", "завтра", "просрочено на 4 дня", "27.10.2026", "https://x.test"} {
		if !strings.Contains(body, want) {
			t.Errorf("body misses %q:\n%s", want, body)
		}
	}
}

type fakeStore struct {
	cands    []model.EmailCandidate
	enqueued []model.EmailEnqueue
	rows     []model.EmailReminderRow
	sent     []string
	cancel   []string
	failed   []string
	dead     bool
}

func (f *fakeStore) Candidates(context.Context) ([]model.EmailCandidate, error) { return f.cands, nil }
func (f *fakeStore) Enqueue(_ context.Context, it []model.EmailEnqueue) error {
	f.enqueued = append(f.enqueued, it...)
	return nil
}
func (f *fakeStore) DueRows(context.Context, time.Time, int) ([]model.EmailReminderRow, error) {
	return f.rows, nil
}
func (f *fakeStore) MarkSent(_ context.Context, ids []string) error {
	f.sent = append(f.sent, ids...)
	return nil
}
func (f *fakeStore) Cancel(_ context.Context, ids []string) error {
	f.cancel = append(f.cancel, ids...)
	return nil
}
func (f *fakeStore) MarkFailed(_ context.Context, ids []string, _ time.Time, _ string, dead bool) error {
	f.failed = append(f.failed, ids...)
	f.dead = dead
	return nil
}
func (f *fakeStore) Cleanup(context.Context, time.Time) error { return nil }

type fakeMailer struct {
	err   error
	calls []string
}

func (m *fakeMailer) Send(_ context.Context, to, subject, _ string) error {
	m.calls = append(m.calls, to+"|"+subject)
	return m.err
}

func newWorker(st *fakeStore, ml *fakeMailer, now time.Time) *Worker {
	w := NewWorker(st, ml, "")
	w.now = func() time.Time { return now }
	return w
}

func TestEnqueueRespectsSendTimeAndTimezone(t *testing.T) {
	// 05:30 UTC = 08:30 в Москве: письмо на 09:00 ещё рано, в Новосибирске (12:30) уже пора.
	now := time.Date(2026, 10, 24, 5, 30, 0, 0, time.UTC)
	st := &fakeStore{cands: []model.EmailCandidate{
		{UserID: "msk", PaymentID: "p1", DueDate: "2026-10-26", Timezone: "Europe/Moscow", SendTime: "09:00", DaysBefore: 3},
		{UserID: "nsk", PaymentID: "p2", DueDate: "2026-10-26", Timezone: "Asia/Novosibirsk", SendTime: "09:00", DaysBefore: 3},
		{UserID: "far", PaymentID: "p3", DueDate: "2026-11-30", Timezone: "Asia/Novosibirsk", SendTime: "09:00", DaysBefore: 3},
	}}
	newWorker(st, &fakeMailer{}, now).enqueue(context.Background())
	if len(st.enqueued) != 1 || st.enqueued[0].UserID != "nsk" || st.enqueued[0].Kind != KindDueSoon {
		t.Fatalf("enqueued: %+v", st.enqueued)
	}
}

func TestSendGroupsPerUserAndCancelsStale(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	row := func(id, user, pay string) model.EmailReminderRow {
		return model.EmailReminderRow{ID: id, UserID: user, Kind: KindDueSoon, DueDate: "2026-10-25", PaymentDue: "2026-10-25",
			PaymentID: pay, Title: "T" + id, Amount: 1000, Currency: "RUB", Email: user + "@x.y", FullName: user,
			Timezone: "UTC", Enabled: true, Verified: true}
	}
	paid := row("3", "a", "p3")
	paid.Paid = true
	st := &fakeStore{rows: []model.EmailReminderRow{row("1", "a", "p1"), row("2", "a", "p2"), paid, row("4", "b", "p4")}}
	ml := &fakeMailer{}
	newWorker(st, ml, now).send(context.Background())

	if len(ml.calls) != 2 {
		t.Fatalf("expected one digest per user, got %v", ml.calls)
	}
	if strings.Join(st.sent, ",") != "1,2,4" || strings.Join(st.cancel, ",") != "3" {
		t.Fatalf("sent=%v cancel=%v", st.sent, st.cancel)
	}
}

func TestSendFailureBacksOff(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	r := model.EmailReminderRow{ID: "1", UserID: "a", DueDate: "2026-10-25", PaymentDue: "2026-10-25", Title: "T", Email: "a@x.y",
		Timezone: "UTC", Enabled: true, Verified: true, Attempts: maxAttempts - 1}
	st := &fakeStore{rows: []model.EmailReminderRow{r}}
	newWorker(st, &fakeMailer{err: errors.New("535 auth")}, now).send(context.Background())
	if len(st.failed) != 1 || !st.dead || len(st.sent) != 0 {
		t.Fatalf("failed=%v dead=%v sent=%v", st.failed, st.dead, st.sent)
	}
}
