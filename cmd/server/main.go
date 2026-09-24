// Command server runs the whole system: the USSD callback, the public
// citizen JSON API, the authenticated admin JSON API, and the static web
// portal — one process, one binary, one SQLite file. See ARCHITECTURE.md.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/api"
	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/logging"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
	"github.com/murungaowen/corruption_report_ussd/internal/store/sqlite"
)

func main() {
	// Support `server healthcheck`, used by Docker's HEALTHCHECK. The
	// production image is FROM scratch (no shell, no curl/wget), so the
	// binary has to be able to probe itself.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func runHealthcheck() int {
	addr := envOr("ADDR", ":8080")
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost" + addr + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	logger := logging.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if err := os.MkdirAll(cfg.UploadsDir, 0o755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}

	if err := bootstrapAdmin(ctx, s, logger); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	webDir := envOr("WEB_DIR", "web")
	srv := api.New(cfg, s, logger, webDir)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go sweepSessionsPeriodically(ctx, s, cfg, logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", cfg.Addr, "environment", cfg.Environment, "db_path", cfg.DBPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed, forcing close", "error", err)
		return httpServer.Close()
	}
	logger.Info("server stopped cleanly")
	return nil
}

// bootstrapAdmin ensures the system is never left with zero admin users —
// a fresh deployment with no way to log in is a fresh deployment nobody
// can operate. The generated password is printed once, to the process's
// own log output, and never stored anywhere in plaintext.
func bootstrapAdmin(ctx context.Context, s store.Store, logger *slog.Logger) error {
	count, err := s.CountAdminUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	email := envOr("BOOTSTRAP_ADMIN_EMAIL", "admin@ussd.local")
	password, err := randomPassword()
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	u := &domain.AdminUser{Email: email, Name: "Bootstrap Administrator", PasswordHash: hash, Role: domain.RoleAdmin}
	if err := s.CreateAdminUser(ctx, u); err != nil {
		return err
	}
	logger.Warn("bootstrap admin account created — log in and create a named account, then consider disabling this one",
		"email", email, "password", password)
	return nil
}

func randomPassword() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sweepSessionsPeriodically(ctx context.Context, s store.Store, cfg config.Config, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.SweepExpiredSessions(ctx, time.Now())
			if err != nil {
				logger.Warn("session sweep failed", "error", err)
				continue
			}
			if n > 0 {
				logger.Debug("swept expired ussd sessions", "count", n)
			}
		}
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
