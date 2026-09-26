package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
	"github.com/murungaowen/corruption_report_ussd/internal/idcard"
	"github.com/murungaowen/corruption_report_ussd/internal/phone"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

// handlePublicGetOfficial is the "verify an official" endpoint behind
// /v/{work_id} on the citizen site. It is the primary defense against
// impersonation (ARCHITECTURE.md layer 1: photo match) and it logs the
// lookup (layer 2: spike detection) and evaluates any ID-card token
// (layer 4) presented via ?t=.
func (s *Server) handlePublicGetOfficial(w http.ResponseWriter, r *http.Request) {
	workID := r.PathValue("work_id")
	if workID == "" {
		httpx.Error(w, http.StatusBadRequest, "work_id is required")
		return
	}

	official, err := s.store.GetOfficialByWorkID(r.Context(), workID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "no official found with that work ID")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	requesterKey := phone.Hash(s.cfg.PhonePepper, httpx.ClientIP(r, s.cfg.TrustProxyHeaders))
	_ = s.store.RecordVerification(r.Context(), domain.VerificationEvent{
		WorkID: workID, Source: domain.VerifyViaWeb, RequesterKey: requesterKey,
	})
	count, distinct, _ := s.store.CountRecentVerifications(r.Context(), workID, s.cfg.VerifyAnomalyWindow)
	highScrutiny := count >= s.cfg.VerifyAnomalyThreshold && distinct >= 3

	view := domain.PublicOfficial{
		WorkID: official.WorkID, Name: official.Name, Position: official.Position,
		Department: official.Department, Status: official.Status,
		PhotoURL: s.photoURL(official.PhotoPath), HighScrutiny: highScrutiny,
	}

	if token := r.URL.Query().Get("t"); token != "" {
		expired, expiresAt, err := s.cards.Verify(token, official.WorkID)
		valid := err == nil && !expired
		view.CardValid = &valid
		if err == nil {
			view.CardExpires = &expiresAt
		}
		_ = idcard.ErrMalformed // referenced only for doc linkage; real handling is the err check above
	}

	httpx.JSON(w, http.StatusOK, view)
}

type impersonationReportRequest struct {
	Phone       string `json:"phone"`
	Description string `json:"description"`
}

// handlePublicImpersonationReport is the "this doesn't match — report it"
// one-tap action on the verification page (ARCHITECTURE.md layer 3).
func (s *Server) handlePublicImpersonationReport(w http.ResponseWriter, r *http.Request) {
	workID := r.PathValue("work_id")
	official, err := s.store.GetOfficialByWorkID(r.Context(), workID)
	if err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	var req impersonationReportRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = "Citizen reported that the person presenting this work ID did not match the registered photo."
	}

	rpt := &domain.Report{
		PhoneHash:   phone.Hash(s.cfg.PhonePepper, req.Phone),
		PhoneLast4:  phone.Last4(req.Phone),
		Description: description,
		OfficialID:  &official.ID,
		Language:    "en",
		ReportType:  domain.ReportTypeImpersonation,
		Channel:     "web",
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		rpt.IdempotencyKey = "impersonation:" + key
	}
	if _, err := s.store.CreateReport(r.Context(), rpt, s.cfg.EvidenceWindow); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not file report")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"report_id": rpt.PublicID})
}

func (s *Server) handlePublicReportStatus(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("public_id")
	report, err := s.store.GetReportByPublicID(r.Context(), publicID)
	if err != nil {
		respondStoreErr(w, err, "report")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"public_id":  report.PublicID,
		"status":     report.Status,
		"created_at": report.CreatedAt,
	})
}

