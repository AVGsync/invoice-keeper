package model

// Фильтры серверного поиска платежей (трек B, пункт 4). Поля-строки с пустым
// значением "" означают «ограничение не задано». Множественные значения уже
// нормализованы парсером (request/search.go): без пустых, без дублей.

// OccurrenceFilter — разобранные параметры GET /payment-occurrences/search.
type OccurrenceFilter struct {
	Query              string   // подстрока описания/ФИО (как ввёл пользователь, экранируется в SQL)
	Statuses           []string // overdue/pending/paid/ended (эффективный статус)
	Currencies         []string // RUB/USD/EUR
	AssigneeIDs        []string // payments.user_id; assignee=me подставляет [actorID]
	PersonaIDs         []string // users.persona_id ответственного
	RecurrentPaymentID string   // UUID шаблона; "" = не задан
	DueFrom            string   // "YYYY-MM-DD" или ""
	DueTo              string
	PaidFrom           string
	PaidTo             string
	AmountMin          *int64 // минимальные единицы; nil = не задан
	AmountMax          *int64
	Sort               string // due_date/amount/created_at/description/paid_date
	Order              string // asc/desc
	Limit              int
	Offset             int
}

// RecurringFilter — разобранные параметры GET /recurring-payments/search.
type RecurringFilter struct {
	Query       string
	Currencies  []string
	Frequencies []string // weekly/monthly
	AssigneeIDs []string
	PersonaIDs  []string
	Statuses    []string // active/stopped
	AmountMin   *int64
	AmountMax   *int64
	StartFrom   string
	StartTo     string
	NextDueFrom string
	NextDueTo   string
	Sort        string // description/amount/created_at/next_due/start_date
	Order       string
	Limit       int
	Offset      int
}

// RecurringSearchRow — шаблон вместе с ближайшим неоплаченным сроком (next_due).
// NextDueDate — nil, если будущих неоплаченных периодов нет.
type RecurringSearchRow struct {
	RecurrentPayment
	NextDueDate *string
}
