package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ошибки проверки initData Mini App.
var (
	// ErrInitDataInvalid — подпись не совпала или данные не разобрались.
	ErrInitDataInvalid = errors.New("init data is invalid")
	// ErrInitDataStale — auth_date старше maxAge.
	ErrInitDataStale = errors.New("init data is expired")
)

// InitData — разобранные и проверенные поля initData Mini App.
type InitData struct {
	UserID          int64
	Username        string
	FirstName       string
	LastName        string
	AllowsWriteToPM bool
	AuthDate        time.Time
}

// initDataUser — поле user из initData (JSON).
type initDataUser struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	AllowsWriteToPM bool   `json:"allows_write_to_pm"`
}

// ValidateInitData проверяет подпись initData по алгоритму Telegram и возраст auth_date.
func ValidateInitData(raw, botToken string, maxAge time.Duration, now time.Time) (InitData, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return InitData{}, ErrInitDataInvalid
	}

	hash := values.Get("hash")
	if hash == "" {
		return InitData{}, ErrInitDataInvalid
	}

	pairs := make([]string, 0, len(values))
	for key, vs := range values {
		if key == "hash" {
			continue
		}
		pairs = append(pairs, key+"="+vs[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expected := hmacSHA256(secret, []byte(dataCheckString))
	if !hmac.Equal([]byte(hex.EncodeToString(expected)), []byte(hash)) {
		return InitData{}, ErrInitDataInvalid
	}

	authDateUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return InitData{}, ErrInitDataInvalid
	}
	authDate := time.Unix(authDateUnix, 0)
	if now.Sub(authDate) > maxAge {
		return InitData{}, ErrInitDataStale
	}

	var u initDataUser
	if raw := values.Get("user"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &u); err != nil {
			return InitData{}, ErrInitDataInvalid
		}
	}
	if u.ID == 0 {
		return InitData{}, ErrInitDataInvalid
	}

	return InitData{
		UserID:          u.ID,
		Username:        u.Username,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		AllowsWriteToPM: u.AllowsWriteToPM,
		AuthDate:        authDate,
	}, nil
}

func hmacSHA256(key, msg []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return h.Sum(nil)
}
