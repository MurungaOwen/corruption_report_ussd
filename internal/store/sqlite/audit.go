package sqlite

import (
	"context"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
)

func (s *Store) RecordAudit(ctx context.Context, e domain.AuditEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (actor, action, target, detail, created_at)
		VALUES (?, ?, ?, ?, ?)`, e.Actor, e.Action, e.Target, e.Detail, nowStr())
	return err
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor, action, target, detail, created_at FROM audit_log
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var created string
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &e.Detail, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}
