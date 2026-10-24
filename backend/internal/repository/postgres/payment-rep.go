package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

var ErrPaymentNotFound = errors.New("payment not found")

type PaymentRepository struct {
	database *DB
}

func NewPaymentRepository(database *DB) *PaymentRepository {
	return &PaymentRepository{
		database: database,
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPayment(row rowScanner) (model.Payment, error) {
	var payment model.Payment

	err := row.Scan(
		&payment.ID,
		&payment.UserID,
		&payment.RecurrentPaymentID,
		&payment.Amount,
		&payment.Currency,
		&payment.Description,
		&payment.DueDate,
		&payment.PaidAt,
		&payment.Status,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return model.Payment{}, err
	}

	return payment, nil
}

const paymentSelect = `
	SELECT
		id,
		user_id,
		recurrent_payment_id,
		amount,
		currency,
		description,
		due_date,
		paid_at,
		status,
		created_at,
		updated_at
	FROM payments
`

func (r *PaymentRepository) CreatePayment(
	payment model.Payment,
	ctx context.Context,
) (string, error) {
	const query = `
		INSERT INTO payments (
			user_id,
			recurrent_payment_id,
			amount,
			currency,
			description,
			due_date,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id
	`

	var id string

	err := r.database.db.QueryRowContext(
		ctx,
		query,
		payment.UserID,
		payment.RecurrentPaymentID,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.DueDate,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("create payment: %w", err)
	}

	return id, nil
}

func (r *PaymentRepository) GetPaymentByID(
	id string,
	ctx context.Context,
) (model.Payment, error) {
	query := paymentSelect + ` WHERE id = $1`

	payment, err := scanPayment(r.database.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payment{}, ErrPaymentNotFound
		}

		return model.Payment{}, fmt.Errorf("get payment by id: %w", err)
	}

	return payment, nil
}

func (r *PaymentRepository) GetPaymentsByUserID(
	userID string,
	ctx context.Context,
) ([]model.Payment, error) {
	query := paymentSelect + `
		WHERE user_id = $1
		ORDER BY due_date DESC, created_at DESC
	`

	rows, err := r.database.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get payments by user id: %w", err)
	}
	defer rows.Close()

	payments := make([]model.Payment, 0)

	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}

		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}

	return payments, nil
}

func (r *PaymentRepository) GetAllPayments(
	ctx context.Context,
) ([]model.Payment, error) {
	query := paymentSelect + ` ORDER BY due_date DESC, created_at DESC`

	rows, err := r.database.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all payments: %w", err)
	}
	defer rows.Close()

	payments := make([]model.Payment, 0)

	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}

		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}

	return payments, nil
}

func (r *PaymentRepository) UpdatePayment(
	payment model.Payment,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET
			amount = $2,
			currency = $3,
			description = $4,
			due_date = $5,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.database.db.ExecContext(
		ctx,
		query,
		payment.ID,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.DueDate,
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows after payment update: %w", err)
	}

	if affected == 0 {
		return ErrPaymentNotFound
	}

	return nil
}

func (r *PaymentRepository) MarkPaymentAsPaid(
	id string,
	paidAt time.Time,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET
			status = 'paid',
			paid_at = $2,
			updated_at = NOW()
		WHERE id = $1
			AND status IN ('pending', 'overdue')
		RETURNING id
	`

	var updatedID string

	err := r.database.db.QueryRowContext(ctx, query, id, paidAt).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPaymentNotFound
		}

		return fmt.Errorf("mark payment as paid: %w", err)
	}

	return nil
}

func (r *PaymentRepository) MarkPaymentAsOverdue(
	id string,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET
			status = 'overdue',
			updated_at = NOW()
		WHERE id = $1
			AND status = 'pending'
			AND due_date < CURRENT_DATE
		RETURNING id
	`

	var updatedID string

	err := r.database.db.QueryRowContext(ctx, query, id).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPaymentNotFound
		}

		return fmt.Errorf("mark payment as overdue: %w", err)
	}

	return nil
}

func (r *PaymentRepository) DeletePayment(
	id string,
	ctx context.Context,
) error {
	const query = `DELETE FROM payments WHERE id = $1`

	result, err := r.database.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete payment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows after payment deletion: %w", err)
	}

	if affected == 0 {
		return ErrPaymentNotFound
	}

	return nil
}
