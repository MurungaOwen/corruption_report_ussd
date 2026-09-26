package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

func (s *Store) CreateOfficial(ctx context.Context, o *domain.Official) error {
	now := time.Now().UTC()
	nowFmt := now.Format(time.RFC3339)
	if o.Status == "" {
		o.Status = domain.OfficialUnverified
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO officials (work_id, name, position, department, status, photo_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		o.WorkID, o.Name, o.Position, o.Department, o.Status, o.PhotoPath, nowFmt, nowFmt)
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrConflict
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	o.ID = id
	o.CreatedAt = now
	o.UpdatedAt = now
	return nil
}

func (s *Store) GetOfficialByID(ctx context.Context, id int64) (*domain.Official, error) {
	return s.scanOfficial(s.db.QueryRowContext(ctx, officialSelect+` WHERE id = ?`, id))
}

func (s *Store) GetOfficialByWorkID(ctx context.Context, workID string) (*domain.Official, error) {
	return s.scanOfficial(s.db.QueryRowContext(ctx, officialSelect+` WHERE work_id = ?`, workID))
}

const officialSelect = `SELECT id, work_id, name, position, department, status, photo_path, created_at, updated_at FROM officials`

func (s *Store) scanOfficial(row *sql.Row) (*domain.Official, error) {
	var o domain.Official
	var created, updated string
	err := row.Scan(&o.ID, &o.WorkID, &o.Name, &o.Position, &o.Department, &o.Status, &o.PhotoPath, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	o.CreatedAt = parseTime(created)
	o.UpdatedAt = parseTime(updated)
	return &o, nil
}

func (s *Store) ListOfficials(ctx context.Context) ([]domain.Official, error) {
	rows, err := s.db.QueryContext(ctx, officialSelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Official
	for rows.Next() {
		var o domain.Official
		var created, updated string
		if err := rows.Scan(&o.ID, &o.WorkID, &o.Name, &o.Position, &o.Department, &o.Status, &o.PhotoPath, &created, &updated); err != nil {
			return nil, err
		}
		o.CreatedAt = parseTime(created)
		o.UpdatedAt = parseTime(updated)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) UpdateOfficialStatus(ctx context.Context, id int64, status domain.OfficialStatus) error {
	res, err := s.db.ExecContext(ctx, `UPDATE officials SET status = ?, updated_at = ? WHERE id = ?`, status, nowStr(), id)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (s *Store) SetOfficialPhoto(ctx context.Context, id int64, photoPath string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE officials SET photo_path = ?, updated_at = ? WHERE id = ?`, photoPath, nowStr(), id)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
