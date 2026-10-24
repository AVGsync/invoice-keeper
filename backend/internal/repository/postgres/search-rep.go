package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// SearchPayments ищет периоды по фильтру.
func (r *PaymentRepository) SearchPayments(
	ctx context.Context,
	f model.OccurrenceFilter,
	today string,
) ([]model.Payment, int, error) {
	w := &whereBuilder{}

	// today подставляем лениво: он нужен только предикатам overdue/pending.
	todayPH := ""
	ensureToday := func() string {
		if todayPH == "" {
			todayPH = w.arg(today)
		}
		return todayPH
	}

	if f.Query != "" {
		ph := w.arg(escapeLike(f.Query))
		w.add(fmt.Sprintf(
			"(COALESCE(NULLIF(p.description,''), rp.description, '') ILIKE '%%' || %s || '%%' ESCAPE '\\'"+
				" OR u.full_name ILIKE '%%' || %s || '%%' ESCAPE '\\')", ph, ph))
	}

	if len(f.Statuses) > 0 {
		parts := make([]string, 0, len(f.Statuses))
		for _, s := range f.Statuses {
			switch s {
			case "paid":
				parts = append(parts, "(p.paid_date IS NOT NULL)")
			case "ended":
				parts = append(parts,
					"(p.paid_date IS NULL AND (p.recurrent_payment_id IS NULL OR rp.stopped_at IS NOT NULL))")
			case "overdue":
				parts = append(parts, fmt.Sprintf(
					"(p.paid_date IS NULL AND p.recurrent_payment_id IS NOT NULL AND rp.stopped_at IS NULL AND p.due_date < %s::date)",
					ensureToday()))
			case "pending":
				parts = append(parts, fmt.Sprintf(
					"(p.paid_date IS NULL AND p.recurrent_payment_id IS NOT NULL AND rp.stopped_at IS NULL AND p.due_date >= %s::date)",
					ensureToday()))
			}
		}
		w.add("(" + joinOr(parts) + ")")
	}

	if len(f.Currencies) > 0 {
		w.add("p.currency = ANY(" + w.arg(pq.Array(f.Currencies)) + "::text[])")
	}
	if len(f.AssigneeIDs) > 0 {
		w.add("p.user_id = ANY(" + w.arg(pq.Array(f.AssigneeIDs)) + "::uuid[])")
	}
	if len(f.PersonaIDs) > 0 {
		w.add("u.persona_id = ANY(" + w.arg(pq.Array(f.PersonaIDs)) + "::uuid[])")
	}
	if f.RecurrentPaymentID != "" {
		w.add("p.recurrent_payment_id = " + w.arg(f.RecurrentPaymentID) + "::uuid")
	}
	if f.DueFrom != "" {
		w.add("p.due_date >= " + w.arg(f.DueFrom) + "::date")
	}
	if f.DueTo != "" {
		w.add("p.due_date <= " + w.arg(f.DueTo) + "::date")
	}
	if f.PaidFrom != "" {
		w.add("p.paid_date >= " + w.arg(f.PaidFrom) + "::date")
	}
	if f.PaidTo != "" {
		w.add("p.paid_date <= " + w.arg(f.PaidTo) + "::date")
	}
	if f.AmountMin != nil {
		w.add("p.amount >= " + w.arg(*f.AmountMin))
	}
	if f.AmountMax != nil {
		w.add("p.amount <= " + w.arg(*f.AmountMax))
	}

	whereSQL := w.sql()

	countArgs := append([]any(nil), w.args...)
	var total int
	countQuery := `
		SELECT count(*)
		FROM payments p
		LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
		LEFT JOIN users u ON u.id = p.user_id` + whereSQL
	if err := r.database.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search payments: count: %w", err)
	}

	orderSQL := occurrenceOrderBy(f.Sort, direction(f.Order))
	limitPH := w.arg(f.Limit)
	offsetPH := w.arg(f.Offset)
	dataQuery := paymentSelect + whereSQL + " ORDER BY " + orderSQL +
		" LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.database.db.QueryContext(ctx, dataQuery, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search payments: query: %w", err)
	}
	defer rows.Close()

	payments := make([]model.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("search payments: scan: %w", err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("search payments: iterate: %w", err)
	}

	return payments, total, nil
}

