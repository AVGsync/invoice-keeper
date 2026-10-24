package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// Client — клиент Bot API поверх net/http.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient собирает клиент.
func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	return &Client{
		token:   token,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// apiResponse — конверт ответа Bot API.
type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  *struct {
		RetryAfter      int   `json:"retry_after"`
		MigrateToChatID int64 `json:"migrate_to_chat_id"`
	} `json:"parameters"`
}

// call выполняет метод Bot API с JSON-телом и разбирает конверт.
func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram client: %s: marshal: %w", method, err)
	}

	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram client: %s: new request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// *url.Error.Error() включает полный URL запроса, а в нём путь /bot<TOKEN>/.
		var ue *neturl.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("telegram client: %s: do: %w", method, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("telegram client: %s: read body: %w", method, err)
	}

	var env apiResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("telegram client: %s: decode (http %d): %w", method, resp.StatusCode, err)
	}

	if !env.OK {
		apiErr := &APIError{Code: env.ErrorCode, Description: env.Description}
		if apiErr.Code == 0 {
			apiErr.Code = resp.StatusCode
		}
		if env.Parameters != nil {
			apiErr.RetryAfter = env.Parameters.RetryAfter
			apiErr.MigrateToChatID = env.Parameters.MigrateToChatID
		}
		return apiErr
	}

	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("telegram client: %s: decode result: %w", method, err)
		}
	}
	return nil
}

// GetMe проверяет токен и возвращает профиль бота.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	err := c.call(ctx, "getMe", struct{}{}, &u)
	return u, err
}

// SendMessageParams — параметры sendMessage.
type SendMessageParams struct {
	ChatID                int64                 `json:"chat_id"`
	Text                  string                `json:"text"`
	ParseMode             string                `json:"parse_mode,omitempty"`
	ReplyMarkup           *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	DisableWebPagePreview bool                  `json:"disable_web_page_preview,omitempty"`
}

// SendMessage отправляет сообщение.
func (c *Client) SendMessage(ctx context.Context, p SendMessageParams) (Message, error) {
	var m Message
	err := c.call(ctx, "sendMessage", p, &m)
	return m, err
}

// SendChatActionParams — параметры sendChatAction (индикатор «печатает…»).
type SendChatActionParams struct {
	ChatID int64  `json:"chat_id"`
	Action string `json:"action"`
}

// SendChatAction показывает действие бота в чате (например, "typing").
func (c *Client) SendChatAction(ctx context.Context, p SendChatActionParams) error {
	return c.call(ctx, "sendChatAction", p, nil)
}

// EditMessageTextParams — параметры editMessageText.
type EditMessageTextParams struct {
	ChatID      int64                 `json:"chat_id"`
	MessageID   int64                 `json:"message_id"`
	Text        string                `json:"text"`
	ParseMode   string                `json:"parse_mode,omitempty"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// EditMessageText правит текст и клавиатуру ранее отправленного сообщения.
func (c *Client) EditMessageText(ctx context.Context, p EditMessageTextParams) (Message, error) {
	var m Message
	err := c.call(ctx, "editMessageText", p, &m)
	return m, err
}

// EditMessageReplyMarkupParams — параметры editMessageReplyMarkup.
type EditMessageReplyMarkupParams struct {
	ChatID      int64                 `json:"chat_id"`
	MessageID   int64                 `json:"message_id"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// EditMessageReplyMarkup меняет только клавиатуру (например, убирает кнопки).
func (c *Client) EditMessageReplyMarkup(ctx context.Context, p EditMessageReplyMarkupParams) error {
	return c.call(ctx, "editMessageReplyMarkup", p, nil)
}

// AnswerCallbackParams — параметры answerCallbackQuery.
type AnswerCallbackParams struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}

// AnswerCallbackQuery гасит спиннер на кнопке и показывает уведомление/alert.
func (c *Client) AnswerCallbackQuery(ctx context.Context, p AnswerCallbackParams) error {
	return c.call(ctx, "answerCallbackQuery", p, nil)
}

// GetUpdatesParams — параметры getUpdates (long polling).
type GetUpdatesParams struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

// GetUpdates забирает апдейты long polling.
func (c *Client) GetUpdates(ctx context.Context, p GetUpdatesParams) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", p, &updates)
	return updates, err
}

// SetWebhookParams — параметры setWebhook.
type SetWebhookParams struct {
	URL            string   `json:"url"`
	SecretToken    string   `json:"secret_token,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

// SetWebhook регистрирует webhook.
func (c *Client) SetWebhook(ctx context.Context, p SetWebhookParams) error {
	return c.call(ctx, "setWebhook", p, nil)
}

// GetChat возвращает сведения о чате.
func (c *Client) GetChat(ctx context.Context, chatID int64) (Chat, error) {
	var ch Chat
	err := c.call(ctx, "getChat", struct {
		ChatID int64 `json:"chat_id"`
	}{ChatID: chatID}, &ch)
	return ch, err
}
