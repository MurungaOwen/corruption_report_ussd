// Package phone centralizes how phone numbers are hashed and truncated for
// display, so the "never store a full phone number in the clear, never
// show one in the admin UI" policy (MANIFESTO.md §6) has exactly one
// implementation to get right.
package phone

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns a deterministic, non-reversible identifier for a phone
// number, salted with an operator-controlled pepper so a leaked database
// alone can't be dictionary-attacked back to real numbers.
func Hash(pepper, number string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(number))
	return hex.EncodeToString(mac.Sum(nil))
}

// Last4 returns the last 4 digits of a phone number for light-touch
// display in the admin portal (enough for a case officer to informally
// cross-check a caller, never enough to be the full number).
func Last4(number string) string {
	if len(number) <= 4 {
		return number
	}
	return number[len(number)-4:]
}
