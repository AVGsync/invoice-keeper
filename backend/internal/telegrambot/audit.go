package telegrambot

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/google/uuid"
)

// botCtx оборачивает контекст метаданными журнала для действий, инициированных
// ботом: канал всегда bot, актор — привязанный пользователь. role передаётся,
// если известна (иначе пусто).
func (s *Service) botCtx(ctx context.Context, userID, role string) context.Context {
	return audit.WithMeta(ctx, audit.Meta{
		ActorID:   userID,
		ActorRole: role,
		Channel:   audit.ChannelBot,
	})
}

// miniAppAuthReason переводит ошибку входа в Mini App в код причины для журнала.
// Пустая строка — событие не аудируется (напр. бот выключен).
func miniAppAuthReason(err error) string {
	switch {
	case errors.Is(err, telegram.ErrInitDataInvalid):
		return "invalid_init_data"
	case errors.Is(err, telegram.ErrInitDataStale):
		return "init_data_expired"
	case errors.Is(err, model.ErrTelegramNotLinked):
		return "not_linked"
	default:
		return ""
	}
}

// miniAppLinkReason переводит ошибку привязки через Mini App в код причины.
func miniAppLinkReason(err error) string {
	switch {
	case errors.Is(err, ErrMiniAppInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, model.ErrTelegramAlreadyLinked):
		return "already_linked"
	case errors.Is(err, telegram.ErrInitDataInvalid), errors.Is(err, telegram.ErrInitDataStale):
		return "invalid_init_data"
	default:
		return ""
	}
}

// occurrenceIDFromHex превращает hex из callback_data (p:<uuid hex>) в UUID-строку.
// При ошибке возвращает исходный hex — чтобы запись журнала не потерялась.
func occurrenceIDFromHex(occHex string) string {
	b, err := hex.DecodeString(occHex)
	if err != nil || len(b) != 16 {
		return occHex
	}
	id, err := uuid.FromBytes(b)
	if err != nil {
		return occHex
	}
	return id.String()
}
