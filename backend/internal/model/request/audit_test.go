package request

import (
	"net/url"
	"testing"
	"time"
)

func auditVals(kv map[string]string) url.Values {
	v := url.Values{}
	for k, val := range kv {
		v.Set(k, val)
	}
	return v
}

func TestParseAuditFilterDefaults(t *testing.T) {
	f, err := ParseAuditFilter(url.Values{}, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Order != "desc" {
		t.Errorf("default order = %q, want desc", f.Order)
	}
	if f.Limit != 50 || f.Offset != 0 {
		t.Errorf("default page = %d/%d, want 50/0", f.Limit, f.Offset)
	}
}

func TestParseAuditFilterErrors(t *testing.T) {
	long := ""
	for i := 0; i < 129; i++ {
		long += "a"
	}
	q101 := ""
	for i := 0; i < 101; i++ {
		q101 += "a"
	}
	cases := []struct {
		name string
		vals map[string]string
		want string
	}{
		{"actor_id", map[string]string{"actor_id": "not-uuid"}, "actor_id must be a UUID"},
		{"action upper", map[string]string{"action": "Recurring.Update"}, "action is invalid"},
		{"action double dot", map[string]string{"action": "a..b"}, "action is invalid"},
		{"entity_type digit", map[string]string{"entity_type": "user1"}, "entity_type is invalid"},
		{"entity_id long", map[string]string{"entity_id": long}, "entity_id must be at most 128 characters"},
		{"channel", map[string]string{"channel": "sms"}, "channel must be one of web, miniapp, bot, system"},
		{"outcome", map[string]string{"outcome": "maybe"}, "outcome must be success or failure"},
		{"from bad", map[string]string{"from": "2026/01/01"}, "from must be YYYY-MM-DD or RFC3339"},
		{"to bad", map[string]string{"to": "nope"}, "to must be YYYY-MM-DD or RFC3339"},
		{"from after to", map[string]string{"from": "2026-10-10", "to": "2026-10-01"}, "from must be <= to"},
		{"q long", map[string]string{"q": q101}, "q must be at most 100 characters"},
		{"order", map[string]string{"order": "sideways"}, "order must be asc or desc"},
		{"limit", map[string]string{"limit": "500"}, "limit must be between 1 and 200"},
		{"offset", map[string]string{"offset": "-1"}, "offset must be between 0 and 100000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseAuditFilter(auditVals(c.vals), time.UTC)
			if err == nil || err.Error() != c.want {
				t.Errorf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestParseAuditFilterValid(t *testing.T) {
	v := url.Values{}
	v.Add("actor_id", "11111111-1111-4111-8111-111111111111,22222222-2222-4222-8222-222222222222")
	v.Add("actor_id", "33333333-3333-4333-8333-333333333333")
	v.Set("action", "recurring.*,occurrence.confirm")
	v.Set("channel", "web,bot")
	v.Set("outcome", "success")
	v.Set("from", "2026-10-01")
	v.Set("to", "2026-10-31")
	v.Set("order", "asc")

	f, err := ParseAuditFilter(v, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.ActorIDs) != 3 {
		t.Errorf("actor ids = %v", f.ActorIDs)
	}
	if len(f.Actions) != 2 || f.Actions[0] != "recurring.*" {
		t.Errorf("actions = %v", f.Actions)
	}
	if len(f.Channels) != 2 {
		t.Errorf("channels = %v", f.Channels)
	}
	if f.From == nil || f.To == nil {
		t.Fatalf("from/to must be set")
	}
	// to = следующий день (эксклюзивная верхняя граница).
	if f.To.Day() != 1 || f.To.Month() != time.November {
		t.Errorf("to not next-day exclusive: %v", f.To)
	}
	if f.Order != "asc" {
		t.Errorf("order = %q", f.Order)
	}
}

func TestParseAuditFilterRFC3339(t *testing.T) {
	v := url.Values{}
	v.Set("from", "2026-10-24T09:00:00Z")
	v.Set("to", "2026-10-24T18:00:00Z")
	f, err := ParseAuditFilter(v, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.From == nil || f.To == nil {
		t.Fatalf("from/to must be set")
	}
	if !f.From.Equal(time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("from = %v", f.From)
	}
}