// occurrenceOrderBy — белый список сортировок периодов.
func occurrenceOrderBy(sort, d string) string {
	switch sort {
	case "amount":
		return fmt.Sprintf("p.amount %s, p.due_date ASC, p.id", d)
	case "created_at":
		return fmt.Sprintf("p.created_at %s, p.id", d)
	case "description":
		return fmt.Sprintf("lower(COALESCE(NULLIF(p.description,''), rp.description, '')) %s, p.due_date ASC, p.id", d)
	case "paid_date":
		return fmt.Sprintf("p.paid_date %s NULLS LAST, p.due_date ASC, p.id", d)
	default: // due_date
		return fmt.Sprintf("p.due_date %s, p.created_at %s, p.id", d, d)
	}
}

// recurringSearchSelect — проекция шаблонов с ближайшим неоплаченным сроком.
const recurringSearchSelect = `
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
		COALESCE(u.full_name, ''),
		nx.next_due::text
	FROM recurrent_payments rp
	LEFT JOIN LATERAL (
		SELECT ur.user_id
		FROM user_recurrent_payments ur
		WHERE ur.recurrent_payment_id = rp.id
		ORDER BY ur.user_id
		LIMIT 1
	) a ON TRUE
	LEFT JOIN users u ON u.id = a.user_id
	LEFT JOIN LATERAL (
		SELECT min(o.due_date) AS next_due
		FROM payments o
		WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NULL AND o.due_date >= %s::date
	) nx ON TRUE
`

// recurringSearchFrom — тот же FROM/JOIN для count(*).
const recurringSearchFrom = `
	FROM recurrent_payments rp
	LEFT JOIN LATERAL (
		SELECT ur.user_id
		FROM user_recurrent_payments ur
		WHERE ur.recurrent_payment_id = rp.id
		ORDER BY ur.user_id
		LIMIT 1
	) a ON TRUE
	LEFT JOIN users u ON u.id = a.user_id
	LEFT JOIN LATERAL (
		SELECT min(o.due_date) AS next_due
		FROM payments o
		WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NULL AND o.due_date >= %s::date
	) nx ON TRUE
`

// scanRecurringSearchRow читает шаблон плюс next_due.
func scanRecurringSearchRow(row rowScanner) (model.RecurringSearchRow, error) {
	var (
		out          model.RecurringSearchRow
		assigneeID   sql.NullString
		assigneeName sql.NullString
		nextDue      sql.NullString
	)

	err := row.Scan(
		&out.ID,
		&out.Amount,
		&out.Currency,
		&out.Description,
		&out.Frequency,
		&out.Interval,
		&out.DayOffset,
		&out.StartDate,
		&out.StoppedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
		&assigneeID,
		&assigneeName,
		&nextDue,
	)
	if err != nil {
		return out, err
	}

	if assigneeID.Valid && assigneeID.String != "" {
		id := assigneeID.String
		out.AssigneeID = &id
	}
	if assigneeName.Valid {
		out.AssigneeName = assigneeName.String
	}
	if nextDue.Valid && nextDue.String != "" {
		nd := nextDue.String
		out.NextDueDate = &nd
	}

	return out, nil
}

