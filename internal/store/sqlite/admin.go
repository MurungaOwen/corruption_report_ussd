package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

func (s *Store) CreateAdminUser(ctx context.Context, u *domain.AdminUser) error {
	if u.Role == "" {
		u.Role = domain.RoleOfficer
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_users (email, name, password_hash, role, created_at)
		VALUES (?, ?, ?, ?, ?)`, u.Email, u.Name, u.PasswordHash, u.Role, now.Format(time.RFC3339))
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
	u.ID = id
	u.CreatedAt = now
	return nil
}

func (s *Store) GetAdminUserByEmail(ctx context.Context, email string) (*domain.AdminUser, error) {
	var u domain.AdminUser
	var created string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, role, created_at FROM admin_users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	return &u, nil
}

func (s *Store) CountAdminUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n, err
}

func (s *Store) ListAdminUsers(ctx context.Context) ([]domain.AdminUser, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, name, password_hash, role, created_at FROM admin_users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AdminUser
	for rows.Next() {
		var u domain.AdminUser
		var created string
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &created); err != nil {
			return nil, err
		}
		u.CreatedAt = parseTime(created)
		out = append(out, u)
	}
	return out, rows.Err()
}
