package response

// AssistantMemoryInfo — сводка о памяти диалога (из конфига). Отдаётся в
// GET /assistant/history и в GET /assistant/status.
type AssistantMemoryInfo struct {
	// Включена ли память (NLQ_MEMORY_ENABLED).
	Enabled bool `json:"enabled" example:"true"`
	// Сколько последних ходов идёт в контекст промпта (NLQ_MEMORY_TURNS).
	ContextTurns int `json:"context_turns" example:"4"`
	// Срок жизни контекста в секундах (NLQ_MEMORY_TTL).
	TTLSeconds int `json:"ttl_seconds" example:"7200"`
}

// AssistantHistoryItem — один ход диалога в ответе GET /assistant/history.
// Поле memory наружу не отдаётся.
type AssistantHistoryItem struct {
	// ID хода.
	ID string `json:"id" example:"dddddddd-0000-4000-8000-000000000001"`
	// AnswerID совпадает с X-Request-Id ответа.
	AnswerID string `json:"answer_id" example:"f16cc96e-aaf4-44fb-a462-836ed4d0b135"`
	// Текст вопроса.
	Question string `json:"question" example:"что просрочено?"`
	// Короткий текст ответа после grounding.
	AnswerText string `json:"answer_text" example:"Просрочено 2 платежа."`
	// Статус: ok | partial | error.
	Status string `json:"status" enums:"ok,partial,error" example:"ok"`
	// Код ошибки при неуспехе (иначе null).
	ErrorCode *string `json:"error_code" extensions:"x-nullable" example:"timeout"`
	// Режим: llm | rules | fake.
	Mode string `json:"mode" example:"llm"`
	// Момент хода (RFC3339).
	CreatedAt string `json:"created_at" example:"2026-10-24T09:00:00Z"`
	// Попадает ли ход в контекст следующего вопроса (та же логика, что при
	// отправке агенту).
	InContext bool `json:"in_context" example:"true"`
}

// AssistantHistoryResponse — ответ GET /assistant/history. items —
// последние limit ходов по возрастанию времени; total — всего ходов в разговоре.
type AssistantHistoryResponse struct {
	// Канал: web | miniapp | bot.
	Channel string `json:"channel" example:"miniapp"`
	// Область диалога.
	Conversation string `json:"conversation" example:"miniapp"`
	// Ходы по возрастанию времени.
	Items []AssistantHistoryItem `json:"items"`
	// Всего ходов в разговоре.
	Total int `json:"total" example:"12"`
	// Сводка о памяти диалога.
	Memory AssistantMemoryInfo `json:"memory"`
}
