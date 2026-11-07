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

export interface CountPoint {
  ts: number;
  count: number;
}

export interface DetectionSeries {
  key: string;
  points: CountPoint[];
}

export interface StatusSeries {
  key: string;
  points: CountPoint[];
}

export interface BytePoint {
  ts: number;
  in: number;
  out: number;
}

export interface ShadowRow {
  site: string;
  count: number;
}

export interface PerSiteRow {
  site: string;
  requests: number;
  blocked: number;
  block_rate: number;
  p95: number;
  series?: number[];
}

export interface TopRuleRow {
  rule_id: string;
  category: string;
  hits: number;
  blocked: number;
  recorded: number;
  last_seen: number;
}

export interface HeatmapPoint {
  dow: number;
  hour: number;
  count: number;
}

export interface SystemInfo {
  db_size_bytes: number;
  retention_days: Record<string, number>;
  rate_limit: { enabled: boolean; requests_per_minute: number; burst: number; buckets: number };
}

export interface OverviewResponse {
  rate_limit_enabled: boolean;
  detections_series: DetectionSeries[];
  latency_series: { ts: number; p50: number; p95: number }[];
  status_series: StatusSeries[] | null;
  methods: CountItem[] | null;
  bytes_series: BytePoint[] | null;
  shadow_wouldblock: ShadowRow[];
  per_site: PerSiteRow[];
  top_rules: TopRuleRow[];
  top_user_agents: CountItem[] | null;
  heatmap: HeatmapPoint[] | null;
  raw_available: boolean;
  system: SystemInfo;
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

export interface LogSummary {
  rows: number;
  blocked: number;
  recorded: number;
  unique_ips: number;
  first_ts: number;
  last_ts: number;
  series: { ts: number; rows: number; blocked: number }[];
}

export interface RuleStatsRow {
  rule_id: string;
  category: string;
  hits: number;
  blocked: number;
  recorded: number;
  last_seen: number;
  series: number[];
}

export interface RuleStatsResponse {
  range: { from: string; to: string; bucket: string; approximate: boolean; notes: string[] };
  raw_available: boolean;
  total_rules: number;
  rules: RuleStatsRow[];
}

export interface AccessStatsResponse {
  range: { from: string; to: string; bucket: string; approximate: boolean; notes: string[] };
  entries: { rule_id: string; hits: number; last_hit: number }[];
  denied_requests: number;
  ratelimited_series: { ts: number; count: number }[];
  ratelimited_total: number;
  top_offenders: { ip: string; requests: number }[];
  rate_limit_enabled: boolean;
  rate_limit_buckets: number;
  allow_count: number;
  deny_count: number;
}

export interface ProbeSample {
  ts: number;
  status: string;
  latency_ms: number;
}

export interface ModeChange {
  ts: number;
  actor: string;
  before: string;
  after: string;
}

export interface SiteDetailResponse {
  host: string;
  health_history: ProbeSample[];
  mode_history: ModeChange[];
  recent_blocked: LogEntry[];
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
