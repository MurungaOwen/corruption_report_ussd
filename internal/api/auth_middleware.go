package api

import (
	"context"
	"net/http"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/httpx"
)

// Role ranks — a numeric rank lets requireAuth express "this role or
// higher" with a single comparison instead of an allow-list per route.
type Role = domain.AdminRole

const (
	RoleViewer  = domain.RoleViewer
	RoleOfficer = domain.RoleOfficer
	RoleAdmin   = domain.RoleAdmin
)

func rank(r Role) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleOfficer:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

type claimsCtxKey struct{}

func claimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(*auth.Claims)
	return c, ok
}

// requireAuth wraps a handler so it only runs for a valid, unexpired JWT
// whose role meets or exceeds minRole. It never trusts a role claim it
// hasn't itself verified the signature of first.
func (s *Server) requireAuth(minRole Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr, ok := auth.BearerToken(r)
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := s.tokens.Verify(tokenStr)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		if rank(claims.Role) < rank(minRole) {
			httpx.Error(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		ctx := context.WithValue(r.Context(), claimsCtxKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
