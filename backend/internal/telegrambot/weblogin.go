package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

const webLoginTTL = 5 * time.Minute

// WebLoginStore — хранилище запросов на вход через бота.
type WebLoginStore interface {
	CreateWebLogin(ctx context.Context, codeHash string, expiresAt time.Time) (string, error)
	FindWebLogin(ctx context.Context, codeHash string) (string, error)
	ConfirmWebLogin(ctx context.Context, id, userID string) error
	CancelWebLogin(ctx context.Context, id string) error
	PollWebLogin(ctx context.Context, codeHash string) (status, userID string, err error)
}

// SetWebLoginStore подключает хранилище входа через бота.
func (s *Service) SetWebLoginStore(w WebLoginStore) { s.webLogins = w }

// StartWebLogin создаёт запрос на вход и deep link для бота.
func (s *Service) StartWebLogin(ctx context.Context) (code, deepLink string, expiresIn int, err error) {
	if !s.cfg.Enabled || s.cfg.BotUsername == "" || s.webLogins == nil {
		return "", "", 0, ErrBotDisabled
	}
	code, err = newLinkCode()
	if err != nil {
		return "", "", 0, fmt.Errorf("telegram service: start web login: %w", err)
	}
	if _, err := s.webLogins.CreateWebLogin(ctx, hashCode(code), time.Now().Add(webLoginTTL)); err != nil {
		return "", "", 0, fmt.Errorf("telegram service: start web login: %w", err)
	}
	return code, fmt.Sprintf("https://t.me/%s?start=login_%s", s.cfg.BotUsername, code), int(webLoginTTL.Seconds()), nil
}

// PollWebLogin: pending — ждём бота, confirmed — userID можно выпускать сессию, invalid — истёк или отклонён.
func (s *Service) PollWebLogin(ctx context.Context, code string) (status, userID string, err error) {
	if s.webLogins == nil {
		return "", "", ErrBotDisabled
	}
	return s.webLogins.PollWebLogin(ctx, hashCode(code))
}

func (s *Service) handleStartLogin(ctx context.Context, msg *telegram.Message, code string) {
	if msg.Chat.Type != "private" {
		s.reply(ctx, msg.Chat.ID, "Откройте ссылку входа в личном чате с ботом.")
		return
	}
	if msg.From == nil || s.webLogins == nil {
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, msg.From.ID)
	if err != nil {
		if errors.Is(err, model.ErrTelegramNotLinked) {
			s.reply(ctx, msg.Chat.ID, "Этот Telegram не привязан к аккаунту. Войдите по логину и паролю и нажмите «Привязать Telegram» в профиле.")
			return
		}
		slog.Error("telegram bot: web login link", "error", err)
		return
	}
	id, err := s.webLogins.FindWebLogin(ctx, hashCode(code))
	if err != nil {
		s.reply(ctx, msg.Chat.ID, "Ссылка входа недействительна или устарела. Вернитесь на сайт и нажмите «Войти через Telegram» ещё раз.")
		return
	}
	user, err := s.users.GetUserByID(link.UserID, ctx)
	if err != nil {
		slog.Error("telegram bot: web login user", "error", err)
		s.reply(ctx, msg.Chat.ID, "Не удалось найти аккаунт. Попробуйте позже.")
		return
	}
	s.send(ctx, telegram.SendMessageParams{
		ChatID:    msg.Chat.ID,
		Text:      fmt.Sprintf("Войти на сайт Invoice Keeper как «%s» (%s)?\nЕсли вы не запрашивали вход, нажмите «Это не я».", html.EscapeString(user.FullName), html.EscapeString(user.Login)),
		ParseMode: "HTML",
		ReplyMarkup: &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{{
			{Text: "Войти", CallbackData: "wl:" + id},
			{Text: "Это не я", CallbackData: "wx:" + id},
		}}},
	})
}

func (s *Service) confirmWebLogin(ctx context.Context, cb *telegram.CallbackQuery, id string) {
	if s.webLogins == nil {
		return
	}
	link, err := s.repo.GetLinkByTGUserID(ctx, cb.From.ID)
	if err == nil {
		err = s.webLogins.ConfirmWebLogin(ctx, id, link.UserID)
	}
	if err != nil {
		s.answer(ctx, cb.ID, "Ссылка недействительна или устарела.", true)
		s.editCallbackMessage(ctx, cb, "Вход не выполнен: ссылка недействительна или устарела.")
		return
	}
	s.answer(ctx, cb.ID, "Готово", false)
	s.editCallbackMessage(ctx, cb, "Вход подтверждён. Вернитесь на сайт.")
}

func (s *Service) cancelWebLogin(ctx context.Context, cb *telegram.CallbackQuery, id string) {
	if s.webLogins != nil {
		if err := s.webLogins.CancelWebLogin(ctx, id); err != nil {
			slog.Error("telegram bot: cancel web login", "error", err)
		}
	}
	s.answer(ctx, cb.ID, "Отменено", false)
	s.editCallbackMessage(ctx, cb, "Вход отменён.")
}
