// Package emailreminder рассылает письма-напоминания о платежах.
package emailreminder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

const (
	KindDueSoon = "due_soon"
	KindOverdue = "overdue"

	overdueMaxDays = 3
	maxAttempts    = 8
	sendBatch      = 500
)

type Store interface {
	Candidates(ctx context.Context) ([]model.EmailCandidate, error)
	Enqueue(ctx context.Context, items []model.EmailEnqueue) error
	DueRows(ctx context.Context, now time.Time, limit int) ([]model.EmailReminderRow, error)
	MarkSent(ctx context.Context, ids []string) error
	Cancel(ctx context.Context, ids []string) error
	MarkFailed(ctx context.Context, ids []string, next time.Time, lastErr string, dead bool) error
	Cleanup(ctx context.Context, before time.Time) error
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Worker раз в минуту ставит напоминания в очередь и отправляет по одному письму-сводке на человека.
type Worker struct {
	store       Store
	mailer      Mailer
	appURL      string
	tick        time.Duration
	now         func() time.Time
	lastCleanup time.Time
	locs        map[string]*time.Location
}

func NewWorker(store Store, mailer Mailer, appURL string) *Worker {
	return &Worker{store: store, mailer: mailer, appURL: appURL, tick: time.Minute, now: time.Now, locs: map[string]*time.Location{}}
}

func (w *Worker) Run(ctx context.Context) {
	slog.Info("email reminders: started", "tick", w.tick)
	w.Tick(ctx)
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("email reminders: stopped")
			return
		case <-t.C:
			w.Tick(ctx)
		}
	}
}

func (w *Worker) Tick(ctx context.Context) {
	w.enqueue(ctx)
	w.send(ctx)
	if now := w.now(); now.Sub(w.lastCleanup) > time.Hour {
		w.lastCleanup = now
		if err := w.store.Cleanup(ctx, now.AddDate(0, 0, -60)); err != nil {
			slog.Error("email reminders: cleanup", "error", err)
		}
	}
}

func (w *Worker) location(name string) *time.Location {
	if loc, ok := w.locs[name]; ok {
		return loc
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		slog.Warn("email reminders: bad timezone", "timezone", name)
		loc = nil
	}
	w.locs[name] = loc
	return loc
}

// KindFor решает, нужно ли письмо: diff — дней от сегодня до срока.
func KindFor(diff, daysBefore int) (string, bool) {
	switch {
	case diff >= 0 && diff <= daysBefore:
		return KindDueSoon, true
	case diff < 0 && -diff <= overdueMaxDays:
		return KindOverdue, true
	}
	return "", false
}

func daysBetween(from, to string) (int, bool) {
	a, err1 := time.Parse("2006-01-02", from)
	b, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return int(b.Sub(a).Hours() / 24), true
}

func (w *Worker) enqueue(ctx context.Context) {
	cands, err := w.store.Candidates(ctx)
	if err != nil {
		slog.Error("email reminders: candidates", "error", err)
		return
	}
	now := w.now()
	var items []model.EmailEnqueue
	for _, c := range cands {
		loc := w.location(c.Timezone)
		if loc == nil {
			continue
		}
		local := now.In(loc)
		if local.Format("15:04") < c.SendTime {
			continue
		}
		diff, ok := daysBetween(local.Format("2006-01-02"), c.DueDate)
		if !ok {
			continue
		}
		if kind, ok := KindFor(diff, c.DaysBefore); ok {
			items = append(items, model.EmailEnqueue{UserID: c.UserID, PaymentID: c.PaymentID, DueDate: c.DueDate, Kind: kind})
		}
	}
	if err := w.store.Enqueue(ctx, items); err != nil {
		slog.Error("email reminders: enqueue", "error", err)
	}
}

func (w *Worker) send(ctx context.Context) {
	now := w.now()
	rows, err := w.store.DueRows(ctx, now, sendBatch)
	if err != nil {
		slog.Error("email reminders: due rows", "error", err)
		return
	}
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].UserID == rows[i].UserID {
			j++
		}
		w.sendGroup(ctx, rows[i:j], now)
		i = j
	}
}

