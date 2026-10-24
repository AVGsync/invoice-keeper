package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"olga@example.com": "o***@example.com",
		"a@b.co":           "a***@b.co",
		"noatsign":         "***",
		"@domain.com":      "***",
		"":                 "***",
	}
	for in, want := range cases {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedact(t *testing.T) {
	in := map[string]any{
		"login":         "olga",
		"password":      "secret-pw",
		"Authorization": "Bearer xyz",
		"api_token":     "tok",
		"code":          "123456",
		"init_data":     "query_id=...",
		"email":         "olga@example.com",
		"nested": map[string]any{
			"refresh_token": "rt",
			"note":          "ok",
			"email":         "a@b.co",
		},
		"list": []any{"x", map[string]any{"hash": "h"}},
	}
	out := Redact(in)

	if out["password"] != redactedValue {
		t.Errorf("password not redacted: %v", out["password"])
	}
	if out["Authorization"] != redactedValue {
		t.Errorf("authorization not redacted: %v", out["Authorization"])
	}
	if out["api_token"] != redactedValue {
		t.Errorf("token substring not redacted: %v", out["api_token"])
	}
	if out["code"] != redactedValue {
		t.Errorf("code not redacted: %v", out["code"])
	}
	if out["init_data"] != redactedValue {
		t.Errorf("init_data not redacted: %v", out["init_data"])
	}
	if out["email"] != "o***@example.com" {
		t.Errorf("email not masked: %v", out["email"])
	}
	if out["login"] != "olga" {
		t.Errorf("login should be kept: %v", out["login"])
	}
	nested := out["nested"].(map[string]any)
	if nested["refresh_token"] != redactedValue {
		t.Errorf("nested token not redacted: %v", nested["refresh_token"])
	}
	if nested["email"] != "a***@b.co" {
		t.Errorf("nested email not masked: %v", nested["email"])
	}
	if nested["note"] != "ok" {
		t.Errorf("nested note changed: %v", nested["note"])
	}
	list := out["list"].([]any)
	if list[1].(map[string]any)["hash"] != redactedValue {
		t.Errorf("hash in list not redacted: %v", list[1])
	}

	// Исходная карта не изменилась.
	if in["password"] != "secret-pw" {
		t.Errorf("source map mutated")
	}
}

func TestRedactTruncatesStrings(t *testing.T) {
	long := strings.Repeat("я", 300)
	out := Redact(map[string]any{"note": long})
	if got := []rune(out["note"].(string)); len(got) != maxStringRunes {
		t.Errorf("string not truncated to %d runes, got %d", maxStringRunes, len(got))
	}
}

func TestDiffOnlyChanged(t *testing.T) {
	d := Diff().
		Add("a", 1, 1).
		Add("b", "x", "y").
		Add("c", nil, "set")
	m := d.Map()
	if _, ok := m["a"]; ok {
		t.Errorf("unchanged field a must be absent")
	}
	if m["b"].From != "x" || m["b"].To != "y" {
		t.Errorf("b change wrong: %+v", m["b"])
	}
	if m["c"].From != nil || m["c"].To != "set" {
		t.Errorf("c change wrong: %+v", m["c"])
	}
}

func TestDiffEmptyReturnsNil(t *testing.T) {
	if m := Diff().Add("a", 1, 1).Map(); m != nil {
		t.Errorf("empty diff must be nil, got %v", m)
	}
}

type fakeRepo struct {
	rows []Row
	err  error
}

func (f *fakeRepo) Insert(_ context.Context, row Row) error {
	if f.err != nil {
		return f.err
	}
	f.rows = append(f.rows, row)
	return nil
}

func TestLoggerMergesMetaAndEntry(t *testing.T) {
	repo := &fakeRepo{}
	l := NewLogger(repo)
	ctx := WithMeta(context.Background(), Meta{
		ActorID:   "meta-actor",
		ActorRole: "user",
		Channel:   ChannelWeb,
		IP:        "203.0.113.5",
		UserAgent: "UA",
		RequestID: "req-1",
	})
	l.Record(ctx, Entry{
		Action:     ActionUserUpdate,
		EntityType: EntityUser,
		EntityID:   "u1",
		Changes:    Diff().Add("login", "a", "b").Map(),
	})
	if len(repo.rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(repo.rows))
	}
	r := repo.rows[0]
	if r.Channel != ChannelWeb || *r.ActorID != "meta-actor" || *r.ActorRole != "user" {
		t.Errorf("meta not merged: %+v", r)
	}
	if r.Outcome != OutcomeSuccess {
		t.Errorf("default outcome should be success, got %q", r.Outcome)
	}
	if *r.IP != "203.0.113.5" || *r.RequestID != "req-1" {
		t.Errorf("ip/request_id not set: %+v", r)
	}
	var changes map[string]Change
	if err := json.Unmarshal(r.Changes, &changes); err != nil {
		t.Fatalf("changes not valid json: %v", err)
	}
	if changes["login"].To != "b" {
		t.Errorf("changes wrong: %+v", changes)
	}
}

