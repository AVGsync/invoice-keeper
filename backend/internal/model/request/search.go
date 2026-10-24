package request

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/google/uuid"
)

const (
	maxQueryRunes = 100
	defaultLimit  = 50
	maxLimit      = 200
	maxOffset     = 100000
)

// multiValues собирает мульти-значения: и через запятую (?k=a,b), и повтором (?k=a&k=b).
func multiValues(q url.Values, key string) []string {
	seen := make(map[string]struct{})
	out := []string{}
	for _, raw := range q[key] {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

// ParsePage разбирает limit/offset с дефолтами 50/0.
func ParsePage(q url.Values) (limit, offset int, err error) {
	limit = defaultLimit
	offset = 0

	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 || n > maxLimit {
			return 0, 0, errors.New("limit must be between 1 and 200")
		}
		limit = n
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 0 || n > maxOffset {
			return 0, 0, errors.New("offset must be between 0 and 100000")
		}
		offset = n
	}
	return limit, offset, nil
}

// parseDateParam проверяет формат "YYYY-MM-DD".
func parseDateParam(q url.Values, key, onErr string) (string, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return "", nil
	}
	if _, err := parseISODate(raw); err != nil {
		return "", errors.New(onErr)
	}
	return raw, nil
}

// parseISODate — строгий разбор "YYYY-MM-DD" без зависимости от clock (модель не должна зависеть от clock).
func parseISODate(s string) (bool, error) {
	// Повторяем формат clock.ParseDate, но без импорта: достаточно формата.
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false, errors.New("bad date")
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false, errors.New("bad date")
		}
	}
	mm, _ := strconv.Atoi(s[5:7])
	dd, _ := strconv.Atoi(s[8:10])
	if mm < 1 || mm > 12 || dd < 1 || dd > 31 {
		return false, errors.New("bad date")
	}
	return true, nil
}

// parseQuery проверяет длину q (≤100 рун) и возвращает trim-строку.
func parseQuery(q url.Values) (string, error) {
	s := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(s) > maxQueryRunes {
		return "", errors.New("q must be at most 100 characters")
	}
	return s, nil
}

// parseCurrencies нормализует валюты к верхнему регистру, проверяет RUB/USD/EUR.
func parseCurrencies(q url.Values) ([]string, error) {
	raw := multiValues(q, "currency")
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		up := strings.ToUpper(c)
		switch up {
		case "RUB", "USD", "EUR":
		default:
			return nil, errors.New("currency must be one of RUB, USD, EUR")
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		out = append(out, up)
	}
	return out, nil
}

// parseUUIDs проверяет, что каждый элемент — UUID.
func parseUUIDs(values []string, onErr string) ([]string, error) {
	for _, v := range values {
		if _, err := uuid.Parse(v); err != nil {
			return nil, errors.New(onErr)
		}
	}
	return values, nil
}

// parseAmount разбирает неотрицательное int64; nil, если не задан.
func parseAmount(q url.Values, key, onErr string) (*int64, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return nil, errors.New(onErr)
	}
	return &n, nil
}

// parseOrder разбирает направление сортировки.
func parseOrder(q url.Values) (string, error) {
	raw := strings.TrimSpace(q.Get("order"))
	if raw == "" {
		return "asc", nil
	}
	if raw != "asc" && raw != "desc" {
		return "", errors.New("order must be asc or desc")
	}
	return raw, nil
}

// parseAssignees разбирает assignee=me и assignee_id (мульти UUID), запрещая их совместное использование.
func parseAssignees(q url.Values, actorID string) ([]string, error) {
	assignee := strings.TrimSpace(q.Get("assignee"))
	ids := multiValues(q, "assignee_id")

	hasMe := assignee != ""
	if hasMe && assignee != "me" {
		return nil, errors.New("assignee must be me")
	}
	if hasMe && len(ids) > 0 {
		return nil, errors.New("use either assignee=me or assignee_id")
	}
	if hasMe {
		return []string{actorID}, nil
	}
	if len(ids) > 0 {
		if _, err := parseUUIDs(ids, "assignee_id must be a UUID"); err != nil {
			return nil, err
		}
		return ids, nil
	}
	return nil, nil
}

