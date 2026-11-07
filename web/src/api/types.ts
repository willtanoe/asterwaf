export interface KpiValue {
  value: number;
  previous: number;
  delta_pct: number | null;
  series?: number[];
}

export interface SeriesPoint {
  ts: number;
  requests: number;
  blocked: number;
  shadow: number;
  ratelimited: number;
}

export interface CountItem {
  key: string;
  count: number;
}

export interface IpRow {
  ip: string;
  requests: number;
  blocked: number;
}

export interface PathRow {
  path: string;
  requests: number;
  blocked: number;
}

export interface LogEntry {
  id: number;
  ts: number;
  request_id: string;
  site: string;
  method: string;
  path: string;
  query: string;
  status: number;
  action: string;
  category: string;
  rule_id: string;
  shadow: boolean;
  client_ip: string;
  user_agent: string;
  duration_ms: number;
  request_headers?: string;
  request_body?: string;
  matched?: string;
  seed: boolean;
}

export interface OverviewResponse {
  rate_limit_enabled: boolean;
  range: {
    from: string;
    to: string;
    bucket: string;
    approximate: boolean;
    notes: string[];
  };
  kpis: Record<string, KpiValue>;
  series: SeriesPoint[];
  breakdowns: {
    categories: CountItem[];
    rules: CountItem[];
    actions: CountItem[];
    sites: CountItem[];
  };
  tables: {
    top_ips: IpRow[];
    top_paths: PathRow[];
    latest_blocked: LogEntry[];
  };
}

export interface SiteHealth {
  host: string;
  status: string;
  latency_ms: number;
  failures: number;
  last_check: number;
}

export interface StatusResponse {
  mode: string;
  ruleset_version: number;
  uptime_seconds: number;
  dropped_logs: number;
  sites: SiteHealth[];
}

export interface MeResponse {
  username: string;
  expires_at: string;
}

export interface LoginResponse {
  username: string;
  expires_at: string;
}

export interface SiteView {
  host: string;
  upstream: string;
  mode: string;
  health_path: string;
  max_body_bytes: number;
  status: string;
  latency_ms: number;
  failures: number;
  last_check: number;
  skip_stats: boolean;
}

export interface LogsResponse {
  items: LogEntry[];
  next_cursor?: string;
}

export interface Rule {
  id: string;
  name: string;
  category: string;
  pattern: string;
  enabled: boolean;
  score: number;
  source: string;
  defaults_version: number;
  position: number;
  created_at: number;
  updated_at: number;
}

export interface RulesResponse {
  rules: Rule[];
  version: number;
}

export interface RuleMatch {
  rule_id: string;
  category: string;
  name: string;
  match: string;
}

export interface RuleTestResponse {
  matched: boolean;
  rule_id?: string;
  category?: string;
  matches?: RuleMatch[];
}

export interface RuleVersion {
  id: number;
  created_at: number;
  actor: string;
  note: string;
  rule_count: number;
}

export interface AccessEntry {
  id: string;
  action: "allow" | "deny";
  address: string;
  note?: string;
  enabled: boolean;
  source_log_id?: number;
  created_at?: number;
}

export interface AccessList {
  allow_only: boolean;
  entries: AccessEntry[];
}

export interface RateLimitConfig {
  enabled: boolean;
  requests_per_minute: number;
  burst: number;
}

export interface ApiErrorBody {
  error: { code: string; message: string };
}
