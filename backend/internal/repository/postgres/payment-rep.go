package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Алиасы на единые sentinel-ошибки модели — чтобы код B, ссылающийся на postgres.ErrPaymentNotFound, продолжал собираться.
var (
	ErrPaymentNotFound    = model.ErrPaymentNotFound
	ErrPaymentAlreadyPaid = model.ErrPaymentAlreadyPaid
)

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

// scanPayment читает период.
func scanPayment(row rowScanner) (model.Payment, error) {
	var (
		payment      model.Payment
		paidDate     sql.NullString
		assigneeName sql.NullString
	)

	err := row.Scan(
		&payment.ID,
		&payment.UserID,
		&payment.RecurrentPaymentID,
		&payment.Amount,
		&payment.Currency,
		&payment.Description,
		&payment.DueDate,
		&paidDate,
		&payment.PaidAt,
		&payment.Status,
		&assigneeName,
		&payment.Ended,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return model.Payment{}, err
	}

	if paidDate.Valid && paidDate.String != "" {
		pd := paidDate.String
		payment.PaidDate = &pd
	}
	if assigneeName.Valid {
		payment.AssigneeName = assigneeName.String
	}

	return payment, nil
}

const paymentSelect = `
	SELECT
		p.id::text,
		p.user_id::text,
		p.recurrent_payment_id::text,
		p.amount,
		p.currency,
		p.description,
		p.due_date::text,
		p.paid_date::text,
		p.paid_at,
		p.status,
		COALESCE(u.full_name, ''),
		(p.recurrent_payment_id IS NULL OR rp.stopped_at IS NOT NULL) AS ended,
		p.created_at,
		p.updated_at
	FROM payments p
	LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
	LEFT JOIN users u ON u.id = p.user_id
`

// CreatePayment создаёт одиночный период.
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
		VALUES ($1, $2, $3, $4, $5, $6::date, 'pending')
		RETURNING id::text
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

// InsertGenerated вставляет сгенерированные периоды, пропуская дубли по (recurrent_payment_id, due_date).
func (r *PaymentRepository) InsertGenerated(
	rows []model.Payment,
	ctx context.Context,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("insert generated: begin: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO payments (
			user_id, recurrent_payment_id, amount, currency, description, due_date, status
		)
		SELECT $1, $2, $3, $4, $5, $6::date, $7
		WHERE EXISTS (
			SELECT 1 FROM recurrent_payments WHERE id = $2 AND stopped_at IS NULL
		)
		ON CONFLICT (recurrent_payment_id, due_date) WHERE recurrent_payment_id IS NOT NULL
		DO NOTHING
	`

	inserted := 0
	for _, p := range rows {
		res, err := tx.ExecContext(ctx, query,
			p.UserID, p.RecurrentPaymentID, p.Amount, p.Currency, p.Description, p.DueDate, p.Status,
		)
		if err != nil {
			return 0, fmt.Errorf("insert generated: exec: %w", err)
		}
		if n, err := res.RowsAffected(); err == nil {
			inserted += int(n)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("insert generated: commit: %w", err)
	}

	return inserted, nil
}

func (r *PaymentRepository) GetPaymentByID(
	id string,
	ctx context.Context,
) (model.Payment, error) {
	query := paymentSelect + ` WHERE p.id = $1`

	payment, err := scanPayment(r.database.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payment{}, model.ErrPaymentNotFound
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
		WHERE p.user_id = $1
		ORDER BY p.due_date, p.created_at
	`

	return r.queryPayments(ctx, query, userID)
}

func (r *PaymentRepository) GetAllPayments(
	ctx context.Context,
) ([]model.Payment, error) {
	query := paymentSelect + ` ORDER BY p.due_date, p.created_at`

	return r.queryPayments(ctx, query)
}

