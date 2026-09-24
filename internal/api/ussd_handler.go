package api

import (
	"net/http"

	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
	"github.com/murungaowen/corruption_report_ussd/internal/logging"
)

// handleUSSD is the aggregator callback. Aggregators retry on timeout, so
// the very first thing this does is check whether this exact
// (sessionId, text) pair was already answered — if so, the prior response
// is replayed verbatim and the engine never runs a second time for the
// same keypress. See MANIFESTO.md §2 and ARCHITECTURE.md.
func (s *Server) handleUSSD(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeUSSD(w, "END Invalid request.")
		return
	}
	sessionID := r.FormValue("sessionId")
	phoneNumber := r.FormValue("phoneNumber")
	text := r.FormValue("text")

	if sessionID == "" || phoneNumber == "" {
		writeUSSD(w, "END Invalid request.")
		return
	}

	ctx := r.Context()

	if cached, found, err := s.store.GetIdempotent(ctx, sessionID, text); err == nil && found {
		writeUSSD(w, cached)
		return
	}

	resp, err := s.engine.Handle(ctx, sessionID, phoneNumber, text)
	if err != nil {
		s.logger.Error("ussd engine error", "request_id", logging.RequestID(ctx), "error", err)
		writeUSSD(w, "END Sorry, something went wrong on our end. Please try again shortly.")
		return
	}

	if err := httpx.RetryBusy(ctx, 3, func() error {
		return s.store.PutIdempotent(ctx, sessionID, text, resp)
	}); err != nil {
		s.logger.Warn("failed to persist idempotency key", "request_id", logging.RequestID(ctx), "error", err)
	}

	writeUSSD(w, resp)
}

func writeUSSD(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
