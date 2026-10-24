package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/AVGsync/invoice-keeper/backend/internal/transport/http/api"
)

func TestDetectChannel(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(r *http.Request)
		expect string
	}{
		{"x-client miniapp", func(r *http.Request) { r.Header.Set("X-Client", "miniapp") }, audit.ChannelMiniApp},
		{"x-client web", func(r *http.Request) { r.Header.Set("X-Client", "web") }, audit.ChannelWeb},
		{"x-client overrides bearer", func(r *http.Request) {
			r.Header.Set("X-Client", "web")
			r.Header.Set("Authorization", "Bearer tok")
		}, audit.ChannelWeb},
		{"bearer only", func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok") }, audit.ChannelMiniApp},
		{"cookie fallback web", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "AccessToken", Value: "x"})
		}, audit.ChannelWeb},
		{"nothing is web", func(r *http.Request) {}, audit.ChannelWeb},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/x", nil)
			c.setup(r)
			if got := DetectChannel(r); got != c.expect {
				t.Errorf("DetectChannel = %q, want %q", got, c.expect)
			}
		})
	}
}

func TestAuditContextMeta(t *testing.T) {
	mw := NewMiddleware(nil, nil)

	var got audit.Meta
	var ok bool
	h := mw.AuditContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = audit.MetaFrom(r.Context())
	}))

	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("X-Client", "miniapp")
	r.Header.Set("User-Agent", "UA/1.0")
	r.Header.Set("X-Real-IP", "203.0.113.9")
	ctx := context.WithValue(r.Context(), ClaimsKey, &model.Claims{UserID: "u-1", Role: "admin"})
	ctx = requestIDCtx(ctx, "req-42")
	r = r.WithContext(ctx)

	h.ServeHTTP(httptest.NewRecorder(), r)

	if !ok {
		t.Fatal("meta not set in context")
	}
	if got.Channel != audit.ChannelMiniApp {
		t.Errorf("channel = %q", got.Channel)
	}
	if got.ActorID != "u-1" || got.ActorRole != "admin" {
		t.Errorf("actor not from claims: %+v", got)
	}
	if got.IP != "203.0.113.9" {
		t.Errorf("ip = %q (want X-Real-IP)", got.IP)
	}
	if got.UserAgent != "UA/1.0" {
		t.Errorf("user agent = %q", got.UserAgent)
	}
	if got.RequestID != "req-42" {
		t.Errorf("request id = %q", got.RequestID)
	}
}

func TestAuditContextForFixedChannel(t *testing.T) {
	mw := NewMiddleware(nil, nil)
	var got audit.Meta
	h := mw.AuditContextFor(audit.ChannelMiniApp)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = audit.MetaFrom(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.AddCookie(&http.Cookie{Name: "AccessToken", Value: "x"})
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got.Channel != audit.ChannelMiniApp {
		t.Errorf("fixed channel not honoured: %q", got.Channel)
	}
}

func TestAuditContextNoClaims(t *testing.T) {
	mw := NewMiddleware(nil, nil)
	var got audit.Meta
	h := mw.AuditContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = audit.MetaFrom(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if got.ActorID != "" {
		t.Errorf("actor should be empty without claims, got %q", got.ActorID)
	}
}

// requestIDCtx кладёт request_id так же, как api.RequestIDMiddleware, чтобы auditMeta его увидел через api.RequestID.
func requestIDCtx(ctx context.Context, id string) context.Context {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", id)
	var out context.Context
	api.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, rr *http.Request) {
		out = rr.Context()
	})).ServeHTTP(httptest.NewRecorder(), r.WithContext(ctx))
	return out
}
