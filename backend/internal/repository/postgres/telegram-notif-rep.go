package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
)

// TelegramNotificationRepository — outbox напоминаний.
type TelegramNotificationRepository struct {
	database *DB
}

// TelegramNotifications возвращает репозиторий outbox напоминаний.
func (d *DB) TelegramNotifications() *TelegramNotificationRepository {
	return &TelegramNotificationRepository{database: d}
}

const telegramNotifSelect = `
	SELECT id, occurrence_id, due_date::text, kind, recipient, state, attempts,
	       next_attempt_at, last_error, chat_id, message_id, shown_status,
	       created_at, sent_at, updated_at
	FROM telegram_notifications`

func scanNotification(rows interface{ Scan(dest ...any) error }) (model.TelegramNotification, error) {
	var (
		n         model.TelegramNotification
		dueStr    string
		lastErr   sql.NullString
		chatID    sql.NullInt64
		messageID sql.NullInt64
		sentAt    sql.NullTime
	)
	if err := rows.Scan(&n.ID, &n.OccurrenceID, &dueStr, &n.Kind, &n.Recipient, &n.State, &n.Attempts,
		&n.NextAttemptAt, &lastErr, &chatID, &messageID, &n.ShownStatus,
		&n.CreatedAt, &sentAt, &n.UpdatedAt); err != nil {
		return n, err
	}
	d, err := clock.ParseDate(dueStr)
	if err != nil {
		return n, fmt.Errorf("telegram notif repository: parse due_date: %w", err)
	}
	n.DueDate = d
	if lastErr.Valid {
		n.LastError = &lastErr.String
	}
	if chatID.Valid {
		n.ChatID = &chatID.Int64
	}
	if messageID.Valid {
		n.MessageID = &messageID.Int64
	}
	if sentAt.Valid {
		n.SentAt = &sentAt.Time
	}
	return n, nil
}

// Enqueue ставит строки с дедупом; cancelled возвращает в pending.
func (r *TelegramNotificationRepository) Enqueue(ctx context.Context, items []reminder.EnqueueItem) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("telegram notif repository: enqueue: begin: %w", err)
	}
	defer tx.Rollback()

	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO telegram_notifications (occurrence_id, due_date, kind, recipient)
			VALUES ($1, $2::date, $3, $4)
			ON CONFLICT (occurrence_id, due_date, kind, recipient)
			DO UPDATE SET state = 'pending', next_attempt_at = NOW(), updated_at = NOW()
			WHERE telegram_notifications.state = 'cancelled'`,
			it.OccurrenceID, it.DueDate.String(), it.Kind, it.Recipient); err != nil {
			return fmt.Errorf("telegram notif repository: enqueue: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("telegram notif repository: enqueue: commit: %w", err)
	}
	return nil
}

// DueForSend возвращает строки, которым пора отправиться.
func (r *TelegramNotificationRepository) DueForSend(ctx context.Context, now time.Time, limit int) ([]model.TelegramNotification, error) {
	rows, err := r.database.db.QueryContext(ctx, telegramNotifSelect+`
		WHERE state IN ('pending','failed') AND next_attempt_at <= $1
		ORDER BY next_attempt_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("telegram notif repository: due for send: %w", err)
	}
	return collectNotifications(rows)
}

// OpenForReconcile — отправленные открытые/оплаченные сообщения для правок.
func (r *TelegramNotificationRepository) OpenForReconcile(ctx context.Context, dueOnOrAfter clock.Date, limit int) ([]model.TelegramNotification, error) {
	rows, err := r.database.db.QueryContext(ctx, telegramNotifSelect+`
		WHERE state = 'sent' AND shown_status IN ('open','paid') AND due_date >= $1::date
		ORDER BY updated_at
		LIMIT $2`, dueOnOrAfter.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("telegram notif repository: open for reconcile: %w", err)
	}
	return collectNotifications(rows)
}

