# The Manifesto

This system exists so an ordinary citizen, on a KES 1,000 feature phone with
no data bundle, can report a corrupt official at 2am and trust that the
report survives — survives a dropped session, a retried gateway callback, a
restarted server, a bad network day. That is the whole job. Everything below
exists in service of that one guarantee.

## 1. A report, once accepted, is never lost.

The moment we say "END Report received, your ID is X", that report is on
disk, in a transaction, before the byte leaves the process. Not queued in
memory. Not "will be flushed eventually." If the process dies the instant
after writing the HTTP response, the report still exists on restart.

## 2. The network lies. Design for it.

USSD aggregators (Africa's Talking, Safaricom, etc.) retry on timeout. The
same `sessionId` + `text` can arrive twice, three times, out of order. A
system that isn't idempotent under retry will double-report, double-charge,
or double-anything. Every mutating USSD step is keyed and deduplicated
before it touches business state. This is not an edge case we handle — it
is the normal operating condition we design for from line one.

## 3. Nothing is ever really deleted.

Corruption reports are evidence. Official status changes are decisions with
consequences. Every state transition is appended to an audit trail with an
actor, a timestamp, and a reason. We soft-delete, we don't hard-delete. A
government auditor six months from now should be able to reconstruct
exactly what happened to any report or any official record.

## 4. Fail loud to operators, fail soft to citizens.

A citizen on USSD never sees a stack trace, a 500, or "internal server
error." They see a calm, translated message and, where possible, a path
forward. Meanwhile the operator dashboard, logs, and health checks scream
immediately when something is actually wrong. We never trade citizen-facing
clarity for developer convenience.

## 5. Boring technology, used correctly, beats clever technology.

One static Go binary. One embedded SQLite database file (WAL mode) that can
be backed up with `cp`. No message queue, no microservices, no Kubernetes
required to run this in a sub-county office with a single VPS. Horizontal
scale is a later problem for a later success. Reliability on day one is not
negotiable.

## 6. Every screen respects the person on the other end.

Officials are public servants whose verification status is sensitive.
Reporters may be at real personal risk. Phone numbers are never displayed
in full to anyone but the case owner. Reports are pseudonymous by default,
identifiable only through a report ID the citizen controls.

## 7. The admin portal is a tool for civil servants, not a toy for developers.

Fast on a 3-year-old office laptop over a weak LAN. Legible at a glance from
across a desk. No auto-playing anything, no dark patterns, no infinite
scroll that hides how many reports are actually open. If a case officer
can't find "reports older than 7 days with no action" in two clicks, the
portal has failed its purpose.

## 8. Observable or it didn't happen.

Every request gets a request ID. Every state change gets a log line with
enough context to answer "what happened and why" without a debugger.
`/healthz` and `/readyz` exist so this can sit behind a load balancer or a
systemd watchdog and be restarted automatically when it's unwell — and stay
running when it's well, even under load, even under attack.
