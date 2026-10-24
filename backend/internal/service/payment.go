package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/response"
)

var ErrInvalidPayment = errors.New("invalid payment")

type PaymentRepository interface {
	CreatePayment(payment model.Payment, ctx context.Context) (string, error)
	GetPaymentByID(id string, ctx context.Context) (model.Payment, error)
	GetPaymentsByUserID(userID string, ctx context.Context) ([]model.Payment, error)
	GetAllPayments(ctx context.Context) ([]model.Payment, error)
	UpdatePayment(payment model.Payment, ctx context.Context) error
	MarkPaymentAsPaid(id string, paidAt time.Time, ctx context.Context) error
	MarkPaymentAsOverdue(id string, ctx context.Context) error
	DeletePayment(id string, ctx context.Context) error
}

type PaymentService struct {
	repository PaymentRepository
}

func NewPaymentService(repository PaymentRepository) *PaymentService {
	return &PaymentService{
		repository: repository,
	}
}

func validatePayment(amount int64, currency string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidPayment)
	}

	if len(currency) != 3 || strings.TrimSpace(currency) != currency {
		return fmt.Errorf("%w: currency must contain three characters", ErrInvalidPayment)
	}

	return nil
}

func (s *PaymentService) CreatePayment(
	req request.CreatePaymentRequest,
	ctx context.Context,
) (string, error) {
	if err := validatePayment(req.Amount, req.Currency); err != nil {
		return "", err
	}

	if strings.TrimSpace(req.UserID) == "" {
		return "", fmt.Errorf("%w: user_id is required", ErrInvalidPayment)
	}

	if req.DueDate.IsZero() {
		return "", fmt.Errorf("%w: due_date is required", ErrInvalidPayment)
	}

	payment := model.Payment{
		UserID:             req.UserID,
		RecurrentPaymentID: req.RecurrentPaymentID,
		Amount:             req.Amount,
		Currency:           strings.ToUpper(req.Currency),
		Description:        req.Description,
		DueDate:            req.DueDate,
		Status:             "pending",
	}

	id, err := s.repository.CreatePayment(payment, ctx)
	if err != nil {
		slog.Error("payment service: create payment failed", "error", err)
		return "", fmt.Errorf("payment service: create payment: %w", err)
	}

	return id, nil
}

func (s *PaymentService) GetPaymentByID(
	id string,
	ctx context.Context,
) (model.Payment, error) {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		slog.Error("payment service: get payment failed",
			"payment_id", id, "error", err)
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

func (s *PaymentService) GetPaymentsByUserIDWithTotal(
	userID string,
	ctx context.Context,
) (response.PaymentListResponse, error) {
	payments, err := s.repository.GetPaymentsByUserID(userID, ctx)
	if err != nil {
		slog.Error("payment service: get payments with total failed",
			"user_id", userID, "error", err)
		return response.PaymentListResponse{},
			fmt.Errorf("payment service: get payments with total: %w", err)
	}

	return response.NewPaymentListResponse(payments), nil
}

func (s *PaymentService) UpdatePayment(
	id string,
	req request.UpdatePaymentRequest,
	ctx context.Context,
) error {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		slog.Error("payment service: update payment: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: update payment: %w", err)
	}

	if payment.Status == "paid" {
		return fmt.Errorf("%w: paid payment cannot be edited", ErrInvalidPayment)
	}

	if req.Amount != nil {
		payment.Amount = *req.Amount
	}

	if req.Currency != nil {
		payment.Currency = strings.ToUpper(*req.Currency)
	}

	if req.Description != nil {
		payment.Description = *req.Description
	}

	if req.DueDate != nil {
		payment.DueDate = *req.DueDate
	}

	if err := validatePayment(payment.Amount, payment.Currency); err != nil {
		return err
	}

	if req.DueDate != nil && req.DueDate.IsZero() {
		return fmt.Errorf("%w: due_date is invalid", ErrInvalidPayment)
	}

	if err := s.repository.UpdatePayment(payment, ctx); err != nil {
		slog.Error("payment service: update payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: update payment: %w", err)
	}

	return nil
}

func (s *PaymentService) MarkPaymentAsPaid(
	id string,
	ctx context.Context,
) error {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		slog.Error("payment service: mark as paid: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: mark as paid: %w", err)
	}

	if payment.Status == "paid" {
		return fmt.Errorf("%w: payment is already paid", ErrInvalidPayment)
	}

	if err := s.repository.MarkPaymentAsPaid(id, time.Now(), ctx); err != nil {
		slog.Error("payment service: mark as paid failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: mark as paid: %w", err)
	}

	return nil
}

func (s *PaymentService) MarkPaymentAsOverdue(
	id string,
	ctx context.Context,
) error {
	payment, err := s.repository.GetPaymentByID(id, ctx)
	if err != nil {
		slog.Error("payment service: mark as overdue: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: mark as overdue: %w", err)
	}

	if payment.Status == "paid" {
		return fmt.Errorf("%w: paid payment cannot become overdue", ErrInvalidPayment)
	}

	today := time.Now().Format("2006-01-02")
	dueDate := payment.DueDate.Format("2006-01-02")

	if dueDate >= today {
		return fmt.Errorf("%w: payment due date has not passed", ErrInvalidPayment)
	}

	if payment.Status == "overdue" {
		return nil
	}

	if err := s.repository.MarkPaymentAsOverdue(id, ctx); err != nil {
		slog.Error("payment service: mark as overdue failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: mark as overdue: %w", err)
	}

	return nil
}

func (s *PaymentService) DeletePayment(
	id string,
	ctx context.Context,
) error {
	if _, err := s.repository.GetPaymentByID(id, ctx); err != nil {
		slog.Error("payment service: delete payment: get payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: delete payment: %w", err)
	}

	if err := s.repository.DeletePayment(id, ctx); err != nil {
		slog.Error("payment service: delete payment failed",
			"payment_id", id, "error", err)
		return fmt.Errorf("payment service: delete payment: %w", err)
	}

	return nil
}
