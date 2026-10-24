package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// signInitData собирает корректную initData по алгоритму Telegram: тот же
// data_check_string и тот же порядок HMAC, что и в ValidateInitData. Служит
// генератором проверочного вектора (§3.3, §9.1).
func signInitData(fields map[string]string, botToken string) string {
	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dcs := strings.Join(pairs, "\n")

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secret := secretMAC.Sum(nil)

	sigMAC := hmac.New(sha256.New, secret)
	sigMAC.Write([]byte(dcs))
	hash := hex.EncodeToString(sigMAC.Sum(nil))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hash)
	return q.Encode()
}

func TestValidateInitData_ValidVector(t *testing.T) {
	const token = "123456:TEST-TOKEN"
	now := time.Unix(1_700_000_000, 0)

	fields := map[string]string{
		"auth_date": "1699999940", // now - 60s
		"query_id":  "AAG123",
		"user":      `{"id":42,"username":"ivan","first_name":"Иван","allows_write_to_pm":true}`,
	}
	raw := signInitData(fields, token)

	data, err := ValidateInitData(raw, token, 10*time.Minute, now)
	if err != nil {
		t.Fatalf("ValidateInitData: unexpected error: %v", err)
	}
	if data.UserID != 42 {
		t.Errorf("UserID = %d, want 42", data.UserID)
	}
	if data.Username != "ivan" {
		t.Errorf("Username = %q, want ivan", data.Username)
	}
	if !data.AllowsWriteToPM {
		t.Error("AllowsWriteToPM = false, want true")
	}
}

func TestValidateInitData_TamperedHash(t *testing.T) {
	const token = "123456:TEST-TOKEN"
	now := time.Unix(1_700_000_000, 0)
	fields := map[string]string{
		"auth_date": "1699999940",
		"user":      `{"id":42,"username":"ivan"}`,
	}
	raw := signInitData(fields, token)

	// Портим подпись: другой токен не должен пройти проверку.
	if _, err := ValidateInitData(raw, "999:OTHER", 10*time.Minute, now); err != ErrInitDataInvalid {
		t.Fatalf("err = %v, want ErrInitDataInvalid", err)
	}

	// Меняем полезные данные после подписи — hash перестаёт совпадать.
	bad := strings.Replace(raw, "id%3A42", "id%3A99", 1)
	if bad != raw {
		if _, err := ValidateInitData(bad, token, 10*time.Minute, now); err != ErrInitDataInvalid {
			t.Fatalf("tampered payload err = %v, want ErrInitDataInvalid", err)
		}
	}
}

func TestValidateInitData_Stale(t *testing.T) {
	const token = "123456:TEST-TOKEN"
	now := time.Unix(1_700_000_000, 0)
	fields := map[string]string{
		"auth_date": "1699998800", // now - 20 мин
		"user":      `{"id":42}`,
	}
	raw := signInitData(fields, token)

	if _, err := ValidateInitData(raw, token, 10*time.Minute, now); err != ErrInitDataStale {
		t.Fatalf("err = %v, want ErrInitDataStale", err)
	}
}

// TestValidateInitData_SignatureIncluded проверяет, что поле signature (Bot API
// 8.0+) учитывается в data_check_string при проверке по hash: оно подписывается
// как обычная пара и успешно валидируется.
func TestValidateInitData_SignatureIncluded(t *testing.T) {
	const token = "123456:TEST-TOKEN"
	now := time.Unix(1_700_000_000, 0)
	fields := map[string]string{
		"auth_date": "1699999940",
		"user":      `{"id":7,"username":"adm"}`,
		"signature": "abcDEF123", // часть подписываемых полей, как шлют клиенты 8.0+
	}
	raw := signInitData(fields, token)

	data, err := ValidateInitData(raw, token, 10*time.Minute, now)
	if err != nil {
		t.Fatalf("ValidateInitData with signature: unexpected error: %v", err)
	}
	if data.UserID != 7 {
		t.Errorf("UserID = %d, want 7", data.UserID)
	}

	// signature, добавленная ПОСЛЕ подписи (не вошедшая в data_check_string),
	// обязана ломать проверку: иначе её молча игнорировали бы.
	tampered := signInitData(map[string]string{
		"auth_date": "1699999940",
		"user":      `{"id":7,"username":"adm"}`,
	}, token) + "&signature=injected"
	if _, err := ValidateInitData(tampered, token, 10*time.Minute, now); err != ErrInitDataInvalid {
		t.Fatalf("неподписанная signature должна ломать hash: got %v want ErrInitDataInvalid", err)
	}
}

// TestValidateInitData_RealVector прогоняет проверку на НАСТОЯЩЕЙ initData,
// снятой из dev-бота (§3.3, §9.1). Вектор и токен задаются через окружение,
// иначе тест пропускается — секреты в репозиторий не кладём:
//
//	TG_TEST_INIT_DATA='query_id=...&user=...&auth_date=...&signature=...&hash=...'
//	TG_TEST_BOT_TOKEN='<dev bot token>'
//
// Возраст initData не ограничиваем (вектор может быть старым): maxAge огромный.
func TestValidateInitData_RealVector(t *testing.T) {
	raw := os.Getenv("TG_TEST_INIT_DATA")
	token := os.Getenv("TG_TEST_BOT_TOKEN")
	if raw == "" || token == "" {
		t.Skip("TG_TEST_INIT_DATA/TG_TEST_BOT_TOKEN не заданы — нет реального вектора")
	}
	authUnix := extractAuthDate(t, raw)
	now := time.Unix(authUnix, 0).Add(time.Minute)

	data, err := ValidateInitData(raw, token, 100000*time.Hour, now)
	if err != nil {
		t.Fatalf("реальная initData не прошла проверку: %v", err)
	}
	if data.UserID == 0 {
		t.Error("реальная initData: не разобран user.id")
	}
}

// extractAuthDate достаёт auth_date из сырой initData для теста реального вектора.
func extractAuthDate(t *testing.T, raw string) int64 {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("разбор TG_TEST_INIT_DATA: %v", err)
	}
	v, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		t.Fatalf("auth_date в TG_TEST_INIT_DATA: %v", err)
	}
	return v
}
