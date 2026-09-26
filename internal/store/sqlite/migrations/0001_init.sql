-- Initial schema. SQLite, WAL mode (set at connection time, not here).
--
-- Design notes:
--  * Timestamps are stored as TEXT in RFC3339 (UTC) — SQLite has no native
--    datetime type, and RFC3339 text sorts and compares correctly as-is.
--  * Foreign keys are enabled at connection time (PRAGMA foreign_keys=ON).
--  * Nothing here is ever hard-deleted by the application; status columns
--    and the *_history / audit_log tables are the source of truth for what
--    happened over time (see MANIFESTO.md §3).

CREATE TABLE IF NOT EXISTS officials (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id     TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    position    TEXT NOT NULL,
    department  TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'unverified',
    photo_path  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_officials_status ON officials(status);

CREATE TABLE IF NOT EXISTS reports (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    public_id             TEXT NOT NULL UNIQUE,
    phone_hash            TEXT NOT NULL,
    phone_last4           TEXT NOT NULL,
    description           TEXT NOT NULL,
    official_id           INTEGER REFERENCES officials(id),
    status                TEXT NOT NULL DEFAULT 'pending',
    language              TEXT NOT NULL DEFAULT 'english',
    report_type           TEXT NOT NULL DEFAULT 'corruption',
    channel               TEXT NOT NULL DEFAULT 'ussd',
    idempotency_key       TEXT NOT NULL DEFAULT '',
    evidence_code_hash    TEXT NOT NULL DEFAULT '',
    evidence_code_expires TEXT NOT NULL DEFAULT '',
    evidence_count        INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT NOT NULL,
    updated_at            TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_phone_hash ON reports(phone_hash);
CREATE INDEX IF NOT EXISTS idx_reports_created_at ON reports(created_at);
-- Partial unique index: only non-empty idempotency keys are deduped, so
-- reports without one (e.g. web submissions with no Idempotency-Key
-- header) never collide with each other.
CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_idempotency ON reports(idempotency_key) WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS report_status_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id   INTEGER NOT NULL REFERENCES reports(id),
    from_status TEXT NOT NULL,
    to_status   TEXT NOT NULL,
    actor       TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_status_history_report ON report_status_history(report_id);

CREATE TABLE IF NOT EXISTS report_evidence (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id    INTEGER NOT NULL REFERENCES reports(id),
    file_path    TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evidence_report ON report_evidence(report_id);

CREATE TABLE IF NOT EXISTS admin_users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'officer',
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ussd_sessions (
    session_id  TEXT PRIMARY KEY,
    phone       TEXT NOT NULL,
    language    TEXT NOT NULL DEFAULT '',
    cursor      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON ussd_sessions(expires_at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    session_id     TEXT NOT NULL,
    text           TEXT NOT NULL,
    response_body  TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    PRIMARY KEY (session_id, text)
);

CREATE TABLE IF NOT EXISTS verification_events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    work_id        TEXT NOT NULL,
    source         TEXT NOT NULL,
    requester_key  TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_verification_workid_time ON verification_events(work_id, created_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    target      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
