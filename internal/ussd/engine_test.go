package ussd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store/sqlite"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	s, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	cfg := config.Config{
		PhonePepper:   "test-pepper",
		PublicBaseURL: "https://ussd.test",
		SessionTTL:    3 * time.Minute,
	}
	official := &domain.Official{
		WorkID: "OFF001", Name: "Jane Doe", Position: "Inspector", Department: "Traffic",
		Status: domain.OfficialVerified,
	}
	if err := s.CreateOfficial(context.Background(), official); err != nil {
		t.Fatalf("seed official: %v", err)
	}

	return NewEngine(s, cfg)
}

func TestLanguagePrompt(t *testing.T) {
	e := newTestEngine(t)
	resp, err := e.Handle(context.Background(), "sess1", "0700000000", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp, "CON") {
		t.Errorf("expected CON prefix, got %q", resp)
	}
}

func TestVerifyOfficialFlow(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	resp, _ := e.Handle(ctx, "sess2", "0700000001", "1")
	if !strings.Contains(resp, "Anti-Corruption") {
		t.Errorf("expected main menu, got %q", resp)
	}

	resp, _ = e.Handle(ctx, "sess2", "0700000001", "1*1")
	if !strings.Contains(resp, "work/badge ID") {
		t.Errorf("expected ask work id, got %q", resp)
	}

	resp, err := e.Handle(ctx, "sess2", "0700000001", "1*1*OFF001")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp, "END") || !strings.Contains(resp, "Jane Doe") || !strings.Contains(resp, "VERIFIED") {
		t.Errorf("expected verified official response, got %q", resp)
	}

	resp, _ = e.Handle(ctx, "sess3", "0700000002", "1*1*NOPE")
	if !strings.Contains(resp, "No official found") {
		t.Errorf("expected not-found response, got %q", resp)
	}
}

func TestReportAndTrackFlow(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	resp, err := e.Handle(ctx, "sess4", "0711000000", "1*2*Bribery at the local office, involving badge 001*with extra*stars")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp, "END") || !strings.Contains(resp, "report ID is RPT-") {
		t.Fatalf("expected report-filed response, got %q", resp)
	}

	// Extract the public ID out of the response to track it.
	idx := strings.Index(resp, "RPT-")
	end := strings.IndexAny(resp[idx:], " \n")
	publicID := resp[idx : idx+end]

	resp, err = e.Handle(ctx, "sess5", "0711000001", "1*3*"+publicID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "Pending review") {
		t.Errorf("expected pending status, got %q", resp)
	}

	resp, _ = e.Handle(ctx, "sess6", "0711000002", "1*3*RPT-0000-NOPE")
	if !strings.Contains(resp, "No report found") {
		t.Errorf("expected not-found response, got %q", resp)
	}
}

func TestEmptyDescriptionRejected(t *testing.T) {
	e := newTestEngine(t)
	resp, err := e.Handle(context.Background(), "sess7", "0722000000", "1*2*")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "description is required") {
		t.Errorf("expected description-required response, got %q", resp)
	}
}

func TestHighScrutinyNoteSurfacesOverUSSD(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	// Simulate a spike: many distinct "callers" checking the same work ID
	// within the anomaly window. Config in newTestEngine leaves
	// VerifyAnomalyThreshold/Window at their zero values, so set sane ones.
	e.cfg.VerifyAnomalyThreshold = 3
	e.cfg.VerifyAnomalyWindow = 15 * time.Minute

	for i := 0; i < 4; i++ {
		phoneN := "07000000" + string(rune('0'+i))
		_, err := e.Handle(ctx, "spike"+string(rune('0'+i)), phoneN, "1*1*OFF001")
		if err != nil {
			t.Fatal(err)
		}
	}

	resp, err := e.Handle(ctx, "spikeFinal", "0799999999", "1*1*OFF001")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp, "END CAUTION") {
		t.Errorf("expected high-scrutiny caution prefix over plain USSD text, got %q", resp)
	}
}

func TestInvalidLanguageDigit(t *testing.T) {
	e := newTestEngine(t)
	resp, err := e.Handle(context.Background(), "sess8", "0733000000", "9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "Invalid choice") {
		t.Errorf("expected invalid choice, got %q", resp)
	}
}
