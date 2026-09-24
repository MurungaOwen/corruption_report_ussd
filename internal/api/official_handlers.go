package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
)

// officialView is the admin-portal JSON shape for an official: everything
// domain.Official has, plus a browsable photo_url in place of the raw
// on-disk PhotoPath (which is intentionally excluded from Official's own
// JSON tags since most contexts shouldn't leak filesystem layout).
type officialView struct {
	domain.Official
	PhotoURL string `json:"photo_url,omitempty"`
}

func (s *Server) toOfficialView(o domain.Official) officialView {
	return officialView{Official: o, PhotoURL: s.photoURL(o.PhotoPath)}
}

func (s *Server) handleListOfficials(w http.ResponseWriter, r *http.Request) {
	officials, err := s.store.ListOfficials(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not list officials")
		return
	}
	views := make([]officialView, len(officials))
	for i, o := range officials {
		views[i] = s.toOfficialView(o)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"officials": views})
}

type createOfficialRequest struct {
	WorkID     string `json:"work_id"`
	Name       string `json:"name"`
	Position   string `json:"position"`
	Department string `json:"department"`
}

func (s *Server) handleCreateOfficial(w http.ResponseWriter, r *http.Request) {
	var req createOfficialRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkID == "" || req.Name == "" || req.Position == "" {
		httpx.Error(w, http.StatusBadRequest, "work_id, name, and position are required")
		return
	}
	o := &domain.Official{
		WorkID: req.WorkID, Name: req.Name, Position: req.Position, Department: req.Department,
		Status: domain.OfficialUnverified,
	}
	if err := s.store.CreateOfficial(r.Context(), o); err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	claims, _ := claimsFromContext(r.Context())
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{Actor: claims.Email, Action: "official_created", Target: "official:" + o.WorkID})
	httpx.JSON(w, http.StatusCreated, s.toOfficialView(*o))
}

type updateOfficialStatusRequest struct {
	Status domain.OfficialStatus `json:"status"`
}

func (s *Server) handleUpdateOfficialStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid official id")
		return
	}
	var req updateOfficialStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	valid := map[domain.OfficialStatus]bool{
		domain.OfficialVerified: true, domain.OfficialUnverified: true, domain.OfficialInvestigation: true,
	}
	if !valid[req.Status] {
		httpx.Error(w, http.StatusBadRequest, "invalid status value")
		return
	}
	if err := s.store.UpdateOfficialStatus(r.Context(), id, req.Status); err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	claims, _ := claimsFromContext(r.Context())
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{
		Actor: claims.Email, Action: "official_status_change", Target: "official:" + strconv.FormatInt(id, 10), Detail: string(req.Status),
	})
	o, err := s.store.GetOfficialByID(r.Context(), id)
	if err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	httpx.JSON(w, http.StatusOK, s.toOfficialView(*o))
}

func (s *Server) handleUploadOfficialPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid official id")
		return
	}
	official, err := s.store.GetOfficialByID(r.Context(), id)
	if err != nil {
		respondStoreErr(w, err, "official")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes + 1<<20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "could not parse upload")
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "photo file is required")
		return
	}
	file.Close()

	relPath, _, _, err := s.saveUploadedImage("officials", header)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SetOfficialPhoto(r.Context(), id, relPath); err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	claims, _ := claimsFromContext(r.Context())
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{Actor: claims.Email, Action: "official_photo_uploaded", Target: "official:" + official.WorkID})

	httpx.JSON(w, http.StatusOK, map[string]string{"photo_url": s.photoURL(relPath)})
}

type issueCardTokenRequest struct {
	TTLDays int `json:"ttl_days"`
}

func (s *Server) handleIssueCardToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid official id")
		return
	}
	official, err := s.store.GetOfficialByID(r.Context(), id)
	if err != nil {
		respondStoreErr(w, err, "official")
		return
	}
	var req issueCardTokenRequest
	_ = httpx.DecodeJSON(r, &req) // optional body
	if req.TTLDays < 0 {
		httpx.Error(w, http.StatusBadRequest, "ttl_days must not be negative")
		return
	}

	ttl := s.cfg.IDCardTTL
	if req.TTLDays > 0 {
		ttl = time.Duration(req.TTLDays) * 24 * time.Hour
	}
	token := s.cards.Issue(official.WorkID, ttl)
	verifyURL := s.cfg.PublicBaseURL + "/v/" + official.WorkID + "?t=" + token

	claims, _ := claimsFromContext(r.Context())
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{Actor: claims.Email, Action: "idcard_token_issued", Target: "official:" + official.WorkID})

	httpx.JSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"verify_url": verifyURL,
		"expires_at": time.Now().Add(ttl).Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (s *Server) photoURL(relPath string) string {
	if relPath == "" {
		return ""
	}
	return s.cfg.PublicBaseURL + "/uploads/" + relPath
}
