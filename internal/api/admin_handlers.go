package api

import (
	"net/http"
	"strconv"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
)

func (s *Server) handleListAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListAdminUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not list users")
		return
	}
	for i := range users {
		users[i].PasswordHash = ""
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": users})
}

type createAdminUserRequest struct {
	Email    string           `json:"email"`
	Name     string           `json:"name"`
	Password string           `json:"password"`
	Role     domain.AdminRole `json:"role"`
}

func (s *Server) handleCreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req createAdminUserRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || len(req.Password) < 10 {
		httpx.Error(w, http.StatusBadRequest, "email is required and password must be at least 10 characters")
		return
	}
	validRoles := map[domain.AdminRole]bool{domain.RoleAdmin: true, domain.RoleOfficer: true, domain.RoleViewer: true}
	if !validRoles[req.Role] {
		req.Role = domain.RoleOfficer
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	u := &domain.AdminUser{Email: req.Email, Name: req.Name, PasswordHash: hash, Role: req.Role}
	if err := s.store.CreateAdminUser(r.Context(), u); err != nil {
		respondStoreErr(w, err, "admin user")
		return
	}
	claims, _ := claimsFromContext(r.Context())
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{Actor: claims.Email, Action: "admin_user_created", Target: "admin_users:" + u.Email})

	u.PasswordHash = ""
	httpx.JSON(w, http.StatusCreated, u)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetStats(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not load stats")
		return
	}
	httpx.JSON(w, http.StatusOK, stats)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.ListAudit(r.Context(), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not load audit log")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"events": events})
}
