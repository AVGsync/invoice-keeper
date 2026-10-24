package security

import (
	"fmt"
	"time"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTManager struct {
	secretAccess  []byte
	secretRefresh []byte
	ttlAccess     time.Duration
	ttlRefresh    time.Duration
}

func NewJWTManager(secretAccess string, secretRefresh string, ttlAccess time.Duration, ttlRefresh time.Duration) *JWTManager {
	return &JWTManager{
		secretAccess:  []byte(secretAccess),
		secretRefresh: []byte(secretRefresh),
		ttlAccess:     ttlAccess,
		ttlRefresh:    ttlRefresh,
	}
}

func (j *JWTManager) GenerateAccess(userID string, role string) (string, error) {
	claims := model.Claims{
		UserID: userID,
		Role:   role,
		Type:   "access",

		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.ttlAccess)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(j.secretAccess)
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}

	return signed, nil
}

func (j *JWTManager) GenerateRefresh(userID string, role string) (string, error) {
	claims := model.Claims{
		UserID: userID,
		Role:   role,
		Type:   "refresh",

		RegisteredClaims: jwt.RegisteredClaims{
			// jti делает каждый токен уникальным.
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.ttlRefresh)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(j.secretRefresh)
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}

	return signed, nil
}

func (j *JWTManager) ValidateAccess(tokenStr string) (*model.Claims, error) {
	tokenAccess, err := jwt.ParseWithClaims(
		tokenStr,
		&model.Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return j.secretAccess, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}

	claims, ok := tokenAccess.Claims.(*model.Claims)
	if !ok || !tokenAccess.Valid {
		return nil, fmt.Errorf("jwt: invalid claims")
	}

	return claims, nil
}

func (j *JWTManager) ValidateRefresh(tokenStr string) (*model.Claims, error) {
	tokenRefresh, err := jwt.ParseWithClaims(
		tokenStr,
		&model.Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return j.secretRefresh, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}

	claims, ok := tokenRefresh.Claims.(*model.Claims)
	if !ok || !tokenRefresh.Valid {
		return nil, fmt.Errorf("jwt: invalid claims")
	}

	return claims, nil
}
