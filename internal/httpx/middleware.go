package httpx

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Timeout bounds request handling so one slow request can't exhaust server
// resources; it responds 503 if the handler doesn't finish in time.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timed out"}`)
	}
}

// WithContextTimeout is applied inside handlers that need the deadline on
// r.Context() itself (e.g. to bound a DB call), rather than just wrapping
// the ResponseWriter as http.TimeoutHandler does.
func WithContextTimeout(r *http.Request, d time.Duration) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), d)
	return r.WithContext(ctx), cancel
}

// RateLimiter is a simple per-key token bucket, safe for concurrent use.
// It exists to slow down brute-force login attempts and abusive USSD
// callback floods without needing an external dependency like Redis —
// appropriate for the single-process deployment this system targets.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	capacity float64
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func NewRateLimiter(ratePerSecond float64, burst float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     ratePerSecond,
		capacity: burst,
	}
	go rl.gc()
	return rl
}

// Allow reports whether a request for key should proceed, consuming a
// token if so.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[key] = b
	}
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gc periodically evicts idle buckets so long-running processes don't leak
// memory tracking clients that stopped calling.
func (rl *RateLimiter) gc() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-30 * time.Minute)
		for k, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware rejects requests with 429 once a key (e.g. client IP) exceeds
// its rate.
func (rl *RateLimiter) Middleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(keyFunc(r)) {
				Error(w, http.StatusTooManyRequests, "too many requests, slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP extracts a best-effort client identifier for rate limiting and
// verification-spike detection. trustProxyHeaders must be true only when
// this process sits behind a reverse proxy that itself sets (overwrites)
// X-Forwarded-For — otherwise any client can put anything in that header
// and either defeat rate limiting or forge distinct "requesters" for the
// anti-impersonation spike signal. See config.Config.TrustProxyHeaders.
func ClientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// The header can be a client-added chain "real, proxy1, proxy2";
			// the leftmost entry is the original client as the nearest
			// trusted proxy recorded it.
			if i := strings.IndexByte(fwd, ','); i >= 0 {
				fwd = fwd[:i]
			}
			if ip := strings.TrimSpace(fwd); ip != "" {
				return ip
			}
		}
	}
	// RemoteAddr is "ip:port" — the port is a new random value on every
	// connection, so keying on the raw string would put every single
	// request in its own bucket and silently disable rate limiting.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
