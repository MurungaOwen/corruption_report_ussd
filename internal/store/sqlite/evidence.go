package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

// AddEvidence attaches a file to a report identified by its public ID,
// after validating the plaintext one-time code against the stored hash,
// expiry, and per-report limit — all inside one transaction, so concurrent
// uploads racing against the limit can't both succeed past it. This is the
// path used by the USSD-originated evidence upload page, where the code is
// the only thing proving the uploader is (or was handed the code by) the
// person who filed the report.
func (s *Store) AddEvidence(ctx context.Context, publicID, code string, ev *domain.ReportEvidence) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var reportID int64
	var codeHash, expiresStr string
	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT id, evidence_code_hash, evidence_code_expires, evidence_count
		FROM reports WHERE public_id = ?`, publicID).Scan(&reportID, &codeHash, &expiresStr, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}

	if hashCode(code) != codeHash {
		return store.ErrInvalidEvidence
	}
	expires := parseTime(expiresStr)
	if time.Now().After(expires) {
		return store.ErrInvalidEvidence
	}
	if count >= domain.MaxEvidencePerReport {
		return store.ErrEvidenceLimit
	}

	if err := insertEvidence(ctx, tx, reportID, ev); err != nil {
		return err
	}
	return tx.Commit()
}

// AddEvidenceDirect attaches a file to a report without a code check. It is
// only used server-side, in the same trusted request that just created the
// report via the web citizen-reporting form (no separate device/session
// hand-off happens, so the code's purpose — proving continuity — is moot).
func (s *Store) AddEvidenceDirect(ctx context.Context, reportID int64, ev *domain.ReportEvidence) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT evidence_count FROM reports WHERE id = ?`, reportID).Scan(&count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}
	if count >= domain.MaxEvidencePerReport {
		return store.ErrEvidenceLimit
	}
	if err := insertEvidence(ctx, tx, reportID, ev); err != nil {
		return err
	}
	return tx.Commit()
}

func insertEvidence(ctx context.Context, tx *sql.Tx, reportID int64, ev *domain.ReportEvidence) error {
	now := nowStr()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO report_evidence (report_id, file_path, content_type, size_bytes, created_at)
		VALUES (?, ?, ?, ?, ?)`, reportID, ev.FilePath, ev.ContentType, ev.SizeBytes, now)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	ev.ID = id
	ev.ReportID = reportID

	if _, err := tx.ExecContext(ctx, `UPDATE reports SET evidence_count = evidence_count + 1, updated_at = ? WHERE id = ?`, now, reportID); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListEvidence(ctx context.Context, reportID int64) ([]domain.ReportEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, report_id, file_path, content_type, size_bytes, created_at
		FROM report_evidence WHERE report_id = ? ORDER BY created_at ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ReportEvidence
	for rows.Next() {
		var e domain.ReportEvidence
		var created string
		if err := rows.Scan(&e.ID, &e.ReportID, &e.FilePath, &e.ContentType, &e.SizeBytes, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEvidenceFile(ctx context.Context, reportID, evidenceID int64) (*domain.ReportEvidence, error) {
	var e domain.ReportEvidence
	var created string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, report_id, file_path, content_type, size_bytes, created_at
		FROM report_evidence WHERE id = ? AND report_id = ?`, evidenceID, reportID).
		Scan(&e.ID, &e.ReportID, &e.FilePath, &e.ContentType, &e.SizeBytes, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.CreatedAt = parseTime(created)
	return &e, nil
}
