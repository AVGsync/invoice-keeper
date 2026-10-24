package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/lib/pq"
)

// TelegramRepository — хранилище ядра бота: привязки, коды привязки, командный чат и дедуп апдейтов.
type TelegramRepository struct {
	database *DB
}

// Telegram возвращает репозиторий ядра Telegram-бота.
func (d *DB) Telegram() *TelegramRepository {
	return &TelegramRepository{database: d}
}

func scanLink(row interface{ Scan(dest ...any) error }) (model.TelegramLink, error) {
	var (
		l        model.TelegramLink
		username *string
	)
	err := row.Scan(&l.UserID, &l.TGUserID, &username, &l.DMEnabled, &l.BotBlocked, &l.LinkedAt, &l.UpdatedAt)
	if username != nil {
		l.Username = *username
	}
	return l, err
}

const telegramLinkSelect = `
	SELECT user_id, tg_user_id, tg_username, dm_enabled, bot_blocked, linked_at, updated_at
	FROM telegram_links`

// GetLinkByUserID возвращает привязку пользователя или model.ErrTelegramNotLinked.
func (r *TelegramRepository) GetLinkByUserID(ctx context.Context, userID string) (model.TelegramLink, error) {
	l, err := scanLink(r.database.db.QueryRowContext(ctx, telegramLinkSelect+`
		WHERE user_id = $1`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return l, model.ErrTelegramNotLinked
	}
	if err != nil {
		return l, fmt.Errorf("telegram repository: get link by user id: %w", err)
	}
	return l, nil
}

// GetLinkByTGUserID возвращает привязку по Telegram id или model.ErrTelegramNotLinked.
func (r *TelegramRepository) GetLinkByTGUserID(ctx context.Context, tgUserID int64) (model.TelegramLink, error) {
	l, err := scanLink(r.database.db.QueryRowContext(ctx, telegramLinkSelect+`
		WHERE tg_user_id = $1`, tgUserID))
	if errors.Is(err, sql.ErrNoRows) {
		return l, model.ErrTelegramNotLinked
	}
	if err != nil {
		return l, fmt.Errorf("telegram repository: get link by tg user id: %w", err)
	}
	return l, nil
}

// UpsertLink создаёт или перепривязывает Telegram к пользователю.
func (r *TelegramRepository) UpsertLink(ctx context.Context, userID string, tgUserID int64, username string) error {
	var usernamePtr *string
	if username != "" {
		usernamePtr = &username
	}
	_, err := r.database.db.ExecContext(ctx, `
		INSERT INTO telegram_links (user_id, tg_user_id, tg_username, bot_blocked, updated_at)
		VALUES ($1, $2, $3, FALSE, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET tg_user_id = EXCLUDED.tg_user_id,
		              tg_username = EXCLUDED.tg_username,
		              bot_blocked = FALSE,
		              updated_at = NOW()
	`, userID, tgUserID, usernamePtr)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return model.ErrTelegramAlreadyLinked
		}
		return fmt.Errorf("telegram repository: upsert link: %w", err)
	}
	return nil
}

// DeleteLink удаляет привязку пользователя.
func (r *TelegramRepository) DeleteLink(ctx context.Context, userID string) error {
	res, err := r.database.db.ExecContext(ctx, `
		DELETE FROM telegram_links WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("telegram repository: delete link: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("telegram repository: delete link: %w", err)
	}
	if n == 0 {
		return model.ErrTelegramNotLinked
	}
	return nil
}

// SetBotBlocked помечает привязку заблокированной/разблокированной по tg_user_id.
func (r *TelegramRepository) SetBotBlocked(ctx context.Context, tgUserID int64, blocked bool) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_links SET bot_blocked = $2, updated_at = NOW()
		WHERE tg_user_id = $1`, tgUserID, blocked)
	if err != nil {
		return fmt.Errorf("telegram repository: set bot blocked: %w", err)
	}
	return nil
}

// SetDMEnabled переключает доставку в личку по user_id.
func (r *TelegramRepository) SetDMEnabled(ctx context.Context, userID string, enabled bool) error {
	res, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_links SET dm_enabled = $2, updated_at = NOW()
		WHERE user_id = $1`, userID, enabled)
	if err != nil {
		return fmt.Errorf("telegram repository: set dm enabled: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("telegram repository: set dm enabled: %w", err)
	}
	if n == 0 {
		return model.ErrTelegramNotLinked
	}
	return nil
}

// CountActiveCodes считает неиспользованные непросроченные коды пользователя.
func (r *TelegramRepository) CountActiveCodes(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.database.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM telegram_link_codes
		WHERE user_id = $1 AND used_at IS NULL AND expires_at > NOW()`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("telegram repository: count active codes: %w", err)
	}
	return n, nil
}

