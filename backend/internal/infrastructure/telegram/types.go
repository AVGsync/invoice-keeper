// Package telegram — минимальный клиент Bot API на net/http без внешних
// зависимостей (docs/TELEGRAM_BOT.md §8). Содержит типы апдейтов, типизированные
// ошибки, клиент и его подмену для тестов.
package telegram

// Update — входящий апдейт Telegram. Разбираются только нужные боту поля.
type Update struct {
	UpdateID      int64              `json:"update_id"`
	Message       *Message           `json:"message,omitempty"`
	CallbackQuery *CallbackQuery     `json:"callback_query,omitempty"`
	MyChatMember  *ChatMemberUpdated `json:"my_chat_member,omitempty"`
}

// Message — сообщение. MigrateToChatID/MigrateFromChatID приходят в сервисном
// сообщении при превращении группы в супергруппу.
type Message struct {
	MessageID       int64  `json:"message_id"`
	From            *User  `json:"from,omitempty"`
	SenderChat      *Chat  `json:"sender_chat,omitempty"`
	Chat            Chat   `json:"chat"`
	Text            string `json:"text,omitempty"`
	MigrateToChatID int64  `json:"migrate_to_chat_id,omitempty"`
}

// User — пользователь Telegram.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat — чат (личка, группа, супергруппа, канал).
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

// CallbackQuery — нажатие inline-кнопки.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// ChatMemberUpdated — апдейт my_chat_member: изменение статуса бота в чате.
type ChatMemberUpdated struct {
	Chat          Chat       `json:"chat"`
	From          User       `json:"from"`
	NewChatMember ChatMember `json:"new_chat_member"`
	OldChatMember ChatMember `json:"old_chat_member"`
}

// ChatMember — членство в чате; боту важен статус (member/administrator/left/kicked)
// и, при status="restricted", флаг is_member: ограниченный бот может оставаться
// в чате.
type ChatMember struct {
	Status   string `json:"status"`
	User     User   `json:"user"`
	IsMember bool   `json:"is_member"`
}

// InlineKeyboardMarkup — inline-клавиатура сообщения.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton — кнопка. Либо CallbackData, либо WebApp (web_app-кнопки
// запрещены в группах, см. §5.5).
type InlineKeyboardButton struct {
	Text         string      `json:"text"`
	CallbackData string      `json:"callback_data,omitempty"`
	URL          string      `json:"url,omitempty"`
	WebApp       *WebAppInfo `json:"web_app,omitempty"`
}

// WebAppInfo — ссылка на Mini App.
type WebAppInfo struct {
	URL string `json:"url"`
}

// Статусы членства, при которых бот фактически отсутствует в чате.
const (
	StatusLeft       = "left"
	StatusKicked     = "kicked"
	StatusMember     = "member"
	StatusAdmin      = "administrator"
	StatusCreator    = "creator"
	StatusRestricted = "restricted"
)

// IsPresent сообщает, что при таком статусе бот состоит в чате. Для
// "restricted" бот может оставаться членом (is_member=true): в этом случае не
// деактивируем командный чат из-за ограничения прав.
func (m ChatMember) IsPresent() bool {
	switch m.Status {
	case StatusMember, StatusAdmin, StatusCreator:
		return true
	case StatusRestricted:
		return m.IsMember
	default:
		return false
	}
}
