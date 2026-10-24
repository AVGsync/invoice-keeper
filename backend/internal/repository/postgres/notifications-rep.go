package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
	"github.com/lib/pq"
)

var (
	ErrNotificationSettingsNotFound = model.ErrNotificationSettingsNotFound
	ErrNotificationSettingsExists   = model.ErrNotificationSettingsExists
)

type NotificationsRepository struct {
	database *DB
}

const notificationSettingsSelect = `
	SELECT
		user_id,
		email,
		email_verified,
		email_enabled,
		days_before_due,
		TO_CHAR(send_time, 'HH24:MI'),
		timezone,
		created_at,
		updated_at
	FROM user_notification_settings
`

func scanNotificationSettings(row interface{ Scan(dest ...any) error }) (model.UserNotificationSettings, error) {
	var settings model.UserNotificationSettings

	err := row.Scan(
		&settings.UserID,
		&settings.Email,
		&settings.EmailVerified,
		&settings.EmailEnabled,
		&settings.DaysBeforeDue,
		&settings.SendTime,
		&settings.Timezone,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	return settings, err
}

// GetByUserID возвращает настройки уведомлений пользователя.
func (r *NotificationsRepository) GetByUserID(
	userID string,
	ctx context.Context,
) (model.UserNotificationSettings, error) {
	settings, err := scanNotificationSettings(
		r.database.db.QueryRowContext(
			ctx,
			notificationSettingsSelect+` WHERE user_id = $1`,
			userID,
		),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return settings, ErrNotificationSettingsNotFound
		}
		return settings, fmt.Errorf("notifications repository: get by user id: %w", err)
	}

	return settings, nil
}

// Create создаёт настройки уведомлений для пользователя.
func (r *NotificationsRepository) Create(
	settings model.UserNotificationSettings,
	ctx context.Context,
) error {
	_, err := r.database.db.ExecContext(ctx, `
		INSERT INTO user_notification_settings (
			user_id,
			email,
			email_verified,
			email_enabled,
			days_before_due,
			send_time,
			timezone
		)
		VALUES (
			$1, $2, FALSE, $3, $4, $5::time, $6
		)
	`,
		settings.UserID,
		strings.TrimSpace(settings.Email),
		settings.EmailEnabled,
		settings.DaysBeforeDue,
		settings.SendTime,
		settings.Timezone,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrNotificationSettingsExists
		}
		return fmt.Errorf("notifications repository: create: %w", err)
	}

	return nil
}

// Update полностью обновляет настройки.
func (r *NotificationsRepository) Update(
	settings model.UserNotificationSettings,
	ctx context.Context,
) error {
	result, err := r.database.db.ExecContext(ctx, `
		UPDATE user_notification_settings
		SET
			email_verified = CASE
				WHEN LOWER(email) <> LOWER($2) THEN FALSE
				ELSE email_verified
			END,
			verify_code_hash = CASE WHEN LOWER(email) <> LOWER($2) THEN NULL ELSE verify_code_hash END,
			email = $2,
			email_enabled = $3,
			days_before_due = $4,
			send_time = $5::time,
			timezone = $6,
			updated_at = NOW()
		WHERE user_id = $1
	`,
		settings.UserID,
		strings.TrimSpace(settings.Email),
		settings.EmailEnabled,
		settings.DaysBeforeDue,
		settings.SendTime,
		settings.Timezone,
	)
	if err != nil {
		return fmt.Errorf("notifications repository: update: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("notifications repository: update rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotificationSettingsNotFound
	}

	return nil
}

// Patch обновляет только переданные поля.
func (r *NotificationsRepository) Patch(
	userID string,
	patch request.PatchUserNotificationSettingsRequest,
	ctx context.Context,
) error {
	var (
		sets []string
		args []any
	)

	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if patch.Email != nil {
		email := strings.TrimSpace(*patch.Email)

		args = append(args, email)
		emailArg := len(args)

		sets = append(sets, fmt.Sprintf(
			"email_verified = CASE WHEN LOWER(email) <> LOWER($%d) THEN FALSE ELSE email_verified END",
			emailArg,
		))
		sets = append(sets, fmt.Sprintf(
			"verify_code_hash = CASE WHEN LOWER(email) <> LOWER($%d) THEN NULL ELSE verify_code_hash END",
			emailArg,
		))
		sets = append(sets, fmt.Sprintf("email = $%d", emailArg))
	}
	if patch.EmailEnabled != nil {
		add("email_enabled", *patch.EmailEnabled)
	}
	if patch.DaysBeforeDue != nil {
		add("days_before_due", *patch.DaysBeforeDue)
	}
	if patch.SendTime != nil {
		args = append(args, *patch.SendTime)
		sets = append(sets, fmt.Sprintf("send_time = $%d::time", len(args)))
	}
	if patch.Timezone != nil {
		add("timezone", *patch.Timezone)
	}

	if len(sets) == 0 {
		return errors.New("notifications repository: no fields to update")
	}

	sets = append(sets, "updated_at = NOW()")
	args = append(args, userID)

	query := fmt.Sprintf(
		"UPDATE user_notification_settings SET %s WHERE user_id = $%d",
		strings.Join(sets, ", "),
		len(args),
	)

	result, err := r.database.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("notifications repository: patch: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("notifications repository: patch rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotificationSettingsNotFound
	}

	return nil
}

// Delete удаляет настройки уведомлений пользователя.
func (r *NotificationsRepository) Delete(
	userID string,
	ctx context.Context,
) error {
	result, err := r.database.db.ExecContext(ctx, `
		DELETE FROM user_notification_settings
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("notifications repository: delete: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("notifications repository: delete rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotificationSettingsNotFound
	}

	return nil
}

// GetVerifyState возвращает состояние подтверждения адреса.
func (r *NotificationsRepository) GetVerifyState(ctx context.Context, userID string) (model.EmailVerifyState, error) {
	var st model.EmailVerifyState
	var hash sql.NullString
	err := r.database.db.QueryRowContext(ctx, `
		SELECT email, email_verified, verify_code_hash, verify_code_expires_at, verify_attempts, verify_sent_at
		FROM user_notification_settings WHERE user_id = $1`, userID).
		Scan(&st.Email, &st.Verified, &hash, &st.ExpiresAt, &st.Attempts, &st.SentAt)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ErrNotificationSettingsNotFound
	}
	if err != nil {
		return st, fmt.Errorf("notifications repository: get verify state: %w", err)
	}
	st.CodeHash = hash.String
	return st, nil
}

// SetVerifyCode сохраняет хеш нового кода и сбрасывает счётчик попыток.
func (r *NotificationsRepository) SetVerifyCode(ctx context.Context, userID, hash string, expires time.Time) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE user_notification_settings
		SET verify_code_hash = $2, verify_code_expires_at = $3, verify_attempts = 0, verify_sent_at = NOW()
		WHERE user_id = $1`, userID, hash, expires)
	if err != nil {
		return fmt.Errorf("notifications repository: set verify code: %w", err)
	}
	return nil
}

func (r *NotificationsRepository) AddVerifyAttempt(ctx context.Context, userID string) error {
	_, err := r.database.db.ExecContext(ctx,
		`UPDATE user_notification_settings SET verify_attempts = verify_attempts + 1 WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("notifications repository: add verify attempt: %w", err)
	}
	return nil
}

func (r *NotificationsRepository) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := r.database.db.ExecContext(ctx, `
		UPDATE user_notification_settings
		SET email_verified = TRUE, verify_code_hash = NULL, verify_code_expires_at = NULL, verify_attempts = 0, updated_at = NOW()
		WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("notifications repository: mark verified: %w", err)
	}
	return nil
}
