CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts INTEGER NOT NULL,
    request_id TEXT NOT NULL,
    site TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    query TEXT NOT NULL DEFAULT '',
    status INTEGER NOT NULL,
    action TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    rule_id TEXT NOT NULL DEFAULT '',
    shadow INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL,
    request_headers TEXT NOT NULL DEFAULT '{}',
    request_body TEXT NOT NULL DEFAULT '',
    matched TEXT NOT NULL DEFAULT '',
    seed INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_site_ts ON logs(site, ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_action_ts ON logs(action, ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_rule_ts ON logs(rule_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_ip_ts ON logs(client_ip, ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_live_ts ON logs(ts DESC) WHERE seed = 0;
