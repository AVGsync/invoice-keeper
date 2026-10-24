package telegrambot

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
)

// longPollTimeout — таймаут long polling getUpdates.
const longPollTimeout = 25

// allowedUpdates — апдейты, которые бот просит у Telegram.
var allowedUpdates = []string{"message", "callback_query", "my_chat_member"}

// Poller читает апдейты long polling в одной горутине.
type Poller struct {
	svc    *Service
	client BotAPI
}

// NewPoller собирает поллер.
func NewPoller(svc *Service, client BotAPI) *Poller {
	return &Poller{svc: svc, client: client}
}

// Run крутит long polling до отмены ctx.
func (p *Poller) Run(ctx context.Context) {
	offset, err := p.svc.repo.MaxUpdateID(ctx)
	if err != nil {
		slog.Error("telegram poller: init offset", "error", err)
	} else if offset > 0 {
		offset++ // следующий ожидаемый апдейт
	}

	slog.Info("telegram poller: started", "offset", offset)
	for {
		if ctx.Err() != nil {
			slog.Info("telegram poller: stopped")
			return
		}

		// Контекст запроса: long poll + запас на сеть, но не висим после отмены.
		reqCtx, cancel := context.WithTimeout(ctx, (longPollTimeout+5)*time.Second)
		updates, err := p.client.GetUpdates(reqCtx, telegram.GetUpdatesParams{
			Offset:         offset,
			Timeout:        longPollTimeout,
			AllowedUpdates: allowedUpdates,
		})
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				slog.Info("telegram poller: stopped")
				return
			}
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.Code == 409 {
				// 409: у токена стоит webhook.
				slog.Error("telegram poller: conflict 409 — токен уже используется в режиме webhook; для polling нужен отдельный dev-бот", "error", err)
			} else {
				slog.Error("telegram poller: get updates", "error", err)
			}
			// Небольшая пауза, чтобы не крутить цикл вхолостую на ошибке.
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}

		for _, upd := range updates {
			p.svc.HandleUpdate(ctx, upd)
			if upd.UpdateID >= offset {
				offset = upd.UpdateID + 1
			}
		}
	}
}