// OpenByOccurrence — отправленные открытые/оплаченные строки одного периода.
func (r *TelegramNotificationRepository) OpenByOccurrence(ctx context.Context, occurrenceID string) ([]model.TelegramNotification, error) {
	rows, err := r.database.db.QueryContext(ctx, telegramNotifSelect+`
		WHERE occurrence_id = $1 AND state = 'sent' AND shown_status IN ('open','paid')`, occurrenceID)
	if err != nil {
		return nil, fmt.Errorf("telegram notif repository: open by occurrence: %w", err)
	}
	return collectNotifications(rows)
}

func collectNotifications(rows *sql.Rows) ([]model.TelegramNotification, error) {
	defer rows.Close()
	var out []model.TelegramNotification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("telegram notif repository: scan: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telegram notif repository: rows: %w", err)
	}
	return out, nil
}

// MarkSent фиксирует успешную отправку.
func (r *TelegramNotificationRepository) MarkSent(ctx context.Context, id string, chatID, messageID int64, now time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET state = 'sent', chat_id = $2, message_id = $3, sent_at = $4, last_error = NULL, updated_at = NOW()
		WHERE id = $1`, id, chatID, messageID, now)
	if err != nil {
		return fmt.Errorf("telegram notif repository: mark sent: %w", err)
	}
	return nil
}

// MarkFailed откладывает повтор; incrementAttempt=false для 429.
func (r *TelegramNotificationRepository) MarkFailed(ctx context.Context, id string, nextAttemptAt time.Time, incrementAttempt bool, lastErr string, now time.Time) error {
	inc := 0
	if incrementAttempt {
		inc = 1
	}
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET state = 'failed', next_attempt_at = $2, attempts = attempts + $3, last_error = $4, updated_at = NOW()
		WHERE id = $1`, id, nextAttemptAt, inc, lastErr)
	if err != nil {
		return fmt.Errorf("telegram notif repository: mark failed: %w", err)
	}
	return nil
}

// MarkDead переводит строку в финальное состояние dead.
func (r *TelegramNotificationRepository) MarkDead(ctx context.Context, id, lastErr string, now time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET state = 'dead', last_error = $2, updated_at = NOW()
		WHERE id = $1`, id, lastErr)
	if err != nil {
		return fmt.Errorf("telegram notif repository: mark dead: %w", err)
	}
	return nil
}

// Cancel отменяет строку (период оплачен/завершён/срок перенесён).
func (r *TelegramNotificationRepository) Cancel(ctx context.Context, id string, now time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET state = 'cancelled', updated_at = NOW()
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("telegram notif repository: cancel: %w", err)
	}
	return nil
}

// SetShownStatus меняет показанный статус отправленного сообщения.
func (r *TelegramNotificationRepository) SetShownStatus(ctx context.Context, id, shownStatus string, now time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET shown_status = $2, updated_at = NOW()
		WHERE id = $1`, id, shownStatus)
	if err != nil {
		return fmt.Errorf("telegram notif repository: set shown status: %w", err)
	}
	return nil
}

// ResetFailedByRecipient возвращает отложенные строки адресата к немедленной отправке.
func (r *TelegramNotificationRepository) ResetFailedByRecipient(ctx context.Context, recipient string, now time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET next_attempt_at = $2, updated_at = NOW()
		WHERE recipient = $1 AND state = 'failed'`, recipient, now)
	if err != nil {
		return fmt.Errorf("telegram notif repository: reset failed: %w", err)
	}
	return nil
}

// Cleanup удаляет давно завершённые строки outbox (dead/cancelled).
func (r *TelegramNotificationRepository) Cleanup(ctx context.Context, before time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		DELETE FROM telegram_notifications
		WHERE state IN ('dead','cancelled') AND updated_at < $1`, before)
	if err != nil {
		return fmt.Errorf("telegram notif repository: cleanup: %w", err)
	}
	return nil
}
