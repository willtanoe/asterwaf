CREATE TABLE IF NOT EXISTS rollup_minute (
    minute INTEGER NOT NULL,
    site TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    rule_id TEXT NOT NULL DEFAULT '',
    requests INTEGER NOT NULL DEFAULT 0,
    latency_sum_ms INTEGER NOT NULL DEFAULT 0,
    b0 INTEGER NOT NULL DEFAULT 0,
    b1 INTEGER NOT NULL DEFAULT 0,
    b2 INTEGER NOT NULL DEFAULT 0,
    b3 INTEGER NOT NULL DEFAULT 0,
    b4 INTEGER NOT NULL DEFAULT 0,
    b5 INTEGER NOT NULL DEFAULT 0,
    b6 INTEGER NOT NULL DEFAULT 0,
    b7 INTEGER NOT NULL DEFAULT 0,
    b8 INTEGER NOT NULL DEFAULT 0,
    b9 INTEGER NOT NULL DEFAULT 0,
    b10 INTEGER NOT NULL DEFAULT 0,
    b11 INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (minute, site, action, category, rule_id)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS rollup_hour_ip (
    hour INTEGER NOT NULL,
    site TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0,
    blocked INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (hour, site, client_ip)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS rollup_hour_path (
    hour INTEGER NOT NULL,
    site TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0,
    blocked INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (hour, site, path)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts INTEGER NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    before TEXT NOT NULL DEFAULT '',
    after TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts DESC);
CREATE INDEX IF NOT EXISTS idx_rollup_minute_minute ON rollup_minute(minute);
CREATE INDEX IF NOT EXISTS idx_rollup_hour_ip_hour ON rollup_hour_ip(hour);
CREATE INDEX IF NOT EXISTS idx_rollup_hour_path_hour ON rollup_hour_path(hour);
