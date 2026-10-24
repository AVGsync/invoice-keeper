package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// TelegramSourceRepository — SQL-чтение периодов модуля платежей B. Реализует
// reminder.OccurrenceSource. Это ЕДИНСТВЕННОЕ место, где бот читает таблицы B,
// без VIEW и FK (docs/TELEGRAM_BOT.md §2.3). Фабрика объявлена здесь, db.go не
// трогаем.
//
// Реальная схема B (миграции 000006/000008 + 000010):
//
//	payments(id uuid, user_id uuid, recurrent_payment_id uuid NULL, amount bigint
//	         минимальные единицы, currency, description, due_date date,
//	         paid_date date NULL, status)
//	recurrent_payments(id uuid, description, stopped_at timestamptz NULL)
//	users(id uuid, full_name)
//
// Заголовок берётся из payments.description c откатом на recurrent_payments.description;
// amount переводится в мажорные единицы (amount/100) десятичной строкой без float.
type TelegramSourceRepository struct {
	database *DB
}

// TelegramSource возвращает SQL-источник периодов.
func (d *DB) TelegramSource() *TelegramSourceRepository {
	return &TelegramSourceRepository{database: d}
}

// Общая проекция. due_date как ::text — чтобы DATE не сдвигалась зоной (§5.2);
// amount как numeric/100::text — десятичная строка без float. ended = платёж
// удалён (recurrent_payment_id IS NULL) или остановлен (stopped_at IS NOT NULL).
const telegramSourceSelect = `
	SELECT p.id::text,
	       COALESCE(p.recurrent_payment_id::text, ''),
	       COALESCE(NULLIF(p.description, ''), rp.description, ''),
	       (p.amount::numeric / 100)::numeric(20,2)::text,
	       p.currency,
	       p.due_date::text,
	       (p.paid_date IS NOT NULL) AS paid,
	       p.paid_date::text,
	       (p.recurrent_payment_id IS NULL OR rp.stopped_at IS NOT NULL) AS ended,
	       p.user_id::text,
	       COALESCE(u.full_name, '')
	FROM payments p
	LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
	LEFT JOIN users u ON u.id = p.user_id`

func scanOccurrence(rows interface{ Scan(dest ...any) error }) (model.ReminderOccurrence, error) {
	var (
		o          model.ReminderOccurrence
		dueStr     string
		paidStr    sql.NullString
		assigneeID sql.NullString
	)
	if err := rows.Scan(&o.ID, &o.RecurringPaymentID, &o.Title, &o.Amount, &o.Currency,
		&dueStr, &o.Paid, &paidStr, &o.Ended, &assigneeID, &o.AssigneeName); err != nil {
		return o, err
	}
	d, err := clock.ParseDate(dueStr)
	if err != nil {
		return o, fmt.Errorf("telegram source repository: parse due_date: %w", err)
	}
	o.DueDate = d
	if paidStr.Valid && paidStr.String != "" {
		pd, err := clock.ParseDate(paidStr.String)
		if err != nil {
			return o, fmt.Errorf("telegram source repository: parse paid_date: %w", err)
		}
		o.PaidDate = &pd
	}
	if assigneeID.Valid && assigneeID.String != "" {
		id := assigneeID.String
		o.AssigneeID = &id
	}
	return o, nil
}

// DueBetween — неоплаченные периоды активных платежей со сроком в [from, to].
func (r *TelegramSourceRepository) DueBetween(ctx context.Context, from, to clock.Date) ([]model.ReminderOccurrence, error) {
	rows, err := r.database.db.QueryContext(ctx, telegramSourceSelect+`
		WHERE p.paid_date IS NULL
		  AND p.recurrent_payment_id IS NOT NULL
		  AND rp.stopped_at IS NULL
		  AND p.due_date BETWEEN $1::date AND $2::date`, from.String(), to.String())
	if err != nil {
		return nil, fmt.Errorf("telegram source repository: due between: %w", err)
	}
	return collectOccurrences(rows)
}

// GetByIDs — текущее состояние периодов по id, включая оплаченные/завершённые.
func (r *TelegramSourceRepository) GetByIDs(ctx context.Context, ids []string) ([]model.ReminderOccurrence, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.database.db.QueryContext(ctx, telegramSourceSelect+`
		WHERE p.id = ANY($1::uuid[])`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("telegram source repository: get by ids: %w", err)
	}
	return collectOccurrences(rows)
}

func collectOccurrences(rows *sql.Rows) ([]model.ReminderOccurrence, error) {
	defer rows.Close()
	var out []model.ReminderOccurrence
	for rows.Next() {
		o, err := scanOccurrence(rows)
		if err != nil {
			return nil, fmt.Errorf("telegram source repository: scan: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telegram source repository: rows: %w", err)
	}
	return out, nil
}