// SearchRecurrentPayments ищет шаблоны по фильтру.
func (r *RecurrentPaymentRepository) SearchRecurrentPayments(
	ctx context.Context,
	f model.RecurringFilter,
	today string,
) ([]model.RecurringSearchRow, int, error) {
	w := &whereBuilder{}
	todayPH := w.arg(today)

	if f.Query != "" {
		ph := w.arg(escapeLike(f.Query))
		w.add(fmt.Sprintf(
			"(rp.description ILIKE '%%' || %s || '%%' ESCAPE '\\'"+
				" OR u.full_name ILIKE '%%' || %s || '%%' ESCAPE '\\')", ph, ph))
	}
	if len(f.Currencies) > 0 {
		w.add("rp.currency = ANY(" + w.arg(pq.Array(f.Currencies)) + "::text[])")
	}
	if len(f.Frequencies) > 0 {
		w.add("rp.frequency = ANY(" + w.arg(pq.Array(f.Frequencies)) + "::text[])")
	}
	if len(f.AssigneeIDs) > 0 {
		w.add("EXISTS (SELECT 1 FROM user_recurrent_payments x WHERE x.recurrent_payment_id = rp.id AND x.user_id = ANY(" +
			w.arg(pq.Array(f.AssigneeIDs)) + "::uuid[]))")
	}
	if len(f.PersonaIDs) > 0 {
		w.add("u.persona_id = ANY(" + w.arg(pq.Array(f.PersonaIDs)) + "::uuid[])")
	}
	if len(f.Statuses) > 0 && len(f.Statuses) < 2 {
		switch f.Statuses[0] {
		case "active":
			w.add("rp.stopped_at IS NULL")
		case "stopped":
			w.add("rp.stopped_at IS NOT NULL")
		}
	}
	if f.AmountMin != nil {
		w.add("rp.amount >= " + w.arg(*f.AmountMin))
	}
	if f.AmountMax != nil {
		w.add("rp.amount <= " + w.arg(*f.AmountMax))
	}
	if f.StartFrom != "" {
		w.add("rp.start_date >= " + w.arg(f.StartFrom) + "::date")
	}
	if f.StartTo != "" {
		w.add("rp.start_date <= " + w.arg(f.StartTo) + "::date")
	}
	if f.NextDueFrom != "" {
		w.add("nx.next_due >= " + w.arg(f.NextDueFrom) + "::date")
	}
	if f.NextDueTo != "" {
		w.add("nx.next_due <= " + w.arg(f.NextDueTo) + "::date")
	}

	whereSQL := w.sql()

	countArgs := append([]any(nil), w.args...)
	var total int
	countQuery := "SELECT count(*)" + fmt.Sprintf(recurringSearchFrom, todayPH) + whereSQL
	if err := r.database.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search recurring: count: %w", err)
	}

	orderSQL := recurringOrderBy(f.Sort, direction(f.Order))
	limitPH := w.arg(f.Limit)
	offsetPH := w.arg(f.Offset)
	dataQuery := fmt.Sprintf(recurringSearchSelect, todayPH) + whereSQL +
		" ORDER BY " + orderSQL + " LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.database.db.QueryContext(ctx, dataQuery, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search recurring: query: %w", err)
	}
	defer rows.Close()

	out := make([]model.RecurringSearchRow, 0)
	for rows.Next() {
		row, err := scanRecurringSearchRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("search recurring: scan: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("search recurring: iterate: %w", err)
	}

	return out, total, nil
}

// recurringOrderBy — белый список сортировок шаблонов.
func recurringOrderBy(sort, d string) string {
	switch sort {
	case "amount":
		return fmt.Sprintf("rp.amount %s, rp.description, rp.id", d)
	case "created_at":
		return fmt.Sprintf("rp.created_at %s, rp.id", d)
	case "next_due":
		return fmt.Sprintf("nx.next_due %s NULLS LAST, rp.description, rp.id", d)
	case "start_date":
		return fmt.Sprintf("rp.start_date %s, rp.description, rp.id", d)
	default: // description
		return fmt.Sprintf("lower(rp.description) %s, rp.created_at DESC, rp.id", d)
	}
}

// direction переводит order ("asc"/"desc") в SQL-ключевое слово.
func direction(order string) string {
	if order == "desc" {
		return "DESC"
	}
	return "ASC"
}

// joinOr соединяет предикаты через OR.
func joinOr(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " OR "
		}
		out += p
	}
	return out
}
