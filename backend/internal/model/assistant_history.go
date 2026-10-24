package model

import "time"

// AssistantTurn — один ход диалога с ассистентом (таблица assistant_turns,
// миграция 000013). Память для уточняющих вопросов: вопрос, короткий текст
// ответа (после grounding), статус/режим и компактный объект memory
// (аргументы инструментов, факты, «заземлённые» числа). Сырые строки данных,
// токены и initData сюда не попадают (docs/NL_QUERIES.md, память диалога).
type AssistantTurn struct {
	// ID — UUID хода.
	ID string
	// UserID — владелец диалога.
	UserID string
	// Channel — канал: web | miniapp | bot.
	Channel string
	// Conversation — область диалога: web | miniapp | tg:dm | tg:<chat_id>.
	Conversation string
	// AnswerID совпадает с X-Request-Id ответа.
	AnswerID string
	// Question — текст вопроса (1..500).
	Question string
	// AnswerText — короткий текст ответа после grounding (≤1200).
	AnswerText string
	// Status: ok | partial | error.
	Status string
	// ErrorCode заполнен при неуспехе (nil — иначе).
	ErrorCode *string
	// Mode: llm | rules | fake (или пусто).
	Mode string
	// Memory — компактный объект памяти хода (сырой JSON). Наружу не отдаётся.
	Memory []byte
	// CreatedAt — момент записи.
	CreatedAt time.Time
	// InContext — попадёт ли ход в промпт/контекст (считается selectContext при
	// чтении, в БД не хранится).
	InContext bool
}
