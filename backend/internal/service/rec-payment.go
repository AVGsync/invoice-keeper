package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"

	"github.com/google/uuid"
)

// Алиас на единый sentinel; ErrInvalidRecurrentPayment — ошибка валидации (400).
var ErrRecurrentPaymentNotFound = model.ErrRecurrentPaymentNotFound
var ErrInvalidRecurrentPayment = errors.New("invalid recurrent payment")

const maxDescriptionLen = 255

type RecurrentPaymentRepository interface {
	CreateRecurrentPayment(payment model.RecurrentPayment, ctx context.Context) (string, error)
	CreateWithAssignee(payment model.RecurrentPayment, assigneeID string, ctx context.Context) (string, error)
	SetAssignee(paymentID string, userID string, ctx context.Context) error
	GetRecurrentPaymentByID(id string, ctx context.Context) (model.RecurrentPayment, error)
	GetAllRecurrentPayments(ctx context.Context) ([]model.RecurrentPayment, error)
	UpdateRecurrentPayment(payment model.RecurrentPayment, ctx context.Context) error
	Stop(id string, today string, ctx context.Context) error
	DeleteRecurrentPayment(id string, ctx context.Context) error
	SubscribeUsers(paymentID string, userIDs []string, ctx context.Context) error
	UnsubscribeUsers(paymentID string, userIDs []string, ctx context.Context) error
	GetUsersByRecurrentPaymentID(paymentID string, ctx context.Context) ([]string, error)
	GetRecurrentPaymentsByUserID(userID string, ctx context.Context) ([]model.RecurrentPayment, error)
}

// FuturePaymentsUpdater переносит правки шаблона на неоплаченные будущие периоды.
type FuturePaymentsUpdater interface {
	UpdateFutureUnpaid(rpID string, today string, amount int64, currency string, description string, userID string, ctx context.Context) error
}

type RecurrentPaymentService struct {
	repository RecurrentPaymentRepository
	future     FuturePaymentsUpdater
	clk        clock.Clock
}

func NewRecurrentPaymentService(
	repository RecurrentPaymentRepository,
	future FuturePaymentsUpdater,
	clk clock.Clock,
) *RecurrentPaymentService {
	return &RecurrentPaymentService{
		repository: repository,
		future:     future,
		clk:        clk,
	}
}

// dayOffsetFor вычисляет day_offset из даты: monthly → день месяца, weekly →
// день недели (0 = воскресенье, как time.Weekday).
func dayOffsetFor(frequency string, d clock.Date) int {
	if frequency == "weekly" {
		return int(time.Date(d.Y, d.M, d.D, 0, 0, 0, 0, time.UTC).Weekday())
	}
	return d.D
}

func validateRecurrentFields(amount int64, currency, frequency string, interval int) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRecurrentPayment)
	}
	if !validCurrency(currency) {
		return fmt.Errorf("%w: currency must be one of RUB, USD, EUR", ErrInvalidRecurrentPayment)
	}
	if frequency != "weekly" && frequency != "monthly" {
		return fmt.Errorf("%w: frequency must be weekly or monthly", ErrInvalidRecurrentPayment)
	}
	if interval <= 0 {
		return fmt.Errorf("%w: interval must be greater than zero", ErrInvalidRecurrentPayment)
	}
	return nil
}

func (s *RecurrentPaymentService) CreateRecurrentPayment(
	req request.CreateRecurrentPaymentRequest,
	ctx context.Context,
) (string, error) {
	interval := req.Interval
	if interval == 0 {
		interval = 1
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		return "", fmt.Errorf("%w: description is required", ErrInvalidRecurrentPayment)
	}
	if len([]rune(description)) > maxDescriptionLen {
		return "", fmt.Errorf("%w: description must be at most 255 characters", ErrInvalidRecurrentPayment)
	}

	if err := validateRecurrentFields(req.Amount, req.Currency, req.Frequency, interval); err != nil {
		return "", err
	}

	if _, err := uuid.Parse(req.AssigneeID); err != nil {
		return "", fmt.Errorf("%w: assignee_id must be a valid UUID", ErrInvalidRecurrentPayment)
	}

	startDate, err := clock.ParseDate(req.StartDate)
	if err != nil {
		return "", fmt.Errorf("%w: start_date must be YYYY-MM-DD", ErrInvalidRecurrentPayment)
	}
	if startDate.Before(s.clk.Today().AddDays(-365)) {
		return "", fmt.Errorf("%w: start_date is too far in the past", ErrInvalidRecurrentPayment)
	}

	dayOffset := dayOffsetFor(req.Frequency, startDate)
	if req.DayOffset != nil && *req.DayOffset != dayOffset {
		return "", fmt.Errorf("%w: day_offset does not match start_date", ErrInvalidRecurrentPayment)
	}

	payment := model.RecurrentPayment{
		Amount:      req.Amount,
		Currency:    strings.ToUpper(strings.TrimSpace(req.Currency)),
		Description: description,
		Frequency:   req.Frequency,
		Interval:    interval,
		DayOffset:   dayOffset,
		StartDate:   startDate.String(),
	}

	id, err := s.repository.CreateWithAssignee(payment, req.AssigneeID, ctx)
	if err != nil {
		if errors.Is(err, ErrInvalidRecurrentPayment) {
			return "", err
		}
		slog.Error("recurrent payment service: create payment failed", "error", err)
		return "", fmt.Errorf("recurrent payment service: create payment: %w", err)
	}

	return id, nil
}

