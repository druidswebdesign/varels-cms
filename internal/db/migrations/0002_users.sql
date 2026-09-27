-- +goose Up

-- Auth: Google OAuth whitelist plus a break-glass local admin (ADR-0004).
-- Only two roles log in: admin and staff (ADR-0018). Clients are customers and
-- providers are suppliers; neither authenticates.

CREATE TABLE approved_users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE,
    display_name  TEXT,
    role          TEXT    NOT NULL CHECK (role IN ('admin','staff')),
    provider      TEXT    NOT NULL DEFAULT 'google' CHECK (provider IN ('google','local')),
    password_hash TEXT,
    is_active     INTEGER NOT NULL DEFAULT 1,
    invited_by    INTEGER REFERENCES approved_users(id),
    last_login_at TEXT,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    CHECK (email = lower(email)),
    CHECK (provider <> 'local' OR password_hash IS NOT NULL)
);

CREATE TABLE sessions (
    token      TEXT    PRIMARY KEY,
    user_id    INTEGER REFERENCES approved_users(id),
    data       BLOB,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Default settings (settings.updated_by -> approved_users, created above).
INSERT INTO settings (key, value) VALUES
    ('currency', 'ARS'),
    ('timezone', 'America/Argentina/Buenos_Aires'),
    ('iva_rate_bps', '2100'),
    ('deadstock_stale_days', '60');
-- +goose Down

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS approved_users;
