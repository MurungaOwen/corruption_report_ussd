package httpx

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"
)

// RetryBusy retries fn with exponential backoff + jitter when the SQLite
// engine reports the database is locked/busy under concurrent writes.
// SQLite (even in WAL mode) serializes writers, so under bursty load a
// write can transiently fail with SQLITE_BUSY; the correct response is a
// short bounded retry, not surfacing an error to the citizen or the admin.
//
// Any other error is returned immediately without retrying.
func RetryBusy(ctx context.Context, attempts int, fn func() error) error {
	var err error
	backoff := 20 * time.Millisecond
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isBusyErr(err) {
			return err
		}
		jitter := time.Duration(rand.Int63n(int64(backoff)))
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff *= 2
		if backoff > 500*time.Millisecond {
			backoff = 500 * time.Millisecond
		}
	}
	return err
}

func isBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "busy")
}

// ErrTimeout is returned by callers that want a sentinel for "gave up".
var ErrTimeout = errors.New("operation timed out")
