// Package sqlite implements store.Store on top of a single SQLite file
// using the pure-Go modernc.org/sqlite driver — no CGO, so the resulting
// binary has zero system-library dependencies and can run FROM SCRATCH in
// Docker. WAL mode + a busy timeout absorb the write contention a
// single-file database sees under concurrent USSD + admin-portal traffic;
// internal/httpx.RetryBusy handles the rest at the call site.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

//go:embed all:migrations
var embeddedMigrations embed.FS

// migrationsFS lets tests point at the repo-root migrations directory
// directly; production uses the embedded copy above via NewFromDefaultFS.
type Store struct {
	db *sql.DB
}

// Open connects to (creating if necessary) the SQLite file at path, tunes
// it for a single-writer-many-readers workload, and applies all pending
// migrations. It is safe to call concurrently from multiple processes only
// in the sense that SQLite itself serializes them — this system is
// designed to run as one process (see MANIFESTO.md §5).
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single physical writer is how SQLite works best; capping the pool
	// avoids SQLITE_BUSY storms under load rather than fixing them after
	// the fact.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx, embeddedMigrations, "migrations"); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context, fsys embed.FS, dir string) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var already int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, name).Scan(&already); err != nil {
			return err
		}
		if already > 0 {
			continue
		}
		content, err := fsys.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename, applied_at) VALUES (?, ?)`, name, time.Now().UTC().Format(time.RFC3339)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

var _ store.Store = (*Store)(nil)
