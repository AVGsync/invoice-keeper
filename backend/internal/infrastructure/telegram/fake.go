package telegram

import (
	"context"
	"sync"
)

// FakeClient — подмена клиента Bot API для тестов. Пишет все вызовы в срезы и
// возвращает заранее заданные ответы/ошибки. Потокобезопасен.
type FakeClient struct {
	mu sync.Mutex

	// Записанные вызовы.
	Sent     []SendMessageParams
	Edited   []EditMessageTextParams
	Markups  []EditMessageReplyMarkupParams
	Answers  []AnswerCallbackParams
	Webhooks []SetWebhookParams
	Actions  []SendChatActionParams

	// Управление ответами.
	Me            User
	NextMessageID int64
	// Ошибки на конкретные методы (nil — успех).
	SendErr error
	// sendErrQueue — ошибки по одной на вызов SendMessage (nil => успех). Имеет
	// приоритет над SendErr и позволяет проверять ветки handleSendError и повтор
	// после migrate. Заполняется QueueSendErr.
	sendErrQueue []error
	EditErr      error
	MarkupErr    error
	AnswerErr    error
	GetMeErr     error
	// Очередь ответов getUpdates: по одному срезу за вызов, затем пусто.
	Updates [][]Update
}

// NewFakeClient создаёт готовый к работе фейковый клиент.
func NewFakeClient() *FakeClient {
	return &FakeClient{Me: User{ID: 1, IsBot: true, Username: "test_bot"}, NextMessageID: 1000}
}

func (f *FakeClient) GetMe(ctx context.Context) (User, error) {
	if f.GetMeErr != nil {
		return User{}, f.GetMeErr
	}
	return f.Me, nil
}

func (f *FakeClient) SendMessage(ctx context.Context, p SendMessageParams) (Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sendErrQueue) > 0 {
		err := f.sendErrQueue[0]
		f.sendErrQueue = f.sendErrQueue[1:]
		if err != nil {
			return Message{}, err
		}
	} else if f.SendErr != nil {
		return Message{}, f.SendErr
	}
	f.Sent = append(f.Sent, p)
	f.NextMessageID++
	return Message{MessageID: f.NextMessageID, Chat: Chat{ID: p.ChatID}, Text: p.Text}, nil
}

// QueueSendErr задаёт последовательность ответов SendMessage: по одному на вызов
// (nil => успех). Опустошается по мере вызовов, затем работает обычная логика.
func (f *FakeClient) QueueSendErr(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendErrQueue = append(f.sendErrQueue, errs...)
}

func (f *FakeClient) EditMessageText(ctx context.Context, p EditMessageTextParams) (Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.EditErr != nil {
		return Message{}, f.EditErr
	}
	f.Edited = append(f.Edited, p)
	return Message{MessageID: p.MessageID, Chat: Chat{ID: p.ChatID}, Text: p.Text}, nil
}

func (f *FakeClient) EditMessageReplyMarkup(ctx context.Context, p EditMessageReplyMarkupParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.MarkupErr != nil {
		return f.MarkupErr
	}
	f.Markups = append(f.Markups, p)
	return nil
}

func (f *FakeClient) AnswerCallbackQuery(ctx context.Context, p AnswerCallbackParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.AnswerErr != nil {
		return f.AnswerErr
	}
	f.Answers = append(f.Answers, p)
	return nil
}

func (f *FakeClient) GetUpdates(ctx context.Context, p GetUpdatesParams) ([]Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Updates) == 0 {
		return nil, nil
	}
	batch := f.Updates[0]
	f.Updates = f.Updates[1:]
	return batch, nil
}

func (f *FakeClient) SetWebhook(ctx context.Context, p SetWebhookParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Webhooks = append(f.Webhooks, p)
	return nil
}

func (f *FakeClient) SendChatAction(ctx context.Context, p SendChatActionParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Actions = append(f.Actions, p)
	return nil
}

func (f *FakeClient) GetChat(ctx context.Context, chatID int64) (Chat, error) {
	return Chat{ID: chatID, Type: "group"}, nil
}

// LastSent возвращает последнее отправленное сообщение и true, если оно было.
func (f *FakeClient) LastSent() (SendMessageParams, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Sent) == 0 {
		return SendMessageParams{}, false
	}
	return f.Sent[len(f.Sent)-1], true
}
