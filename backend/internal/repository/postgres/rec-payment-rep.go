package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

var ErrRecurrentPaymentNotFound = model.ErrRecurrentPaymentNotFound

// ErrInvalidRecurrentPayment — нарушение ссылочной целостности при назначении (например, несуществующий ответственный).
var ErrInvalidRecurrentPayment = errors.New("invalid recurrent payment")

type RecurrentPaymentRepository struct {
	database *DB
}

// scanRecurrentPayment читает строку шаблона.
func scanRecurrentPayment(row interface{ Scan(dest ...any) error }) (model.RecurrentPayment, error) {
	var (
		payment      model.RecurrentPayment
		assigneeID   sql.NullString
		assigneeName sql.NullString
	)

	err := row.Scan(
		&payment.ID,
		&payment.Amount,
		&payment.Currency,
		&payment.Description,
		&payment.Frequency,
		&payment.Interval,
		&payment.DayOffset,
		&payment.StartDate,
		&payment.StoppedAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&assigneeID,
		&assigneeName,
	)
	if err != nil {
		return payment, err
	}

	if assigneeID.Valid && assigneeID.String != "" {
		id := assigneeID.String
		payment.AssigneeID = &id
	}
	if assigneeName.Valid {
		payment.AssigneeName = assigneeName.String
	}

	return payment, nil
}

// recurrentPaymentSelect — общая проекция.
const recurrentPaymentSelect = `
	SELECT
		rp.id::text,
		rp.amount,
		rp.currency,
		rp.description,
		rp.frequency,
		rp."interval",
		rp.day_offset,
		rp.start_date::text,
		rp.stopped_at,
		rp.created_at,
		rp.updated_at,
		a.user_id::text,
		COALESCE(u.full_name, '')
	FROM recurrent_payments rp
	LEFT JOIN LATERAL (
		SELECT ur.user_id
		FROM user_recurrent_payments ur
		WHERE ur.recurrent_payment_id = rp.id
		ORDER BY ur.user_id
		LIMIT 1
	) a ON TRUE
	LEFT JOIN users u ON u.id = a.user_id
`