func TestLoggerEntryOverridesActor(t *testing.T) {
	repo := &fakeRepo{}
	l := NewLogger(repo)
	ctx := WithMeta(context.Background(), Meta{ActorID: "meta", Channel: ChannelMiniApp})
	l.Record(ctx, Entry{Action: ActionAuthLogin, ActorID: "entry-actor", ActorRole: "admin"})
	r := repo.rows[0]
	if *r.ActorID != "entry-actor" || *r.ActorRole != "admin" {
		t.Errorf("entry should override actor: %+v", r)
	}
}

func TestLoggerNoMetaIsSystem(t *testing.T) {
	repo := &fakeRepo{}
	l := NewLogger(repo)
	l.Record(context.Background(), Entry{Action: ActionUserDelete, EntityType: EntityUser, EntityID: "u"})
	r := repo.rows[0]
	if r.Channel != ChannelSystem {
		t.Errorf("no meta should be system channel, got %q", r.Channel)
	}
	if r.ActorID != nil {
		t.Errorf("no actor expected, got %v", *r.ActorID)
	}
}

func TestLoggerInvalidIPDropped(t *testing.T) {
	repo := &fakeRepo{}
	l := NewLogger(repo)
	ctx := WithMeta(context.Background(), Meta{Channel: ChannelWeb, IP: "not-an-ip"})
	l.Record(ctx, Entry{Action: ActionAuthLogin})
	if repo.rows[0].IP != nil {
		t.Errorf("invalid ip must be dropped")
	}
}

func TestLoggerTruncatesLargePayload(t *testing.T) {
	repo := &fakeRepo{}
	l := NewLogger(repo)
	big := strings.Repeat("a", 50) // будет обрезан до 256 рун на значение,
	payload := map[string]any{}
	for i := 0; i < 500; i++ {
		payload[string(rune('A'+i%26))+strings_itoa(i)] = big
	}
	l.Record(context.Background(), Entry{Action: ActionUserCreate, Payload: payload})
	var decoded map[string]any
	if err := json.Unmarshal(repo.rows[0].Payload, &decoded); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
	if len(repo.rows[0].Payload) > maxPayloadBytes {
		t.Errorf("payload exceeds limit: %d", len(repo.rows[0].Payload))
	}
	if decoded["truncated"] != true {
		t.Errorf("oversized payload must be replaced by {truncated:true}, got %v", decoded)
	}
}

func TestLoggerRepoErrorDoesNotPanic(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db down")}
	l := NewLogger(repo)
	// Не должно паниковать и не должно всплывать (Record без возврата ошибки).
	l.Record(context.Background(), Entry{Action: ActionAuthLogin})
}

func TestLabel(t *testing.T) {
	if Label(ActionRecurringUpdate) != "Изменён регулярный платёж" {
		t.Errorf("unexpected label: %q", Label(ActionRecurringUpdate))
	}
	if Label("unknown.action") != "unknown.action" {
		t.Errorf("unknown action should return itself")
	}
}

// strings_itoa — крошечный помощник, чтобы не тянуть strconv в тест.
func strings_itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
