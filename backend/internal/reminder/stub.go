package reminder

import (
	"context"
	"sync"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// MemoryStore — in-memory заглушка источника периодов и оплаты (§2.4). Реализует
// OccurrenceSource и Payer: источник и запись работают на одних данных, поэтому
// MarkPaid сразу виден в DueBetween/GetByIDs. Так движок, бот и Mini App можно
// писать и тестировать, пока таблиц модуля B нет. Те же sentinel-ошибки, что у
// реального адаптера; правило №4 (отметить может ответственный или админ)
// проверяется здесь. Потокобезопасна.
type MemoryStore struct {
	mu   sync.Mutex
	occs map[string]*model.ReminderOccurrence
	// isAdmin сообщает, админ ли пользователь. В проде/демо подключается к
	// users.role из БД (см. buildReminder), чтобы реальный привязанный админ мог
	// нажать «Оплачено». В тестах — замыкание над статическим множеством id.
	isAdmin func(ctx context.Context, userID string) bool
}

// NewMemoryStore собирает заглушку. isAdmin проверяет роль admin пользователя;
// nil означает «админов нет» (кнопку жмёт только сам ответственный).
func NewMemoryStore(occs []model.ReminderOccurrence, isAdmin func(ctx context.Context, userID string) bool) *MemoryStore {
	m := &MemoryStore{
		occs:    make(map[string]*model.ReminderOccurrence, len(occs)),
		isAdmin: isAdmin,
	}
	for i := range occs {
		o := occs[i]
		m.occs[o.ID] = &o
	}
	return m
}

// StaticAdmins — помощник для тестов и демо: isAdmin по фиксированному набору id.
func StaticAdmins(ids ...string) func(ctx context.Context, userID string) bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return func(_ context.Context, userID string) bool { return set[userID] }
}

// DueBetween возвращает неоплаченные периоды активных платежей с due_date в
// [from, to] включительно.
func (m *MemoryStore) DueBetween(ctx context.Context, from, to clock.Date) ([]model.ReminderOccurrence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.ReminderOccurrence
	for _, o := range m.occs {
		if o.Paid || o.Ended {
			continue
		}
		if o.DueDate.Before(from) || o.DueDate.After(to) {
			continue
		}
		out = append(out, *o)
	}
	return out, nil
}

// GetByIDs возвращает текущее состояние периодов (включая оплаченные/завершённые).
func (m *MemoryStore) GetByIDs(ctx context.Context, ids []string) ([]model.ReminderOccurrence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.ReminderOccurrence, 0, len(ids))
	for _, id := range ids {
		if o, ok := m.occs[id]; ok {
			out = append(out, *o)
		}
	}
	return out, nil
}

// MarkPaid отмечает период оплаченным от имени actorUserID. Порядок проверок:
// не найден -> завершён -> нет прав -> уже оплачен (та же политика у B).
func (m *MemoryStore) MarkPaid(ctx context.Context, occurrenceID string, paidDate clock.Date, actorUserID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.occs[occurrenceID]
	if !ok {
		return ErrOccurrenceNotFound
	}
	if o.Ended {
		return ErrPaymentEnded
	}
	if !m.canPay(ctx, o, actorUserID) {
		return ErrForbidden
	}
	if o.Paid {
		return ErrAlreadyPaid
	}
	o.Paid = true
	pd := paidDate
	o.PaidDate = &pd
	return nil
}

// canPay — ответственный или админ (решение пользователя №4).
func (m *MemoryStore) canPay(ctx context.Context, o *model.ReminderOccurrence, actorUserID string) bool {
	if m.isAdmin != nil && m.isAdmin(ctx, actorUserID) {
		return true
	}
	return o.AssigneeID != nil && *o.AssigneeID == actorUserID
}

// SetEnded помечает платёж периода завершённым (для тестов и демо-сценариев).
func (m *MemoryStore) SetEnded(occurrenceID string, ended bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o, ok := m.occs[occurrenceID]; ok {
		o.Ended = ended
	}
}

// Delete убирает период из источника (для тестов: имитирует удаление будущего
// неоплаченного периода при остановке платежа, D7).
func (m *MemoryStore) Delete(occurrenceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.occs, occurrenceID)
}

// SetUnpaid снимает отметку оплаты (B это разрешает — проверяем reconcile-возврат).
func (m *MemoryStore) SetUnpaid(occurrenceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o, ok := m.occs[occurrenceID]; ok {
		o.Paid = false
		o.PaidDate = nil
	}
}

// DemoFixtures — демонстрационные периоды относительно сегодняшней даты для
// REMINDER_SOURCE=stub (§2.4): один со сроком через 2 дня (попадёт в окно),
// один просрочен 6 дней назад (не шлём: старше 3 дней, §5.7), один оплачен,
// один со сроком через 10 дней (вне окна) и один у завершённого платежа.
// assigneeID — кого назначить ответственным по демо-платежам: вызывающий
// подставляет реальный users.id (например, админа), чтобы «Оплачено» работало;
// при пустом значении берётся заметный плейсхолдер.
func DemoFixtures(today clock.Date, assigneeID string) []model.ReminderOccurrence {
	assignee := assigneeID
	if assignee == "" {
		assignee = "11111111-1111-1111-1111-111111111111"
	}
	paidDate := today.AddDays(-4)
	occs := []model.ReminderOccurrence{
		{
			ID: "aaaaaaaa-0000-0000-0000-000000000001", RecurringPaymentID: "rp-1",
			Title: "Аренда сервера", Amount: "1500.00", Currency: "RUB",
			DueDate: today.AddDays(2), AssigneeID: &assignee, AssigneeName: "Иван Петров",
		},
		{
			ID: "aaaaaaaa-0000-0000-0000-000000000002", RecurringPaymentID: "rp-2",
			Title: "Домен example.com", Amount: "20.00", Currency: "USD",
			DueDate: today.AddDays(-6), AssigneeID: &assignee, AssigneeName: "Иван Петров",
		},
		{
			ID: "aaaaaaaa-0000-0000-0000-000000000003", RecurringPaymentID: "rp-3",
			Title: "Подписка на дизайн", Amount: "900.00", Currency: "EUR",
			DueDate: today.AddDays(1), Paid: true, PaidDate: &paidDate,
			AssigneeName: "Ольга Смирнова",
		},
		{
			ID: "aaaaaaaa-0000-0000-0000-000000000004", RecurringPaymentID: "rp-4",
			Title: "Бухгалтерия (квартал)", Amount: "30000.00", Currency: "RUB",
			DueDate: today.AddDays(10), AssigneeName: "Пётр Сидоров",
		},
		{
			ID: "aaaaaaaa-0000-0000-0000-000000000005", RecurringPaymentID: "rp-5",
			Title: "Старый сервис (завершён)", Amount: "500.00", Currency: "RUB",
			DueDate: today.AddDays(1), Ended: true, AssigneeName: "Иван Петров",
		},
	}
	return occs
}