// CreateRecurrentPayment создаёт рекуррентный платёж без назначения.
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
			"interval",
			day_offset,
			start_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date)
		RETURNING id::text
	`,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.Frequency,
		payment.Interval,
		payment.DayOffset,
		payment.StartDate,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("recurrent payment repository: create payment: %w", err)
	}

	return id, nil
}

// CreateWithAssignee создаёт шаблон и назначает ответственного в одной транзакции.
func (r *RecurrentPaymentRepository) CreateWithAssignee(
	payment model.RecurrentPayment,
	assigneeID string,
	ctx context.Context,
) (string, error) {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("recurrent payment repository: begin create transaction: %w", err)
	}
	defer tx.Rollback()

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO recurrent_payments (
			amount, currency, description, frequency, "interval", day_offset, start_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date)
		RETURNING id::text
	`,
		payment.Amount,
		payment.Currency,
		payment.Description,
		payment.Frequency,
		payment.Interval,
		payment.DayOffset,
		payment.StartDate,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("recurrent payment repository: create payment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id)
		VALUES ($1, $2)
	`, assigneeID, id); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return "", fmt.Errorf("%w: assignee not found", ErrInvalidRecurrentPayment)
		}
		return "", fmt.Errorf("recurrent payment repository: assign user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("recurrent payment repository: commit create transaction: %w", err)
	}

	return id, nil
}

// SetAssignee заменяет единственного ответственного (delete + insert в транзакции).
func (r *RecurrentPaymentRepository) SetAssignee(
	paymentID string,
	userID string,
	ctx context.Context,
) error {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: begin set assignee: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM user_recurrent_payments WHERE recurrent_payment_id = $1
	`, paymentID); err != nil {
		return fmt.Errorf("recurrent payment repository: clear assignee: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_recurrent_payments (user_id, recurrent_payment_id)
		VALUES ($1, $2)
	`, userID, paymentID); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return fmt.Errorf("%w: assignee not found", ErrInvalidRecurrentPayment)
		}
		return fmt.Errorf("recurrent payment repository: set assignee: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recurrent payment repository: commit set assignee: %w", err)
	}

	return nil
}

// GetRecurrentPaymentByID получает платёж по ID (включая остановленные).
func (r *RecurrentPaymentRepository) GetRecurrentPaymentByID(
	id string,
	ctx context.Context,
) (model.RecurrentPayment, error) {
	payment, err := scanRecurrentPayment(
		r.database.db.QueryRowContext(
			ctx,
			recurrentPaymentSelect+` WHERE rp.id = $1`,
			id,
		),
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return payment, model.ErrRecurrentPaymentNotFound
		}
		return payment, fmt.Errorf("recurrent payment repository: get payment by ID: %w", err)
	}

	return payment, nil
}

// GetAllRecurrentPayments получает все шаблоны (вместе с остановленными), отсортированные по описанию.
func (r *RecurrentPaymentRepository) GetAllRecurrentPayments(
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		recurrentPaymentSelect+` ORDER BY rp.description, rp.created_at DESC`,
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

func (r *RecurrentPaymentRepository) ListActiveForSchedule(
	ctx context.Context,
) ([]model.RecurrentPayment, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		recurrentPaymentSelect+` WHERE rp.stopped_at IS NULL ORDER BY rp.created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("recurrent payment repository: list active for schedule: %w", err)
	}
	defer rows.Close()

	payments := []model.RecurrentPayment{}
	for rows.Next() {
		payment, err := scanRecurrentPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("recurrent payment repository: scan active: %w", err)
		}
		payments = append(payments, payment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recurrent payment repository: iterate active: %w", err)
	}

	return payments, nil
}

// UpdateRecurrentPayment обновляет изменяемые поля (сумма, валюта, описание).
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
			updated_at = NOW()
		WHERE id = $4
	`,
		payment.Amount,
		payment.Currency,
		payment.Description,
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
		return model.ErrRecurrentPaymentNotFound
	}

	return nil
}

// Stop — мягкая остановка (D7): проставляет stopped_at и удаляет неоплаченные будущие периоды (due_date > today).
func (r *RecurrentPaymentRepository) Stop(
	id string,
	today string,
	ctx context.Context,
) error {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: begin stop: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE recurrent_payments
		SET stopped_at = COALESCE(stopped_at, NOW()), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("recurrent payment repository: stop payment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("recurrent payment repository: stop rows affected: %w", err)
	}
	if affected == 0 {
		return model.ErrRecurrentPaymentNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM payments
		WHERE recurrent_payment_id = $1
		  AND paid_date IS NULL
		  AND due_date > $2::date
	`, id, today); err != nil {
		return fmt.Errorf("recurrent payment repository: delete future unpaid: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("recurrent payment repository: commit stop: %w", err)
	}

	return nil
}

// DeleteRecurrentPayment — жёсткое удаление шаблона (через HTTP не вызывается; FK payments.recurrent_payment_id = SET NULL сохраняет историю).
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
		return model.ErrRecurrentPaymentNotFound
	}

	return nil
}

// SubscribeUsers назначает пользователей на платёж.
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

// GetUsersByRecurrentPaymentID получает ID пользователей, назначенных на конкретный платёж.
func (r *RecurrentPaymentRepository) GetUsersByRecurrentPaymentID(
	paymentID string,
	ctx context.Context,
) ([]string, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT user_id::text
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
	rows, err := r.database.db.QueryContext(
		ctx,
		recurrentPaymentSelect+`
		WHERE EXISTS (
			SELECT 1 FROM user_recurrent_payments urp
			WHERE urp.recurrent_payment_id = rp.id AND urp.user_id = $1
		)
		ORDER BY rp.created_at DESC`,
		userID,
	)
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
