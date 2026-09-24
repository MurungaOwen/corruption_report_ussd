package api

import (
	"errors"
	"net/http"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
	"github.com/murungaowen/corruption_report_ussd/internal/store"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string           `json:"token"`
	ExpiresAt string           `json:"expires_at"`
	User      domain.AdminUser `json:"user"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.store.GetAdminUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Deliberately identical to the wrong-password error: never let
			// a login response reveal whether an email is registered.
			httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "login failed")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, expiresAt, err := s.tokens.Issue(*user)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	user.PasswordHash = ""
	_ = s.store.RecordAudit(r.Context(), domain.AuditEvent{Actor: user.Email, Action: "login", Target: "admin_users:" + user.Email})

	httpx.JSON(w, http.StatusOK, loginResponse{Token: token, ExpiresAt: expiresAt.Format("2006-01-02T15:04:05Z07:00"), User: *user})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user_id": claims.UserID,
		"email":   claims.Email,
		"role":    claims.Role,
	})
}
