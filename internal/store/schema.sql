CREATE TABLE IF NOT EXISTS hosts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    domain          TEXT NOT NULL UNIQUE,
    upstream        TEXT NOT NULL,
    request_headers TEXT NOT NULL DEFAULT '{}',
    group_label     TEXT NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS host_access_rules (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id    INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('basic_auth', 'ip_allow', 'ip_deny')),
    -- basic_auth: JSON {"username":"...","bcrypt_hash":"..."}
    -- ip_allow / ip_deny: one CIDR or bare IP per row
    value      TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_host_access_rules_host_id ON host_access_rules(host_id);
