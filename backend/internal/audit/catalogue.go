package audit

// Коды действий журнала.
const (
	ActionAuthLogin   = "auth.login"
	ActionAuthLogout  = "auth.logout"
	ActionAuthRefresh = "auth.refresh"
	ActionAuthMiniApp = "auth.miniapp"

	ActionTelegramLink           = "telegram.link"
	ActionTelegramLinkCode       = "telegram.link_code"
	ActionTelegramUnlink         = "telegram.unlink"
	ActionTelegramTeamChatBind   = "telegram.team_chat_bind"
	ActionTelegramTeamChatUnbind = "telegram.team_chat_unbind"
	ActionTelegramTeamChatTest   = "telegram.team_chat_test"

	ActionUserCreate = "user.create"
	ActionUserUpdate = "user.update"
	ActionUserDelete = "user.delete"

	ActionPersonaCreate = "persona.create"
	ActionPersonaUpdate = "persona.update"
	ActionPersonaDelete = "persona.delete"

	ActionRecurringCreate = "recurring.create"
	ActionRecurringUpdate = "recurring.update"
	ActionRecurringStop   = "recurring.stop"

	ActionOccurrenceConfirm        = "occurrence.confirm"
	ActionOccurrencePaidDateChange = "occurrence.paid_date_change"
	ActionOccurrenceCancel         = "occurrence.cancel"
	ActionOccurrenceUpdate         = "occurrence.update"

	ActionReceiptAttach = "receipt.attach"
	ActionReceiptDelete = "receipt.delete"

	ActionNotificationEmailVerify    = "notification.email_verify"
	ActionNotificationTestEmail      = "notification.test_email"
	ActionNotificationSettingsCreate = "notification_settings.create"
	ActionNotificationSettingsUpdate = "notification_settings.update"
	ActionNotificationSettingsDelete = "notification_settings.delete"

	ActionBotPaidButton = "bot.paid_button"

	ActionAssistantAsk          = "assistant.ask"
	ActionAssistantHistoryClear = "assistant.history_clear"
)

// Типы сущностей журнала.
const (
	EntityUser                  = "user"
	EntityTelegramLink          = "telegram_link"
	EntityTelegramChat          = "telegram_chat"
	EntityPersona               = "persona"
	EntityRecurringPayment      = "recurring_payment"
	EntityOccurrence            = "occurrence"
	EntityReceipt               = "receipt"
	EntityNotificationSettings  = "notification_settings"
	EntityAssistantTurn         = "assistant_turn"
	EntityAssistantConversation = "assistant_conversation"
)

// ActionInfo — описание действия для фильтров UI (каталог).
type ActionInfo struct {
	Action     string `json:"action"`
	Label      string `json:"label"`
	EntityType string `json:"entity_type"`
}

// Catalogue — список всех действий журнала с русскими метками.
var Catalogue = []ActionInfo{
	{ActionAuthLogin, "Вход", EntityUser},
	{ActionAuthLogout, "Выход", EntityUser},
	{ActionAuthRefresh, "Неудачное обновление сессии", ""},
	{ActionAuthMiniApp, "Вход в Mini App", EntityUser},

	{ActionTelegramLink, "Привязан Telegram", EntityTelegramLink},
	{ActionTelegramLinkCode, "Создана ссылка привязки", EntityTelegramLink},
	{ActionTelegramUnlink, "Отвязан Telegram", EntityTelegramLink},
	{ActionTelegramTeamChatBind, "Назначен командный чат", EntityTelegramChat},
	{ActionTelegramTeamChatUnbind, "Командный чат отвязан", EntityTelegramChat},
	{ActionTelegramTeamChatTest, "Проверка командного чата", EntityTelegramChat},

	{ActionUserCreate, "Создан пользователь", EntityUser},
	{ActionUserUpdate, "Изменён пользователь", EntityUser},
	{ActionUserDelete, "Удалён пользователь", EntityUser},

	{ActionPersonaCreate, "Создана персона", EntityPersona},
	{ActionPersonaUpdate, "Изменена персона", EntityPersona},
	{ActionPersonaDelete, "Удалена персона", EntityPersona},

	{ActionRecurringCreate, "Создан регулярный платёж", EntityRecurringPayment},
	{ActionRecurringUpdate, "Изменён регулярный платёж", EntityRecurringPayment},
	{ActionRecurringStop, "Остановлен регулярный платёж", EntityRecurringPayment},

	{ActionOccurrenceConfirm, "Отмечена оплата", EntityOccurrence},
	{ActionOccurrencePaidDateChange, "Изменена дата оплаты", EntityOccurrence},
	{ActionOccurrenceCancel, "Снята оплата", EntityOccurrence},
	{ActionOccurrenceUpdate, "Изменён период", EntityOccurrence},

	{ActionReceiptAttach, "Прикреплён чек", EntityReceipt},
	{ActionReceiptDelete, "Удалён чек", EntityReceipt},

	{ActionNotificationEmailVerify, "Подтверждена почта", EntityNotificationSettings},
	{ActionNotificationTestEmail, "Тестовое письмо", EntityNotificationSettings},
	{ActionNotificationSettingsCreate, "Настройки уведомлений созданы", EntityNotificationSettings},
	{ActionNotificationSettingsUpdate, "Настройки уведомлений изменены", EntityNotificationSettings},
	{ActionNotificationSettingsDelete, "Настройки уведомлений удалены", EntityNotificationSettings},

	{ActionBotPaidButton, "Нажата «Оплачено» в боте", EntityOccurrence},

	{ActionAssistantAsk, "Вопрос ассистенту", EntityAssistantTurn},
	{ActionAssistantHistoryClear, "Очищена история ассистента", EntityAssistantConversation},
}

// labelByAction — индекс для быстрого Label.
var labelByAction = func() map[string]string {
	m := make(map[string]string, len(Catalogue))
	for _, a := range Catalogue {
		m[a.Action] = a.Label
	}
	return m
}()

// Label возвращает русскую метку действия или само действие, если его нет в каталоге (чтобы незнакомый код всё равно показывался в UI).
func Label(action string) string {
	if l, ok := labelByAction[action]; ok {
		return l
	}
	return action
}
