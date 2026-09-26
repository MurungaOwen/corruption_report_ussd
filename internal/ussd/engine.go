// Package ussd implements the USSD menu state machine: a citizen's whole
// interaction is one HTTP round trip per keypress, correlated by
// sessionId, so the "engine" is really a pure function from (persisted
// session state, this keypress) to (new state, response text). See
// ARCHITECTURE.md "Request lifecycle: a USSD report, end to end".
package ussd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/phone"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

// sessionIdempotencyKey derives a deterministic dedup key for a report
// creation request from the aggregator's own retry identity
// (sessionId + the exact accumulated text). It doesn't need to be secret,
// only stable and collision-resistant.
func sessionIdempotencyKey(sessionID, text string) string {
	sum := sha256.Sum256([]byte(sessionID + "|" + text))
	return hex.EncodeToString(sum[:])
}

type Engine struct {
	store store.Store
	cfg   config.Config
}

func NewEngine(s store.Store, cfg config.Config) *Engine {
	return &Engine{store: s, cfg: cfg}
}

// Handle processes one USSD keypress. text is the full accumulated input
// the aggregator sends (e.g. "1*2*Bribery at the DMV"). It never returns a
// Go error for citizen-facing problems — those become a translated END
// message — only for genuine infrastructure failures the caller should log
// and turn into a generic apology.
func (e *Engine) Handle(ctx context.Context, sessionID, rawPhone, text string) (string, error) {
	text = strings.TrimSpace(text)

	if text == "" {
		e.touchSession(ctx, sessionID, rawPhone, "", "")
		return LanguagePrompt, nil
	}

	langDigit := text[:1]
	lang, ok := map[string]string{"1": LangEnglish, "2": LangSwahili}[langDigit]
	if !ok {
		return stringsFor(LangEnglish).InvalidChoice, nil
	}
	s := stringsFor(lang)

	sub := strings.TrimPrefix(text[1:], "*")
	e.touchSession(ctx, sessionID, rawPhone, lang, sub)

	if sub == "" {
		return s.MainMenu, nil
	}

	switch {
	case sub == "1":
		return s.AskWorkID, nil
	case strings.HasPrefix(sub, "1*"):
		workID := strings.TrimSpace(afterN(sub, "*", 2))
		return e.verifyOfficial(ctx, lang, s, workID, rawPhone)

	case sub == "2":
		return s.AskDescription, nil
	case strings.HasPrefix(sub, "2*"):
		description := strings.TrimSpace(afterN(sub, "*", 2))
		return e.fileReport(ctx, lang, s, rawPhone, description, sessionID, text)

	case sub == "3":
		return s.AskReportID, nil
	case strings.HasPrefix(sub, "3*"):
		reportID := strings.TrimSpace(afterN(sub, "*", 2))
		return e.trackReport(ctx, s, reportID)

	default:
		return s.InvalidChoice, nil
	}
}

// afterN returns everything in s after the Nth occurrence-delimited
// prefix, preserving any further separators verbatim — used so free text
// (a description, an ID) that happens to contain the menu separator isn't
// truncated the way a naive strings.Split(s, "*")[n] would truncate it.
func afterN(s, sep string, n int) string {
	parts := strings.SplitN(s, sep, n)
	if len(parts) < n {
		return ""
	}
	return parts[n-1]
}

func (e *Engine) verifyOfficial(ctx context.Context, lang string, s Strings, workID, rawPhone string) (string, error) {
	if workID == "" {
		return s.InvalidChoice, nil
	}
	_ = e.store.RecordVerification(ctx, domain.VerificationEvent{
		WorkID: workID, Source: domain.VerifyViaUSSD, RequesterKey: phone.Hash(e.cfg.PhonePepper, rawPhone),
	})

	official, err := e.store.GetOfficialByWorkID(ctx, workID)
	if err != nil {
		return s.OfficialNotFound, nil
	}

	verifyURL := fmt.Sprintf("%s/v/%s", e.cfg.PublicBaseURL, official.WorkID)
	var msg string
	switch official.Status {
	case domain.OfficialVerified:
		msg = s.OfficialVerified(official.Name, official.Position, official.Department, verifyURL)
	case domain.OfficialInvestigation:
		msg = s.OfficialInvestigation(official.Name, official.Position, official.Department, verifyURL)
	default:
		msg = s.OfficialUnverified(official.Name, official.Position, official.Department, verifyURL)
	}

	// The photo comparison (layer 1 of ARCHITECTURE.md's anti-impersonation
	// defense) can't render over plain USSD text — but the spike-detection
	// signal (layer 2) is just a sentence, so a feature-phone-only citizen
	// with no way to open the verify link still gets this caution.
	count, distinct, err := e.store.CountRecentVerifications(ctx, workID, e.cfg.VerifyAnomalyWindow)
	if err == nil && count >= e.cfg.VerifyAnomalyThreshold && distinct >= 3 {
		// msg is "END <body>" — the caution has to land inside the body,
		// right after the single leading "END" the USSD protocol requires,
		// not before it.
		msg = "END " + s.HighScrutinyNote + "\n" + strings.TrimPrefix(msg, "END ")
	}
	return msg, nil
}

func (e *Engine) fileReport(ctx context.Context, lang string, s Strings, rawPhone, description, sessionID, text string) (string, error) {
	if description == "" {
		return s.DescriptionRequired, nil
	}
	if len(description) > 640 {
		description = description[:640]
	}

	r := &domain.Report{
		PhoneHash:      phone.Hash(e.cfg.PhonePepper, rawPhone),
		PhoneLast4:     phone.Last4(rawPhone),
		Description:    description,
		Language:       lang,
		ReportType:     domain.ReportTypeCorruption,
		Channel:        "ussd",
		IdempotencyKey: sessionIdempotencyKey(sessionID, text),
	}
	code, err := e.store.CreateReport(ctx, r, e.cfg.EvidenceWindow)
	if err != nil {
		return "", err
	}
	evidenceURL := fmt.Sprintf("%s/e/%s", e.cfg.PublicBaseURL, r.PublicID)
	return s.ReportFiled(r.PublicID, evidenceURL, code), nil
}

func (e *Engine) trackReport(ctx context.Context, s Strings, publicID string) (string, error) {
	if publicID == "" {
		return s.InvalidChoice, nil
	}
	r, err := e.store.GetReportByPublicID(ctx, publicID)
	if err != nil {
		return s.ReportNotFound, nil
	}
	label := s.StatusLabel[string(r.Status)]
	if label == "" {
		label = string(r.Status)
	}
	return s.ReportStatus(r.PublicID, label), nil
}

// touchSession persists session position best-effort; a failure here
// should never block a citizen's response, only cost them resumability if
// the server restarts at exactly the wrong instant, so errors are
// swallowed rather than surfaced.
func (e *Engine) touchSession(ctx context.Context, sessionID, rawPhone, lang, cursor string) {
	_ = e.store.SaveSession(ctx, &domain.USSDSession{
		SessionID: sessionID,
		Phone:     rawPhone,
		Language:  lang,
		Cursor:    cursor,
		ExpiresAt: time.Now().Add(e.cfg.SessionTTL),
	})
}
