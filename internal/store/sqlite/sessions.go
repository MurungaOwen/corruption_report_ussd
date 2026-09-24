package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

func (s *Store) GetSession(ctx context.Context, sessionID string) (*domain.USSDSession, error) {
	var sess domain.USSDSession
	var created, updated, expires string
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, phone, language, cursor, created_at, updated_at, expires_at
		FROM ussd_sessions WHERE session_id = ?`, sessionID).
		Scan(&sess.SessionID, &sess.Phone, &sess.Language, &sess.Cursor, &created, &updated, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess.CreatedAt = parseTime(created)
	sess.UpdatedAt = parseTime(updated)
	sess.ExpiresAt = parseTime(expires)
	return &sess, nil
}

// SaveSession upserts session state. Persisting it (rather than keeping it
// in a process-local map) means a server restart mid-USSD-session resumes
// the citizen where they left off instead of silently dropping them back
// to the language prompt.
func (s *Store) SaveSession(ctx context.Context, sess *domain.USSDSession) error {
	now := nowStr()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ussd_sessions (session_id, phone, language, cursor, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			phone = excluded.phone,
			language = excluded.language,
			cursor = excluded.cursor,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at`,
		sess.SessionID, sess.Phone, sess.Language, sess.Cursor, now, now, sess.ExpiresAt.UTC().Format(time.RFC3339))
	return err
}

// SweepExpiredSessions deletes sessions whose expiry has passed, keeping
// the table from growing without bound. Session *rows* being pruned is
// fine — the citizen-visible reports, evidence, and audit trail they may
// have produced along the way are untouched.
func (s *Store) SweepExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ussd_sessions WHERE expires_at < ?`, before.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
