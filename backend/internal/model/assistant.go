package model

// AssistantAnswer — разобранный конверт ответа агента (docs/NL_QUERIES.md §7.9).
// Используется потребителями внутри процесса (Telegram-бот через порт
// NLAnswerer): текст и блоки уже проверены агентом (grounding), боту остаётся
// только компактно их отрендерить. HTTP-ручка, наоборот, проксирует сырой
// конверт байт-в-байт, чтобы фронт получил точную схему блоков и meta.
type AssistantAnswer struct {
	// AnswerID совпадает с X-Request-Id.
	AnswerID string
	// Status: ok | partial | error.
	Status string
	// ErrorCode заполнен при Status=error: llm_unavailable | data_unavailable |
	// timeout | bad_question.
	ErrorCode string
	// Mode: llm | rules | fake.
	Mode string
	// Text — короткий текст ответа, plain text (рендерить как текст).
	Text string
	// Blocks — блоки данных (occurrence_list, currency_totals, payment_list,
	// people_list). Хранятся как разобранный JSON, чтобы бот мог рендерить их не
	// завися от точной схемы.
	Blocks []map[string]any
	// Followups — короткие подсказки «можно спросить…».
	Followups []string
	// Notes — оговорки, добавленные кодом агента (валюты не складывались и т. п.).
	Notes []string
}
