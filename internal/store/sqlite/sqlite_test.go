package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateReportIdempotency(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	r1 := &domain.Report{Description: "bribery", PhoneHash: "h1", PhoneLast4: "1234", Channel: "ussd", IdempotencyKey: "key-abc"}
	code1, err := s.CreateReport(ctx, r1, 72*time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	r2 := &domain.Report{Description: "bribery", PhoneHash: "h1", PhoneLast4: "1234", Channel: "ussd", IdempotencyKey: "key-abc"}
	code2, err := s.CreateReport(ctx, r2, 72*time.Hour)
	if err != nil {
		t.Fatalf("create retry: %v", err)
	}

	if r1.PublicID != r2.PublicID {
		t.Errorf("expected the same report to be reused on retry, got %s and %s", r1.PublicID, r2.PublicID)
	}
	if code1 == code2 {
		t.Errorf("expected a fresh evidence code on retry (old one should stop working)")
	}

	_, total, err := s.ListReports(ctx, store.ReportFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected exactly one report row despite the retry, got %d", total)
	}
}

func TestEvidenceCodeExpiryAndLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	r := &domain.Report{Description: "test", PhoneHash: "h2", PhoneLast4: "5678", Channel: "ussd"}
	code, err := s.CreateReport(ctx, r, 72*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.AddEvidence(ctx, r.PublicID, "wrong-code", &domain.ReportEvidence{FilePath: "x.jpg", ContentType: "image/jpeg", SizeBytes: 100}); err != store.ErrInvalidEvidence {
		t.Errorf("expected ErrInvalidEvidence for wrong code, got %v", err)
	}

	for i := 0; i < domain.MaxEvidencePerReport; i++ {
		if err := s.AddEvidence(ctx, r.PublicID, code, &domain.ReportEvidence{FilePath: "x.jpg", ContentType: "image/jpeg", SizeBytes: 100}); err != nil {
			t.Fatalf("evidence %d: %v", i, err)
		}
	}
	if err := s.AddEvidence(ctx, r.PublicID, code, &domain.ReportEvidence{FilePath: "x.jpg", ContentType: "image/jpeg", SizeBytes: 100}); err != store.ErrEvidenceLimit {
		t.Errorf("expected ErrEvidenceLimit once the cap is reached, got %v", err)
	}
}

func TestCreateReportRespectsEvidenceWindow(t *testing.T) {
	// Regression test: CreateReport used to hardcode a 72h evidence-code
	// expiry, silently ignoring the caller-supplied window (which in turn
	// meant the documented EVIDENCE_WINDOW env var had no effect at all).
	// Proof: a report created with a 1ms window must have an already-
	// expired code by the time we try to use it.
	s := newTestStore(t)
	ctx := context.Background()

	r := &domain.Report{Description: "test", PhoneHash: "h9", PhoneLast4: "1111", Channel: "ussd"}
	code, err := s.CreateReport(ctx, r, 1*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	err = s.AddEvidence(ctx, r.PublicID, code, &domain.ReportEvidence{FilePath: "x.jpg", ContentType: "image/jpeg", SizeBytes: 10})
	if err != store.ErrInvalidEvidence {
		t.Fatalf("expected the code to already be expired under a 1ms window, got %v", err)
	}

	// Sanity check the other direction: a generous window must still work.
	r2 := &domain.Report{Description: "test2", PhoneHash: "h11", PhoneLast4: "3333", Channel: "ussd"}
	code2, err := s.CreateReport(ctx, r2, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddEvidence(ctx, r2.PublicID, code2, &domain.ReportEvidence{FilePath: "y.jpg", ContentType: "image/jpeg", SizeBytes: 10}); err != nil {
		t.Fatalf("expected evidence to be accepted within a 24h window: %v", err)
	}
}

func TestOfficialUniqueWorkID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	o1 := &domain.Official{WorkID: "DUP-1", Name: "A", Position: "P"}
	if err := s.CreateOfficial(ctx, o1); err != nil {
		t.Fatal(err)
	}
	o2 := &domain.Official{WorkID: "DUP-1", Name: "B", Position: "P"}
	if err := s.CreateOfficial(ctx, o2); err != store.ErrConflict {
		t.Errorf("expected ErrConflict for duplicate work_id, got %v", err)
	}
}

func TestSessionPersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	s1, err := Open(ctx, dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	err = s1.SaveSession(ctx, &domain.USSDSession{SessionID: "sess-x", Phone: "0700000000", Language: "en", Cursor: "2", ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	s1.Close()

	// Reopening the same file simulates a server restart: the session must
	// still be there so a citizen mid-flow isn't stranded.
	s2, err := Open(ctx, dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	sess, err := s2.GetSession(ctx, "sess-x")
	if err != nil {
		t.Fatalf("expected session to survive restart: %v", err)
	}
	if sess.Cursor != "2" {
		t.Errorf("expected cursor '2', got %q", sess.Cursor)
	}
}

func TestReportStatusHistoryAppendOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	r := &domain.Report{Description: "test", PhoneHash: "h3", PhoneLast4: "9999", Channel: "web"}
	if _, err := s.CreateReport(ctx, r, 72*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateReportStatus(ctx, r.ID, domain.ReportUnderReview, "officer@gov.ke", "started looking into it"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateReportStatus(ctx, r.ID, domain.ReportResolved, "officer@gov.ke", "confirmed and closed"); err != nil {
		t.Fatal(err)
	}

	history, err := s.ReportHistory(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 { // filed, under_review, resolved
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[2].ToStatus != domain.ReportResolved || history[2].FromStatus != domain.ReportUnderReview {
		t.Errorf("unexpected final transition: %+v", history[2])
	}
}

func TestVerificationAnomalyCounting(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := s.RecordVerification(ctx, domain.VerificationEvent{
			WorkID: "NPS-1", Source: domain.VerifyViaWeb, RequesterKey: "requester-" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	count, distinct, err := s.CountRecentVerifications(ctx, "NPS-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 || distinct != 5 {
		t.Errorf("expected count=5 distinct=5, got count=%d distinct=%d", count, distinct)
	}

	count, _, err = s.CountRecentVerifications(ctx, "NPS-1", -time.Hour) // window in the past: nothing should match
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 with a negative window, got %d", count)
	}
}
