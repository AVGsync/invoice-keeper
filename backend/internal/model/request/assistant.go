package request

// AssistantAskRequest — тело POST /api/v1/assistant/ask и /ask/stream. Вопрос валидируется на сервере: trim, 1–500 рун,
// без управляющих символов. Канал bot по HTTP запрещён (его ставит только бот).
type AssistantAskRequest struct {
	// Вопрос на естественном языке, 1–500 символов.
	Question string `json:"question" example:"что просрочено?"`
	// Канал обращения: web (по умолчанию) или miniapp.
	Channel string `json:"channel,omitempty" enums:"web,miniapp" example:"web"`
}
