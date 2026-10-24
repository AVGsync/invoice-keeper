package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
)

var ErrRecurrentPaymentNotFound = errors.New("recurrent payment not found")
var ErrInvalidRecurrentPayment = errors.New("invalid recurrent payment")

type RecurrentPaymentRepository interface {
	CreateRecurrentPayment(payment model.RecurrentPayment, ctx context.Context) (string, error)
	GetRecurrentPaymentByID(id string, ctx context.Context) (model.RecurrentPayment, error)
	GetAllRecurrentPayments(ctx context.Context) ([]model.RecurrentPayment, error)
	UpdateRecurrentPayment(payment model.RecurrentPayment, ctx context.Context) error
	DeleteRecurrentPayment(id string, ctx context.Context) error
	SubscribeUsers(paymentID string, userIDs []string, ctx context.Context) error
	UnsubscribeUsers(paymentID string, userIDs []string, ctx context.Context) error
	GetUsersByRecurrentPaymentID(paymentID string, ctx context.Context) ([]string, error)
	GetRecurrentPaymentsByUserID(userID string, ctx context.Context) ([]model.RecurrentPayment, error)
}

type RecurrentPaymentService struct {
	repository RecurrentPaymentRepository
}

func NewRecurrentPaymentService(
	repository RecurrentPaymentRepository,
) *RecurrentPaymentService {
	return &RecurrentPaymentService{
		repository: repository,
	}
}

func validateRecurrentPayment(
	amount int64,
	currency string,
	frequency string,
	interval int,
	dayOffset int,
) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRecurrentPayment)
	}

	if len(currency) != 3 || strings.TrimSpace(currency) != currency {
		return fmt.Errorf("%w: currency must contain three characters", ErrInvalidRecurrentPayment)
	}

	if interval <= 0 {
		return fmt.Errorf("%w: interval must be greater than zero", ErrInvalidRecurrentPayment)
	}

	switch frequency {
	case "weekly":
		if dayOffset < 0 || dayOffset > 6 {
			return fmt.Errorf("%w: weekly day_offset must be between 0 and 6", ErrInvalidRecurrentPayment)
		}
	case "monthly":
		if dayOffset < 1 || dayOffset > 31 {
			return fmt.Errorf("%w: monthly day_offset must be between 1 and 31", ErrInvalidRecurrentPayment)
		}
	default:
		return fmt.Errorf("%w: frequency must be weekly or monthly", ErrInvalidRecurrentPayment)
	}

	return nil
}

func (s *RecurrentPaymentService) CreateRecurrentPayment(
	req request.CreateRecurrentPaymentRequest,
	ctx context.Context,
) (string, error) {
	if err := validateRecurrentPayment(
		req.Amount,
		req.Currency,
		req.Frequency,
		req.Interval,
		req.DayOffset,
	); err != nil {
		return "", err
	}

	payment := model.RecurrentPayment{
		Amount:      req.Amount,
		Currency:    strings.ToUpper(req.Currency),
		Description: req.Description,
		Frequency:   req.Frequency,
		Interval:    req.Interval,
		DayOffset:   req.DayOffset,
	}

	id, err := s.repository.CreateRecurrentPayment(payment, ctx)
	if err != nil {
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
		if errors.Is(err, sql.ErrNoRows) {
			return model.RecurrentPayment{}, ErrRecurrentPaymentNotFound
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
	if err := validateRecurrentPayment(
		req.Amount,
		req.Currency,
		req.Frequency,
		req.Interval,
		req.DayOffset,
	); err != nil {
		return err
	}

	payment, err := s.repository.GetRecurrentPaymentByID(id, ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecurrentPaymentNotFound
		}

		slog.Error("recurrent payment service: update payment: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: update payment: get payment: %w", err)
	}

	payment.Amount = req.Amount
	payment.Currency = strings.ToUpper(req.Currency)
	payment.Description = req.Description
	payment.Frequency = req.Frequency
	payment.Interval = req.Interval
	payment.DayOffset = req.DayOffset

	if err := s.repository.UpdateRecurrentPayment(payment, ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecurrentPaymentNotFound
		}

		slog.Error("recurrent payment service: update payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: update payment: %w", err)
	}

	return nil
}

func (s *RecurrentPaymentService) DeleteRecurrentPayment(
	id string,
	ctx context.Context,
) error {
	_, err := s.repository.GetRecurrentPaymentByID(id, ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecurrentPaymentNotFound
		}

		slog.Error("recurrent payment service: delete payment: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: delete payment: get payment: %w", err)
	}

	if err := s.repository.DeleteRecurrentPayment(id, ctx); err != nil {
		slog.Error("recurrent payment service: delete payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("recurrent payment service: delete payment: %w", err)
	}

	return nil
}

func (s *RecurrentPaymentService) SubscribeUsers(
	paymentID string,
	userIDs []string,
	ctx context.Context,
) error {
	if len(userIDs) == 0 {
		return nil
	}

	if _, err := s.GetRecurrentPaymentByID(paymentID, ctx); err != nil {
		return err
	}

	if err := s.repository.SubscribeUsers(paymentID, userIDs, ctx); err != nil {
		slog.Error("recurrent payment service: subscribe users failed",
			"payment_id", paymentID, "error", err)
		return fmt.Errorf("recurrent payment service: subscribe users: %w", err)
	}

	return nil
}

func (s *RecurrentPaymentService) UnsubscribeUsers(
	paymentID string,
	userIDs []string,
	ctx context.Context,
) error {
	if len(userIDs) == 0 {
		return nil
	}

	if _, err := s.GetRecurrentPaymentByID(paymentID, ctx); err != nil {
		return err
	}

	if err := s.repository.UnsubscribeUsers(paymentID, userIDs, ctx); err != nil {
		slog.Error("recurrent payment service: unsubscribe users failed",
			"payment_id", paymentID, "error", err)
		return fmt.Errorf("recurrent payment service: unsubscribe users: %w", err)
	}

	return nil
}

func (s *RecurrentPaymentService) GetUsersByRecurrentPaymentID(
	paymentID string,
	ctx context.Context,
) ([]string, error) {
	if _, err := s.GetRecurrentPaymentByID(paymentID, ctx); err != nil {
		return nil, err
	}

	userIDs, err := s.repository.GetUsersByRecurrentPaymentID(paymentID, ctx)
	if err != nil {
		slog.Error("recurrent payment service: get payment users failed",
			"payment_id", paymentID, "error", err)
		return nil, fmt.Errorf("recurrent payment service: get payment users: %w", err)
	}

	return userIDs, nil
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
