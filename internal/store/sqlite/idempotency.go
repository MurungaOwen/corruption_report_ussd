package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

// GetIdempotent looks up a previously computed USSD response for this
// exact (sessionId, text) pair. When an aggregator retries a callback
// after a timeout, this lets the handler replay the original response
// byte-for-byte instead of re-running (and potentially duplicating) the
// underlying state change.
func (s *Store) GetIdempotent(ctx context.Context, sessionID, text string) (string, bool, error) {
	var resp string
	err := s.db.QueryRowContext(ctx, `
		SELECT response_body FROM idempotency_keys WHERE session_id = ? AND text = ?`, sessionID, text).Scan(&resp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return resp, true, nil
}

func (s *Store) PutIdempotent(ctx context.Context, sessionID, text, response string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (session_id, text, response_body, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id, text) DO NOTHING`, sessionID, text, response, nowStr())
	return err
}
