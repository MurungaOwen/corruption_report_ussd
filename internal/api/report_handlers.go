package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

type reportListResponse struct {
	Reports []domain.Report `json:"reports"`
	Total   int             `json:"total"`
}

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	filter := store.ReportFilter{
		Status: q.Get("status"),
		Query:  q.Get("q"),
		Limit:  limit,
		Offset: offset,
	}
	reports, total, err := s.store.ListReports(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not list reports")
		return
	}
	httpx.JSON(w, http.StatusOK, reportListResponse{Reports: reports, Total: total})
}

type reportDetailResponse struct {
	domain.Report
	History  []domain.ReportStatusEvent `json:"history"`
	Evidence []domain.ReportEvidence    `json:"evidence"`
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid report id")
		return
	}
	report, err := s.store.GetReportByID(r.Context(), id)
	if err != nil {
		respondStoreErr(w, err, "report")
		return
	}
	history, err := s.store.ReportHistory(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not load report history")
		return
	}
	evidence, err := s.store.ListEvidence(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not load report evidence")
		return
	}
	httpx.JSON(w, http.StatusOK, reportDetailResponse{Report: *report, History: history, Evidence: evidence})
}

type updateStatusRequest struct {
	Status domain.ReportStatus `json:"status"`
	Reason string              `json:"reason"`
}

func (s *Server) handleUpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid report id")
		return
	}
	var req updateStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !domain.ValidReportStatuses[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status value")
		return
	}
	claims, _ := claimsFromContext(r.Context())

	err = httpx.RetryBusy(r.Context(), 3, func() error {
		return s.store.UpdateReportStatus(r.Context(), id, req.Status, claims.Email, req.Reason)
	})
	if err != nil {
		respondStoreErr(w, err, "report")
		return
	}
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{
		Actor: claims.Email, Action: "report_status_change", Target: "report:" + strconv.FormatInt(id, 10),
		Detail: string(req.Status) + ": " + req.Reason,
	})

	report, err := s.store.GetReportByID(r.Context(), id)
	if err != nil {
		respondStoreErr(w, err, "report")
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}

// handleReportEvidenceFile streams an evidence file to an authenticated
// admin-portal user. Evidence is never served publicly — unlike an
// official's reference photo, which exists specifically to be shown to
// citizens, evidence may contain sensitive material about a reporter or
// an incident.
func (s *Server) handleReportEvidenceFile(w http.ResponseWriter, r *http.Request) {
	reportID, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid report id")
		return
	}
	evidenceID, err := parseIDParam(r, "evidence_id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid evidence id")
		return
	}
	ev, err := s.store.GetEvidenceFile(r.Context(), reportID, evidenceID)
	if err != nil {
		respondStoreErr(w, err, "evidence")
		return
	}
	fullPath := filepath.Join(s.cfg.UploadsDir, ev.FilePath)
	f, err := os.Open(fullPath)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "evidence file missing on disk")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", ev.ContentType)
	http.ServeContent(w, r, filepath.Base(fullPath), ev.CreatedAt, f)
}

func parseIDParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func respondStoreErr(w http.ResponseWriter, err error, noun string) {
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, noun+" not found")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		httpx.Error(w, http.StatusConflict, noun+" already exists")
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal error")
}
