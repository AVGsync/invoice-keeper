package request

import (
	"net/url"
	"testing"
)

const actorID = "11111111-0000-4000-8000-000000000001"

func values(raw string) url.Values {
	v, err := url.ParseQuery(raw)
	if err != nil {
		panic(err)
	}
	return v
}

func TestParseOccurrenceSearchErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"q too long", "q=" + longString(101), "q must be at most 100 characters"},
		{"bad status", "status=paid,frozen", "status must be one of overdue, pending, paid, ended"},
		{"bad currency", "currency=rub,gbp", "currency must be one of RUB, USD, EUR"},
		{"bad assignee", "assignee=them", "assignee must be me"},
		{"assignee conflict", "assignee=me&assignee_id=" + actorID, "use either assignee=me or assignee_id"},
		{"bad assignee_id", "assignee_id=not-a-uuid", "assignee_id must be a UUID"},
		{"bad persona_id", "persona_id=xxx", "persona_id must be a UUID"},
		{"bad recurrent id", "recurrent_payment_id=nope", "recurrent_payment_id must be a UUID"},
		{"bad due_from", "due_from=2026-13-01", "due_from must be YYYY-MM-DD"},
		{"bad due_to", "due_to=foo", "due_to must be YYYY-MM-DD"},
		{"due range", "due_from=2026-02-01&due_to=2026-01-01", "due_from must be <= due_to"},
		{"bad paid_from", "paid_from=foo", "paid_from must be YYYY-MM-DD"},
		{"bad paid_to", "paid_to=foo", "paid_to must be YYYY-MM-DD"},
		{"paid range", "paid_from=2026-02-01&paid_to=2026-01-01", "paid_from must be <= paid_to"},
		{"bad amount_min", "amount_min=-5&currency=RUB", "amount_min must be a non-negative integer"},
		{"bad amount_max", "amount_max=x&currency=RUB", "amount_max must be a non-negative integer"},
		{"amount range", "amount_min=10&amount_max=5&currency=RUB", "amount_min must be <= amount_max"},
		{"amount no currency", "amount_min=10", "amount_min/amount_max require exactly one currency"},
		{"amount two currencies", "amount_min=10&currency=RUB,USD", "amount_min/amount_max require exactly one currency"},
		{"bad sort", "sort=color", "sort must be one of due_date, amount, created_at, description, paid_date"},
		{"bad order", "order=up", "order must be asc or desc"},
		{"bad limit", "limit=0", "limit must be between 1 and 200"},
		{"limit too big", "limit=201", "limit must be between 1 and 200"},
		{"bad offset", "offset=-1", "offset must be between 0 and 100000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseOccurrenceSearch(values(c.raw), actorID)
			if err == nil {
				t.Fatalf("expected error %q, got nil", c.want)
			}
			if err.Error() != c.want {
				t.Fatalf("want %q, got %q", c.want, err.Error())
			}
		})
	}
}

func TestParseOccurrenceSearchDefaults(t *testing.T) {
	f, err := ParseOccurrenceSearch(values(""), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Limit != 50 || f.Offset != 0 {
		t.Fatalf("defaults limit/offset: got %d/%d", f.Limit, f.Offset)
	}
	if f.Sort != "due_date" || f.Order != "asc" {
		t.Fatalf("defaults sort/order: got %q/%q", f.Sort, f.Order)
	}
}

func TestParseOccurrenceSearchMulti(t *testing.T) {
	// Запятая + повтор, дубли схлопываются, пустые отброшены.
	f, err := ParseOccurrenceSearch(values("status=paid,overdue&status=paid&status=,"), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Statuses) != 2 || f.Statuses[0] != "paid" || f.Statuses[1] != "overdue" {
		t.Fatalf("multi parse: %#v", f.Statuses)
	}
}

func TestParseOccurrenceSearchAssigneeMe(t *testing.T) {
	f, err := ParseOccurrenceSearch(values("assignee=me"), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.AssigneeIDs) != 1 || f.AssigneeIDs[0] != actorID {
		t.Fatalf("assignee=me should map to actor: %#v", f.AssigneeIDs)
	}
}

func TestParseOccurrenceSearchCurrencyNormalize(t *testing.T) {
	f, err := ParseOccurrenceSearch(values("currency=rub,Usd,rub"), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Currencies) != 2 || f.Currencies[0] != "RUB" || f.Currencies[1] != "USD" {
		t.Fatalf("currency normalize: %#v", f.Currencies)
	}
}

func TestParseRecurringSearchErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bad frequency", "frequency=daily", "frequency must be weekly or monthly"},
		{"bad status", "status=paused", "status must be active or stopped"},
		{"status+include", "status=active&include_stopped=true", "use either status or include_stopped"},
		{"bad include", "include_stopped=maybe", "include_stopped must be true or false"},
		{"bad sort", "sort=color", "sort must be one of description, amount, created_at, next_due, start_date"},
		{"bad start_from", "start_from=foo", "start_from must be YYYY-MM-DD"},
		{"start range", "start_from=2026-02-01&start_to=2026-01-01", "start_from must be <= start_to"},
		{"next range", "next_due_from=2026-02-01&next_due_to=2026-01-01", "next_due_from must be <= next_due_to"},
		{"amount no currency", "amount_min=10", "amount_min/amount_max require exactly one currency"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRecurringSearch(values(c.raw), actorID)
			if err == nil {
				t.Fatalf("expected error %q, got nil", c.want)
			}
			if err.Error() != c.want {
				t.Fatalf("want %q, got %q", c.want, err.Error())
			}
		})
	}
}

func TestParseRecurringSearchDefaults(t *testing.T) {
	f, err := ParseRecurringSearch(values(""), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Sort != "description" || f.Order != "asc" {
		t.Fatalf("defaults sort/order: got %q/%q", f.Sort, f.Order)
	}
	if f.Limit != 50 || f.Offset != 0 {
		t.Fatalf("defaults limit/offset: got %d/%d", f.Limit, f.Offset)
	}
	// По умолчанию — только активные.
	if len(f.Statuses) != 1 || f.Statuses[0] != "active" {
		t.Fatalf("default status should be active: %#v", f.Statuses)
	}
}

func TestParseRecurringIncludeStopped(t *testing.T) {
	f, err := ParseRecurringSearch(values("include_stopped=true"), actorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Statuses) != 2 {
		t.Fatalf("include_stopped=true ⇒ active+stopped: %#v", f.Statuses)
	}
}

func longString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
