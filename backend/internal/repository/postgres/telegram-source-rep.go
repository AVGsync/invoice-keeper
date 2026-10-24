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
// ВНИМАНИЕ — имена колонок приняты по §2.3 как [Д] (допущение) и подлежат сверке
// с B ([В-B 3]). Если B использует другие имена, правится ТОЛЬКО этот файл; его
// защищает контрактный тест (telegram-source-rep_test.go, build-тег integration).
// Приняты:
//
//	payment_occurrences(id, recurring_payment_id, amount numeric, currency text,
//	                    due_date date, paid_date date NULL)
//	recurring_payments(id, title text, assignee_id uuid NULL, ended_at timestamptz NULL)
//	users(id, full_name text)
type TelegramSourceRepository struct {
	database *DB
}

// TelegramSource возвращает SQL-источник периодов.
func (d *DB) TelegramSource() *TelegramSourceRepository {
	return &TelegramSourceRepository{database: d}
}

// Общая проекция. due_date как ::text — чтобы DATE не сдвигалась зоной (§5.2);
// amount как ::text — десятичная строка без float.
const telegramSourceSelect = `
	SELECT o.id,
	       o.recurring_payment_id,
	       rp.title,
	       o.amount::text,
	       o.currency,
	       o.due_date::text,
	       (o.paid_date IS NOT NULL)  AS paid,
	       o.paid_date::text,
	       (rp.ended_at IS NOT NULL)  AS ended,
	       rp.assignee_id,
	       COALESCE(u.full_name, '')
	FROM payment_occurrences o
	JOIN recurring_payments rp ON rp.id = o.recurring_payment_id
	LEFT JOIN users u ON u.id = rp.assignee_id`

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
		WHERE o.paid_date IS NULL
		  AND rp.ended_at IS NULL
		  AND o.due_date BETWEEN $1::date AND $2::date`, from.String(), to.String())
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
		WHERE o.id = ANY($1)`, pq.Array(ids))
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
