package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/audit"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

const (
	emailCodeTTL        = 15 * time.Minute
	emailCodeCooldown   = 60 * time.Second
	emailCodeMaxAttempt = 5
)

var (
	ErrMailDisabled         = errors.New("smtp is not configured")
	ErrEmailAlreadyVerified = errors.New("email already verified")
	ErrEmailCodeInvalid     = errors.New("invalid verification code")
	ErrEmailCodeExpired     = errors.New("verification code expired or not requested")
	ErrEmailTooManyAttempts = errors.New("too many wrong codes, request a new one")
)

// EmailCodeTooSoonError — код уже отправляли недавно; RetryAfter — через сколько можно повторить.
type EmailCodeTooSoonError struct{ RetryAfter time.Duration }

func (e *EmailCodeTooSoonError) Error() string { return "verification code was sent recently" }

type EmailVerifyRepository interface {
	GetVerifyState(ctx context.Context, userID string) (model.EmailVerifyState, error)
	SetVerifyCode(ctx context.Context, userID, hash string, expires time.Time) error
	AddVerifyAttempt(ctx context.Context, userID string) error
	MarkEmailVerified(ctx context.Context, userID string) error
}

type VerifyMailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// EmailVerifyService подтверждает адрес почты шестизначным кодом из письма.
type EmailVerifyService struct {
	repo   EmailVerifyRepository
	mailer VerifyMailer
	secret string
	now    func() time.Time
	audit  audit.Recorder
}

// NewEmailVerifyService: mailer может быть nil, тогда отправка кода возвращает ErrMailDisabled.
func NewEmailVerifyService(repo EmailVerifyRepository, mailer VerifyMailer, secret string) *EmailVerifyService {
	return &EmailVerifyService{repo: repo, mailer: mailer, secret: secret, now: time.Now, audit: audit.Nop{}}
}

func (s *EmailVerifyService) SetAuditRecorder(r audit.Recorder) { s.audit = r }

func (s *EmailVerifyService) hash(userID, code string) string {
	sum := sha256.Sum256([]byte(s.secret + ":" + userID + ":" + code))
	return hex.EncodeToString(sum[:])
}

// SendCode отправляет код на сохранённый адрес пользователя.
func (s *EmailVerifyService) SendCode(ctx context.Context, userID string) error {
	if s.mailer == nil {
		return ErrMailDisabled
	}
	st, err := s.repo.GetVerifyState(ctx, userID)
	if err != nil {
		return err
	}
	if st.Verified {
		return ErrEmailAlreadyVerified
	}
	if st.SentAt != nil {
		if wait := emailCodeCooldown - s.now().Sub(*st.SentAt); wait > 0 {
			return &EmailCodeTooSoonError{RetryAfter: wait}
		}
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return fmt.Errorf("email verify: random: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expires := s.now().Add(emailCodeTTL)
	if err := s.repo.SetVerifyCode(ctx, userID, s.hash(userID, code), expires); err != nil {
		return err
	}

	body := "Код подтверждения почты в Invoice Keeper: " + code + "\n\n" +
		"Он действует 15 минут. Если вы не запрашивали код, просто проигнорируйте это письмо."
	if err := s.mailer.Send(ctx, st.Email, "Invoice Keeper: код подтверждения "+code, body); err != nil {
		return fmt.Errorf("email verify: send: %w", err)
	}
	return nil
}

// Verify сверяет код; после пяти неверных попыток нужен новый.
func (s *EmailVerifyService) Verify(ctx context.Context, userID, code string) error {
	code = strings.TrimSpace(code)
	st, err := s.repo.GetVerifyState(ctx, userID)
	if err != nil {
		return err
	}
	if st.Verified {
		return ErrEmailAlreadyVerified
	}
	if st.CodeHash == "" || st.ExpiresAt == nil || s.now().After(*st.ExpiresAt) {
		return ErrEmailCodeExpired
	}
	if st.Attempts >= emailCodeMaxAttempt {
		return ErrEmailTooManyAttempts
	}
	if subtle.ConstantTimeCompare([]byte(s.hash(userID, code)), []byte(st.CodeHash)) != 1 {
		if err := s.repo.AddVerifyAttempt(ctx, userID); err != nil {
			return err
		}
		return ErrEmailCodeInvalid
	}
	if err := s.repo.MarkEmailVerified(ctx, userID); err != nil {
		return err
	}
	s.audit.Record(ctx, audit.Entry{
		Action:     audit.ActionNotificationEmailVerify,
		EntityType: audit.EntityNotificationSettings,
		EntityID:   userID,
		Payload:    map[string]any{"to_email": st.Email},
	})
	return nil
}
