package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// AuditRepository пишет и читает журнал действий (таблица audit_log, миграция
// 000012). Пользовательский текст (q) в SQL не попадает — только через позиционные
// параметры whereBuilder'а (переиспользуется из трека B).
type AuditRepository struct {
	database *DB
}

// Insert пишет одну строку журнала. jsonb-колонки приводятся из текста ($::jsonb),
// ip — из текста ($::inet). nil-указатели ложатся как NULL.
func (r *AuditRepository) Insert(ctx context.Context, row audit.Row) error {
	const q = `
		INSERT INTO audit_log
			(actor_id, actor_role, channel, action, outcome,
			 entity_type, entity_id, changes, payload, ip, user_agent, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::inet, $11, $12)`
	_, err := r.database.db.ExecContext(ctx, q,
		row.ActorID, row.ActorRole, row.Channel, row.Action, row.Outcome,
		row.EntityType, row.EntityID, jsonArg(row.Changes), jsonArg(row.Payload),
		row.IP, row.UserAgent, row.RequestID,
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

// jsonArg превращает сырой JSON в аргумент для $::jsonb: nil → NULL, иначе текст.
func jsonArg(b []byte) any {
	if b == nil {
		return nil
	}
	return string(b)
}

const auditSelect = `
	SELECT
		al.id,
		al.created_at,
		al.actor_id::text,
		al.actor_role,
		u.full_name,
		al.channel,
		al.action,
		al.outcome,
		al.entity_type,
		al.entity_id,
		al.changes,
		al.payload,
		al.ip::text,
		al.user_agent,
		al.request_id
	FROM audit_log al
	LEFT JOIN users u ON u.id = al.actor_id`

const auditFrom = `
	FROM audit_log al
	LEFT JOIN users u ON u.id = al.actor_id`

// List возвращает страницу журнала и общее число строк по фильтру (без учёта
// limit/offset).
func (r *AuditRepository) List(ctx context.Context, f model.AuditFilter) ([]model.AuditEntry, int, error) {
	w := r.buildAuditWhere(f)
	whereSQL := w.sql()

	countArgs := append([]any(nil), w.args...)
	var total int
	if err := r.database.db.QueryRowContext(ctx, "SELECT count(*)"+auditFrom+whereSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit list: count: %w", err)
	}

	d := direction(f.Order)
	limitPH := w.arg(f.Limit)
	offsetPH := w.arg(f.Offset)
	dataQuery := auditSelect + whereSQL +
		fmt.Sprintf(" ORDER BY al.created_at %s, al.id %s", d, d) +
		" LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.database.db.QueryContext(ctx, dataQuery, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit list: query: %w", err)
	}
	defer rows.Close()

	out := make([]model.AuditEntry, 0)
	for rows.Next() {
		e, err := scanAuditEntry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("audit list: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit list: iterate: %w", err)
	}
	return out, total, nil
}

// buildAuditWhere собирает WHERE по фильтру. Направление сортировки берётся из
// direction() (не из ввода).
func (r *AuditRepository) buildAuditWhere(f model.AuditFilter) *whereBuilder {
	w := &whereBuilder{}

	if len(f.ActorIDs) > 0 {
		w.add("al.actor_id = ANY(" + w.arg(pq.Array(f.ActorIDs)) + "::uuid[])")
	}

	if len(f.Actions) > 0 {
		var parts []string
		var exacts []string
		for _, a := range f.Actions {
			if strings.HasSuffix(a, ".*") {
				prefix := strings.TrimSuffix(a, "*") // оставляем точку: "recurring."
				ph := w.arg(escapeLike(prefix) + "%")
				parts = append(parts, "al.action LIKE "+ph+" ESCAPE '\\'")
			} else {
				exacts = append(exacts, a)
			}
		}
		if len(exacts) > 0 {
			parts = append(parts, "al.action = ANY("+w.arg(pq.Array(exacts))+"::text[])")
		}
		w.add("(" + strings.Join(parts, " OR ") + ")")
	}

	if len(f.EntityTypes) > 0 {
		w.add("al.entity_type = ANY(" + w.arg(pq.Array(f.EntityTypes)) + "::text[])")
	}
	if f.EntityID != "" {
		w.add("al.entity_id = " + w.arg(f.EntityID))
	}
	if len(f.Channels) > 0 {
		w.add("al.channel = ANY(" + w.arg(pq.Array(f.Channels)) + "::text[])")
	}
	if f.Outcome != "" {
		w.add("al.outcome = " + w.arg(f.Outcome))
	}
	if f.From != nil {
		w.add("al.created_at >= " + w.arg(*f.From))
	}
	if f.To != nil {
		w.add("al.created_at < " + w.arg(*f.To))
	}
	if f.Query != "" {
		ph := w.arg(escapeLike(f.Query))
		like := " ILIKE '%' || " + ph + " || '%' ESCAPE '\\'"
		w.add("(al.action" + like +
			" OR COALESCE(al.entity_id,'')" + like +
			" OR COALESCE(al.payload::text,'')" + like +
			" OR COALESCE(al.changes::text,'')" + like +
			" OR COALESCE(u.full_name,'')" + like + ")")
	}

	return w
}

// scanAuditEntry читает строку журнала с присоединённым именем актора.
func scanAuditEntry(row rowScanner) (model.AuditEntry, error) {
	var (
		e          model.AuditEntry
		actorID    sql.NullString
		actorRole  sql.NullString
		actorName  sql.NullString
		entityType sql.NullString
		entityID   sql.NullString
		changes    []byte
		payload    []byte
		ip         sql.NullString
		userAgent  sql.NullString
		requestID  sql.NullString
	)

	err := row.Scan(
		&e.ID,
		&e.CreatedAt,
		&actorID,
		&actorRole,
		&actorName,
		&e.Channel,
		&e.Action,
		&e.Outcome,
		&entityType,
		&entityID,
		&changes,
		&payload,
		&ip,
		&userAgent,
		&requestID,
	)
	if err != nil {
		return e, err
	}

	e.ActorID = nullStr(actorID)
	e.ActorRole = nullStr(actorRole)
	e.ActorName = nullStr(actorName)
	e.EntityType = nullStr(entityType)
	e.EntityID = nullStr(entityID)
	e.IP = nullStr(ip)
	e.UserAgent = nullStr(userAgent)
	e.RequestID = nullStr(requestID)
	if len(changes) > 0 {
		e.Changes = append([]byte(nil), changes...)
	}
	if len(payload) > 0 {
		e.Payload = append([]byte(nil), payload...)
	}

	return e, nil
}

// nullStr превращает sql.NullString в *string (nil, если NULL или пусто для actor).
func nullStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
