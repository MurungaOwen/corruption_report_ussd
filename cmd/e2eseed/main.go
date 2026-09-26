// Command e2eseed provisions deterministic test fixtures — fixed-password
// staff accounts and a handful of officials with known work IDs and
// statuses — so the Playwright UAT suite under e2e/ has stable data to
// assert against. This is test-only tooling: never run it against a real
// database, and never ship these credentials anywhere near production.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/murungaowen/corruption_report_ussd/internal/auth"
	"github.com/murungaowen/corruption_report_ussd/internal/config"
	"github.com/murungaowen/corruption_report_ussd/internal/domain"
	"github.com/murungaowen/corruption_report_ussd/internal/store/sqlite"
)

// Fixed, well-known credentials — test-only. Keep in sync with e2e/fixtures.js.
const (
	AdminEmail      = "e2e-admin@test.local"
	AdminPassword   = "E2eTestAdminPass123!"
	OfficerEmail    = "e2e-officer@test.local"
	OfficerPassword = "E2eTestOfficerPass123!"
	ViewerEmail     = "e2e-viewer@test.local"
	ViewerPassword  = "E2eTestViewerPass123!"

	WorkIDVerifiedWithPhoto = "E2E-VERIFIED-001"
	WorkIDUnverified        = "E2E-UNVERIFIED-002"
	WorkIDInvestigation     = "E2E-INVESTIGATION-003"
)

// A minimal, fully valid 2x2 PNG (not just bytes that pass a magic-number
// sniff — an earlier hand-truncated JPEG literal here passed
// http.DetectContentType's check but wasn't actually decodable, so it
// rendered as a broken image in the browser; caught by the Playwright UAT
// suite in e2e/, not by any Go-level test, since Go's http.DetectContentType
// only reads the leading bytes and never actually decodes the image).
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02,
	0x08, 0x02, 0x00, 0x00, 0x00, 0xfd, 0xd4, 0x9a, 0x73, 0x00, 0x00, 0x00,
	0x0f, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x63, 0x60, 0x70, 0xe8, 0x01,
	0x21, 0x08, 0x05, 0x00, 0x14, 0x2e, 0x03, 0x31, 0x16, 0xd6, 0x49, 0x0d,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	s, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	createUser := func(email, password string, role domain.AdminRole) {
		hash, err := auth.HashPassword(password)
		if err != nil {
			log.Fatalf("hash password: %v", err)
		}
		u := &domain.AdminUser{Email: email, Name: string(role) + " (e2e)", PasswordHash: hash, Role: role}
		if err := s.CreateAdminUser(ctx, u); err != nil {
			fmt.Printf("skip user %s: %v\n", email, err)
		}
	}
	createUser(AdminEmail, AdminPassword, domain.RoleAdmin)
	createUser(OfficerEmail, OfficerPassword, domain.RoleOfficer)
	createUser(ViewerEmail, ViewerPassword, domain.RoleViewer)

	photoDir := cfg.UploadsDir + "/officials"
	if err := os.MkdirAll(photoDir, 0o755); err != nil {
		log.Fatalf("mkdir uploads: %v", err)
	}
	photoRelPath := "officials/e2e-fixture.png"
	if err := os.WriteFile(cfg.UploadsDir+"/"+photoRelPath, tinyPNG, 0o644); err != nil {
		log.Fatalf("write fixture photo: %v", err)
	}

	officials := []struct {
		workID, name, position, department string
		status                             domain.OfficialStatus
		photo                              string
	}{
		{WorkIDVerifiedWithPhoto, "Insp. Jane Wanjiru", "OCS", "National Police Service", domain.OfficialVerified, photoRelPath},
		{WorkIDUnverified, "Peter Otieno", "Revenue Officer", "Kenya Revenue Authority", domain.OfficialUnverified, ""},
		{WorkIDInvestigation, "Mohamed Abdi", "Registration Officer", "Nairobi City County Government", domain.OfficialInvestigation, ""},
	}
	for _, o := range officials {
		official := &domain.Official{WorkID: o.workID, Name: o.name, Position: o.position, Department: o.department, Status: o.status}
		if err := s.CreateOfficial(ctx, official); err != nil {
			fmt.Printf("skip official %s: %v\n", o.workID, err)
			continue
		}
		if o.photo != "" {
			if err := s.SetOfficialPhoto(ctx, official.ID, o.photo); err != nil {
				log.Fatalf("set photo: %v", err)
			}
		}
	}

	fmt.Println("e2e fixtures ready:", cfg.DBPath)
}
