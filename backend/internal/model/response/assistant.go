package response

// AssistantStatusResponse — ответ GET /api/v1/assistant/status. UI прячет ассистента при enabled=false.
type AssistantStatusResponse struct {
	// Включена ли фича (NLQ_ENABLED).
	Enabled bool `json:"enabled" example:"true"`
	// Отвечает ли агент на /healthz (кэш 10 с). При выключенной фиче всегда false.
	Healthy bool `json:"healthy" example:"true"`
	// Режим «мозга» агента: llm | rules | fake. Пусто, если агент недоступен.
	Mode string `json:"mode,omitempty" example:"llm"`
	// Примеры вопросов для подсказки в UI.
	Examples []string `json:"examples" example:"Что просрочено?,Сколько платим в долларах в этом месяце?,Кто отвечает за аренду?"`
	// Сводка о памяти диалога (из конфига): включена ли, сколько ходов в контекст, срок жизни.
	Memory AssistantMemoryInfo `json:"memory"`
}

// AssistantAnswerResponse документирует конверт ответа агента для swagger.
// Фактический ответ /ask проксируется от агента как есть, поэтому поля описаны обзорно.
type AssistantAnswerResponse struct {
	// AnswerID совпадает с X-Request-ID.
	AnswerID string `json:"answer_id" example:"f16cc96e-aaf4-44fb-a462-836ed4d0b135"`
	// Статус: ok | partial | error.
	Status string `json:"status" enums:"ok,partial,error" example:"ok"`
	// ErrorCode при status=error: llm_unavailable | data_unavailable | timeout | bad_question.
	ErrorCode string `json:"error_code,omitempty" example:"llm_unavailable"`
	// Режим: llm | rules | fake.
	Mode string `json:"mode" enums:"llm,rules,fake" example:"llm"`
	// Короткий текст ответа, plain text.
	Text string `json:"text" example:"Просрочено 4 платежа."`
	// Блоки данных (до 3): occurrence_list, currency_totals, payment_list, people_list.
	Blocks []map[string]any `json:"blocks" swaggertype:"array,object"`
	// Подсказки «можно спросить…».
	Followups []string `json:"followups" example:"Кто отвечает за просроченные?"`
	// Оговорки, добавленные агентом.
	Notes []string `json:"notes" example:"другие валюты не учитывались и не пересчитывались"`
	// Метаданные ответа (модель, токены, стоимость, латентность); показываются только админу.
	Meta map[string]any `json:"meta" swaggertype:"object"`
}
