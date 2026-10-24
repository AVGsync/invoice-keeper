package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
)

type Hasher struct{}

func NewHasher() *Hasher {
	return &Hasher{}
}

func (h *Hasher) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (h *Hasher) CheckPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

func (h *Hasher) HashRefreshToken(token string) (string, error) {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	return string(tokenHash), nil
}

func (h *Hasher) CheckRefreshToken(token string, hashed string) bool {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	return subtle.ConstantTimeCompare([]byte(tokenHash), []byte(hashed)) == 1
}
