// Package config loads runtime configuration from environment variables,
// with production-sane defaults so the binary runs out of the box in dev
// and only needs a handful of overrides in a real deployment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Addr is the "host:port" the HTTP server listens on.
	Addr string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// JWTSecret signs admin-portal auth tokens. Must be overridden in prod.
	JWTSecret string
	// PhonePepper is mixed into phone-number hashing so a leaked DB alone
	// can't be dictionary-attacked back to phone numbers.
	PhonePepper string
	// TokenTTL is how long an admin JWT stays valid.
	TokenTTL time.Duration
	// SessionTTL is how long a USSD session survives with no input before
	// it's considered abandoned and swept.
	SessionTTL time.Duration
	// RequestTimeout bounds how long any single HTTP request may run.
	RequestTimeout time.Duration
	// ShutdownGrace is how long the server waits for in-flight requests to
	// finish when told to stop.
	ShutdownGrace time.Duration
	// Environment is "development" or "production"; gates things like
	// verbose error bodies and the seeded default admin account.
	Environment string
	// PublicBaseURL is the externally reachable base URL of this server,
	// used to build the verification and evidence-upload links sent back
	// to citizens over USSD (e.g. "https://ussd.example.go.ke").
	PublicBaseURL string
	// UploadsDir is where official photos and report evidence files are
	// stored on disk. Back this up together with DBPath.
	UploadsDir string
	// MaxUploadBytes caps a single uploaded file's size.
	MaxUploadBytes int64
	// EvidenceWindow is how long after a report is filed its one-time
	// upload code remains valid.
	EvidenceWindow time.Duration
	// VerifyAnomalyThreshold/Window govern the "high scrutiny" flag: a
	// work ID checked this many times within this window is flagged as
	// possibly being actively used in an impersonation scam.
	VerifyAnomalyThreshold int
	VerifyAnomalyWindow    time.Duration
	// IDCardTTL is how long a generated ID-card QR token stays valid.
	IDCardTTL time.Duration
}

func Load() Config {
	cfg := Config{
		Addr:                   envOr("ADDR", ":8080"),
		DBPath:                 envOr("DB_PATH", "data/reports.db"),
		JWTSecret:              envOr("JWT_SECRET", "dev-only-insecure-secret-change-me"),
		PhonePepper:            envOr("PHONE_PEPPER", "dev-only-pepper-change-me"),
		TokenTTL:               envDurationOr("TOKEN_TTL", 12*time.Hour),
		SessionTTL:             envDurationOr("SESSION_TTL", 3*time.Minute),
		RequestTimeout:         envDurationOr("REQUEST_TIMEOUT", 10*time.Second),
		ShutdownGrace:          envDurationOr("SHUTDOWN_GRACE", 15*time.Second),
		Environment:            envOr("ENVIRONMENT", "development"),
		PublicBaseURL:          envOr("PUBLIC_BASE_URL", "http://localhost:8080"),
		UploadsDir:             envOr("UPLOADS_DIR", "data/uploads"),
		MaxUploadBytes:         int64(envIntOr("MAX_UPLOAD_MB", 8)) * 1024 * 1024,
		EvidenceWindow:         envDurationOr("EVIDENCE_WINDOW", 72*time.Hour),
		VerifyAnomalyThreshold: envIntOr("VERIFY_ANOMALY_THRESHOLD", 5),
		VerifyAnomalyWindow:    envDurationOr("VERIFY_ANOMALY_WINDOW", 15*time.Minute),
		IDCardTTL:              envDurationOr("IDCARD_TTL", 90*24*time.Hour),
	}
	return cfg
}

func (c Config) IsProduction() bool { return c.Environment == "production" }

// Validate returns a human-readable error for configuration that would be
// unsafe to run in production, so the server refuses to start rather than
// run insecurely without anyone noticing.
func (c Config) Validate() error {
	if c.IsProduction() {
		if c.JWTSecret == "dev-only-insecure-secret-change-me" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if c.PhonePepper == "dev-only-pepper-change-me" {
			return fmt.Errorf("PHONE_PEPPER must be set in production")
		}
		if len(c.JWTSecret) < 16 {
			return fmt.Errorf("JWT_SECRET is too short for production use")
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
