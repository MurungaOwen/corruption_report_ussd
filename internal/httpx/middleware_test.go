package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPStripsEphemeralPort(t *testing.T) {
	// Regression test: ClientIP used to return the raw RemoteAddr
	// ("ip:port"). Since the port is different on every TCP connection,
	// that put every single request into its own rate-limit bucket,
	// silently disabling rate limiting for any client connecting directly
	// (no reverse proxy in front).
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.RemoteAddr = "127.0.0.1:54321"
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "127.0.0.1:60000"

	ip1 := ClientIP(r1, false)
	ip2 := ClientIP(r2, false)
	if ip1 != ip2 {
		t.Fatalf("expected the same client IP across connections with different ports, got %q and %q", ip1, ip2)
	}
	if ip1 != "127.0.0.1" {
		t.Fatalf("expected port to be stripped, got %q", ip1)
	}
}

func TestClientIPIgnoresForwardedForByDefault(t *testing.T) {
	// A client with no reverse proxy in front must not be able to spoof
	// its rate-limit identity (or the anti-impersonation "distinct
	// requester" signal) by setting X-Forwarded-For itself.
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")

	if got := ClientIP(r, false); got != "203.0.113.9" {
		t.Fatalf("expected X-Forwarded-For to be ignored when trustProxyHeaders=false, got %q", got)
	}
	if got := ClientIP(r, true); got != "1.2.3.4" {
		t.Fatalf("expected X-Forwarded-For to be honored when trustProxyHeaders=true, got %q", got)
	}
}

func TestClientIPTakesLeftmostForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1, 10.0.0.2")

	if got := ClientIP(r, true); got != "9.9.9.9" {
		t.Fatalf("expected leftmost (original client) entry, got %q", got)
	}
}
