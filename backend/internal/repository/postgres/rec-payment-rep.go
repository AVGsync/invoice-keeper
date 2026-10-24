package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

var ErrRecurrentPaymentNotFound = errors.New("recurrent payment not found")

type RecurrentPaymentRepository struct {
	database *DB
}

func scanRecurrentPayment(row interface{ Scan(dest ...any) error }) (model.RecurrentPayment, error) {
	var payment model.RecurrentPayment

	err := row.Scan(
		&payment.ID,
		&payment.Amount,
		&payment.Currency,
		&payment.Description,
		&payment.Frequency,
		&payment.Interval,
		&payment.DayOffset,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)

	return payment, err
}

const recurrentPaymentSelect = `
	SELECT
		id,
		amount,
		currency,
		description,
		frequency,
		interval,
		day_offset,
		created_at,
		updated_at
	FROM recurrent_payments
`

// CreateRecurrentPayment создаёт рекуррентный платёж.
func (r *RecurrentPaymentRepository) CreateRecurrentPayment(
	payment model.RecurrentPayment,
	ctx context.Context,
) (string, error) {
	var id string

	err := r.database.db.QueryRowContext(ctx, `
		INSERT INTO recurrent_payments (
			amount,
			currency,
			description,
			frequency,
			interval,
			day_offset
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.Frequency,
		payment.Interval,
		payment.DayOffset,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("recurrent payment repository: create payment: %w", err)
	}

	return id, nil
}

// GetRecurrentPaymentByID получает платёж по ID.
func (r *RecurrentPaymentRepository) GetRecurrentPaymentByID(
	id string,
	ctx context.Context,
) (model.RecurrentPayment, error) {
	payment, err := scanRecurrentPayment(
		r.database.db.QueryRowContext(
			ctx,
			recurrentPaymentSelect+` WHERE id = $1`,
			id,
		),
	)

	if err != nil {
		if errors.Is(err, errors.New("sql: no rows in result set")) {
			return payment, ErrRecurrentPaymentNotFound
		}
		return payment, fmt.Errorf("recurrent payment repository: get payment by ID: %w", err)
	}

	return payment, nil
}

// GetAllRecurrentPayments получает все рекуррентные платежи.
func (r *RecurrentPaymentRepository) GetAllRecurrentPayments(
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		recurrentPaymentSelect+` ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("recurrent payment repository: get all payments: %w", err)
	}
	defer rows.Close()

	payments := []model.RecurrentPayment{}

	for rows.Next() {
		payment, err := scanRecurrentPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("recurrent payment repository: scan payment: %w", err)
		}
		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recurrent payment repository: iterate payments: %w", err)
	}

	return payments, nil
}

// UpdateRecurrentPayment обновляет платёж.
func (r *RecurrentPaymentRepository) UpdateRecurrentPayment(
	payment model.RecurrentPayment,
	ctx context.Context,
) error {
	result, err := r.database.db.ExecContext(ctx, `
		UPDATE recurrent_payments
		SET
			amount = $1,
			currency = $2,
			description = $3,
			frequency = $4,
			interval = $5,
			day_offset = $6,
			updated_at = NOW()
		WHERE id = $7
	`,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.Frequency,
		payment.Interval,
		payment.DayOffset,
		payment.ID,
	)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: update payment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("recurrent payment repository: update payment rows affected: %w", err)
	}

	if rows == 0 {
		return ErrRecurrentPaymentNotFound
	}

	return nil
}

// DeleteRecurrentPayment удаляет платёж.
func (r *RecurrentPaymentRepository) DeleteRecurrentPayment(
	id string,
	ctx context.Context,
) error {
	result, err := r.database.db.ExecContext(ctx, `
		DELETE FROM recurrent_payments
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: delete payment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("recurrent payment repository: delete payment rows affected: %w", err)
	}

	if rows == 0 {
		return ErrRecurrentPaymentNotFound
	}

	return nil
}

// SubscribeUsers назначает пользователей на платёж.
// Операция выполняется целиком в транзакции.
func (r *RecurrentPaymentRepository) SubscribeUsers(
	paymentID string,
	userIDs []string,
	ctx context.Context,
) error {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: begin subscribe transaction: %w", err)
	}
	defer tx.Rollback()

	for _, userID := range userIDs {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO user_recurrent_payments (
				user_id,
				recurrent_payment_id
			)
			VALUES ($1, $2)
			ON CONFLICT (user_id, recurrent_payment_id) DO NOTHING
		`, userID, paymentID)

		if err != nil {
			return fmt.Errorf(
				"recurrent payment repository: subscribe user %s: %w",
				userID,
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recurrent payment repository: commit subscribe transaction: %w", err)
	}

	return nil
}

// UnsubscribeUsers снимает назначение платежа с пользователей.
func (r *RecurrentPaymentRepository) UnsubscribeUsers(
	paymentID string,
	userIDs []string,
	ctx context.Context,
) error {
	_, err := r.database.db.ExecContext(ctx, `
		DELETE FROM user_recurrent_payments
		WHERE recurrent_payment_id = $1
		  AND user_id = ANY($2::uuid[])
	`, paymentID, pq.Array(userIDs))

	if err != nil {
		return fmt.Errorf("recurrent payment repository: unsubscribe users: %w", err)
	}

	return nil
}

// GetUsersByRecurrentPaymentID получает ID пользователей,
// назначенных на конкретный платёж.
func (r *RecurrentPaymentRepository) GetUsersByRecurrentPaymentID(
	paymentID string,
	ctx context.Context,
) ([]string, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT user_id
		FROM user_recurrent_payments
		WHERE recurrent_payment_id = $1
		ORDER BY user_id
	`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("recurrent payment repository: get payment users: %w", err)
	}
	defer rows.Close()

	userIDs := []string{}

	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("recurrent payment repository: scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recurrent payment repository: iterate payment users: %w", err)
	}

	return userIDs, nil
}

// GetRecurrentPaymentsByUserID получает платежи конкретного пользователя.
func (r *RecurrentPaymentRepository) GetRecurrentPaymentsByUserID(
	userID string,
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT
			rp.id,
			rp.amount,
			rp.currency,
			rp.description,
			rp.frequency,
			rp.interval,
			rp.day_offset,
			rp.created_at,
			rp.updated_at
		FROM recurrent_payments rp
		INNER JOIN user_recurrent_payments urp
			ON urp.recurrent_payment_id = rp.id
		WHERE urp.user_id = $1
		ORDER BY rp.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("recurrent payment repository: get user payments: %w", err)
	}
	defer rows.Close()

	payments := []model.RecurrentPayment{}

	for rows.Next() {
		payment, err := scanRecurrentPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("recurrent payment repository: scan user payment: %w", err)
		}
		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recurrent payment repository: iterate user payments: %w", err)
	}

	return payments, nil
}
