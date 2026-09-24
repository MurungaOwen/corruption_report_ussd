// Package auth handles admin-portal authentication: password hashing and
// JWT issuing/verification. USSD citizens never authenticate — this
// package is only used by the admin REST API.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

// HashPassword bcrypt-hashes a plaintext password for storage.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether plain matches the stored bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Claims is the JWT payload for an authenticated admin session.
type Claims struct {
	UserID int64            `json:"uid"`
	Email  string           `json:"email"`
	Role   domain.AdminRole `json:"role"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

// Issue mints a signed JWT for an authenticated user.
func (t *TokenIssuer) Issue(u domain.AdminUser) (string, time.Time, error) {
	expiresAt := time.Now().Add(t.ttl)
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", u.ID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	return signed, expiresAt, err
}

// Verify parses and validates a signed token, returning its claims.
func (t *TokenIssuer) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

type ctxKey int

const claimsCtxKey ctxKey = iota

// BearerToken extracts the token from an "Authorization: Bearer xxx" header.
func BearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimPrefix(h, prefix), true
}
