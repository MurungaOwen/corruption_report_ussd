# Kenya Integrity Line

A government-grade system for two things that turn out to be the same
problem: letting citizens **verify that a government official is who they
say they are** before handing over money or documents, and letting them
**report corruption** when someone isn't. Works over USSD on any phone —
no internet, no smartphone, no airtime bundle required beyond the USSD
session itself — and in full on the web, with a staff portal for triage.

This started as a small Flask prototype and has been rebuilt in Go as a
real system: one static binary, one SQLite database file, designed to
survive network retries, server restarts, and the everyday chaos of
running on a modest VPS in a sub-county office — see **MANIFESTO.md** for
the principles and **ARCHITECTURE.md** for how it's actually built.

## Why verification, not just reporting

A citizen being stopped by someone claiming to be a police officer, tax
collector, or county inspector has no fast way to check that claim today.
A badge number is not proof — it can be memorized and reused by anyone.
This system treats verification as a first-class feature, defended in
layers (see **ARCHITECTURE.md → "Anti-impersonation defense in depth"**):

1. **Photo match** — every official has a reference photo; the citizen
   compares it to the person in front of them.
2. **Verification-spike detection** — an ID checked unusually often in a
   short window gets flagged automatically.
3. **One-tap impersonation reporting** — "this doesn't match" becomes a
   case immediately, from the same screen.
4. **Signed, expiring ID-card QR tokens** — a photocopied card stops
   validating after it expires.

## What's in the box

- **USSD engine** (`internal/ussd`) — a bilingual (English/Kiswahili) menu
  system: verify an official, report corruption, track a report.
- **Citizen web portal** (`web/`) — the same three actions with a richer
  UI: photo verification, a full report form with direct photo upload,
  report tracking, and the USSD evidence-upload bridge.
- **Admin/staff portal** (`web/admin.html`) — login, a dashboard, report
  triage with a full audit history, officials management (register,
  verify, upload photos, issue ID-card tokens), an audit log, and staff
  account management, gated by role (viewer / officer / admin).
- **One Go binary, one SQLite file** — see MANIFESTO.md §5 for why.

## Quick start

```bash
cp .env.example .env          # edit JWT_SECRET / PHONE_PEPPER for anything beyond local dev
make run                      # starts on :8080
make seed                     # in another terminal: ~200 demo Kenyan officials
```

Open `http://localhost:8080` for the citizen site, `http://localhost:8080/admin`
for the staff portal. On first run, since there are no admin accounts yet,
the server generates one and prints it once to the logs:

```
level=WARN msg="bootstrap admin account created — log in and create a named
account, then consider disabling this one" email=admin@ussd.local password=...
```

Log in with that, then create a real named admin account from **Staff
Accounts** and treat the bootstrap one as disposable.

### Try the USSD flow without a telco

```bash
curl localhost:8080/ussd -d sessionId=s1 -d phoneNumber=254700111222 -d text=""
curl localhost:8080/ussd -d sessionId=s1 -d phoneNumber=254700111222 -d text="1"
curl localhost:8080/ussd -d sessionId=s1 -d phoneNumber=254700111222 --data-urlencode text="1*2*Officer demanded KES 500 at a roadblock"
```

In production, point your USSD aggregator's callback (Africa's Talking,
a telco's own gateway, etc.) at `POST /ussd`.

### Docker

```bash
docker compose up --build
```

Ships a `scratch`-based image (pure-Go SQLite driver, no CGO, no libc) —
this wasn't built or run inside this development sandbox (no Docker
daemon available here), so build and smoke-test it yourself before relying
on it. `/data` is a named volume; back it up — it's the entire system's
state (database + uploaded photos/evidence).

## Configuration

All configuration is environment variables with safe local-dev defaults;
see `.env.example` for the full list and `internal/config/config.go` for
validation (the server refuses to start in `ENVIRONMENT=production` with
default secrets).

## Development

```bash
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt
make build   # static binaries into ./bin
```

## Project layout

```
cmd/server        entrypoint: wires everything together, owns lifecycle
cmd/seed          demo-data seeding (~200 Kenyan officials across real agencies)
internal/domain   plain data types, zero dependencies
internal/store     repository interfaces + the sqlite implementation
internal/ussd       the USSD menu state machine + bilingual strings
internal/api         HTTP handlers: USSD callback, public JSON API, admin JSON API
internal/auth         JWT + password hashing
internal/idcard        signed, expiring ID-card QR tokens
internal/notify        outbound-SMS seam (log-only today; see ARCHITECTURE.md)
internal/phone          phone number hashing/display policy
internal/httpx           shared HTTP helpers: retry/backoff, rate limiting, timeouts
internal/logging          structured logging + request IDs
internal/config             environment-based configuration
web/                          citizen site + admin SPA (Vue 3 via CDN, no build step)
```

## Documents worth reading before changing anything

- **MANIFESTO.md** — the principles this system won't compromise on.
- **ARCHITECTURE.md** — how it's built, the request lifecycle, the
  reliability mechanisms, and the anti-impersonation design in full.
- **DESIGN_BIBLE.md** — the visual/interaction spec the web portals follow.