// ParseOccurrenceSearch разбирает query-параметры GET /payment-occurrences/search.
func ParseOccurrenceSearch(q url.Values, actorID string) (model.OccurrenceFilter, error) {
	var f model.OccurrenceFilter

	query, err := parseQuery(q)
	if err != nil {
		return f, err
	}
	f.Query = query

	statuses := multiValues(q, "status")
	for _, s := range statuses {
		switch s {
		case "overdue", "pending", "paid", "ended":
		default:
			return f, errors.New("status must be one of overdue, pending, paid, ended")
		}
	}
	f.Statuses = statuses

	currencies, err := parseCurrencies(q)
	if err != nil {
		return f, err
	}
	f.Currencies = currencies

	assignees, err := parseAssignees(q, actorID)
	if err != nil {
		return f, err
	}
	f.AssigneeIDs = assignees

	personaIDs, err := parseUUIDs(multiValues(q, "persona_id"), "persona_id must be a UUID")
	if err != nil {
		return f, err
	}
	f.PersonaIDs = personaIDs

	if rp := strings.TrimSpace(q.Get("recurrent_payment_id")); rp != "" {
		if _, err := uuid.Parse(rp); err != nil {
			return f, errors.New("recurrent_payment_id must be a UUID")
		}
		f.RecurrentPaymentID = rp
	}

	if f.DueFrom, err = parseDateParam(q, "due_from", "due_from must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.DueTo, err = parseDateParam(q, "due_to", "due_to must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.DueFrom != "" && f.DueTo != "" && f.DueFrom > f.DueTo {
		return f, errors.New("due_from must be <= due_to")
	}

	if f.PaidFrom, err = parseDateParam(q, "paid_from", "paid_from must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.PaidTo, err = parseDateParam(q, "paid_to", "paid_to must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.PaidFrom != "" && f.PaidTo != "" && f.PaidFrom > f.PaidTo {
		return f, errors.New("paid_from must be <= paid_to")
	}

	if f.AmountMin, err = parseAmount(q, "amount_min", "amount_min must be a non-negative integer"); err != nil {
		return f, err
	}
	if f.AmountMax, err = parseAmount(q, "amount_max", "amount_max must be a non-negative integer"); err != nil {
		return f, err
	}
	if f.AmountMin != nil && f.AmountMax != nil && *f.AmountMin > *f.AmountMax {
		return f, errors.New("amount_min must be <= amount_max")
	}
	if (f.AmountMin != nil || f.AmountMax != nil) && len(f.Currencies) != 1 {
		return f, errors.New("amount_min/amount_max require exactly one currency")
	}

	sort := strings.TrimSpace(q.Get("sort"))
	if sort == "" {
		sort = "due_date"
	}
	switch sort {
	case "due_date", "amount", "created_at", "description", "paid_date":
	default:
		return f, errors.New("sort must be one of due_date, amount, created_at, description, paid_date")
	}
	f.Sort = sort

	if f.Order, err = parseOrder(q); err != nil {
		return f, err
	}

	if f.Limit, f.Offset, err = ParsePage(q); err != nil {
		return f, err
	}

	return f, nil
}

// ParseRecurringSearch разбирает query-параметры GET /recurring-payments/search.
func ParseRecurringSearch(q url.Values, actorID string) (model.RecurringFilter, error) {
	var f model.RecurringFilter

	query, err := parseQuery(q)
	if err != nil {
		return f, err
	}
	f.Query = query

	currencies, err := parseCurrencies(q)
	if err != nil {
		return f, err
	}
	f.Currencies = currencies

	frequencies := multiValues(q, "frequency")
	for _, fr := range frequencies {
		switch fr {
		case "weekly", "monthly":
		default:
			return f, errors.New("frequency must be weekly or monthly")
		}
	}
	f.Frequencies = frequencies

	assignees, err := parseAssignees(q, actorID)
	if err != nil {
		return f, err
	}
	f.AssigneeIDs = assignees

	personaIDs, err := parseUUIDs(multiValues(q, "persona_id"), "persona_id must be a UUID")
	if err != nil {
		return f, err
	}
	f.PersonaIDs = personaIDs

	statuses := multiValues(q, "status")
	for _, s := range statuses {
		switch s {
		case "active", "stopped":
		default:
			return f, errors.New("status must be active or stopped")
		}
	}

	includeStoppedRaw := strings.TrimSpace(q.Get("include_stopped"))
	if includeStoppedRaw != "" {
		if len(statuses) > 0 {
			return f, errors.New("use either status or include_stopped")
		}
		switch includeStoppedRaw {
		case "true":
			statuses = []string{"active", "stopped"}
		case "false":
			statuses = []string{"active"}
		default:
			return f, errors.New("include_stopped must be true or false")
		}
	} else if len(statuses) == 0 {
		// Дефолт — только активные.
		statuses = []string{"active"}
	}
	f.Statuses = statuses

	if f.AmountMin, err = parseAmount(q, "amount_min", "amount_min must be a non-negative integer"); err != nil {
		return f, err
	}
	if f.AmountMax, err = parseAmount(q, "amount_max", "amount_max must be a non-negative integer"); err != nil {
		return f, err
	}
	if f.AmountMin != nil && f.AmountMax != nil && *f.AmountMin > *f.AmountMax {
		return f, errors.New("amount_min must be <= amount_max")
	}
	if (f.AmountMin != nil || f.AmountMax != nil) && len(f.Currencies) != 1 {
		return f, errors.New("amount_min/amount_max require exactly one currency")
	}

	if f.StartFrom, err = parseDateParam(q, "start_from", "start_from must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.StartTo, err = parseDateParam(q, "start_to", "start_to must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.StartFrom != "" && f.StartTo != "" && f.StartFrom > f.StartTo {
		return f, errors.New("start_from must be <= start_to")
	}

	if f.NextDueFrom, err = parseDateParam(q, "next_due_from", "next_due_from must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.NextDueTo, err = parseDateParam(q, "next_due_to", "next_due_to must be YYYY-MM-DD"); err != nil {
		return f, err
	}
	if f.NextDueFrom != "" && f.NextDueTo != "" && f.NextDueFrom > f.NextDueTo {
		return f, errors.New("next_due_from must be <= next_due_to")
	}

	sort := strings.TrimSpace(q.Get("sort"))
	if sort == "" {
		sort = "description"
	}
	switch sort {
	case "description", "amount", "created_at", "next_due", "start_date":
	default:
		return f, errors.New("sort must be one of description, amount, created_at, next_due, start_date")
	}
	f.Sort = sort

	if f.Order, err = parseOrder(q); err != nil {
		return f, err
	}

	if f.Limit, f.Offset, err = ParsePage(q); err != nil {
		return f, err
	}

	return f, nil
}
