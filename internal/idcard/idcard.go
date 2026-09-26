// Package idcard issues and verifies signed, expiring tokens embedded as a
// QR code on a government official's physical ID card. A photocopied or
// long-expired card stops validating even though the underlying work ID
// and reference photo never change — see ARCHITECTURE.md "Anti-
// impersonation defense in depth", layer 4.
//
// This intentionally needs no external service: it's a plain HMAC-SHA256
// over "work_id|expiry_unix", base64url encoded. An admin regenerates the
// token (and reprints the card) whenever it's due to expire.
package idcard

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMalformed = errors.New("malformed card token")
	ErrSignature = errors.New("card token signature mismatch")
)

type Issuer struct {
	secret []byte
}

func NewIssuer(secret string) *Issuer {
	return &Issuer{secret: []byte(secret)}
}

// Issue returns a token valid until now+ttl for the given work ID.
func (i *Issuer) Issue(workID string, ttl time.Duration) string {
	expiry := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%d", workID, expiry)
	sig := i.sign(payload)
	raw := payload + "|" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Verify checks a token's signature and expiry against the expected work
// ID. It returns (expired, error) — an expired-but-validly-signed token is
// not an error, it's information the caller surfaces to the citizen.
func (i *Issuer) Verify(token, expectedWorkID string) (expired bool, expiresAt time.Time, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, time.Time{}, ErrMalformed
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return false, time.Time{}, ErrMalformed
	}
	workID, expiryStr, sig := parts[0], parts[1], parts[2]
	payload := workID + "|" + expiryStr
	expectedSig := i.sign(payload)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) != 1 {
		return false, time.Time{}, ErrSignature
	}
	if workID != expectedWorkID {
		return false, time.Time{}, ErrSignature
	}
	expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return false, time.Time{}, ErrMalformed
	}
	expiresAt = time.Unix(expiryUnix, 0)
	return time.Now().After(expiresAt), expiresAt, nil
}

func (i *Issuer) sign(payload string) string {
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
