# Architecture

## System at a glance

```
                     ┌───────────────────────────┐
  Feature phone ───► │  USSD Aggregator (telco)   │
  (dial *384*X#)     │  Africa's Talking / MNO    │
                     └──────────────┬────────────┘
                                    │ HTTP POST (form-encoded, retried on timeout)
                                    ▼
                    ┌──────────────────────────────────┐
                    │        Go binary: ussdgov          │
                    │                                    │
                    │  cmd/server/main.go (wiring)       │
                    │                                    │
                    │  ┌──────────────┐  ┌─────────────┐ │
   Browser ───────► │  │  Admin REST   │  │  USSD engine │ │
   (Vue3 portal)    │  │  API (JWT)    │  │  (menu tree) │ │
                     │  └──────┬───────┘  └──────┬──────┘ │
                     │         │                 │        │
                     │         ▼                 ▼        │
                     │  ┌────────────────────────────┐    │
                     │  │   internal/store (repos)    │    │
                     │  └──────────────┬─────────────┘    │
                     └─────────────────┼──────────────────┘
                                       ▼
                          SQLite (WAL, single file, on disk)
                          reports.db  ← back this up, that's the state
```

Everything is one process. One binary, one database file. This is
deliberate (see MANIFESTO.md §5) — it is the architecture that a county
government IT office can actually operate without a dedicated platform
team.

## Layering (dependency direction points inward)

```
cmd/server            → wires everything together, owns lifecycle
internal/api           → HTTP handlers, DTOs, routing            (depends on domain, store, auth)
internal/ussd          → USSD menu engine, session state machine  (depends on domain, store, i18n)
internal/auth           → JWT issuing/verification, password hashing
internal/store           → repository interfaces + sqlite implementation (depends on domain only)
internal/domain          → plain structs & enums, zero dependencies
internal/idempotency     → dedupe layer for retried USSD callbacks
internal/logging         → structured slog setup, request-ID middleware
internal/httpx           → shared HTTP helpers: JSON responses, retry/backoff, rate limiter
internal/config          → env-based configuration with validated defaults
```

`domain` is the center. Nothing in `domain` imports anything else in this
repo. `store` defines repository *interfaces*; `api` and `ussd` depend on
those interfaces, never on the concrete sqlite package directly — this is
what lets the whole engine be tested with an in-memory fake and would let a
future Postgres implementation drop in without touching business logic.

## Request lifecycle: a USSD report, end to end

1. Aggregator POSTs `sessionId`, `phoneNumber`, `text` to `/ussd`.
2. `internal/api` recovery + request-ID + logging middleware wraps the call.
3. `internal/idempotency` checks whether this exact `(sessionId, text)` pair
   was already processed. If yes, the previously computed response is
   replayed verbatim — the aggregator's retry is invisible to the citizen
   and to business state. If no, it proceeds and records the key inside the
   *same database transaction* as the resulting state change, so a crash
   between "processed" and "recorded" cannot happen.
