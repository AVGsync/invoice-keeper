package telegrambot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/infrastructure/telegram"
	"github.com/AVGsync/invoice-keeper/backend/internal/model"
)

const testBotToken = "123456:TEST-TOKEN"

func buildInitData(tgUserID int64, username string, allowsWrite bool, authDate time.Time) string {
	userJSON := fmt.Sprintf(`{"id":%d,"username":%q,"allows_write_to_pm":%t}`, tgUserID, username, allowsWrite)
	fields := map[string]string{
		"user":      userJSON,
		"auth_date": fmt.Sprintf("%d", authDate.Unix()),
		"query_id":  "AAH",
	}

	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dcs := strings.Join(pairs, "\n")

	secret := hmacSum([]byte("WebAppData"), []byte(testBotToken))
	hash := hex.EncodeToString(hmacSum(secret, []byte(dcs)))

	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	vals.Set("hash", hash)
	return vals.Encode()
}

func hmacSum(key, msg []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return h.Sum(nil)
}

// fakeTokens — выдаёт фиксированный токен и фиксирует, кому он выписан.
type fakeTokens struct {
	issuedFor string
	token     string
	expiresIn int
	err       error
}

func (f *fakeTokens) IssueAccessForUser(ctx context.Context, userID string) (string, int, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	f.issuedFor = userID
	return f.token, f.expiresIn, nil
}

// fakeHasher — CheckPassword сравнивает пароль с «хешем» как со строкой.
type fakeHasher struct{}

func (fakeHasher) CheckPassword(plain, hashed string) bool { return plain == hashed }

func newMiniAppService(repo *memRepo, users memUsers, tokens *fakeTokens) *Service {
	return NewService(repo, users, telegram.NewFakeClient(), tokens, fakeHasher{}, Config{
		Enabled:        true,
		BotUsername:    "ik_dev_bot",
		BotToken:       testBotToken,
		InitDataMaxAge: 10 * time.Minute,
	})
}

func TestMiniAppAuth_Linked(t *testing.T) {
	repo := newMemRepo()
	if err := repo.UpsertLink(context.Background(), "u1", 777, "ivan"); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	users := memUsers{users: map[string]model.User{"u1": {ID: "u1", FullName: "Иван Петров", Login: "ivan", Role: "admin"}}}
	tokens := &fakeTokens{token: "access-xyz", expiresIn: 3600}
	svc := newMiniAppService(repo, users, tokens)

	init := buildInitData(777, "ivan", true, time.Now())
	res, err := svc.MiniAppAuth(context.Background(), init)
	if err != nil {
		t.Fatalf("MiniAppAuth: %v", err)
	}
	if res.AccessToken != "access-xyz" || res.ExpiresIn != 3600 {
		t.Errorf("token/exp = %q/%d", res.AccessToken, res.ExpiresIn)
	}
	if res.User.Role != "admin" || res.User.ID != "u1" {
		t.Errorf("user = %+v", res.User)
	}
	if !res.DMAvailable {
		t.Errorf("DMAvailable = false, want true (allows_write_to_pm)")
	}
	if tokens.issuedFor != "u1" {
		t.Errorf("token issued for %q, want u1", tokens.issuedFor)
	}
}

func TestMiniAppAuth_NotLinked(t *testing.T) {
	repo := newMemRepo()
	svc := newMiniAppService(repo, memUsers{}, &fakeTokens{token: "t"})

	init := buildInitData(888, "nobody", false, time.Now())
	_, err := svc.MiniAppAuth(context.Background(), init)
	if !errors.Is(err, model.ErrTelegramNotLinked) {
		t.Fatalf("err = %v, want ErrTelegramNotLinked", err)
	}
}