// handlePublicCreateReport is the full web reporting form: a description,
// an optional official work ID, an optional phone number, and photo
// evidence attached directly in the same request — the two-step
// code-and-link dance exists only to bridge USSD (text-only) reports to
// photo evidence after the fact; a browser session can just attach files
// immediately.
func (s *Server) handlePublicCreateReport(w http.ResponseWriter, r *http.Request) {
	maxBody := s.cfg.MaxUploadBytes*int64(domain.MaxEvidencePerReport) + 1<<20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(maxBody); err != nil {
		httpx.Error(w, http.StatusBadRequest, "could not parse form (is it too large?)")
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		httpx.Error(w, http.StatusBadRequest, "description is required")
		return
	}
	if len(description) > 4000 {
		description = description[:4000]
	}
	phoneNumber := strings.TrimSpace(r.FormValue("phone"))
	workID := strings.TrimSpace(r.FormValue("official_work_id"))

	rpt := &domain.Report{
		PhoneHash:   phone.Hash(s.cfg.PhonePepper, phoneNumber),
		PhoneLast4:  phone.Last4(phoneNumber),
		Description: description,
		Language:    "en",
		ReportType:  domain.ReportTypeCorruption,
		Channel:     "web",
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		rpt.IdempotencyKey = "web-report:" + key
	}
	if workID != "" {
		if official, err := s.store.GetOfficialByWorkID(r.Context(), workID); err == nil {
			rpt.OfficialID = &official.ID
		}
	}

	code, err := s.store.CreateReport(r.Context(), rpt, s.cfg.EvidenceWindow)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not file report")
		return
	}

	files := r.MultipartForm.File["files"]
	attached := 0
	for _, fh := range files {
		if attached >= domain.MaxEvidencePerReport {
			break
		}
		relPath, contentType, size, err := s.saveUploadedImage("evidence/"+rpt.PublicID, fh)
		if err != nil {
			continue // skip a bad file rather than failing the whole report
		}
		ev := &domain.ReportEvidence{FilePath: relPath, ContentType: contentType, SizeBytes: size}
		if err := s.store.AddEvidenceDirect(r.Context(), rpt.ID, ev); err != nil {
			continue
		}
		attached++
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"report_id":         rpt.PublicID,
		"evidence_code":     code,
		"evidence_upload":   s.cfg.PublicBaseURL + "/e/" + rpt.PublicID,
		"evidence_attached": attached,
	})
}

type uploadEvidenceResponse struct {
	Attached  int `json:"attached"`
	Remaining int `json:"remaining"`
}

func (s *Server) handlePublicUploadEvidence(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("public_id")

	maxBody := s.cfg.MaxUploadBytes*int64(domain.MaxEvidencePerReport) + 1<<20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(maxBody); err != nil {
		httpx.Error(w, http.StatusBadRequest, "could not parse form (is it too large?)")
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		httpx.Error(w, http.StatusBadRequest, "at least one file is required")
		return
	}

	attached := 0
	var lastErr error
	for _, fh := range files {
		relPath, contentType, size, err := s.saveUploadedImage("evidence/"+publicID, fh)
		if err != nil {
			lastErr = err
			continue
		}
		ev := &domain.ReportEvidence{FilePath: relPath, ContentType: contentType, SizeBytes: size}
		if err := s.store.AddEvidence(r.Context(), publicID, code, ev); err != nil {
			lastErr = err
			continue
		}
		attached++
	}

	if attached == 0 {
		if errors.Is(lastErr, store.ErrInvalidEvidence) {
			httpx.Error(w, http.StatusForbidden, "invalid or expired code")
			return
		}
		if errors.Is(lastErr, store.ErrEvidenceLimit) {
			httpx.Error(w, http.StatusConflict, "evidence limit reached for this report")
			return
		}
		if errors.Is(lastErr, store.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "report not found")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "no files could be attached (check file type/size)")
		return
	}

	httpx.JSON(w, http.StatusOK, uploadEvidenceResponse{
		Attached:  attached,
		Remaining: domain.MaxEvidencePerReport - attached,
	})
}

// handlePublicOfficialPhoto serves an official's reference photo. This is
// the one upload directory that's intentionally public — the whole point
// of the photo is to be shown to any citizen doing a verification check.
func (s *Server) handlePublicOfficialPhoto(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path) // strips any path traversal attempt
	fullPath := filepath.Join(s.cfg.UploadsDir, "officials", name)
	f, err := os.Open(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), f)
}
