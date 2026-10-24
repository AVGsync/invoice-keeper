// Package remindertest содержит контрактные проверки портов reminder.
package remindertest

import (
	"context"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

// Fixtures — канонический набор периодов и ролей для контрактных проверок.
type Fixtures struct {
	Today      clock.Date
	Lead       int
	OverdueMax int

	Occurrences []model.ReminderOccurrence

	FromID       string // срок == Today (нижняя граница окна)
	ToID         string // срок == Today+Lead (верхняя граница окна)
	OutsideID    string // срок == Today+Lead+1 (вне окна)
	PaidID       string // в окне, но оплачен
	EndedID      string // период завершённого платежа
	NoAssigneeID string // в окне, assignee == nil
	PayableID    string // неоплачен, assignee == Assignee (для Payer)

	Assignee string
	Admin    string
	Other    string
}

func ptr(s string) *string { return &s }

// BuildFixtures собирает детерминированный набор относительно today.
func BuildFixtures(today clock.Date) Fixtures {
	const (
		fromID   = "11111111-1111-1111-1111-111111111101"
		toID     = "11111111-1111-1111-1111-111111111102"
		outside  = "11111111-1111-1111-1111-111111111103"
		paid     = "11111111-1111-1111-1111-111111111104"
		ended    = "11111111-1111-1111-1111-111111111105"
		noAssign = "11111111-1111-1111-1111-111111111106"
		payable  = "11111111-1111-1111-1111-111111111107"
		assignee = "22222222-2222-2222-2222-222222222201"
		admin    = "22222222-2222-2222-2222-222222222202"
		other    = "22222222-2222-2222-2222-222222222203"
		rp       = "33333333-3333-3333-3333-333333333301"
	)
	lead := 3
	paidDate := today.AddDays(-1)
	occs := []model.ReminderOccurrence{
		{ID: fromID, RecurringPaymentID: rp, Title: "Окно: нижняя граница", Amount: "100.00", Currency: "RUB",
			DueDate: today, AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
		{ID: toID, RecurringPaymentID: rp, Title: "Окно: верхняя граница", Amount: "200.00", Currency: "RUB",
			DueDate: today.AddDays(lead), AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
		{ID: outside, RecurringPaymentID: rp, Title: "Вне окна", Amount: "300.00", Currency: "RUB",
			DueDate: today.AddDays(lead + 1), AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
		{ID: paid, RecurringPaymentID: rp, Title: "Оплачен", Amount: "400.00", Currency: "USD",
			DueDate: today.AddDays(1), Paid: true, PaidDate: &paidDate, AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
		{ID: ended, RecurringPaymentID: rp, Title: "Завершён", Amount: "500.00", Currency: "EUR",
			DueDate: today.AddDays(1), Ended: true, AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
		{ID: noAssign, RecurringPaymentID: rp, Title: "Без ответственного", Amount: "600.00", Currency: "RUB",
			DueDate: today.AddDays(2)},
		{ID: payable, RecurringPaymentID: rp, Title: "Оплачиваемый", Amount: "700.00", Currency: "RUB",
			DueDate: today.AddDays(1), AssigneeID: ptr(assignee), AssigneeName: "Иван Петров"},
	}
	return Fixtures{
		Today: today, Lead: lead, OverdueMax: 3, Occurrences: occs,
		FromID: fromID, ToID: toID, OutsideID: outside, PaidID: paid, EndedID: ended,
		NoAssigneeID: noAssign, PayableID: payable,
		Assignee: assignee, Admin: admin, Other: other,
	}
}

func contains(occs []model.ReminderOccurrence, id string) bool {
	for _, o := range occs {
		if o.ID == id {
			return true
		}
	}
	return false
}

func byID(occs []model.ReminderOccurrence, id string) (model.ReminderOccurrence, bool) {
	for _, o := range occs {
		if o.ID == id {
			return o, true
		}
	}
	return model.ReminderOccurrence{}, false
}

// SourceContractOption настраивает проверку контракта источника.
type SourceContractOption func(*sourceContractOpts)

type sourceContractOpts struct {
	assigneeRequired bool
}

func AssigneeRequired() SourceContractOption {
	return func(o *sourceContractOpts) { o.assigneeRequired = true }
}

// RunSourceContract проверяет инварианты OccurrenceSource.
func RunSourceContract(t *testing.T, newSrc func(t *testing.T, fx Fixtures) reminder.OccurrenceSource, opts ...SourceContractOption) {
	t.Helper()
	var o sourceContractOpts
	for _, opt := range opts {
		opt(&o)
	}
	today := clock.Date{Y: 2026, M: 10, D: 24}
	fx := BuildFixtures(today)
	src := newSrc(t, fx)
	ctx := context.Background()

	from, to := fx.Today, fx.Today.AddDays(fx.Lead)
	due, err := src.DueBetween(ctx, from, to)
	if err != nil {
		t.Fatalf("DueBetween: %v", err)
	}

	// 1: оплаченный не попадает.
	if contains(due, fx.PaidID) {
		t.Error("DueBetween вернул оплаченный период")
	}
	// 2: завершённый не попадает в DueBetween.
	if contains(due, fx.EndedID) {
		t.Error("DueBetween вернул период завершённого платежа")
	}
	if !contains(due, fx.FromID) {
		t.Error("нижняя граница окна не попала в выборку")
	}
	if !contains(due, fx.ToID) {
		t.Error("верхняя граница окна не попала в выборку")
	}
	if contains(due, fx.OutsideID) {
		t.Error("период за верхней границей (to+1) попал в выборку")
	}
	// 5: assignee nil не роняет выборку.
	if !contains(due, fx.NoAssigneeID) {
		t.Error("период без ответственного не попал в выборку")
	}
	if !o.assigneeRequired {
		if occ, ok := byID(due, fx.NoAssigneeID); ok && occ.AssigneeID != nil {
			t.Error("ожидался nil AssigneeID у периода без ответственного")
		}
	}

	// 4: поля совпадают, due_date не сдвинута.
	if o, ok := byID(due, fx.FromID); ok {
		if o.DueDate != fx.Today {
			t.Errorf("due_date сдвинута: got %v want %v", o.DueDate, fx.Today)
		}
		if o.Title != "Окно: нижняя граница" || o.Amount != "100.00" || o.Currency != "RUB" {
			t.Errorf("поля не совпали: %+v", o)
		}
	}

	got, err := src.GetByIDs(ctx, []string{fx.EndedID, fx.PaidID})
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if o, ok := byID(got, fx.EndedID); !ok || !o.Ended {
		t.Errorf("GetByIDs не вернул завершённый период с Ended=true: %+v", o)
	}
	if o, ok := byID(got, fx.PaidID); !ok || !o.Paid {
		t.Errorf("GetByIDs не вернул оплаченный период с Paid=true: %+v", o)
	}
}

// RunPayerContract проверяет инвариант 6: права и sentinel-ошибки.
func RunPayerContract(t *testing.T, newPayer func(t *testing.T, fx Fixtures) reminder.Payer) {
	t.Helper()
	today := clock.Date{Y: 2026, M: 10, D: 24}
	fx := BuildFixtures(today)
	payer := newPayer(t, fx)
	ctx := context.Background()

	// Чужой пользователь без прав.
	if err := payer.MarkPaid(ctx, fx.PayableID, today, fx.Other); err != reminder.ErrForbidden {
		t.Errorf("MarkPaid чужим: got %v want ErrForbidden", err)
	}
	if err := payer.MarkPaid(ctx, fx.PayableID, today, fx.Assignee); err != nil {
		t.Errorf("MarkPaid ответственным: got %v want nil", err)
	}
	// Повторно — уже оплачено.
	if err := payer.MarkPaid(ctx, fx.PayableID, today, fx.Assignee); err != reminder.ErrAlreadyPaid {
		t.Errorf("повторный MarkPaid: got %v want ErrAlreadyPaid", err)
	}
	if err := payer.MarkPaid(ctx, fx.EndedID, today, fx.Admin); err != reminder.ErrPaymentEnded {
		t.Errorf("MarkPaid завершённого: got %v want ErrPaymentEnded", err)
	}
	if err := payer.MarkPaid(ctx, "44444444-4444-4444-4444-444444444444", today, fx.Admin); err != reminder.ErrOccurrenceNotFound {
		t.Errorf("MarkPaid неизвестного: got %v want ErrOccurrenceNotFound", err)
	}
}