func (r *PaymentRepository) queryPayments(
	ctx context.Context,
	query string,
	args ...any,
) ([]model.Payment, error) {
	rows, err := r.database.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
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

// UpdatePayment меняет сумму, валюту и описание.
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
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows after payment update: %w", err)
	}

	if affected == 0 {
		return model.ErrPaymentNotFound
	}

	return nil
}

func (r *PaymentRepository) UpdateFutureUnpaid(
	rpID string,
	today string,
	amount int64,
	currency string,
	description string,
	userID string,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET amount = $3, currency = $4, description = $5, user_id = $6, updated_at = NOW()
		WHERE recurrent_payment_id = $1
		  AND paid_date IS NULL
		  AND due_date >= $2::date
	`
	if _, err := r.database.db.ExecContext(ctx, query,
		rpID, today, amount, currency, description, userID,
	); err != nil {
		return fmt.Errorf("update future unpaid: %w", err)
	}
	return nil
}

// MarkPaymentAsPaid отмечает оплату на дату paidDate ("YYYY-MM-DD").
func (r *PaymentRepository) MarkPaymentAsPaid(
	id string,
	paidDate string,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET
			status = 'paid',
			paid_date = $2::date,
			paid_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND paid_date IS NULL
	`

	result, err := r.database.db.ExecContext(ctx, query, id, paidDate)
	if err != nil {
		return fmt.Errorf("mark payment as paid: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark payment as paid rows affected: %w", err)
	}
	if affected == 0 {
		return r.notFoundOrAlreadyPaid(id, ctx)
	}

	return nil
}

// ChangePaidDate меняет дату уже оплаченного периода (исправление факта).
func (r *PaymentRepository) ChangePaidDate(
	id string,
	paidDate string,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET paid_date = $2::date, paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND paid_date IS NOT NULL
	`

	result, err := r.database.db.ExecContext(ctx, query, id, paidDate)
	if err != nil {
		return fmt.Errorf("change paid date: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("change paid date rows affected: %w", err)
	}
	if affected == 0 {
		// Строки нет вовсе, либо она ещё не оплачена.
		if _, err := r.GetPaymentByID(id, ctx); err != nil {
			return err
		}
		return model.ErrPaymentNotFound
	}

	return nil
}

// MarkPaymentAsUnpaid снимает оплату, очищает дату и пересчитывает статус.
func (r *PaymentRepository) MarkPaymentAsUnpaid(
	id string,
	today string,
	ctx context.Context,
) error {
	const query = `
		UPDATE payments
		SET
			status = CASE WHEN due_date < $2::date THEN 'overdue' ELSE 'pending' END,
			paid_date = NULL,
			paid_at = NULL,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.database.db.ExecContext(ctx, query, id, today)
	if err != nil {
		return fmt.Errorf("mark payment as unpaid: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark payment as unpaid rows affected: %w", err)
	}
	if affected == 0 {
		return model.ErrPaymentNotFound
	}

	return nil
}

// notFoundOrAlreadyPaid различает «нет строки» и «уже оплачено» после UPDATE 0 rows.
func (r *PaymentRepository) notFoundOrAlreadyPaid(id string, ctx context.Context) error {
	var exists bool
	err := r.database.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM payments WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check payment existence: %w", err)
	}
	if exists {
		return model.ErrPaymentAlreadyPaid
	}
	return model.ErrPaymentNotFound
}

// SweepOverdue массово переводит pending → overdue для периодов с истёкшим сроком.
func (r *PaymentRepository) SweepOverdue(
	today string,
	ctx context.Context,
) (int64, error) {
	const query = `
		UPDATE payments
		SET status = 'overdue', updated_at = NOW()
		WHERE status = 'pending' AND paid_date IS NULL AND due_date < $1::date
	`

	result, err := r.database.db.ExecContext(ctx, query, today)
	if err != nil {
		return 0, fmt.Errorf("sweep overdue: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sweep overdue rows affected: %w", err)
	}
	return affected, nil
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
		return model.ErrPaymentNotFound
	}

	return nil
}
