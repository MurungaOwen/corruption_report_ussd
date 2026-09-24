package sqlite

import (
	"context"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
)

func (s *Store) RecordVerification(ctx context.Context, e domain.VerificationEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO verification_events (work_id, source, requester_key, created_at)
		VALUES (?, ?, ?, ?)`, e.WorkID, e.Source, e.RequesterKey, nowStr())
	return err
}

// CountRecentVerifications powers the "high scrutiny" anomaly flag: a work
// ID looked up an unusual number of times by an unusual number of distinct
// requesters in a short window is a sign it's being actively used in an
// impersonation scam right now. See ARCHITECTURE.md.
func (s *Store) CountRecentVerifications(ctx context.Context, workID string, window time.Duration) (int, int, error) {
	since := time.Now().Add(-window).UTC().Format(time.RFC3339)
	var count, distinct int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM verification_events WHERE work_id = ? AND created_at >= ?`, workID, since).Scan(&count); err != nil {
		return 0, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT requester_key) FROM verification_events WHERE work_id = ? AND created_at >= ?`, workID, since).Scan(&distinct); err != nil {
		return 0, 0, err
	}
	return count, distinct, nil
}
