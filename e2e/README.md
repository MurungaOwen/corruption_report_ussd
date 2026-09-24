# UAT suite (Playwright)

Real-browser user-acceptance tests against the actual running Go server —
not mocked, not curl-only. They exercise the citizen journeys (verify an
official, report corruption, track a report, the USSD evidence bridge)
and the staff journeys (login, role-based access control, report triage,
officials management) end to end, the way a person actually would: click
buttons, fill forms, read what's on screen.

This caught real bugs that unit tests couldn't: a CDN-loaded script
silently failing under a restrictive network policy (so the whole UI
looked fine but nothing worked), unassociated `<label>`/`<input>` pairs
that broke both screen readers and test selectors the same way, and a
login rate limit tuned tight enough to lock out a legitimate sequence of
staff logins. See `../ARCHITECTURE.md` for how those were fixed.

## Running it

```bash
make e2e   # from the repo root — builds, seeds fixtures, runs the suite, tears down
```

Or manually, if you want the server running in one terminal to poke at
with a real browser too:

```bash
# terminal 1
DB_PATH=/tmp/e2e-data/reports.db UPLOADS_DIR=/tmp/e2e-data/uploads \
  ADDR=:8097 PUBLIC_BASE_URL=http://localhost:8097 ENVIRONMENT=development \
  go run ./cmd/server

# terminal 2 — seed deterministic fixtures (fixed accounts, fixed officials)
DB_PATH=/tmp/e2e-data/reports.db UPLOADS_DIR=/tmp/e2e-data/uploads go run ./cmd/e2eseed

# terminal 2 — run the suite
cd e2e && npm install && E2E_BASE_URL=http://localhost:8097 npx playwright test
```

Fixed credentials/work IDs used by the suite live in `fixtures.js`,
mirrored in `cmd/e2eseed/main.go` — keep the two in sync if either changes.

## What's covered

- `tests/citizen-verify.spec.js` — the core anti-impersonation flow: photo
  shown for a verified official, honest no-photo warning, investigation
  status flagged, not-found guidance, deep-link auto-lookup, one-tap
  impersonation reporting.
- `tests/citizen-report-and-track.spec.js` — the web report form
  (description, optional official lookup-as-you-type, optional phone,
  direct photo upload) and tracking a filed report by ID.
- `tests/citizen-evidence-bridge.spec.js` — filing a report through the
  raw USSD HTTP endpoint (as a feature phone would), then attaching
  photos afterward through the web bridge with the one-time code.
- `tests/admin-auth-and-rbac.spec.js` — login success/failure, and that
  each role (viewer/officer/admin) sees exactly the controls the backend
  will actually let them use — not more.
- `tests/admin-reports.spec.js` — dashboard numbers, opening a report
  (history, authenticated evidence-photo viewing), updating its status,
  filtering the table.
- `tests/admin-officials.spec.js` — registering an official, changing
  status, uploading a photo, issuing an ID-card token — each confirmed
  from the citizen-facing verification page too, not just the admin table.

## Notes

- `workers: 1`, `fullyParallel: false` — tests share server-side state
  (the same database), so they run sequentially and each test that needs
  specific data creates it itself via the API rather than assuming
  another spec file ran first.
- `e2e/assets/evidence.png` is a hand-built, fully valid tiny PNG (not
  just bytes that pass a magic-number sniff) — an earlier truncated JPEG
  literal passed the server's `http.DetectContentType` check but wasn't
  actually decodable, so it rendered as a broken image in the browser.
  If you need a different fixture image, export a real one from any image
  tool rather than hand-truncating raw bytes.
