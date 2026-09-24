// Package api wires together the USSD callback, the public citizen-facing
// JSON endpoints, and the authenticated admin-portal JSON endpoints into a
// single http.Handler. See ARCHITECTURE.md for the request lifecycle and
// MANIFESTO.md for why things are built the way they are (idempotency,
// persisted sessions, never-trust-the-network).
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
	"github.com/murungaowen/corruption_report_ussd/internal/idcard"
	"github.com/murungaowen/corruption_report_ussd/internal/logging"
	"github.com/murungaowen/corruption_report_ussd/internal/notify"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
	"github.com/murungaowen/corruption_report_ussd/internal/ussd"
)

type Server struct {
	cfg    config.Config
	store  store.Store
	logger *slog.Logger
	engine *ussd.Engine
	tokens *auth.TokenIssuer
	cards  *idcard.Issuer
	sms    notify.SMSSender
	webDir string
}

func New(cfg config.Config, s store.Store, logger *slog.Logger, webDir string) *Server {
	return &Server{
		cfg:    cfg,
		store:  s,
		logger: logger,
		engine: ussd.NewEngine(s, cfg),
		tokens: auth.NewTokenIssuer(cfg.JWTSecret, cfg.TokenTTL),
		cards:  idcard.NewIssuer(cfg.JWTSecret),
		sms:    notify.LogSMSSender{Logger: logger},
		webDir: webDir,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// --- Health ---
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// --- USSD callback (public, high volume, idempotent) ---
	mux.HandleFunc("POST /ussd", s.handleUSSD)

	// --- Public citizen-facing JSON API ---
	mux.HandleFunc("GET /api/v1/public/officials/{work_id}", s.handlePublicGetOfficial)
	mux.HandleFunc("POST /api/v1/public/officials/{work_id}/impersonation-report", s.handlePublicImpersonationReport)
	mux.HandleFunc("POST /api/v1/public/reports", s.handlePublicCreateReport)
	mux.HandleFunc("GET /api/v1/public/reports/{public_id}/status", s.handlePublicReportStatus)
	mux.HandleFunc("POST /api/v1/public/reports/{public_id}/evidence", s.handlePublicUploadEvidence)

	// --- Public static file serving of official reference photos ---
	mux.HandleFunc("GET /uploads/officials/", s.handlePublicOfficialPhoto)

	// --- Auth ---
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(RoleViewer, s.handleMe))

	// --- Admin: reports ---
	mux.HandleFunc("GET /api/v1/reports", s.requireAuth(RoleViewer, s.handleListReports))
	mux.HandleFunc("GET /api/v1/reports/{id}", s.requireAuth(RoleViewer, s.handleGetReport))
	mux.HandleFunc("PATCH /api/v1/reports/{id}/status", s.requireAuth(RoleOfficer, s.handleUpdateReportStatus))
	mux.HandleFunc("GET /api/v1/reports/{id}/evidence/{evidence_id}/file", s.requireAuth(RoleViewer, s.handleReportEvidenceFile))

	// --- Admin: officials ---
	mux.HandleFunc("GET /api/v1/officials", s.requireAuth(RoleViewer, s.handleListOfficials))
	mux.HandleFunc("POST /api/v1/officials", s.requireAuth(RoleAdmin, s.handleCreateOfficial))
	mux.HandleFunc("PATCH /api/v1/officials/{id}/status", s.requireAuth(RoleAdmin, s.handleUpdateOfficialStatus))
	mux.HandleFunc("POST /api/v1/officials/{id}/photo", s.requireAuth(RoleAdmin, s.handleUploadOfficialPhoto))
	mux.HandleFunc("POST /api/v1/officials/{id}/idcard-token", s.requireAuth(RoleAdmin, s.handleIssueCardToken))

	// --- Admin: users, stats, audit ---
	mux.HandleFunc("GET /api/v1/admin-users", s.requireAuth(RoleAdmin, s.handleListAdminUsers))
	mux.HandleFunc("POST /api/v1/admin-users", s.requireAuth(RoleAdmin, s.handleCreateAdminUser))
	mux.HandleFunc("GET /api/v1/stats", s.requireAuth(RoleViewer, s.handleStats))
	mux.HandleFunc("GET /api/v1/audit", s.requireAuth(RoleAdmin, s.handleAudit))

	// --- Citizen web portal pages (each reads any dynamic path segment
	// client-side via location.pathname; no server-side templating) ---
	mux.HandleFunc("GET /verify", s.serveWebFile("verify.html"))
	mux.HandleFunc("GET /v/{work_id}", s.serveWebFile("verify.html"))
	mux.HandleFunc("GET /report", s.serveWebFile("report.html"))
	mux.HandleFunc("GET /track", s.serveWebFile("track.html"))
	mux.HandleFunc("GET /e/{public_id}", s.serveWebFile("evidence.html"))
	mux.HandleFunc("GET /admin", s.serveWebFile("admin.html"))

	// --- Static assets + citizen homepage ---
	mux.Handle("/", http.FileServer(http.Dir(s.webDir)))

	loginLimiter := httpx.NewRateLimiter(0.2, 5) // ~1 attempt / 5s sustained, burst 5
	publicLimiter := httpx.NewRateLimiter(2, 20) // generous but bounded

	var handler http.Handler = mux
	handler = s.rateLimitPaths(handler, loginLimiter, publicLimiter)
	handler = httpx.Timeout(s.cfg.RequestTimeout)(handler)
	handler = corsMiddleware(handler)
	handler = logging.Middleware(s.logger)(handler)
	return handler
}

func (s *Server) rateLimitPaths(next http.Handler, loginLimiter, publicLimiter *httpx.RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := httpx.ClientIP(r)
		switch {
		case r.URL.Path == "/api/v1/auth/login":
			if !loginLimiter.Allow(key) {
				httpx.Error(w, http.StatusTooManyRequests, "too many login attempts, slow down")
				return
			}
		case len(r.URL.Path) >= len("/api/v1/public") && r.URL.Path[:len("/api/v1/public")] == "/api/v1/public":
			if !publicLimiter.Allow(key) {
				httpx.Error(w, http.StatusTooManyRequests, "too many requests, slow down")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// serveWebFile always serves the same static file for a route regardless
// of any dynamic path segment (e.g. the work ID in /v/{work_id}) — the
// page itself reads that segment client-side via location.pathname. This
// keeps the citizen site a set of plain static files with zero
// server-side templating.
func (s *Server) serveWebFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, s.webDir+"/"+name)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if _, err := s.store.GetStats(ctx); err != nil {
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