4. `internal/ussd.Engine` loads (or creates) the session's position in the
   menu tree from the `ussd_sessions` table — session state is persisted,
   not held in an in-process map, so a server restart mid-session does not
   strand the citizen (they resume, they aren't kicked back to the start).
5. The engine walks the declarative menu tree (`internal/ussd/menu.go`),
   applying the citizen's input, and either returns a `CON` (continue) or
   `END` (terminal) prompt, translated via `internal/ussd/i18n`.
6. Terminal actions that mutate state (submitting a report, nothing else
   does) go through `internal/store` inside a transaction: write the report,
   write the audit log entry, write the idempotency record, commit once.
7. Response is written back to the aggregator as `text/plain`.

## Reliability mechanisms (see MANIFESTO.md for the why)

| Concern | Mechanism |
|---|---|
| Aggregator retries the same step | Idempotency table keyed on `(session_id, text)`, response replay |
| Server restarts mid-USSD-session | Session state persisted to `ussd_sessions`, not in-memory |
| SQLite lock contention | WAL mode, `busy_timeout`, bounded retry-with-jitter on `SQLITE_BUSY` |
| Panic in a handler | Recovery middleware turns it into a translated `END` / JSON 500, never a crash |
| Slow/hung request | Per-request context timeout, applied at the router |
| Silent failure | Structured `slog` JSON logs with request ID on every request; `/healthz`, `/readyz` |
| Brute-forced admin login | Password hashing with bcrypt, login rate limiting, JWT short expiry + refresh |
| Lost audit trail | Append-only `audit_log` table; report/official mutations are never hard-deleted |
| Graceful deploys | `cmd/server` listens for SIGTERM/SIGINT, drains in-flight requests, closes DB cleanly |

## Data model (see `internal/domain` for exact structs)

- **officials** — id, name, position, department, work_id, status (verified /
  unverified / under_investigation), created_at, updated_at
- **reports** — id (public, e.g. `RPT-2026-000123`), phone_number_hash,
  phone_number_last4, description, official_id (nullable), status
  (pending / under_review / resolved / dismissed), language, created_at,
  updated_at
- **report_status_history** — report_id, from_status, to_status, actor,
  reason, created_at (append-only audit trail for a single report)
- **audit_log** — generic append-only event log for admin actions
- **admin_users** — id, email, password_hash, role (admin / officer /
  viewer), created_at
- **ussd_sessions** — session_id, phone_number, cursor (menu path),
  language, created_at, updated_at, expires_at
- **idempotency_keys** — session_id, text, response_body, created_at

Phone numbers are stored hashed (SHA-256 + pepper) for lookups plus the last
4 digits in the clear for display — full numbers never render in the admin
UI. This is a policy decision from MANIFESTO.md §6, not an afterthought.

## The admin portal

Every page under `web/` is a static file, Vue 3 loaded from
`web/assets/vue.global.prod.js` — **vendored into the repo, not fetched
from a CDN at runtime.** No build step, no npm install, no bundler for the
app itself. The Go server serves it directly from `/`. It talks to the
JSON API under `/api/v1/*` with a bearer JWT stored in `localStorage`.
This keeps the deployment story identical to the backend: one binary plus
one directory, `scp` them, run it.

Vendoring Vue instead of loading it from unpkg/jsdelivr at runtime is a
deliberate reliability decision, not a style preference: a live UAT run
against this system inside a network with a restrictive egress allowlist
(exactly the kind a government IT department runs) demonstrated that a
CDN-loaded script fails silently in the browser — the page loads, but
nothing on it works, because `Vue` is never defined and no error surfaces
to an end user. A perfectly healthy Go backend behind a broken frontend is
still a broken system. Re-vendor with `npm pack vue@<version>` and copy
`package/dist/vue.global.prod.js` over the existing file if it ever needs
updating; never re-add a `<script src="https://...">` CDN reference.

## Anti-impersonation defense in depth

A work ID is a number. Numbers are memorizable, overhearable, and cheap for
a scammer to reuse. We never rely on "the ID matched" alone to mean "this
person is who they say they are." Four layers, cheapest and most universal
first:

1. **Photo match** — every official has an admin-uploaded reference photo.
   The public verification page (`/v/{work_id}`) always shows it. A citizen
   compares the face on screen to the face in front of them. This alone
   defeats the most common case: someone who only knows or overheard a
   real officer's ID.
2. **Verification-spike detection** — `verification_events` logs every
   lookup (work_id, source, hashed requester, timestamp). If a single work
   ID is checked more than `VERIFY_ANOMALY_THRESHOLD` times by more than 3
   distinct requesters within `VERIFY_ANOMALY_WINDOW` (defaults: 5 checks /
   3 requesters / 15 minutes), the verification response includes a
   `high_scrutiny: true` flag and the admin dashboard surfaces it — that
   pattern means an ID is actively being used in a scam run somewhere right
   now, not that one citizen is being careful.
3. **One-tap impersonation reporting** — the verification page has a
   "this doesn't match — report it" action that immediately opens a report
   against that official (`report_type = impersonation`), so a failed
   verification becomes a case, not a shrug.
4. **Signed, expiring ID-card tokens** — `internal/idcard` issues an
   HMAC-signed token (`work_id|expiry|signature`, base64) an admin can
   embed as a QR code on a printed physical ID card. The verification page
   accepts an optional `?t=` token and reports whether it's a currently
   valid, unexpired card — a photocopied or long-expired card stops
   validating even though the underlying work ID and photo are unchanged,
   forcing periodic reissue rather than one-time cloning.

**Phase 2 (documented, not built — needs a live SMS gateway account we
don't have credentials for in this environment):** a challenge-response
flow where the verification request also SMS's a one-time code to the
official's own registered phone, and the citizen asks the person in front
of them to read it back. The `internal/notify` package defines the
`SMSSender` interface this would plug into; today it ships with a
log-only implementation so the seam exists without a live dependency.

## Deployment

`Dockerfile` produces a ~15MB static binary image (CGO disabled — the
sqlite driver is pure Go, so no libc dependency at all, and the image can
be `FROM scratch`). `docker-compose.yml` mounts a volume for the database
file. A single `systemd` unit file works equally well for a bare VPS; see
README.md.
