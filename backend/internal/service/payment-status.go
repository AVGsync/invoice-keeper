package service

import (
	"context"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Оплата/снятие оплаты периода. Единая политика прав для HTTP и Telegram (D10):
// отметить может ответственный (payments.user_id) или администратор. «Завершён»
// здесь не проверяется — историю остановленного платежа можно править из веба.

// checkActor возвращает период и проверяет права. Порядок: не найден → нет прав.
func (s *PaymentService) checkActor(
	ctx context.Context,
	id string,
	actor PaymentActor,
) (model.Payment, error) {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		return model.Payment{}, err // model.ErrPaymentNotFound обёрнут в репозитории
	}
	if !actor.IsAdmin && payment.UserID != actor.UserID {
		return model.Payment{}, model.ErrPaymentForbidden
	}
	return payment, nil
}

// MarkOccurrencePaid отмечает период оплаченным на дату paidDate. Порядок
// ошибок: NotFound → Forbidden → AlreadyPaid.
func (s *PaymentService) MarkOccurrencePaid(
	ctx context.Context,
	id string,
	paidDate clock.Date,
	actor PaymentActor,
) error {
	payment, err := s.checkActor(ctx, id, actor)
	if err != nil {
		return err
	}
	if paidDate.After(s.clk.Today()) {
		return fmt.Errorf("%w: paid date is in the future", ErrInvalidPayment)
	}
	if err := s.repository.MarkPaymentAsPaid(id, paidDate.String(), ctx); err != nil {
		return err
	}

	// paid_late = оплачено позже срока. Срок парсим; при ошибке считаем вовремя.
	paidLate := false
	if due, derr := clock.ParseDate(payment.DueDate); derr == nil {
		paidLate = paidDate.After(due)
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionOccurrenceConfirm,
		EntityType: audit.EntityOccurrence,
		EntityID:   id,
		Payload: map[string]any{
			"paid_date":   paidDate.String(),
			"due_date":    payment.DueDate,
			"amount":      payment.Amount,
			"currency":    payment.Currency,
			"description": payment.Description,
			"paid_late":   paidLate,
		},
	})
	return nil
}

// ChangeOccurrencePaidDate меняет фактическую дату уже оплаченного периода.
func (s *PaymentService) ChangeOccurrencePaidDate(
	ctx context.Context,
	id string,
	paidDate clock.Date,
	actor PaymentActor,
) error {
	payment, err := s.checkActor(ctx, id, actor)
	if err != nil {
		return err
	}
	if paidDate.After(s.clk.Today()) {
		return fmt.Errorf("%w: paid date is in the future", ErrInvalidPayment)
	}
	if err := s.repository.ChangePaidDate(id, paidDate.String(), ctx); err != nil {
		return err
	}

	var oldPaid any
	if payment.PaidDate != nil {
		oldPaid = *payment.PaidDate
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionOccurrencePaidDateChange,
		EntityType: audit.EntityOccurrence,
		EntityID:   id,
		Changes:    audit.Diff().Add("paid_date", oldPaid, paidDate.String()).Map(),
	})
	return nil
}

// MarkOccurrenceUnpaid снимает оплату и очищает дату. Идемпотентна.
func (s *PaymentService) MarkOccurrenceUnpaid(
	ctx context.Context,
	id string,
	actor PaymentActor,
) error {
	payment, err := s.checkActor(ctx, id, actor)
	if err != nil {
		return err
	}
	if err := s.repository.MarkPaymentAsUnpaid(id, s.clk.Today().String(), ctx); err != nil {
		return err
	}

	entry := audit.Entry{
		Action:     audit.ActionOccurrenceCancel,
		EntityType: audit.EntityOccurrence,
		EntityID:   id,
	}
	if payment.PaidDate != nil {
		// Был оплачен — фиксируем снятие оплаты (paid_date → null).
		entry.Changes = audit.Diff().Add("paid_date", *payment.PaidDate, nil).Map()
	} else {
		// Не был оплачен — операция ничего не изменила.
		entry.Payload = map[string]any{"noop": true}
	}
	s.audit.Record(ctx, entry)
	return nil
}

// EffectiveStatus вычисляет статус периода из paid_date, due_date и «сегодня»
// (D6). paidLate = оплачено после срока. Для неоплаченного периода остановленного
// или удалённого шаблона (Ended) возвращается статус "ended": просрочкой он не
// считается, чтобы веб, сводка, NL (is_ended) и /overdue бота давали один ответ
// (NL_QUERIES.md §6.4 п.5). История оплат остаётся: оплаченный период — "paid".
func EffectiveStatus(p model.Payment, today clock.Date) (status string, paidLate bool) {
	if p.PaidDate != nil {
		paid, errP := clock.ParseDate(*p.PaidDate)
		due, errD := clock.ParseDate(p.DueDate)
		if errP == nil && errD == nil {
			paidLate = paid.After(due)
		}
		return "paid", paidLate
	}
	if p.Ended {
		return model.ShownStatusEnded, false
	}
	if due, err := clock.ParseDate(p.DueDate); err == nil && due.Before(today) {
		return "overdue", false
	}
	return "pending", false
}
