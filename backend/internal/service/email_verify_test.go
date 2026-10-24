package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

type memVerify struct{ st model.EmailVerifyState }

func (m *memVerify) GetVerifyState(context.Context, string) (model.EmailVerifyState, error) {
	return m.st, nil
}
func (m *memVerify) SetVerifyCode(_ context.Context, _, hash string, exp time.Time) error {
	now := time.Now()
	m.st.CodeHash, m.st.ExpiresAt, m.st.Attempts, m.st.SentAt = hash, &exp, 0, &now
	return nil
}
func (m *memVerify) AddVerifyAttempt(context.Context, string) error { m.st.Attempts++; return nil }
func (m *memVerify) MarkEmailVerified(context.Context, string) error {
	m.st.Verified, m.st.CodeHash = true, ""
	return nil
}

type captureMailer struct{ body string }

func (c *captureMailer) Send(_ context.Context, _, _, body string) error { c.body = body; return nil }

var codeRe = regexp.MustCompile(`\d{6}`)

func TestEmailVerifyFlow(t *testing.T) {
	ctx := context.Background()
	repo := &memVerify{st: model.EmailVerifyState{Email: "a@b.co"}}
	mail := &captureMailer{}
	svc := NewEmailVerifyService(repo, mail, "secret")

	if err := svc.Verify(ctx, "u", "123456"); !errors.Is(err, ErrEmailCodeExpired) {
		t.Fatalf("no code requested: %v", err)
	}
	if err := svc.SendCode(ctx, "u"); err != nil {
		t.Fatal(err)
	}
	var tooSoon *EmailCodeTooSoonError
	if err := svc.SendCode(ctx, "u"); !errors.As(err, &tooSoon) {
		t.Fatalf("cooldown: %v", err)
	}
	code := codeRe.FindString(mail.body)
	if code == "" {
		t.Fatalf("no code in %q", mail.body)
	}

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < emailCodeMaxAttempt; i++ {
		if err := svc.Verify(ctx, "u", wrong); !errors.Is(err, ErrEmailCodeInvalid) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if err := svc.Verify(ctx, "u", code); !errors.Is(err, ErrEmailTooManyAttempts) {
		t.Fatalf("locked after max attempts even with right code: %v", err)
	}

	repo.st.Attempts = 0
	if err := svc.Verify(ctx, "u", " "+code+" "); err != nil {
		t.Fatalf("right code: %v", err)
	}
	if !repo.st.Verified {
		t.Fatal("not verified")
	}
	if err := svc.SendCode(ctx, "u"); !errors.Is(err, ErrEmailAlreadyVerified) {
		t.Fatalf("already verified: %v", err)
	}
}

func TestEmailVerifyDisabledMailer(t *testing.T) {
	svc := NewEmailVerifyService(&memVerify{}, nil, "s")
	if err := svc.SendCode(context.Background(), "u"); !errors.Is(err, ErrMailDisabled) {
		t.Fatalf("got %v", err)
	}
}

func TestEmailVerifyExpired(t *testing.T) {
	repo := &memVerify{st: model.EmailVerifyState{Email: "a@b.co"}}
	svc := NewEmailVerifyService(repo, &captureMailer{}, "s")
	svc.now = func() time.Time { return time.Now().Add(-time.Hour) }
	_ = svc.SendCode(context.Background(), "u")
	svc.now = time.Now
	if err := svc.Verify(context.Background(), "u", "123456"); !errors.Is(err, ErrEmailCodeExpired) {
		t.Fatalf("got %v", err)
	}
}
