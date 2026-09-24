// Package store defines the persistence interfaces used by the rest of the
// application. Handlers and the USSD engine depend only on these
// interfaces, never on the concrete sqlite package — that's what keeps
// business logic testable with an in-memory fake and keeps the door open
// to a Postgres implementation later without touching callers.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrInvalidEvidence = errors.New("invalid or expired evidence code")
	ErrEvidenceLimit   = errors.New("evidence limit reached for this report")
)

// ReportFilter narrows a report listing for the admin portal.
type ReportFilter struct {
	Status string // "" = any
	Query  string // matches description / public_id substring
	Limit  int
	Offset int
}

type Store interface {
	// Officials
	CreateOfficial(ctx context.Context, o *domain.Official) error
	GetOfficialByID(ctx context.Context, id int64) (*domain.Official, error)
	GetOfficialByWorkID(ctx context.Context, workID string) (*domain.Official, error)
	ListOfficials(ctx context.Context) ([]domain.Official, error)
	UpdateOfficialStatus(ctx context.Context, id int64, status domain.OfficialStatus) error
	SetOfficialPhoto(ctx context.Context, id int64, photoPath string) error

	// Reports. CreateReport also generates and returns the plaintext
	// one-time evidence code (only ever available at creation time — only
	// its hash is persisted).
	CreateReport(ctx context.Context, r *domain.Report) (evidenceCode string, err error)
	GetReportByID(ctx context.Context, id int64) (*domain.Report, error)
	GetReportByPublicID(ctx context.Context, publicID string) (*domain.Report, error)
	ListReports(ctx context.Context, f ReportFilter) ([]domain.Report, int, error)
	UpdateReportStatus(ctx context.Context, id int64, to domain.ReportStatus, actor, reason string) error
	ReportHistory(ctx context.Context, reportID int64) ([]domain.ReportStatusEvent, error)

	// Evidence. AddEvidence validates the plaintext code against the
	// report's stored hash/expiry/limit inside the same transaction as the
	// insert + counter bump, so concurrent uploads can't race past the
	// MaxEvidencePerReport cap.
	AddEvidence(ctx context.Context, publicID, code string, ev *domain.ReportEvidence) error
	// AddEvidenceDirect skips the code check — used only server-side, in
	// the same request that created a web-originated report.
	AddEvidenceDirect(ctx context.Context, reportID int64, ev *domain.ReportEvidence) error
	ListEvidence(ctx context.Context, reportID int64) ([]domain.ReportEvidence, error)
	GetEvidenceFile(ctx context.Context, reportID, evidenceID int64) (*domain.ReportEvidence, error)

	// Idempotency: dedupe retried USSD callbacks.
	GetIdempotent(ctx context.Context, sessionID, text string) (string, bool, error)
	PutIdempotent(ctx context.Context, sessionID, text, response string) error

	// USSD sessions
	GetSession(ctx context.Context, sessionID string) (*domain.USSDSession, error)
	SaveSession(ctx context.Context, s *domain.USSDSession) error
	SweepExpiredSessions(ctx context.Context, before time.Time) (int64, error)

	// Admin users
	CreateAdminUser(ctx context.Context, u *domain.AdminUser) error
	GetAdminUserByEmail(ctx context.Context, email string) (*domain.AdminUser, error)
	CountAdminUsers(ctx context.Context) (int, error)
	ListAdminUsers(ctx context.Context) ([]domain.AdminUser, error)

	// Audit
	RecordAudit(ctx context.Context, e domain.AuditEvent) error
	ListAudit(ctx context.Context, limit int) ([]domain.AuditEvent, error)

	// Verification events: logged on every official lookup (USSD or web)
	// so unusually frequent checks against one work ID can be detected.
	// See ARCHITECTURE.md "Anti-impersonation defense in depth".
	RecordVerification(ctx context.Context, e domain.VerificationEvent) error
	// CountRecentVerifications returns the number of lookups and the
	// number of distinct requesters for workID within window.
	CountRecentVerifications(ctx context.Context, workID string, window time.Duration) (count int, distinctRequesters int, err error)

	// Stats
	GetStats(ctx context.Context) (domain.Stats, error)

	Close() error
}
