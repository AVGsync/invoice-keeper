package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
)

var (
	ErrNotificationSettingsNotFound = errors.New("notification settings not found")
	ErrNotificationSettingsExists   = errors.New("notification settings already exist")
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
		return fmt.Errorf("notifications repository: create: %w", err)
	}

	return nil
}

// Update полностью обновляет настройки.
// Подтверждение email сбрасывается, если адрес изменился.
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
// Указатель nil означает, что поле не меняется.
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