// CreateLinkCode сохраняет хеш кода привязки.
func (r *TelegramRepository) CreateLinkCode(ctx context.Context, userID, codeHash string, expiresAt time.Time) (string, error) {
	var id string
	err := r.database.db.QueryRowContext(ctx, `
		INSERT INTO telegram_link_codes (user_id, code_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id`, userID, codeHash, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("telegram repository: create link code: %w", err)
	}
	return id, nil
}

// ClaimLinkCode находит активный код по хешу и отмечает его открытым этим Telegram-пользователем (claimed_by_tg).
func (r *TelegramRepository) ClaimLinkCode(ctx context.Context, codeHash string, tgUserID int64) (codeID, userID string, err error) {
	err = r.database.db.QueryRowContext(ctx, `
		UPDATE telegram_link_codes
		SET claimed_by_tg = $2
		WHERE code_hash = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		  AND (claimed_by_tg IS NULL OR claimed_by_tg = $2)
		RETURNING id, user_id`, codeHash, tgUserID).Scan(&codeID, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", model.ErrTelegramCodeInvalid
	}
	if err != nil {
		return "", "", fmt.Errorf("telegram repository: claim link code: %w", err)
	}
	return codeID, userID, nil
}

// ConfirmLinkCode атомарно сжигает код (used_at) при подтверждении кнопкой тем же tg_user_id и возвращает user_id.
func (r *TelegramRepository) ConfirmLinkCode(ctx context.Context, codeID string, tgUserID int64) (string, error) {
	var userID string
	err := r.database.db.QueryRowContext(ctx, `
		UPDATE telegram_link_codes
		SET used_at = NOW()
		WHERE id = $1
		  AND claimed_by_tg = $2
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id`, codeID, tgUserID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", model.ErrTelegramCodeInvalid
	}
	if err != nil {
		return "", fmt.Errorf("telegram repository: confirm link code: %w", err)
	}
	return userID, nil
}

// CancelLinkCode гасит код (used_at) при отмене пользователем.
func (r *TelegramRepository) CancelLinkCode(ctx context.Context, codeID string, tgUserID int64) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_link_codes SET used_at = NOW()
		WHERE id = $1 AND claimed_by_tg = $2 AND used_at IS NULL`, codeID, tgUserID)
	if err != nil {
		return fmt.Errorf("telegram repository: cancel link code: %w", err)
	}
	return nil
}

func scanChat(row interface{ Scan(dest ...any) error }) (model.TelegramChat, error) {
	var c model.TelegramChat
	err := row.Scan(&c.ID, &c.TGChatID, &c.Type, &c.Title, &c.IsActive, &c.BoundBy, &c.BoundAt, &c.DeactivatedAt, &c.LastError)
	return c, err
}

const telegramChatSelect = `
	SELECT id, tg_chat_id, type, title, is_active, bound_by, bound_at, deactivated_at, last_error
	FROM telegram_chats`

// GetActiveChat возвращает активный командный чат или model.ErrTelegramNoTeamChat.
func (r *TelegramRepository) GetActiveChat(ctx context.Context) (model.TelegramChat, error) {
	c, err := scanChat(r.database.db.QueryRowContext(ctx, telegramChatSelect+`
		WHERE is_active`))
	if errors.Is(err, sql.ErrNoRows) {
		return c, model.ErrTelegramNoTeamChat
	}
	if err != nil {
		return c, fmt.Errorf("telegram repository: get active chat: %w", err)
	}
	return c, nil
}

func (r *TelegramRepository) GetLastDeactivatedChat(ctx context.Context) (model.TelegramChat, error) {
	c, err := scanChat(r.database.db.QueryRowContext(ctx, telegramChatSelect+`
		WHERE NOT is_active
		ORDER BY deactivated_at DESC NULLS LAST
		LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return c, model.ErrTelegramNoTeamChat
	}
	if err != nil {
		return c, fmt.Errorf("telegram repository: get last deactivated chat: %w", err)
	}
	return c, nil
}

