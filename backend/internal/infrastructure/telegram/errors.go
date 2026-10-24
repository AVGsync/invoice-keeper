package telegram

import (
	"fmt"
	"strings"
)

// APIError — ошибка Bot API: ok=false. Поля parameters (retry_after,
// migrate_to_chat_id) разбираются из ответа, по ним движок принимает решения
// (docs/TELEGRAM_BOT.md §5.4).
type APIError struct {
	Code            int
	Description     string
	RetryAfter      int
	MigrateToChatID int64
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram api error %d: %s", e.Code, e.Description)
}

// IsRetryAfter — флуд-контроль 429 с указанием retry_after.
func (e *APIError) IsRetryAfter() bool {
	return e.Code == 429 || e.RetryAfter > 0
}

// IsBotBlocked — бот заблокирован пользователем в личке (403).
func (e *APIError) IsBotBlocked() bool {
	return e.Code == 403
}

// IsChatNotFound — чат не найден (бот удалён из группы и т.п.).
func (e *APIError) IsChatNotFound() bool {
	if e.Code != 400 && e.Code != 403 {
		return false
	}
	d := strings.ToLower(e.Description)
	return strings.Contains(d, "chat not found") ||
		strings.Contains(d, "bot was kicked") ||
		strings.Contains(d, "not a member") ||
		strings.Contains(d, "group chat was deactivated")
}

// IsMigrate — группа превратилась в супергруппу, нужно сменить chat_id.
func (e *APIError) IsMigrate() bool {
	return e.MigrateToChatID != 0
}

// IsMessageNotModified — editMessage без изменений: безопасно игнорировать (§5.6).
func (e *APIError) IsMessageNotModified() bool {
	return strings.Contains(strings.ToLower(e.Description), "message is not modified")
}

// IsEditUnrecoverable — правку сообщения уже не выполнить и повтор не поможет:
// сообщение удалено/недоступно или чат потерян. Reconcile на такой ошибке
// фиксирует целевой shown_status, чтобы не долбить editMessageText вечно (§5.6).
func (e *APIError) IsEditUnrecoverable() bool {
	if e.Code != 400 && e.Code != 403 {
		return false
	}
	d := strings.ToLower(e.Description)
	return strings.Contains(d, "message to edit not found") ||
		strings.Contains(d, "message can't be edited") ||
		strings.Contains(d, "message_id_invalid") ||
		e.IsChatNotFound()
}
