package notifications

import (
	"strings"
	"testing"
)

func TestBuildMessageEncodesSubjectAndAddsHeaders(t *testing.T) {
	msg := string(buildMessage("noreply@example.com", "a@b.co", "Код подтверждения 123456", "Привет\nмир"))
	for _, want := range []string{
		"Subject: =?utf-8?q?", "Date: ", "Message-ID: <", "@example.com>",
		"Content-Type: text/plain; charset=UTF-8", "Привет\r\nмир",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message misses %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "Subject: Код") {
		t.Error("subject must not be sent as raw UTF-8")
	}
}
