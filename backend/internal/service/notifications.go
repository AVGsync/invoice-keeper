package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
}

func NewNotificationsService(repository NotificationsRepository) *NotificationsService {
	return &NotificationsService{
		repository: repository,
	}
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

	if err := s.repository.Update(settings, ctx); err != nil {
		slog.Error(
			"notifications service: update settings: failed to update settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: update settings: %w", err)
	}

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

	return nil
}

// Delete удаляет настройки уведомлений пользователя.
func (s *NotificationsService) Delete(
	userID string,
	ctx context.Context,
) error {
	if err := s.repository.Delete(userID, ctx); err != nil {
		slog.Error(
			"notifications service: delete settings: failed to delete settings",
			"user_id", userID,
			"error", err,
		)
		return fmt.Errorf("notifications service: delete settings: %w", err)
	}

	return nil
}
