package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/logging"
	"github.com/murungaowen/corruption_report_ussd/internal/store/sqlite"
)

func newTestServer(t *testing.T) (*httptest.Server, func(email, password string) string) {
	t.Helper()
	dir := t.TempDir()
	s, err := sqlite.Open(context.Background(), dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	cfg := config.Config{
		JWTSecret: "test-secret-at-least-16-bytes", PhonePepper: "test-pepper",
		TokenTTL: time.Hour, RequestTimeout: 5 * time.Second, PublicBaseURL: "http://test.local",
		MaxUploadBytes: 5 << 20, VerifyAnomalyThreshold: 5, VerifyAnomalyWindow: 15 * time.Minute,
	}
	logger := logging.New("development")

	// admin fixture
	hash, _ := auth.HashPassword("supersecretpassword")
	if err := s.CreateAdminUser(context.Background(), &domain.AdminUser{Email: "admin@test.local", PasswordHash: hash, Role: domain.RoleAdmin}); err != nil {
		t.Fatal(err)
	}

	srv := New(cfg, s, logger, t.TempDir()) // empty webDir is fine, no static files needed
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	login := func(email, password string) string {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		res, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("login failed: %d", res.StatusCode)
		}
		var out struct{ Token string }
		json.NewDecoder(res.Body).Decode(&out)
		return out.Token
	}

	return ts, login
}

func TestLoginAndListReports(t *testing.T) {
	ts, login := newTestServer(t)
	token := login("admin@test.local", "supersecretpassword")
	if token == "" {
		t.Fatal("expected a token")
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/reports", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ts, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "admin@test.local", "password": "wrong-password"})
	res, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestReportsRequireAuth(t *testing.T) {
	ts, _ := newTestServer(t)
	res, err := http.Get(ts.URL + "/api/v1/reports")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", res.StatusCode)
	}
}

func TestViewerCannotCreateOfficial(t *testing.T) {
	ts, login := newTestServer(t)
	adminToken := login("admin@test.local", "supersecretpassword")

	// Create a viewer-role user as admin, then confirm they're blocked from
	// admin-only actions like registering an official.
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/admin-users", strings.NewReader(`{"email":"viewer@test.local","password":"viewerpassword1","role":"viewer"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	if _, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	viewerToken := login("viewer@test.local", "viewerpassword1")

	req2, _ := http.NewRequest("POST", ts.URL+"/api/v1/officials", strings.NewReader(`{"work_id":"X-1","name":"A","position":"B"}`))
	req2.Header.Set("Authorization", "Bearer "+viewerToken)
	req2.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer creating an official, got %d", res.StatusCode)
	}
}

func TestUSSDEndToEndThroughHTTP(t *testing.T) {
	ts, _ := newTestServer(t)

	form := url.Values{"sessionId": {"http-sess-1"}, "phoneNumber": {"254700111222"}, "text": {""}}
	res, err := http.PostForm(ts.URL+"/ussd", form)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	res.Body.Close()
	if !strings.HasPrefix(buf.String(), "CON") {
		t.Fatalf("expected CON language prompt, got %q", buf.String())
	}
}
