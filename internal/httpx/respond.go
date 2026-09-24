// Package httpx holds small, dependency-free HTTP building blocks shared by
// the admin API and the USSD handler: JSON helpers, a request-timeout
// middleware, a token-bucket rate limiter, and a generic retry-with-backoff
// helper for the SQLite-busy / transient-error cases.
package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response body with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a consistent {"error": "..."} JSON body.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// DecodeJSON reads and decodes a JSON request body into v, rejecting
// unknown fields so typos in client payloads fail loudly instead of being
// silently ignored.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
