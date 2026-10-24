package service

import (
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, c *captureBody) {
	t.Helper()
	if _, err := io.ReadAll(c); err != nil {
		t.Fatalf("read: %v", err)
	}
}

func TestCaptureJSON_FullyRead(t *testing.T) {
	const body = `{"status":"ok","answer_id":"a1","text":"hi"}`
	var got []byte
	c := newCaptureBody(io.NopCloser(strings.NewReader(body)), false, func(raw []byte) { got = append([]byte(nil), raw...) })
	readAll(t, c)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("onEnvelope raw = %q", got)
	}
}

func TestCaptureJSON_NotFullyRead(t *testing.T) {
	const body = `{"status":"ok"}`
	called := false
	c := newCaptureBody(io.NopCloser(strings.NewReader(body)), false, func([]byte) { called = true })
	buf := make([]byte, 4)
	_, _ = c.Read(buf) // читаем частично, не до EOF
	_ = c.Close()
	if called {
		t.Fatal("onEnvelope must not fire when body was not read to EOF")
	}
}

func TestCaptureSSE_DoneFrame(t *testing.T) {
	const stream = "event: step\ndata: {\"tool\":\"x\"}\n\nevent: done\ndata: {\"status\":\"ok\",\"answer_id\":\"a1\"}\n\n"
	var got string
	c := newCaptureBody(io.NopCloser(strings.NewReader(stream)), true, func(raw []byte) { got = string(raw) })
	readAll(t, c)
	_ = c.Close()
	if got != `{"status":"ok","answer_id":"a1"}` {
		t.Fatalf("done data = %q", got)
	}
}

func TestCaptureSSE_NoDone(t *testing.T) {
	const stream = "event: step\ndata: {}\n\nevent: text\ndata: hi\n\n"
	called := false
	c := newCaptureBody(io.NopCloser(strings.NewReader(stream)), true, func([]byte) { called = true })
	readAll(t, c)
	_ = c.Close()
	if called {
		t.Fatal("onEnvelope must not fire without a done frame")
	}
}

func TestCaptureSSE_DoneTooBig(t *testing.T) {
	big := strings.Repeat("a", maxCaptureBytes+10)
	stream := "event: done\ndata: " + big + "\n\n"
	called := false
	c := newCaptureBody(io.NopCloser(strings.NewReader(stream)), true, func([]byte) { called = true })
	readAll(t, c)
	_ = c.Close()
	if called {
		t.Fatal("oversized done frame must be dropped")
	}
}

func TestCaptureCloseTwice(t *testing.T) {
	const body = `{"status":"ok"}`
	n := 0
	c := newCaptureBody(io.NopCloser(strings.NewReader(body)), false, func([]byte) { n++ })
	readAll(t, c)
	_ = c.Close()
	_ = c.Close()
	if n != 1 {
		t.Fatalf("onEnvelope fired %d times, want 1", n)
	}
}
