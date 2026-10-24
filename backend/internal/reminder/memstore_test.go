package reminder

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// memNotif — in-memory реализация NotificationStore для тестов движка.
type memNotif struct {
	mu   sync.Mutex
	seq  int
	rows map[string]*model.TelegramNotification
}

func newMemNotif() *memNotif {
	return &memNotif{rows: map[string]*model.TelegramNotification{}}
}

func (m *memNotif) key(n *model.TelegramNotification) string {
	return fmt.Sprintf("%s|%s|%s|%s", n.OccurrenceID, n.DueDate.String(), n.Kind, n.Recipient)
}

func (m *memNotif) Enqueue(ctx context.Context, items []EnqueueItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range items {
		k := fmt.Sprintf("%s|%s|%s|%s", it.OccurrenceID, it.DueDate.String(), it.Kind, it.Recipient)
		if existing := m.find(k); existing != nil {
			if existing.State == model.NotificationStateCancelled {
				existing.State = model.NotificationStatePending
				existing.NextAttemptAt = time.Time{}
			}
			continue
		}
		m.seq++
		id := fmt.Sprintf("row-%d", m.seq)
		m.rows[id] = &model.TelegramNotification{
			ID: id, OccurrenceID: it.OccurrenceID, DueDate: it.DueDate, Kind: it.Kind,
			Recipient: it.Recipient, State: model.NotificationStatePending,
			ShownStatus: model.ShownStatusOpen,
		}
	}
	return nil
}

func (m *memNotif) find(key string) *model.TelegramNotification {
	for _, r := range m.rows {
		if m.key(r) == key {
			return r
		}
	}
	return nil
}

func (m *memNotif) DueForSend(ctx context.Context, now time.Time, limit int) ([]model.TelegramNotification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.TelegramNotification
	for _, r := range m.rows {
		if (r.State == model.NotificationStatePending || r.State == model.NotificationStateFailed) &&
			!r.NextAttemptAt.After(now) {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextAttemptAt.Before(out[j].NextAttemptAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memNotif) MarkSent(ctx context.Context, id string, chatID, messageID int64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.State = model.NotificationStateSent
		r.ChatID = &chatID
		r.MessageID = &messageID
		r.SentAt = &now
	}
	return nil
}

func (m *memNotif) MarkFailed(ctx context.Context, id string, nextAttemptAt time.Time, incrementAttempt bool, lastErr string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.State = model.NotificationStateFailed
		r.NextAttemptAt = nextAttemptAt
		if incrementAttempt {
			r.Attempts++
		}
		e := lastErr
		r.LastError = &e
	}
	return nil
}

func (m *memNotif) MarkDead(ctx context.Context, id, lastErr string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.State = model.NotificationStateDead
		e := lastErr
		r.LastError = &e
	}
	return nil
}

func (m *memNotif) Cancel(ctx context.Context, id string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.State = model.NotificationStateCancelled
	}
	return nil
}

func (m *memNotif) OpenForReconcile(ctx context.Context, dueOnOrAfter clock.Date, limit int) ([]model.TelegramNotification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.TelegramNotification
	for _, r := range m.openSent() {
		if !r.DueDate.Before(dueOnOrAfter) {
			out = append(out, r)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memNotif) openSent() []model.TelegramNotification {
	var out []model.TelegramNotification
	for _, r := range m.rows {
		if r.State == model.NotificationStateSent &&
			(r.ShownStatus == model.ShownStatusOpen || r.ShownStatus == model.ShownStatusPaid) {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *memNotif) OpenByOccurrence(ctx context.Context, occurrenceID string) ([]model.TelegramNotification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.TelegramNotification
	for _, r := range m.openSent() {
		if r.OccurrenceID == occurrenceID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memNotif) SetShownStatus(ctx context.Context, id, shownStatus string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		r.ShownStatus = shownStatus
	}
	return nil
}

func (m *memNotif) ResetFailedByRecipient(ctx context.Context, recipient string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.Recipient == recipient && r.State == model.NotificationStateFailed {
			r.NextAttemptAt = now
		}
	}
	return nil
}

func (m *memNotif) Cleanup(ctx context.Context, before time.Time) error { return nil }

func (m *memNotif) count(state string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.rows {
		if r.State == state {
			n++
		}
	}
	return n
}

func (m *memNotif) all() []model.TelegramNotification {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.TelegramNotification, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
