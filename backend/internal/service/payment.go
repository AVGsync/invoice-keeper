package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
)

// Алиас на единый sentinel валидации платежей.
var ErrInvalidPayment = errors.New("invalid payment")

// PaymentActor — субъект операции над периодом. Права проверяются только здесь,
// в сервисе, который общий для HTTP и Telegram (D10).
type PaymentActor struct {
	UserID  string
	IsAdmin bool
}

type PaymentRepository interface {
	CreatePayment(payment model.Payment, ctx context.Context) (string, error)
	GetPaymentByID(id string, ctx context.Context) (model.Payment, error)
	GetPaymentsByUserID(userID string, ctx context.Context) ([]model.Payment, error)
	GetAllPayments(ctx context.Context) ([]model.Payment, error)
	UpdatePayment(payment model.Payment, ctx context.Context) error
	MarkPaymentAsPaid(id string, paidDate string, ctx context.Context) error
	ChangePaidDate(id string, paidDate string, ctx context.Context) error
	MarkPaymentAsUnpaid(id string, today string, ctx context.Context) error
	DeletePayment(id string, ctx context.Context) error
}

type PaymentService struct {
	repository PaymentRepository
	clk        clock.Clock
	// audit — журнал действий; по умолчанию Nop (конструктор не меняется).
	audit audit.Recorder
}

func NewPaymentService(repository PaymentRepository, clk clock.Clock) *PaymentService {
	return &PaymentService{
		repository: repository,
		clk:        clk,
		audit:      audit.Nop{},
	}
}

// SetAuditRecorder подключает журнал действий.
func (s *PaymentService) SetAuditRecorder(r audit.Recorder) { s.audit = r }

// validCurrency — только RUB/USD/EUR (D12).
func validCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "RUB", "USD", "EUR":
		return true
	default:
		return false
	}
}

func validatePayment(amount int64, currency string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidPayment)
	}

	if !validCurrency(currency) {
		return fmt.Errorf("%w: currency must be one of RUB, USD, EUR", ErrInvalidPayment)
	}

	return nil
}

func (s *PaymentService) GetPaymentByID(
	id string,
	ctx context.Context,
) (model.Payment, error) {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		if !errors.Is(err, model.ErrPaymentNotFound) {
			slog.Error("payment service: get payment failed", "payment_id", id, "error", err)
		}
		return model.Payment{}, fmt.Errorf("payment service: get payment: %w", err)
	}

	return payment, nil
}

func (s *PaymentService) GetPaymentsByUserID(
	userID string,
	ctx context.Context,
) ([]model.Payment, error) {
	payments, err := s.repository.GetPaymentsByUserID(userID, ctx)
	if err != nil {
		slog.Error("payment service: get payments by user failed",
			"user_id", userID, "error", err)
		return nil, fmt.Errorf("payment service: get payments by user: %w", err)
	}

	return payments, nil
}

func (s *PaymentService) GetAllPayments(
	ctx context.Context,
) ([]model.Payment, error) {
	payments, err := s.repository.GetAllPayments(ctx)
	if err != nil {
		slog.Error("payment service: get all payments failed", "error", err)
		return nil, fmt.Errorf("payment service: get all payments: %w", err)
	}

	return payments, nil
}

// UpdatePayment меняет сумму/валюту/описание неоплаченного периода. Доступ
// только администратору — проверяется в HTTP-слое (D10). due_date неизменяема.
func (s *PaymentService) UpdatePayment(
	id string,
	req request.UpdatePaymentRequest,
	ctx context.Context,
) error {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		return fmt.Errorf("payment service: update payment: %w", err)
	}

	if payment.PaidDate != nil {
		return fmt.Errorf("%w: paid payment cannot be edited", ErrInvalidPayment)
	}

	oldAmount, oldCurrency, oldDescription := payment.Amount, payment.Currency, payment.Description

	if req.Amount != nil {
		payment.Amount = *req.Amount
	}

	if req.Currency != nil {
		// Нормализуем так же, как валидирует validCurrency (TrimSpace+ToUpper),
		// иначе " usd" пройдёт проверку и упадёт на CHECK/VARCHAR(3) с 500.
		payment.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}

	if req.Description != nil {
		payment.Description = *req.Description
	}

	if err := validatePayment(payment.Amount, payment.Currency); err != nil {
		return err
	}

	if err := s.repository.UpdatePayment(payment, ctx); err != nil {
		slog.Error("payment service: update payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: update payment: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionOccurrenceUpdate,
		EntityType: audit.EntityOccurrence,
		EntityID:   id,
		Changes: audit.Diff().
			Add("amount", oldAmount, payment.Amount).
			Add("currency", oldCurrency, payment.Currency).
			Add("description", oldDescription, payment.Description).
			Map(),
	})

	return nil
}

func (s *PaymentService) DeletePayment(
	id string,
	ctx context.Context,
) error {
	if err := s.repository.DeletePayment(id, ctx); err != nil {
		if !errors.Is(err, model.ErrPaymentNotFound) {
			slog.Error("payment service: delete payment failed", "payment_id", id, "error", err)
		}
		return fmt.Errorf("payment service: delete payment: %w", err)
	}

	return nil
}
