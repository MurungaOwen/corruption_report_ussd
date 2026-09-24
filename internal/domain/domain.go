// Package domain holds the plain data types shared across the system.
// Nothing in this package imports anything else in this module: every
// other package depends inward on domain, never the other way around.
package domain

import "time"

// OfficialStatus is the verification state of a government official record.
type OfficialStatus string

const (
	OfficialVerified      OfficialStatus = "verified"
	OfficialUnverified    OfficialStatus = "unverified"
	OfficialInvestigation OfficialStatus = "under_investigation"
)

// Official is a government officer citizens can look up by work ID.
//
// PhotoPath is the reference photo an admin uploads at registration time —
// the whole point of it is to be shown back to a citizen on the public
// verification page, so someone impersonating an officer by reciting a
// memorized work ID can be caught by a face that doesn't match.
type Official struct {
	ID         int64          `json:"id"`
	WorkID     string         `json:"work_id"`
	Name       string         `json:"name"`
	Position   string         `json:"position"`
	Department string         `json:"department"`
	Status     OfficialStatus `json:"status"`
	PhotoPath  string         `json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// PublicOfficial is the deliberately minimal, safe-to-publish view of an
// Official served from the unauthenticated verification endpoint.
type PublicOfficial struct {
	WorkID       string         `json:"work_id"`
	Name         string         `json:"name"`
	Position     string         `json:"position"`
	Department   string         `json:"department"`
	Status       OfficialStatus `json:"status"`
	PhotoURL     string         `json:"photo_url,omitempty"`
	HighScrutiny bool           `json:"high_scrutiny"`
	CardValid    *bool          `json:"card_valid,omitempty"` // nil = no card token presented
	CardExpires  *time.Time     `json:"card_expires,omitempty"`
}

// VerificationSource distinguishes how a citizen looked an official up.
type VerificationSource string

const (
	VerifyViaUSSD VerificationSource = "ussd"
	VerifyViaWeb  VerificationSource = "web"
)

// VerificationEvent is one lookup of an official's identity, logged so
// unusually frequent checks against a single work ID (a sign of an active
// impersonation scam) can be detected. See ARCHITECTURE.md.
type VerificationEvent struct {
	ID           int64              `json:"id"`
	WorkID       string             `json:"work_id"`
	Source       VerificationSource `json:"source"`
	RequesterKey string             `json:"-"` // hashed phone or IP, never raw
	CreatedAt    time.Time          `json:"created_at"`
}

// ReportStatus is the lifecycle state of a corruption report.
type ReportStatus string

const (
	ReportPending     ReportStatus = "pending"
	ReportUnderReview ReportStatus = "under_review"
	ReportResolved    ReportStatus = "resolved"
	ReportDismissed   ReportStatus = "dismissed"
)

// ValidReportStatuses is used to validate admin-supplied transitions.
var ValidReportStatuses = map[ReportStatus]bool{
	ReportPending:     true,
	ReportUnderReview: true,
	ReportResolved:    true,
	ReportDismissed:   true,
}

// Report is a citizen-submitted corruption report.
//
// PhoneHash and PhoneLast4 exist so the admin UI never needs to display or
// even load a full phone number: PhoneLast4 is enough for the case officer
// to informally verify a caller, PhoneHash lets us look up "reports by this
// phone" without ever storing the number in the clear at rest.
type Report struct {
	ID                  int64        `json:"id"`
	PublicID            string       `json:"public_id"`
	PhoneHash           string       `json:"-"`
	PhoneLast4          string       `json:"phone_last4"`
	Description         string       `json:"description"`
	OfficialID          *int64       `json:"official_id,omitempty"`
	Status              ReportStatus `json:"status"`
	Language            string       `json:"language"`
	ReportType          ReportType   `json:"report_type"`
	Channel             string       `json:"channel"` // "ussd" | "web"
	IdempotencyKey      string       `json:"-"`       // dedupes retried creation requests at the DB layer
	EvidenceCodeHash    string       `json:"-"`
	EvidenceCodeExpires time.Time    `json:"-"`
	EvidenceCount       int          `json:"evidence_count"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

// ReportType distinguishes an ordinary corruption report from a "the
// person I just checked doesn't match their photo" impersonation report
// filed straight from the public verification page.
type ReportType string

const (
	ReportTypeCorruption    ReportType = "corruption"
	ReportTypeImpersonation ReportType = "impersonation"
)

// ReportEvidence is one photo (or other small file) a citizen attached to a
// report through the public upload page. Evidence is never public — only
// admin-portal users with an authenticated session may view the files.
type ReportEvidence struct {
	ID          int64     `json:"id"`
	ReportID    int64     `json:"report_id"`
	FilePath    string    `json:"-"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

// MaxEvidencePerReport bounds how many files can be attached to a single
// report, so the one-time code can't be abused to fill the disk.
const MaxEvidencePerReport = 6

// ReportStatusEvent is one append-only entry in a report's audit trail.
type ReportStatusEvent struct {
	ID         int64        `json:"id"`
	ReportID   int64        `json:"report_id"`
	FromStatus ReportStatus `json:"from_status"`
	ToStatus   ReportStatus `json:"to_status"`
	Actor      string       `json:"actor"`
	Reason     string       `json:"reason,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// AdminRole controls what an admin portal user may do.
type AdminRole string

const (
	RoleAdmin   AdminRole = "admin"   // full access, including user + official management
	RoleOfficer AdminRole = "officer" // can triage and update reports
	RoleViewer  AdminRole = "viewer"  // read-only
)

// AdminUser is a person who can sign into the web portal.
type AdminUser struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	Role         AdminRole `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// USSDSession is persisted so a server restart or an aggregator retry never
// strands a citizen mid-flow: session position lives on disk, not in a
// process-local map.
type USSDSession struct {
	SessionID string    `json:"session_id"`
	Phone     string    `json:"phone"`
	Language  string    `json:"language"`
	Cursor    string    `json:"cursor"` // last full USSD text path, e.g. "1*2"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AuditEvent is a generic append-only log entry for admin-portal actions.
type AuditEvent struct {
	ID        int64     `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Stats is the dashboard summary shown on the admin portal home page.
type Stats struct {
	TotalReports      int            `json:"total_reports"`
	ReportsByStatus   map[string]int `json:"reports_by_status"`
	TotalOfficials    int            `json:"total_officials"`
	OfficialsByStatus map[string]int `json:"officials_by_status"`
	ReportsLast7Days  int            `json:"reports_last_7_days"`
}