func TestMiniAppAuth_BadSignature(t *testing.T) {
	repo := newMemRepo()
	svc := newMiniAppService(repo, memUsers{}, &fakeTokens{token: "t"})

	init := buildInitData(777, "ivan", true, time.Now()) + "&tampered=1"
	_, err := svc.MiniAppAuth(context.Background(), init)
	if !errors.Is(err, telegram.ErrInitDataInvalid) {
		t.Fatalf("err = %v, want ErrInitDataInvalid", err)
	}
}

func TestMiniAppAuth_Stale(t *testing.T) {
	repo := newMemRepo()
	svc := newMiniAppService(repo, memUsers{}, &fakeTokens{token: "t"})

	init := buildInitData(777, "ivan", true, time.Now().Add(-20*time.Minute))
	_, err := svc.MiniAppAuth(context.Background(), init)
	if !errors.Is(err, telegram.ErrInitDataStale) {
		t.Fatalf("err = %v, want ErrInitDataStale", err)
	}
}

func TestMiniAppAuth_Disabled(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, memUsers{}, telegram.NewFakeClient(), nil, nil, Config{Enabled: false})
	_, err := svc.MiniAppAuth(context.Background(), "whatever")
	if !errors.Is(err, ErrBotDisabled) {
		t.Fatalf("err = %v, want ErrBotDisabled", err)
	}
}

func TestMiniAppLink_Success(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{
		"u1": {ID: "u1", FullName: "Иван Петров", Login: "ivan", Role: "user", PasswordHash: "secret"},
	}}
	tokens := &fakeTokens{token: "access-link", expiresIn: 3600}
	svc := newMiniAppService(repo, users, tokens)

	init := buildInitData(999, "ivan_tg", true, time.Now())
	res, err := svc.MiniAppLink(context.Background(), init, "ivan", "secret")
	if err != nil {
		t.Fatalf("MiniAppLink: %v", err)
	}
	if res.AccessToken != "access-link" {
		t.Errorf("token = %q", res.AccessToken)
	}
	link, err := repo.GetLinkByTGUserID(context.Background(), 999)
	if err != nil || link.UserID != "u1" {
		t.Errorf("link = %+v err=%v", link, err)
	}
}

func TestMiniAppLink_BadPassword(t *testing.T) {
	repo := newMemRepo()
	users := memUsers{users: map[string]model.User{
		"u1": {ID: "u1", Login: "ivan", Role: "user", PasswordHash: "secret"},
	}}
	svc := newMiniAppService(repo, users, &fakeTokens{token: "t"})

	init := buildInitData(999, "ivan_tg", false, time.Now())
	_, err := svc.MiniAppLink(context.Background(), init, "ivan", "wrong")
	if !errors.Is(err, ErrMiniAppInvalidCredentials) {
		t.Fatalf("err = %v, want ErrMiniAppInvalidCredentials", err)
	}
}

func TestMiniAppLink_UnknownLogin(t *testing.T) {
	repo := newMemRepo()
	svc := newMiniAppService(repo, memUsers{}, &fakeTokens{token: "t"})

	init := buildInitData(999, "ivan_tg", false, time.Now())
	_, err := svc.MiniAppLink(context.Background(), init, "ghost", "secret")
	if !errors.Is(err, ErrMiniAppInvalidCredentials) {
		t.Fatalf("err = %v, want ErrMiniAppInvalidCredentials", err)
	}
}

func TestMiniAppLink_AlreadyLinkedToAnother(t *testing.T) {
	repo := newMemRepo()
	if err := repo.UpsertLink(context.Background(), "u2", 999, "other"); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	users := memUsers{users: map[string]model.User{
		"u1": {ID: "u1", Login: "ivan", Role: "user", PasswordHash: "secret"},
	}}
	svc := newMiniAppService(repo, users, &fakeTokens{token: "t"})

	init := buildInitData(999, "ivan_tg", false, time.Now())
	_, err := svc.MiniAppLink(context.Background(), init, "ivan", "secret")
	if !errors.Is(err, model.ErrTelegramAlreadyLinked) {
		t.Fatalf("err = %v, want ErrTelegramAlreadyLinked", err)
	}
}