func (s *RecurrentPaymentService) GetRecurrentPaymentByID(
	id string,
	ctx context.Context,
) (model.RecurrentPayment, error) {
	payment, err := s.repository.GetRecurrentPaymentByID(id, ctx)
	if err != nil {
		if errors.Is(err, model.ErrRecurrentPaymentNotFound) {
			return model.RecurrentPayment{}, model.ErrRecurrentPaymentNotFound
		}
		slog.Error("recurrent payment service: get payment failed", "payment_id", id, "error", err)
		return model.RecurrentPayment{}, fmt.Errorf("recurrent payment service: get payment: %w", err)
	}

	return payment, nil
}

func (s *RecurrentPaymentService) GetAllRecurrentPayments(
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	payments, err := s.repository.GetAllRecurrentPayments(ctx)
	if err != nil {
		slog.Error("recurrent payment service: get all payments failed", "error", err)
		return nil, fmt.Errorf("recurrent payment service: get all payments: %w", err)
	}

	return payments, nil
}

func (s *RecurrentPaymentService) UpdateRecurrentPayment(
	id string,
	req request.UpdateRecurrentPaymentRequest,
	ctx context.Context,
) error {
	payment, err := s.repository.GetRecurrentPaymentByID(id, ctx)
	if err != nil {
		return err
	}

	// Поля графика неизменяемы (D9).
	if req.Frequency != nil && *req.Frequency != payment.Frequency {
		return fmt.Errorf("%w: schedule is immutable (frequency)", ErrInvalidRecurrentPayment)
	}
	if req.Interval != nil && *req.Interval != payment.Interval {
		return fmt.Errorf("%w: schedule is immutable (interval)", ErrInvalidRecurrentPayment)
	}
	if req.DayOffset != nil && *req.DayOffset != payment.DayOffset {
		return fmt.Errorf("%w: schedule is immutable (day_offset)", ErrInvalidRecurrentPayment)
	}
	if req.StartDate != nil && *req.StartDate != payment.StartDate {
		return fmt.Errorf("%w: schedule is immutable (start_date)", ErrInvalidRecurrentPayment)
	}

	if req.Amount == nil && req.Currency == nil && req.Description == nil && req.AssigneeID == nil {
		return fmt.Errorf("%w: at least one field must be provided", ErrInvalidRecurrentPayment)
	}

	if req.Amount != nil {
		payment.Amount = *req.Amount
	}
	if req.Currency != nil {
		payment.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if description == "" || len([]rune(description)) > maxDescriptionLen {
			return fmt.Errorf("%w: description must be 1..255 characters", ErrInvalidRecurrentPayment)
		}
		payment.Description = description
	}

	if err := validateRecurrentFields(payment.Amount, payment.Currency, payment.Frequency, payment.Interval); err != nil {
		return err
	}

	assigneeID := ""
	if req.AssigneeID != nil {
		if _, err := uuid.Parse(*req.AssigneeID); err != nil {
			return fmt.Errorf("%w: assignee_id must be a valid UUID", ErrInvalidRecurrentPayment)
		}
		assigneeID = *req.AssigneeID
	}

	if err := s.repository.UpdateRecurrentPayment(payment, ctx); err != nil {
		if errors.Is(err, model.ErrRecurrentPaymentNotFound) {
			return model.ErrRecurrentPaymentNotFound
		}
		slog.Error("recurrent payment service: update payment failed", "payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: update payment: %w", err)
	}

	if assigneeID != "" {
		if err := s.repository.SetAssignee(id, assigneeID, ctx); err != nil {
			if errors.Is(err, ErrInvalidRecurrentPayment) {
				return err
			}
			slog.Error("recurrent payment service: set assignee failed", "payment_id", id, "error", err)
			return fmt.Errorf("recurrent payment service: set assignee: %w", err)
		}
		payment.AssigneeID = &assigneeID
	}

	// Переносим правки на неоплаченные будущие периоды.
	userID := ""
	if payment.AssigneeID != nil {
		userID = *payment.AssigneeID
	}
	if s.future != nil && userID != "" {
		if err := s.future.UpdateFutureUnpaid(
			id, s.clk.Today().String(),
			payment.Amount, payment.Currency, payment.Description, userID, ctx,
		); err != nil {
			slog.Error("recurrent payment service: update future unpaid", "payment_id", id, "error", err)
		}
	}

	return nil
}

// Stop мягко останавливает шаблон (D7).
func (s *RecurrentPaymentService) Stop(
	id string,
	ctx context.Context,
) error {
	if err := s.repository.Stop(id, s.clk.Today().String(), ctx); err != nil {
		if errors.Is(err, model.ErrRecurrentPaymentNotFound) {
			return model.ErrRecurrentPaymentNotFound
		}
		slog.Error("recurrent payment service: stop payment failed", "payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: stop payment: %w", err)
	}
	return nil
}

func (s *RecurrentPaymentService) GetRecurrentPaymentsByUserID(
	userID string,
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	payments, err := s.repository.GetRecurrentPaymentsByUserID(userID, ctx)
	if err != nil {
		slog.Error("recurrent payment service: get user payments failed",
			"user_id", userID, "error", err)
		return nil, fmt.Errorf("recurrent payment service: get user payments: %w", err)
	}

	return payments, nil
}
