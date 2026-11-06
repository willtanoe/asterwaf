import type {
  AccessList,
  AccessEntry,
  LoginResponse,
  LogsResponse,
  MeResponse,
  OverviewResponse,
  RateLimitConfig,
  Rule,
  RulesResponse,
  RuleVersion,
  SiteView,
  StatusResponse,
} from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: "same-origin",
    headers: {},
  };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }
  const response = await fetch(path, init);
  const text = await response.text();
  let data: unknown = undefined;
  if (text !== "") {
    try {
      data = JSON.parse(text);
    } catch {
      data = undefined;
    }
  }
  if (!response.ok) {
    const shaped = data as { error?: { code?: string; message?: string } } | undefined;
    throw new ApiError(
      response.status,
      shaped?.error?.code ?? "internal",
      shaped?.error?.message ?? `Request failed with status ${response.status}.`,
    );
  }
  return data as T;
}

export const api = {
  me: () => request<MeResponse>("GET", "/api/me"),
  login: (username: string, password: string) =>
    request<LoginResponse>("POST", "/api/login", { username, password }),
  logout: () => request<void>("POST", "/api/logout"),
  status: () => request<StatusResponse>("GET", "/api/status"),
  overview: (range: string, site: string) => {
    const params = new URLSearchParams({ range });
    if (site !== "") {
      params.set("site", site);
    }
    return request<OverviewResponse>("GET", `/api/stats/overview?${params.toString()}`);
  },
  sites: () => request<{ sites: SiteView[] }>("GET", "/api/sites"),
  logs: (query: string) => request<LogsResponse>("GET", `/api/logs?${query}`),
  log: (id: number) => request<import("./types").LogEntry>("GET", `/api/logs/${id}`),
  rules: () => request<RulesResponse>("GET", "/api/rules"),
  createRule: (rule: Partial<Rule>) => request<{ rule: Rule; version: number }>("POST", "/api/rules", rule),
  updateRule: (id: string, rule: Partial<Rule>) =>
    request<{ rule: Rule; version: number }>("PUT", `/api/rules/${encodeURIComponent(id)}`, rule),
  deleteRule: (id: string) => request<void>("DELETE", `/api/rules/${encodeURIComponent(id)}`),
  toggleRule: (id: string) =>
    request<{ rule: Rule; version: number }>("POST", `/api/rules/${encodeURIComponent(id)}/toggle`),
  testRule: (rule: Partial<Rule>, payload: string) =>
    request<{ matched: boolean; rule_id?: string; category?: string }>("POST", "/api/rules/test", {
      rule,
      payload,
    }),
  ruleVersions: () => request<{ versions: RuleVersion[] }>("GET", "/api/rules/versions"),
  rollbackRuleVersion: (id: number) =>
    request<{ version: number; rules: Rule[] }>("POST", `/api/rules/versions/${id}/rollback`),
  accessLists: () => request<AccessList>("GET", "/api/access-lists"),
  createAccessEntry: (entry: Partial<AccessEntry>) =>
    request<AccessEntry>("POST", "/api/access-lists", entry),
  deleteAccessEntry: (id: string) =>
    request<void>("DELETE", `/api/access-lists/${encodeURIComponent(id)}`),
  rateLimit: () => request<RateLimitConfig>("GET", "/api/rate-limit"),
  saveRateLimit: (config: RateLimitConfig) =>
    request<RateLimitConfig>("PUT", "/api/rate-limit", config),
  saveSiteMode: (host: string, mode: string) =>
    request<{ host: string; mode: string }>("PUT", `/api/sites/${encodeURIComponent(host)}/mode`, { mode }),
};
