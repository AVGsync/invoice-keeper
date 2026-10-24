// Package telegrambot — домен бота: привязка аккаунтов, команды, callback-кнопки
// и приём апдейтов (polling/webhook). Движок напоминаний и Mini App auth — в
// отдельных пакетах/этапах (docs/TELEGRAM_BOT.md §8).
package telegrambot

import (
	"context"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// Repository — хранилище ядра бота. Реализует postgres.TelegramRepository.
type Repository interface {
	GetLinkByUserID(ctx context.Context, userID string) (model.TelegramLink, error)
	GetLinkByTGUserID(ctx context.Context, tgUserID int64) (model.TelegramLink, error)
	UpsertLink(ctx context.Context, userID string, tgUserID int64, username string) error
	DeleteLink(ctx context.Context, userID string) error
	SetBotBlocked(ctx context.Context, tgUserID int64, blocked bool) error

	CountActiveCodes(ctx context.Context, userID string) (int, error)
	CreateLinkCode(ctx context.Context, userID, codeHash string, expiresAt time.Time) (string, error)
	ClaimLinkCode(ctx context.Context, codeHash string, tgUserID int64) (codeID, userID string, err error)
	ConfirmLinkCode(ctx context.Context, codeID string, tgUserID int64) (string, error)
	CancelLinkCode(ctx context.Context, codeID string, tgUserID int64) error

	GetActiveChat(ctx context.Context) (model.TelegramChat, error)
	GetLastDeactivatedChat(ctx context.Context) (model.TelegramChat, error)
	BindChat(ctx context.Context, tgChatID int64, chatType, title, boundBy string) (prev model.TelegramChat, hadPrev bool, err error)
	DeactivateChat(ctx context.Context, tgChatID int64, reason string) error
	UpdateChatID(ctx context.Context, oldID, newID int64) error
	SeedChat(ctx context.Context, tgChatID int64, chatType string) error

	MarkUpdateSeen(ctx context.Context, updateID int64) (bool, error)
	MaxUpdateID(ctx context.Context) (int64, error)
}

// UserReader читает пользователей для подтверждения привязки, проверки роли
// admin в /bind и входа в Mini App по логину/паролю. Реализует
// postgres.UserRepository (методы GetUserByID, GetUserByLogin).
type UserReader interface {
	GetUserByID(id string, ctx context.Context) (model.User, error)
	GetUserByLogin(login string, ctx context.Context) (model.User, error)
}

// TokenIssuer выпускает access-токен для входа в Mini App (§3.3). Реализует
// service.UserService (метод IssueAccessForUser). Живёт в auth-домене, не у B.
type TokenIssuer interface {
	IssueAccessForUser(ctx context.Context, userID string) (token string, expiresIn int, err error)
}

// Hasher проверяет пароль при входе в Mini App по логину/паролю. Реализует
// security.Hasher (метод CheckPassword).
type Hasher interface {
	CheckPassword(plain, hashed string) bool
}

// BotAPI — часть клиента Bot API, нужная боту. Реализуют *telegram.Client и
// *telegram.FakeClient.
type BotAPI interface {
	GetMe(ctx context.Context) (telegram.User, error)
	SendMessage(ctx context.Context, p telegram.SendMessageParams) (telegram.Message, error)
	EditMessageText(ctx context.Context, p telegram.EditMessageTextParams) (telegram.Message, error)
	AnswerCallbackQuery(ctx context.Context, p telegram.AnswerCallbackParams) error
	GetUpdates(ctx context.Context, p telegram.GetUpdatesParams) ([]telegram.Update, error)
	SetWebhook(ctx context.Context, p telegram.SetWebhookParams) error
	SendChatAction(ctx context.Context, p telegram.SendChatActionParams) error
}

// NLAnswerer — порт NL-ассистента, объявленный потребителем (ботом, §4.4).
// Реализует *service.AssistantService. channel всегда "bot"; лимиты общие с HTTP.
type NLAnswerer interface {
	// Ask — вопрос с разговором по каналу (bot → tg:dm). Сохранён для совместимости.
	Ask(ctx context.Context, userID, channel, question string) (model.AssistantAnswer, error)
	// AskIn — вопрос с явным разговором (tg:dm в личке, tg:<chat_id> в группе).
	AskIn(ctx context.Context, userID, conversation, question string) (model.AssistantAnswer, error)
	// ResetConversation очищает один разговор пользователя (команды /new, /reset).
	ResetConversation(ctx context.Context, userID, channel, conversation string) (int, error)
}
