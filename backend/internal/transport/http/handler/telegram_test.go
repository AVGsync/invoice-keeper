package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/telegrambot"
)

// fakeTGUseCase — заглушка TelegramUseCase; считает вызовы HandleUpdate.
type fakeTGUseCase struct {
	mu           sync.Mutex
	handleCalls  int
	lastUpdateID int64
}

func (f *fakeTGUseCase) CreateLinkCode(ctx context.Context, userID string) (string, int, error) {
	return "", 0, nil
}
func (f *fakeTGUseCase) GetLinkStatus(ctx context.Context, userID string) (telegrambot.LinkStatus, error) {
	return telegrambot.LinkStatus{}, nil
}
func (f *fakeTGUseCase) Unlink(ctx context.Context, userID string) error { return nil }
func (f *fakeTGUseCase) GetTeamChat(ctx context.Context) (model.TelegramChat, error) {
	return model.TelegramChat{}, model.ErrTelegramNoTeamChat
}
func (f *fakeTGUseCase) SendTeamChatTest(ctx context.Context) error { return nil }
func (f *fakeTGUseCase) HandleUpdate(ctx context.Context, upd telegram.Update) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handleCalls++
	f.lastUpdateID = upd.UpdateID
}
func (f *fakeTGUseCase) MiniAppAuth(ctx context.Context, initData string) (telegrambot.MiniAppResult, error) {
	return telegrambot.MiniAppResult{}, nil
}
func (f *fakeTGUseCase) MiniAppLink(ctx context.Context, initData, login, password string) (telegrambot.MiniAppResult, error) {
	return telegrambot.MiniAppResult{}, nil
}

func TestWebhook(t *testing.T) {
	const secret = "0123456789012345678901234567890123"

	cases := []struct {
		name       string
		header     string
		body       string
		wantStatus int
		wantCalls  int
	}{
		{"valid update calls HandleUpdate once", secret, `{"update_id":77,"message":{"message_id":1,"chat":{"id":5,"type":"private"},"text":"/help"}}`, http.StatusOK, 1},
		{"wrong secret -> 401, body unparsed", "nope", `{"update_id":1}`, http.StatusUnauthorized, 0},
		{"empty secret header -> 401", "", `{"update_id":1}`, http.StatusUnauthorized, 0},
		{"malformed body -> 200, no call", secret, `{not json`, http.StatusOK, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc := &fakeTGUseCase{}
			h := NewTelegramHandler(uc, secret)
			req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(c.body))
			if c.header != "" {
				req.Header.Set("X-Telegram-Bot-Api-Secret-Token", c.header)
			}
			rr := httptest.NewRecorder()
			h.Webhook()(rr, req)

			if rr.Code != c.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, c.wantStatus)
			}
			if uc.handleCalls != c.wantCalls {
				t.Errorf("HandleUpdate calls = %d, want %d", uc.handleCalls, c.wantCalls)
			}
			if c.wantCalls == 1 && uc.lastUpdateID != 77 {
				t.Errorf("update_id = %d, want 77", uc.lastUpdateID)
			}
		})
	}
}
