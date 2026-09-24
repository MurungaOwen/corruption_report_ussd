package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

// CreateReport inserts a report and returns a fresh plaintext one-time
// evidence upload code. Only the code's SHA-256 hash is persisted, so the
// plaintext exists in memory only long enough to hand back to the citizen
// (via the USSD END message) once.
//
// When r.IdempotencyKey is set (the USSD path always sets it, derived from
// the aggregator's sessionId+text), a retried creation request — even one
// arriving after the process crashed between running the engine and
// recording an idempotency-cache entry — cannot produce a second report
// row: the unique index on idempotency_key turns the second INSERT into a
// no-op, detected here and handled by reusing the existing report and
// issuing it a fresh evidence code. This is the "recorded in the same
// transaction as the state change" guarantee described in
// ARCHITECTURE.md, enforced by the database itself rather than by
// caller discipline.
func (s *Store) CreateReport(ctx context.Context, r *domain.Report) (string, error) {
	if r.Status == "" {
		r.Status = domain.ReportPending
	}
	if r.ReportType == "" {
		r.ReportType = domain.ReportTypeCorruption
	}
	code, err := randomDigits(6)
	if err != nil {
		return "", err
	}
	codeHash := hashCode(code)
	nowTime := time.Now().UTC()
	expires := nowTime.Add(72 * time.Hour)
	now := nowTime.Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if r.IdempotencyKey != "" {
		var existingID int64
		var existingPublicID, existingCreatedAt string
		err := tx.QueryRowContext(ctx, `SELECT id, public_id, created_at FROM reports WHERE idempotency_key = ?`, r.IdempotencyKey).
			Scan(&existingID, &existingPublicID, &existingCreatedAt)
		if err == nil {
			// Same request seen before: reuse the existing report, issue a
			// fresh evidence code (the old one, if ever delivered, simply
			// stops working — safe, since this is the same requester retrying).
			if _, err := tx.ExecContext(ctx, `
				UPDATE reports SET evidence_code_hash = ?, evidence_code_expires = ?, updated_at = ? WHERE id = ?`,
				codeHash, expires.UTC().Format(time.RFC3339), now, existingID); err != nil {
				return "", err
			}
			if err := tx.Commit(); err != nil {
				return "", err
			}
			r.ID = existingID
			r.PublicID = existingPublicID
			r.EvidenceCodeExpires = expires
			r.CreatedAt = parseTime(existingCreatedAt)
			r.UpdatedAt = nowTime
			return code, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	publicID, err := generatePublicID()
	if err != nil {
		return "", err
	}
	r.PublicID = publicID

	res, err := tx.ExecContext(ctx, `
		INSERT INTO reports (public_id, phone_hash, phone_last4, description, official_id, status, language,
			report_type, channel, idempotency_key, evidence_code_hash, evidence_code_expires, evidence_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		r.PublicID, r.PhoneHash, r.PhoneLast4, r.Description, r.OfficialID, r.Status, r.Language,
		r.ReportType, r.Channel, r.IdempotencyKey, codeHash, expires.UTC().Format(time.RFC3339), now, now)
	if err != nil {
		return "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	r.ID = id

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO report_status_history (report_id, from_status, to_status, actor, reason, created_at)
		VALUES (?, '', ?, ?, 'report filed', ?)`,
		r.ID, r.Status, actorFor(r.Channel), now); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	r.EvidenceCodeExpires = expires
	r.CreatedAt = nowTime
	r.UpdatedAt = nowTime
	return code, nil
}

func actorFor(channel string) string {
	if channel == "web" {
		return "citizen (web)"
	}
	return "citizen (ussd)"
}

const reportSelect = `SELECT id, public_id, phone_hash, phone_last4, description, official_id, status, language,
	report_type, channel, evidence_count, created_at, updated_at FROM reports`

func (s *Store) scanReport(row interface{ Scan(...any) error }) (*domain.Report, error) {
	var r domain.Report
	var officialID sql.NullInt64
	var created, updated string
	err := row.Scan(&r.ID, &r.PublicID, &r.PhoneHash, &r.PhoneLast4, &r.Description, &officialID, &r.Status,
		&r.Language, &r.ReportType, &r.Channel, &r.EvidenceCount, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if officialID.Valid {
		r.OfficialID = &officialID.Int64
	}
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return &r, nil
}

func (s *Store) GetReportByID(ctx context.Context, id int64) (*domain.Report, error) {
	return s.scanReport(s.db.QueryRowContext(ctx, reportSelect+` WHERE id = ?`, id))
}

func (s *Store) GetReportByPublicID(ctx context.Context, publicID string) (*domain.Report, error) {
	return s.scanReport(s.db.QueryRowContext(ctx, reportSelect+` WHERE public_id = ?`, publicID))
}

func (s *Store) ListReports(ctx context.Context, f store.ReportFilter) ([]domain.Report, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Query != "" {
		where = append(where, "(description LIKE ? OR public_id LIKE ?)")
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE `+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := reportSelect + ` WHERE ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []domain.Report
	for rows.Next() {
		r, err := s.scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *r)
	}
	return out, total, rows.Err()
}

func (s *Store) UpdateReportStatus(ctx context.Context, id int64, to domain.ReportStatus, actor, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var from string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM reports WHERE id = ?`, id).Scan(&from); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}

	now := nowStr()
	if _, err := tx.ExecContext(ctx, `UPDATE reports SET status = ?, updated_at = ? WHERE id = ?`, to, now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO report_status_history (report_id, from_status, to_status, actor, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, from, to, actor, reason, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReportHistory(ctx context.Context, reportID int64) ([]domain.ReportStatusEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, report_id, from_status, to_status, actor, reason, created_at
		FROM report_status_history WHERE report_id = ? ORDER BY created_at ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ReportStatusEvent
	for rows.Next() {
		var e domain.ReportStatusEvent
		var created string
		if err := rows.Scan(&e.ID, &e.ReportID, &e.FromStatus, &e.ToStatus, &e.Actor, &e.Reason, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- helpers ---

func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[idx.Int64()]
	}
	return string(b), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// generatePublicID produces a citizen-facing, unambiguous report ID like
// "RPT-2026-7K3QF2" — Crockford-style base32 (no 0/O/1/I confusion) so it
// can be read aloud or copied off a phone screen without transcription
// errors.
func generatePublicID() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	enc := base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)
	code := enc.EncodeToString(buf)
	if len(code) > 6 {
		code = code[:6]
	}
	return fmt.Sprintf("RPT-%d-%s", time.Now().UTC().Year(), code), nil
}
