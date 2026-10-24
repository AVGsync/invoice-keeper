package service

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/clock"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// fakeUsers — читатель пользователей для сервиса ассистента.
type fakeUsers struct {
	users map[string]model.User
	err   error
}

func (f fakeUsers) GetUserByID(id string, _ context.Context) (model.User, error) {
	if f.err != nil {
		return model.User{}, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return model.User{}, model.ErrUserNotFound
	}
	return u, nil
}

func testKey() ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func newTestService(t *testing.T, agentURL string, cfg AssistantConfig) *AssistantService {
	t.Helper()
	if cfg.ActorKey == nil {
		cfg.ActorKey = testKey()
	}
	if cfg.AgentURL == "" {
		cfg.AgentURL = agentURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.RatePerMin == 0 {
		cfg.RatePerMin = 10
	}
	if cfg.DailyLimit == 0 {
		cfg.DailyLimit = 200
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 4
	}
	users := fakeUsers{users: map[string]model.User{"u1": {ID: "u1", Role: "admin"}}}
	return NewAssistantService(users, cfg, clock.New(time.UTC))
}

// okEnvelope — минимальный валидный конверт агента.
func okEnvelope() []byte {
	b, _ := json.Marshal(map[string]any{
		"answer_id": "rid1", "status": "ok", "mode": "llm",
		"text": "ок", "blocks": []any{}, "followups": []string{}, "notes": []string{},
	})
	return b
}

func TestAssistant_Disabled(t *testing.T) {
	svc := newTestService(t, "http://127.0.0.1:0", AssistantConfig{Enabled: false})
	if svc.Enabled() {
		t.Fatal("Enabled() should be false")
	}
	_, err := svc.Ask(context.Background(), "u1", "bot", "что просрочено?")
	if !errors.Is(err, ErrAssistantDisabled) {
		t.Fatalf("err = %v, want ErrAssistantDisabled", err)
	}
}

func TestAssistant_TokenClaims(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(okEnvelope())
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true})
	if _, err := svc.Ask(context.Background(), "u1", "bot", "что просрочено?"); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	const prefix = "Bearer "
	if len(gotAuth) <= len(prefix) {
		t.Fatalf("no bearer token: %q", gotAuth)
	}
	tokenStr := gotAuth[len(prefix):]

	pub := testKey().Public().(ed25519.PublicKey)
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(tk *jwt.Token) (any, error) {
		if tk.Method.Alg() != "EdDSA" {
			return nil, errors.New("unexpected alg")
		}
		return pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithAudience(actorAudience), jwt.WithIssuer(actorIssuer))
	if err != nil || !tok.Valid {
		t.Fatalf("token did not verify with public key: %v", err)
	}
	if claims["role"] != "admin" {
		t.Errorf("role = %v, want admin (from DB)", claims["role"])
	}
	if claims["ch"] != "bot" {
		t.Errorf("ch = %v, want bot", claims["ch"])
	}
	if claims["sub"] != "u1" {
		t.Errorf("sub = %v, want u1", claims["sub"])
	}
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	if exp-iat != 60 {
		t.Errorf("exp-iat = %v, want 60", exp-iat)
	}
	if gotBody == "" || !containsAll(gotBody, `"question"`, `"channel"`, "bot") {
		t.Errorf("body not forwarded: %q", gotBody)
	}
}

func TestAssistant_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write(okEnvelope())
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true, Timeout: 50 * time.Millisecond})
	_, err := svc.Ask(context.Background(), "u1", "bot", "вопрос")
	if !errors.Is(err, ErrAssistantTimeout) {
		t.Fatalf("err = %v, want ErrAssistantTimeout", err)
	}
}

func TestAssistant_Unavailable_On5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true})
	_, err := svc.Ask(context.Background(), "u1", "bot", "вопрос")
	if !errors.Is(err, ErrAssistantUnavailable) {
		t.Fatalf("err = %v, want ErrAssistantUnavailable", err)
	}
}

func TestAssistant_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(okEnvelope())
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true, RatePerMin: 10})
	for i := 0; i < 10; i++ {
		if _, err := svc.Ask(context.Background(), "u1", "bot", "вопрос"); err != nil {
			t.Fatalf("question #%d: %v", i, err)
		}
	}
	_, err := svc.Ask(context.Background(), "u1", "bot", "вопрос")
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("11th err = %v, want RateLimitedError", err)
	}
	if rl.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %v, want > 0", rl.RetryAfter)
	}
	// Другой пользователь не затронут общим бюджетом.
	svc2 := svc // same service, different user key
	if _, err := svc2.Ask(context.Background(), "u2", "bot", "вопрос"); err != nil {
		// u2 отсутствует в fakeUsers → bad_question, но НЕ rate-limited.
		if errors.Is(err, ErrAssistantRateLimited) {
			t.Fatalf("u2 should not be rate limited")
		}
	}
}

func TestAssistant_Busy_Semaphore(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = w.Write(okEnvelope())
	}))
	defer srv.Close()

	svc := newTestService(t, srv.URL, AssistantConfig{Enabled: true, MaxConcurrent: 1, Timeout: 5 * time.Second})

	var inflight atomic.Bool
	done := make(chan error, 1)
	go func() {
		inflight.Store(true)
		_, err := svc.Ask(context.Background(), "u1", "bot", "первый")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for !inflight.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	_, err := svc.Ask(context.Background(), "u1", "bot", "второй")
	if !errors.Is(err, ErrAssistantBusy) {
		t.Fatalf("second err = %v, want ErrAssistantBusy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first Ask: %v", err)
	}
}

func TestValidateQuestion(t *testing.T) {
	if _, err := ValidateQuestion("  "); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("blank should be bad")
	}
	long := make([]rune, 501)
	for i := range long {
		long[i] = 'я'
	}
	if _, err := ValidateQuestion(string(long)); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("501 runes should be bad")
	}
	if _, err := ValidateQuestion("что\x00просрочено"); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("control char should be bad")
	}
	for _, cp := range []rune{0x200D, 0xFEFF, 0x00AD} {
		bad := "vopros" + string(cp) + "end"
		if _, err := ValidateQuestion(bad); !errors.Is(err, ErrAssistantBadQuestion) {
			t.Errorf("Cf char U+%04X should be bad", cp)
		}
	}
	emoji := string(rune(0x1F468)) + string(rune(0x200D)) + string(rune(0x1F4BB))
	if _, err := ValidateQuestion("emoji " + emoji); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("ZWJ emoji should be bad")
	}
	withNL := strings.Repeat("я", 250) + "\n" + strings.Repeat("я", 250)
	if _, err := ValidateQuestion(withNL); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("newlines must count toward the 500-rune limit")
	}
	// \n и \t разрешены и не считаются управляющими.
	if _, err := ValidateQuestion("строка1\nстрока2\tконец"); err != nil {
		t.Errorf("newline/tab must be allowed: %v", err)
	}
	q, err := ValidateQuestion("  что просрочено?  ")
	if err != nil || q != "что просрочено?" {
		t.Errorf("q = %q, err = %v", q, err)
	}
}

func TestValidateChannel(t *testing.T) {
	if ch, _ := ValidateChannel(""); ch != "web" {
		t.Errorf("empty => web")
	}
	if _, err := ValidateChannel("bot"); !errors.Is(err, ErrAssistantBadQuestion) {
		t.Errorf("bot channel forbidden over HTTP")
	}
	if ch, err := ValidateChannel("miniapp"); err != nil || ch != "miniapp" {
		t.Errorf("miniapp should pass")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
