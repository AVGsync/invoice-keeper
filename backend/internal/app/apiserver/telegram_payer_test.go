package apiserver

import (
	"context"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"
)

type fakeSource struct {
	occs  []model.ReminderOccurrence
	err   error
	empty bool
}

func (f fakeSource) DueBetween(ctx context.Context, from, to clock.Date) ([]model.ReminderOccurrence, error) {
	return nil, nil
}

func (f fakeSource) GetByIDs(ctx context.Context, ids []string) ([]model.ReminderOccurrence, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return nil, nil
	}
	return f.occs, nil
}

type fakePayments struct{ err error }

func (f fakePayments) MarkOccurrencePaid(ctx context.Context, id string, paidDate clock.Date, actor service.PaymentActor) error {
	return f.err
}

const fakeOccUUID = "11111111-1111-1111-1111-111111111101"

func activeOcc() []model.ReminderOccurrence {
	return []model.ReminderOccurrence{{ID: fakeOccUUID, Ended: false}}
}

func TestPaymentsPayerMapping(t *testing.T) {
	ctx := context.Background()
	today := clock.Date{Y: 2026, M: 10, D: 24}
	isAdmin := func(context.Context, string) bool { return false }

	cases := []struct {
		name   string
		occID  string
		source reminder.OccurrenceSource
		svcErr error
		want   error
	}{
		{"bad uuid", "not-a-uuid", fakeSource{occs: activeOcc()}, nil, reminder.ErrOccurrenceNotFound},
		{"empty source", fakeOccUUID, fakeSource{empty: true}, nil, reminder.ErrOccurrenceNotFound},
		{"ended", fakeOccUUID, fakeSource{occs: []model.ReminderOccurrence{{ID: fakeOccUUID, Ended: true}}}, nil, reminder.ErrPaymentEnded},
		{"forbidden", fakeOccUUID, fakeSource{occs: activeOcc()}, model.ErrPaymentForbidden, reminder.ErrForbidden},
		{"already paid", fakeOccUUID, fakeSource{occs: activeOcc()}, model.ErrPaymentAlreadyPaid, reminder.ErrAlreadyPaid},
		{"not found", fakeOccUUID, fakeSource{occs: activeOcc()}, model.ErrPaymentNotFound, reminder.ErrOccurrenceNotFound},
		{"success", fakeOccUUID, fakeSource{occs: activeOcc()}, nil, nil},
	}

	for _, c := range cases {
		payer := paymentsPayer{
			payments: fakePayments{err: c.svcErr},
			source:   c.source,
			isAdmin:  isAdmin,
		}
		got := payer.MarkPaid(ctx, c.occID, today, "actor")
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
