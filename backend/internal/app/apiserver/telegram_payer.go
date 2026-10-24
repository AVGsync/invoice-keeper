package apiserver

import (
	"context"
	"errors"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/service"

	"github.com/google/uuid"
)

// paymentsService — срез PaymentService, нужный адаптеру (для теста с фейком).
type paymentsService interface {
	MarkOccurrencePaid(ctx context.Context, id string, paidDate clock.Date, actor service.PaymentActor) error
}

type paymentsPayer struct {
	payments paymentsService
	source   reminder.OccurrenceSource
	isAdmin  func(ctx context.Context, userID string) bool
}

// MarkPaid отмечает период оплаченным и маппит ошибки сервиса в reminder.Err*.
func (p paymentsPayer) MarkPaid(ctx context.Context, occurrenceID string, paidDate clock.Date, actorUserID string) error {
	if _, err := uuid.Parse(occurrenceID); err != nil {
		return reminder.ErrOccurrenceNotFound
	}

	occs, err := p.source.GetByIDs(ctx, []string{occurrenceID})
	if err != nil {
		return err
	}
	if len(occs) == 0 {
		return reminder.ErrOccurrenceNotFound
	}
	if occs[0].Ended {
		return reminder.ErrPaymentEnded
	}

	admin := false
	if p.isAdmin != nil {
		admin = p.isAdmin(ctx, actorUserID)
	}

	err = p.payments.MarkOccurrencePaid(ctx, occurrenceID, paidDate,
		service.PaymentActor{UserID: actorUserID, IsAdmin: admin})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, model.ErrPaymentNotFound):
		return reminder.ErrOccurrenceNotFound
	case errors.Is(err, model.ErrPaymentForbidden):
		return reminder.ErrForbidden
	case errors.Is(err, model.ErrPaymentAlreadyPaid):
		return reminder.ErrAlreadyPaid
	default:
		return err
	}
}

var _ reminder.Payer = paymentsPayer{}
