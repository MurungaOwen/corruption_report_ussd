// Package notify defines the outbound-SMS seam the system is built around
// but does not activate: Phase 2 of the anti-impersonation design
// (ARCHITECTURE.md) calls for texting a one-time challenge code to an
// official's own registered phone so a citizen can ask the person in front
// of them to read it back. That requires a live SMS gateway account this
// environment has no credentials for, so today only a log-only
// implementation ships. Swapping in a real gateway (Africa's Talking,
// Twilio, etc.) means writing one small adapter against this interface —
// nothing else in the codebase needs to change.
package notify

import (
	"context"
	"log/slog"
)

type SMSSender interface {
	Send(ctx context.Context, toPhone, message string) error
}

// LogSMSSender "sends" by logging, so the rest of the system can depend on
// SMSSender today without a live gateway. It always succeeds.
type LogSMSSender struct {
	Logger *slog.Logger
}

func (l LogSMSSender) Send(ctx context.Context, toPhone, message string) error {
	l.Logger.Info("sms suppressed (no gateway configured)", "to", toPhone, "message", message)
	return nil
}
