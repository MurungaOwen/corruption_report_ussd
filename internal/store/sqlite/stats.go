package sqlite

import (
	"context"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
)

func (s *Store) GetStats(ctx context.Context) (domain.Stats, error) {
	stats := domain.Stats{
		ReportsByStatus:   map[string]int{},
		OfficialsByStatus: map[string]int{},
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports`).Scan(&stats.TotalReports); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM officials`).Scan(&stats.TotalOfficials); err != nil {
		return stats, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM reports GROUP BY status`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return stats, err
		}
		stats.ReportsByStatus[status] = n
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM officials GROUP BY status`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return stats, err
		}
		stats.OfficialsByStatus[status] = n
	}
	rows.Close()

	weekAgo := time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE created_at >= ?`, weekAgo).Scan(&stats.ReportsLast7Days); err != nil {
		return stats, err
	}

	return stats, nil
}