func (w *Worker) sendGroup(ctx context.Context, group []model.EmailReminderRow, now time.Time) {
	var live []model.EmailReminderRow
	var stale []string
	for _, r := range group {
		if r.Paid || r.Stopped || !r.Enabled || !r.Verified || r.PaymentDue != r.DueDate {
			stale = append(stale, r.ID)
			continue
		}
		live = append(live, r)
	}
	if err := w.store.Cancel(ctx, stale); err != nil {
		slog.Error("email reminders: cancel", "error", err)
	}
	if len(live) == 0 {
		return
	}

	loc := w.location(live[0].Timezone)
	if loc == nil {
		loc = time.UTC
	}
	today := now.In(loc).Format("2006-01-02")
	items := make([]Item, 0, len(live))
	ids := make([]string, 0, len(live))
	for _, r := range live {
		diff, _ := daysBetween(today, r.DueDate)
		items = append(items, Item{Title: r.Title, Amount: r.Amount, Currency: r.Currency, Due: r.DueDate, Days: diff})
		ids = append(ids, r.ID)
	}
	subject, body := BuildDigest(live[0].FullName, items, w.appURL)

	sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := w.mailer.Send(sctx, live[0].Email, subject, body); err != nil {
		attempts := 0
		for _, r := range live {
			if r.Attempts > attempts {
				attempts = r.Attempts
			}
		}
		delay := 5 * time.Minute << uint(attempts)
		if delay > time.Hour || delay <= 0 {
			delay = time.Hour
		}
		slog.Error("email reminders: send", "user_id", live[0].UserID, "error", err)
		if ferr := w.store.MarkFailed(ctx, ids, now.Add(delay), err.Error(), attempts+1 >= maxAttempts); ferr != nil {
			slog.Error("email reminders: mark failed", "error", ferr)
		}
		return
	}
	if err := w.store.MarkSent(ctx, ids); err != nil {
		slog.Error("email reminders: mark sent", "error", err)
	}
}

// Item — платёж в письме. Days — дней до срока (отрицательное — просрочка).
type Item struct {
	Title    string
	Amount   int64
	Currency string
	Due      string
	Days     int
}

func money(minor int64, currency string) string {
	return reminder.FormatMoney(fmt.Sprintf("%d.%02d", minor/100, minor%100), currency)
}

func dotted(iso string) string {
	if t, err := time.Parse("2006-01-02", iso); err == nil {
		return t.Format("02.01.2006")
	}
	return iso
}

func plural(n int) string {
	m := n % 100
	switch {
	case m >= 11 && m <= 14:
		return fmt.Sprintf("%d дней", n)
	case n%10 == 1:
		return fmt.Sprintf("%d день", n)
	case n%10 >= 2 && n%10 <= 4:
		return fmt.Sprintf("%d дня", n)
	}
	return fmt.Sprintf("%d дней", n)
}

// BuildDigest собирает письмо-сводку. Суммы в разных валютах не складываются.
func BuildDigest(name string, items []Item, appURL string) (subject, body string) {
	var soon, late []Item
	for _, it := range items {
		if it.Days < 0 {
			late = append(late, it)
		} else {
			soon = append(soon, it)
		}
	}

	var parts []string
	if len(soon) > 0 {
		parts = append(parts, fmt.Sprintf("к оплате %d", len(soon)))
	}
	if len(late) > 0 {
		parts = append(parts, fmt.Sprintf("просрочено %d", len(late)))
	}
	subject = "Invoice Keeper: " + strings.Join(parts, ", ")

	var b strings.Builder
	if name != "" {
		b.WriteString("Здравствуйте, " + name + "!\n\n")
	}
	if len(soon) > 0 {
		b.WriteString("К оплате:\n")
		for _, it := range soon {
			when := "сегодня"
			if it.Days == 1 {
				when = "завтра"
			} else if it.Days > 1 {
				when = "через " + plural(it.Days)
			}
			fmt.Fprintf(&b, "• %s — %s — срок %s (%s)\n", it.Title, money(it.Amount, it.Currency), dotted(it.Due), when)
		}
		b.WriteString("\n")
	}
	if len(late) > 0 {
		b.WriteString("Просрочено:\n")
		for _, it := range late {
			fmt.Fprintf(&b, "• %s — %s — срок был %s (просрочено на %s)\n", it.Title, money(it.Amount, it.Currency), dotted(it.Due), plural(-it.Days))
		}
		b.WriteString("\n")
	}
	if appURL != "" {
		b.WriteString("Открыть приложение: " + appURL + "\n\n")
	}
	b.WriteString("Вы получаете это письмо, потому что включили уведомления на почту в Invoice Keeper. Отключить их можно в профиле: Профиль → Почта.\n")
	return subject, b.String()
}