// BindChat делает чат активным командным, деактивируя прежний.
func (r *TelegramRepository) BindChat(ctx context.Context, tgChatID int64, chatType, title, boundBy string) (prev model.TelegramChat, hadPrev bool, err error) {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return prev, false, fmt.Errorf("telegram repository: bind chat: begin: %w", err)
	}
	defer tx.Rollback()

	prev, err = scanChat(tx.QueryRowContext(ctx, telegramChatSelect+` WHERE is_active`))
	switch {
	case err == nil:
		hadPrev = true
		if prev.TGChatID == tgChatID {
			// Повторный /bind в том же чате: просто обновим заголовок.
			hadPrev = false
		}
	case errors.Is(err, sql.ErrNoRows):
		hadPrev = false
		err = nil
	default:
		return prev, false, fmt.Errorf("telegram repository: bind chat: read active: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE telegram_chats SET is_active = FALSE, deactivated_at = NOW()
		WHERE is_active AND tg_chat_id <> $1`, tgChatID); err != nil {
		return prev, false, fmt.Errorf("telegram repository: bind chat: deactivate: %w", err)
	}

	var titlePtr *string
	if title != "" {
		titlePtr = &title
	}
	var boundByPtr *string
	if boundBy != "" {
		boundByPtr = &boundBy
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO telegram_chats (tg_chat_id, type, title, is_active, bound_by, bound_at, deactivated_at, last_error)
		VALUES ($1, $2, $3, TRUE, $4, NOW(), NULL, NULL)
		ON CONFLICT (tg_chat_id)
		DO UPDATE SET type = EXCLUDED.type,
		              title = EXCLUDED.title,
		              is_active = TRUE,
		              bound_by = EXCLUDED.bound_by,
		              bound_at = NOW(),
		              deactivated_at = NULL,
		              last_error = NULL`, tgChatID, chatType, titlePtr, boundByPtr); err != nil {
		return prev, false, fmt.Errorf("telegram repository: bind chat: upsert: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return prev, false, fmt.Errorf("telegram repository: bind chat: commit: %w", err)
	}
	return prev, hadPrev, nil
}

// DeactivateChat снимает активность с чата по tg_chat_id с указанием причины.
func (r *TelegramRepository) DeactivateChat(ctx context.Context, tgChatID int64, reason string) error {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	res, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_chats
		SET is_active = FALSE, deactivated_at = NOW(), last_error = $2
		WHERE tg_chat_id = $1 AND is_active`, tgChatID, reasonPtr)
	if err != nil {
		return fmt.Errorf("telegram repository: deactivate chat: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("telegram repository: deactivate chat: %w", err)
	}
	if n == 0 {
		return model.ErrTelegramNoTeamChat
	}
	return nil
}

// UpdateChatID переносит tg_chat_id (сервисное migrate_to_chat_id).
func (r *TelegramRepository) UpdateChatID(ctx context.Context, oldID, newID int64) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE telegram_chats SET tg_chat_id = $2 WHERE tg_chat_id = $1`, oldID, newID)
	if err != nil {
		return fmt.Errorf("telegram repository: update chat id: %w", err)
	}
	return nil
}

// SeedChat вставляет начальный командный чат из TELEGRAM_TEAM_CHAT_ID, если активного ещё нет.
func (r *TelegramRepository) SeedChat(ctx context.Context, tgChatID int64, chatType string) error {
	_, err := r.database.db.ExecContext(ctx, `
		INSERT INTO telegram_chats (tg_chat_id, type, is_active)
		VALUES ($1, $2, TRUE)
		ON CONFLICT (tg_chat_id) DO NOTHING`, tgChatID, chatType)
	if err != nil {
		return fmt.Errorf("telegram repository: seed chat: %w", err)
	}
	return nil
}

// MarkUpdateSeen вставляет update_id.
func (r *TelegramRepository) MarkUpdateSeen(ctx context.Context, updateID int64) (bool, error) {
	res, err := r.database.db.ExecContext(ctx, `
		INSERT INTO telegram_updates (update_id) VALUES ($1)
		ON CONFLICT (update_id) DO NOTHING`, updateID)
	if err != nil {
		return false, fmt.Errorf("telegram repository: mark update seen: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("telegram repository: mark update seen: %w", err)
	}
	return n > 0, nil
}

// CleanupStale удаляет мусор ядра бота: обработанные апдейты старше updatesBefore и истёкшие коды привязки старше codesBefore.
func (r *TelegramRepository) CleanupStale(ctx context.Context, updatesBefore, codesBefore time.Time) error {
	if _, err := r.database.db.ExecContext(ctx, `
		DELETE FROM telegram_updates WHERE received_at < $1`, updatesBefore); err != nil {
		return fmt.Errorf("telegram repository: cleanup updates: %w", err)
	}
	if _, err := r.database.db.ExecContext(ctx, `
		DELETE FROM telegram_link_codes WHERE created_at < $1`, codesBefore); err != nil {
		return fmt.Errorf("telegram repository: cleanup link codes: %w", err)
	}
	return nil
}

// MaxUpdateID возвращает наибольший сохранённый update_id (для начального offset поллера).
func (r *TelegramRepository) MaxUpdateID(ctx context.Context) (int64, error) {
	var max sql.NullInt64
	err := r.database.db.QueryRowContext(ctx, `SELECT MAX(update_id) FROM telegram_updates`).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("telegram repository: max update id: %w", err)
	}
	if !max.Valid {
		return 0, nil
	}
	return max.Int64, nil
}
