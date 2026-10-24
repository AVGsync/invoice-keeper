package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// AssistantHistoryRepository хранит ходы диалогов с ассистентом (таблица assistant_turns, миграция 000013).
type AssistantHistoryRepository struct {
	database *DB
}

const assistantTurnSelect = `
	SELECT id::text, user_id::text, channel, conversation, answer_id,
	       question, answer_text, status, error_code, mode, memory, created_at
	FROM assistant_turns`

// Append вставляет ход и возвращает его с проставленными id и created_at.
func (r *AssistantHistoryRepository) Append(ctx context.Context, t model.AssistantTurn) (model.AssistantTurn, error) {
	const q = `
		INSERT INTO assistant_turns
			(user_id, channel, conversation, answer_id, question, answer_text,
			 status, error_code, mode, memory)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		RETURNING id::text, created_at`
	mem := string(t.Memory)
	if mem == "" {
		mem = "{}"
	}
	err := r.database.db.QueryRowContext(ctx, q,
		t.UserID, t.Channel, t.Conversation, t.AnswerID, t.Question, t.AnswerText,
		t.Status, t.ErrorCode, t.Mode, mem,
	).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return model.AssistantTurn{}, fmt.Errorf("assistant history append: %w", err)
	}
	return t, nil
}

// ListRecent возвращает последние limit ходов разговора (новые первыми).
func (r *AssistantHistoryRepository) ListRecent(ctx context.Context, userID, conversation string, limit int, statuses []string) ([]model.AssistantTurn, error) {
	q := assistantTurnSelect + " WHERE user_id = $1 AND conversation = $2"
	args := []any{userID, conversation}
	if len(statuses) > 0 {
		q += " AND status = ANY($3::text[])"
		args = append(args, pq.Array(statuses))
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.database.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("assistant history list: %w", err)
	}
	defer rows.Close()

	out := make([]model.AssistantTurn, 0, limit)
	for rows.Next() {
		t, err := scanAssistantTurn(rows)
		if err != nil {
			return nil, fmt.Errorf("assistant history list: scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assistant history list: iterate: %w", err)
	}
	return out, nil
}

// Count возвращает число ходов в разговоре.
func (r *AssistantHistoryRepository) Count(ctx context.Context, userID, conversation string) (int, error) {
	const q = `SELECT count(*) FROM assistant_turns WHERE user_id = $1 AND conversation = $2`
	var n int
	if err := r.database.db.QueryRowContext(ctx, q, userID, conversation).Scan(&n); err != nil {
		return 0, fmt.Errorf("assistant history count: %w", err)
	}
	return n, nil
}

// DeleteConversation удаляет все ходы разговора пользователя, возвращает число строк.
func (r *AssistantHistoryRepository) DeleteConversation(ctx context.Context, userID, conversation string) (int, error) {
	const q = `DELETE FROM assistant_turns WHERE user_id = $1 AND conversation = $2`
	res, err := r.database.db.ExecContext(ctx, q, userID, conversation)
	if err != nil {
		return 0, fmt.Errorf("assistant history delete conversation: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// DeleteAllForUser удаляет все ходы всех разговоров пользователя (channel=all).
func (r *AssistantHistoryRepository) DeleteAllForUser(ctx context.Context, userID string) (int, error) {
	const q = `DELETE FROM assistant_turns WHERE user_id = $1`
	res, err := r.database.db.ExecContext(ctx, q, userID)
	if err != nil {
		return 0, fmt.Errorf("assistant history delete all: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Prune удаляет ходы разговора сверх keep самых свежих (после вставки).
func (r *AssistantHistoryRepository) Prune(ctx context.Context, userID, conversation string, keep int) error {
	const q = `
		DELETE FROM assistant_turns
		WHERE user_id = $1 AND conversation = $2
		  AND id NOT IN (
		      SELECT id FROM assistant_turns
		      WHERE user_id = $1 AND conversation = $2
		      ORDER BY created_at DESC, id DESC
		      LIMIT $3)`
	if _, err := r.database.db.ExecContext(ctx, q, userID, conversation, keep); err != nil {
		return fmt.Errorf("assistant history prune: %w", err)
	}
	return nil
}

// scanAssistantTurn читает один ход.
func scanAssistantTurn(row rowScanner) (model.AssistantTurn, error) {
	var (
		t         model.AssistantTurn
		errorCode sql.NullString
		memory    []byte
	)
	err := row.Scan(
		&t.ID, &t.UserID, &t.Channel, &t.Conversation, &t.AnswerID,
		&t.Question, &t.AnswerText, &t.Status, &errorCode, &t.Mode, &memory, &t.CreatedAt,
	)
	if err != nil {
		return t, err
	}
	t.ErrorCode = nullStr(errorCode)
	if len(memory) > 0 {
		t.Memory = append([]byte(nil), memory...)
	}
	return t, nil
}
