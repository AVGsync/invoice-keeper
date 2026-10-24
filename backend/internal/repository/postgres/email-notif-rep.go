package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// EmailNotificationRepository — очередь писем-напоминаний (таблица email_notifications).
type EmailNotificationRepository struct {
	database *DB
}

// Candidates отдаёт неоплаченные платежи пользователей с включёнными подтверждёнными письмами.
// Окно грубое: точное решение по часовому поясу и days_before_due принимает воркер.
func (r *EmailNotificationRepository) Candidates(ctx context.Context) ([]model.EmailCandidate, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT s.user_id::text, p.id::text, p.due_date::text, s.timezone,
		       TO_CHAR(s.send_time, 'HH24:MI'), s.days_before_due
		FROM user_notification_settings s
		JOIN payments p ON p.user_id = s.user_id
		JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
		WHERE s.email_enabled AND s.email_verified
		  AND p.paid_date IS NULL AND rp.stopped_at IS NULL
		  AND p.due_date BETWEEN CURRENT_DATE - 5 AND CURRENT_DATE + s.days_before_due + 2`)
	if err != nil {
		return nil, fmt.Errorf("email candidates: %w", err)
	}
	defer rows.Close()
	var out []model.EmailCandidate
	for rows.Next() {
		var c model.EmailCandidate
		if err := rows.Scan(&c.UserID, &c.PaymentID, &c.DueDate, &c.Timezone, &c.SendTime, &c.DaysBefore); err != nil {
			return nil, fmt.Errorf("email candidates scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *EmailNotificationRepository) Enqueue(ctx context.Context, items []model.EmailEnqueue) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("email enqueue: %w", err)
	}
	defer tx.Rollback()
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_notifications (user_id, payment_id, due_date, kind)
			VALUES ($1, $2, $3::date, $4)
			ON CONFLICT (user_id, payment_id, due_date, kind) DO NOTHING`,
			it.UserID, it.PaymentID, it.DueDate, it.Kind); err != nil {
			return fmt.Errorf("email enqueue insert: %w", err)
		}
	}
	return tx.Commit()
}

func (r *EmailNotificationRepository) DueRows(ctx context.Context, now time.Time, limit int) ([]model.EmailReminderRow, error) {
	rows, err := r.database.db.QueryContext(ctx, `
		SELECT en.id::text, en.user_id::text, en.kind, en.due_date::text, en.attempts,
		       p.id::text, p.description, p.amount, p.currency, p.paid_date IS NOT NULL,
		       (rp.id IS NULL OR rp.stopped_at IS NOT NULL), p.due_date::text,
		       s.email, u.full_name, s.timezone, s.email_enabled, s.email_verified
		FROM email_notifications en
		JOIN payments p ON p.id = en.payment_id
		LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
		JOIN users u ON u.id = en.user_id
		JOIN user_notification_settings s ON s.user_id = en.user_id
		WHERE en.state IN ('pending', 'failed') AND en.next_attempt_at <= $1
		ORDER BY en.user_id, p.due_date, p.id
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("email due rows: %w", err)
	}
	defer rows.Close()
	var out []model.EmailReminderRow
	for rows.Next() {
		var x model.EmailReminderRow
		if err := rows.Scan(&x.ID, &x.UserID, &x.Kind, &x.DueDate, &x.Attempts,
			&x.PaymentID, &x.Title, &x.Amount, &x.Currency, &x.Paid, &x.Stopped, &x.PaymentDue,
			&x.Email, &x.FullName, &x.Timezone, &x.Enabled, &x.Verified); err != nil {
			return nil, fmt.Errorf("email due rows scan: %w", err)
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *EmailNotificationRepository) setState(ctx context.Context, ids []string, state string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET state = $2, sent_at = CASE WHEN $2 = 'sent' THEN NOW() ELSE sent_at END, updated_at = NOW()
		WHERE id = ANY($1::uuid[])`, pq.Array(ids), state)
	if err != nil {
		return fmt.Errorf("email set state %s: %w", state, err)
	}
	return nil
}

func (r *EmailNotificationRepository) MarkSent(ctx context.Context, ids []string) error {
	return r.setState(ctx, ids, "sent")
}

func (r *EmailNotificationRepository) Cancel(ctx context.Context, ids []string) error {
	return r.setState(ctx, ids, "cancelled")
}

func (r *EmailNotificationRepository) MarkFailed(ctx context.Context, ids []string, next time.Time, lastErr string, dead bool) error {
	if len(ids) == 0 {
		return nil
	}
	state := "failed"
	if dead {
		state = "dead"
	}
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET state = $2, attempts = attempts + 1, next_attempt_at = $3, last_error = $4, updated_at = NOW()
		WHERE id = ANY($1::uuid[])`, pq.Array(ids), state, next, lastErr)
	if err != nil {
		return fmt.Errorf("email mark failed: %w", err)
	}
	return nil
}

func (r *EmailNotificationRepository) Cleanup(ctx context.Context, before time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		DELETE FROM email_notifications
		WHERE state IN ('sent', 'dead', 'cancelled') AND updated_at < $1`, before)
	if err != nil {
		return fmt.Errorf("email cleanup: %w", err)
	}
	return nil
}
