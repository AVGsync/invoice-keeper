package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/model/request"
)

type NotificationsRepository interface {
	GetByUserID(userID string, ctx context.Context) (model.UserNotificationSettings, error)
	Create(settings model.UserNotificationSettings, ctx context.Context) error
	Update(settings model.UserNotificationSettings, ctx context.Context) error
	Delete(userID string, ctx context.Context) error
}

type NotificationsService struct {
	repository NotificationsRepository
	// audit — журнал действий; по умолчанию Nop (конструктор не меняется).
	audit audit.Recorder
}

func NewNotificationsService(repository NotificationsRepository) *NotificationsService {
	return &NotificationsService{
		repository: repository,
		audit:      audit.Nop{},
	}
}

// SetAuditRecorder подключает журнал действий.
func (s *NotificationsService) SetAuditRecorder(r audit.Recorder) { s.audit = r }

// notificationChanges собирает changes настроек уведомлений. email маскируется
// в audit.Logger при редакции (ключ "email").
func notificationChanges(before, after model.UserNotificationSettings) map[string]audit.Change {
	return audit.Diff().
		Add("email", before.Email, after.Email).
		Add("email_enabled", before.EmailEnabled, after.EmailEnabled).
		Add("days_before_due", before.DaysBeforeDue, after.DaysBeforeDue).
		Add("send_time", before.SendTime, after.SendTime).
		Add("timezone", before.Timezone, after.Timezone).
		Map()
}

// GetByUserID возвращает настройки уведомлений пользователя.
func (s *NotificationsService) GetByUserID(
	userID string,
	ctx context.Context,
) (model.UserNotificationSettings, error) {
	settings, err := s.repository.GetByUserID(userID, ctx)
	if err != nil {
		slog.Error(
			"notifications service: get settings: failed to get settings",
			"user_id", userID,
			"error", err,
		)
		return model.UserNotificationSettings{},
			fmt.Errorf("notifications service: get settings: %w", err)
	}

	return settings, nil
}

// Create создаёт настройки уведомлений пользователя.
func (s *NotificationsService) Create(
	userID string,
	req request.UpdateUserNotificationSettingsRequest,
	ctx context.Context,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	settings := model.UserNotificationSettings{
		UserID:        userID,
		Email:         strings.TrimSpace(req.Email),
		EmailVerified: false,
		EmailEnabled:  req.EmailEnabled,
		DaysBeforeDue: req.DaysBeforeDue,
		SendTime:      req.SendTime,
		Timezone:      req.Timezone,
	}

	if err := s.repository.Create(settings, ctx); err != nil {
		slog.Error(
			"notifications service: create settings: failed to create settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: create settings: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionNotificationSettingsCreate,
		EntityType: audit.EntityNotificationSettings,
		EntityID:   userID,
		Changes:    notificationChanges(model.UserNotificationSettings{}, settings),
	})
	return nil
}

// Update полностью обновляет настройки уведомлений пользователя.
func (s *NotificationsService) Update(
	userID string,
	req request.UpdateUserNotificationSettingsRequest,
	ctx context.Context,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	settings := model.UserNotificationSettings{
		UserID:        userID,
		Email:         strings.TrimSpace(req.Email),
		EmailEnabled:  req.EmailEnabled,
		DaysBeforeDue: req.DaysBeforeDue,
		SendTime:      req.SendTime,
		Timezone:      req.Timezone,
	}

	// Снимок «до» для changes (при ошибке чтения — пустой: PUT считаем заменой).
	before, _ := s.repository.GetByUserID(userID, ctx)

	if err := s.repository.Update(settings, ctx); err != nil {
		slog.Error(
			"notifications service: update settings: failed to update settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: update settings: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionNotificationSettingsUpdate,
		EntityType: audit.EntityNotificationSettings,
		EntityID:   userID,
		Changes:    notificationChanges(before, settings),
	})
	return nil
}

// Patch частично обновляет настройки уведомлений пользователя.
func (s *NotificationsService) Patch(
	userID string,
	req request.PatchUserNotificationSettingsRequest,
	ctx context.Context,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	settings, err := s.repository.GetByUserID(userID, ctx)
	if err != nil {
		slog.Error(
			"notifications service: patch settings: failed to get settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: patch settings: %w", err)
	}

	before := settings // снимок «до» для changes

	if req.Email != nil {
		settings.Email = strings.TrimSpace(*req.Email)
	}
	if req.EmailEnabled != nil {
		settings.EmailEnabled = *req.EmailEnabled
	}
	if req.DaysBeforeDue != nil {
		settings.DaysBeforeDue = *req.DaysBeforeDue
	}
	if req.SendTime != nil {
		settings.SendTime = *req.SendTime
	}
	if req.Timezone != nil {
		settings.Timezone = *req.Timezone
	}

	if err := s.repository.Update(settings, ctx); err != nil {
		slog.Error(
			"notifications service: patch settings: failed to update settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: patch settings: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionNotificationSettingsUpdate,
		EntityType: audit.EntityNotificationSettings,
		EntityID:   userID,
		Changes:    notificationChanges(before, settings),
	})
	return nil
}

// Delete удаляет настройки уведомлений пользователя.
func (s *NotificationsService) Delete(
	userID string,
	ctx context.Context,
) error {
	// Снимок email до удаления — в журнал пишем маскированным.
	before, _ := s.repository.GetByUserID(userID, ctx)

	if err := s.repository.Delete(userID, ctx); err != nil {
		slog.Error(
			"notifications service: delete settings: failed to delete settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: delete settings: %w", err)
	}

	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionNotificationSettingsDelete,
		EntityType: audit.EntityNotificationSettings,
		EntityID:   userID,
		Changes:    audit.Diff().Add("email", before.Email, nil).Map(),
	})
	return nil
}
