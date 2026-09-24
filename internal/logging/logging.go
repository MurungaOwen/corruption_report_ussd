// Package logging sets up structured logging and a request-ID middleware
// so every log line for a request can be correlated end to end.
package logging

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = iota

// New builds a JSON structured logger writing to stdout, suitable for any
// log aggregator (journald, Docker logs, Loki, CloudWatch...) without extra
// configuration.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// RequestID returns the request ID stashed in ctx by the Middleware, or ""
// if none is present.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// Middleware assigns a request ID, logs the request lifecycle, and recovers
// from panics so a single bad handler never takes the whole process down.
// A panic is logged with a stack-free summary and turned into a 500 for API
// callers; USSD handlers additionally wrap this so citizens see a calm
// translated message instead.
func Middleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.NewString()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, reqID)
			w.Header().Set("X-Request-ID", reqID)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"request_id", reqID,
						"path", r.URL.Path,
						"method", r.Method,
						"panic", rec,
					)
					if !sw.wroteHeader {
						http.Error(sw, "internal server error", http.StatusInternalServerError)
					}
				}
				logger.Info("request",
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"status", sw.status,
					"duration_ms", time.Since(start).Milliseconds(),
					"remote", r.RemoteAddr,
				)
			}()

			next.ServeHTTP(sw, r.WithContext(ctx))
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.wroteHeader = true
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	sw.wroteHeader = true
	return sw.ResponseWriter.Write(b)
}
