package response

import (
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

// MiniAppUser — профиль пользователя в ответе входа Mini App (id, full_name, role).
type MiniAppUser struct {
	// Идентификатор пользователя.
	ID string `json:"id" format:"uuid" example:"9a191dfa-5e71-47aa-bcb2-ed5e9c071830"`
	// Полное имя.
	FullName string `json:"full_name" example:"Иван Петров"`
	// Роль: admin | user. Админ получает полные права и в Mini App.
	Role string `json:"role" enums:"admin,user" example:"user"`
}

// MiniAppAuthResponse — ответ на вход в Mini App (auth и link): access-токен,
// его TTL в секундах, профиль пользователя и доступность лички бота (§3.3, §7).
type MiniAppAuthResponse struct {
	// Access-токен (Bearer). Хранится в памяти JS, cookie не выставляются.
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	// Тип токена, всегда Bearer.
	TokenType string `json:"token_type" example:"Bearer"`
	// Сколько секунд живёт access-токен.
	ExpiresIn int `json:"expires_in" example:"3600"`
	// Профиль вошедшего пользователя.
	User MiniAppUser `json:"user"`
	// Может ли бот писать пользователю в личку (allows_write_to_pm из initData).
	DMAvailable bool `json:"dm_available" example:"true"`
}

// NewMiniAppUser проецирует model.User на безопасный для Mini App профиль.
func NewMiniAppUser(u model.User) MiniAppUser {
	return MiniAppUser{ID: u.ID, FullName: u.FullName, Role: u.Role}
}

// TelegramLinkCodeResponse — ответ на создание кода привязки (deep link).
type TelegramLinkCodeResponse struct {
	// Ссылка t.me для привязки, открывается в Telegram.
	DeepLink string `json:"deep_link" example:"https://t.me/ik_dev_bot?start=link_AbC..."`
	// Сколько секунд живёт код.
	ExpiresIn int `json:"expires_in" example:"600"`
}

// TelegramLinkResponse — состояние привязки Telegram у текущего пользователя.
// Поля кроме linked присутствуют только при linked=true.
type TelegramLinkResponse struct {
	// Привязан ли Telegram.
	Linked bool `json:"linked" example:"true"`
	// Username в Telegram (может быть пустым).
	Username string `json:"tg_username,omitempty" example:"ivan"`
	// Включена ли доставка в личку.
	DMEnabled *bool `json:"dm_enabled,omitempty"`
	// Заблокировал ли пользователь бота.
	BotBlocked *bool `json:"bot_blocked,omitempty"`
	// Когда привязан.
	LinkedAt *time.Time `json:"linked_at,omitempty"`
}

// TelegramTeamChatResponse — состояние командного чата (админ).
// Поля кроме bound присутствуют только при bound=true.
type TelegramTeamChatResponse struct {
	// Привязан ли активный командный чат.
	Bound bool `json:"bound" example:"true"`
	// Название чата или null.
	Title *string `json:"title,omitempty" example:"Финансы"`
	// ID чата в Telegram.
	TGChatID int64 `json:"tg_chat_id,omitempty" example:"-1001234567890"`
	// Активен ли чат.
	IsActive *bool `json:"is_active,omitempty"`
	// Последняя ошибка доставки (например, «бот удалён из чата») или null.
	LastError *string `json:"last_error,omitempty"`
	// Когда привязан.
	BoundAt *time.Time `json:"bound_at,omitempty"`
}
