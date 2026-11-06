CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    pattern TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    score INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'custom' CHECK (source IN ('default', 'custom')),
    defaults_version INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ruleset_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at INTEGER NOT NULL,
    actor TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    rules_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS access_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    allow_only INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS access_entries (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL CHECK (action IN ('allow', 'deny')),
    address TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    source_log_id INTEGER,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS rate_limit_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0,
    requests_per_minute INTEGER NOT NULL,
    burst INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS site_modes (
    host TEXT PRIMARY KEY,
    mode TEXT NOT NULL CHECK (mode IN ('off', 'shadow', 'enforce')),
    updated_at INTEGER NOT NULL
);
