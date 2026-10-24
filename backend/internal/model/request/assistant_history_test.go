package request

import (
	"net/url"
	"testing"
)

func histValues(q string) url.Values {
	v, _ := url.ParseQuery(q)
	return v
}

func TestParseAssistantHistoryGet(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		got, err := ParseAssistantHistoryGet(histValues(""))
		if err != nil {
			t.Fatal(err)
		}
		if got.Channel != "web" || got.Conversation != "web" || got.Limit != 20 {
			t.Fatalf("defaults = %+v", got)
		}
	})
	t.Run("miniapp", func(t *testing.T) {
		got, err := ParseAssistantHistoryGet(histValues("channel=miniapp"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Channel != "miniapp" || got.Conversation != "miniapp" {
			t.Fatalf("miniapp = %+v", got)
		}
	})
	t.Run("bot default conversation", func(t *testing.T) {
		got, err := ParseAssistantHistoryGet(histValues("channel=bot"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Conversation != "tg:dm" {
			t.Fatalf("bot conversation = %q", got.Conversation)
		}
	})
	t.Run("bot group conversation", func(t *testing.T) {
		got, err := ParseAssistantHistoryGet(histValues("channel=bot&conversation=tg:-1001234567890"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Conversation != "tg:-1001234567890" {
			t.Fatalf("bot group = %q", got.Conversation)
		}
	})
	t.Run("limit bounds", func(t *testing.T) {
		if _, err := ParseAssistantHistoryGet(histValues("limit=0")); err == nil || err.Error() != "limit must be between 1 and 50" {
			t.Fatalf("limit=0 err = %v", err)
		}
		if _, err := ParseAssistantHistoryGet(histValues("limit=51")); err == nil {
			t.Fatal("limit=51 must error")
		}
	})
	t.Run("bad channel", func(t *testing.T) {
		_, err := ParseAssistantHistoryGet(histValues("channel=all"))
		if err == nil || err.Error() != "channel must be web, miniapp or bot" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad conversation for web", func(t *testing.T) {
		_, err := ParseAssistantHistoryGet(histValues("channel=web&conversation=tg:dm"))
		if err == nil || err.Error() != "conversation is invalid" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad conversation for bot", func(t *testing.T) {
		_, err := ParseAssistantHistoryGet(histValues("channel=bot&conversation=web"))
		if err == nil || err.Error() != "conversation is invalid" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestParseAssistantHistoryDelete(t *testing.T) {
	t.Run("all", func(t *testing.T) {
		got, err := ParseAssistantHistoryDelete(histValues("channel=all"))
		if err != nil {
			t.Fatal(err)
		}
		if !got.All || got.Channel != "all" {
			t.Fatalf("all = %+v", got)
		}
	})
	t.Run("bad channel message mentions all", func(t *testing.T) {
		_, err := ParseAssistantHistoryDelete(histValues("channel=bogus"))
		if err == nil || err.Error() != "channel must be web, miniapp, bot or all" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("conversation for bot", func(t *testing.T) {
		got, err := ParseAssistantHistoryDelete(histValues("channel=bot"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Conversation != "tg:dm" || got.All {
			t.Fatalf("bot delete = %+v", got)
		}
	})
}
