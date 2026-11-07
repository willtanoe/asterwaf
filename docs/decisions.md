# Decisions and technical debt

This file records choices that constrain later work. Each entry is dated and, when it is debt, names the consequence.

## 2026-11-06 Module and toolchain

- Module path is `git.asteriaxlabs.com/willtanoe/asterwaf`; the remote is SSH only.
- Go 1.27.3 and Node 26.13.0 are pinned by URL and SHA256 in `toolchain.env`. Bootstrap verifies the archive before extraction and never resolves "latest".
- Toolchain archives and caches stay under `.toolchain/`, which is gitignored. The system Go is used only to run nothing at all; all builds use the pinned binary through `GOTOOLCHAIN=local`.

## 2026-11-06 Storage engine

- SQLite driver is `modernc.org/sqlite` to keep the binary CGO-free for the release build. Race tests still use CGO because the race detector requires it.
- WAL, `busy_timeout=5000`, `synchronous=NORMAL`. The async log writer is the only writer of request logs; API mutations use short transactions.

## 2026-11-06 Admin sessions

- Sessions live only in memory with a 12-hour absolute expiry. Restarting asterwaf signs every administrator out. A persistent session table would add a new failure mode (token theft at rest) for a single-user tool.
- Login throttling is in memory and therefore also resets on restart; the 15-minute window is short enough that this is acceptable.

## 2026-11-06 Proxy timeouts

- The listener uses fixed read/write timeouts. Large uploads and streaming responses cannot override them per site yet. Sites that need long-lived streams will time out at 60 seconds. This is recorded debt; per-site timeout overrides are out of scope for this build.

## 2026-11-06 Logs and inspection latency

- `duration_ms` measures inspection time only, not upstream latency. The UI labels it "Inspection latency" so it is not mistaken for request duration.
- Rollup percentiles are histogram estimates from 12 buckets, not exact quantiles. The API marks them `approximate`.

## 2026-11-07 Raw-log statistics windows

- The statistics v2 queries split into two tiers. Rollup-backed dimensions
  (requests, actions, categories, rules, per-site aggregates, latency
  percentiles from the b0-b11 histograms) serve every window up to 7 days
  from rollup_minute.
- Raw-log dimensions (status classes, methods, user agents, bytes in/out)
  scan the logs table. They are served only for windows up to 24 hours and
  the overview API returns raw_available=false with a literal note beyond
  that. Recording per-hour status-class and method rollups would require a
  migration of the rollup writer plus storage growth for every request; if
  7-day throughput or status-class views become necessary, extend
  rollup_hour with those columns then.
- All new queries are covered by Test概览扩展查询走索引, which fails on any
  plan that scans logs or rollup_minute without an index.

## 2026-11-07 scanner.path false-positive review

- Source: production logs filtered by `rule_id=scanner.path` (214 hits, 209
  distinct paths, window covering the last days of traffic on this host).
- Every hit came from bot probing: 203 `.env` variants (`/.env.backup`,
  `/config/.env`, `/api/v1/.env`, ...), VCS metadata (`/.git/config`,
  `/.git/HEAD`, `/.svn/entries`, `/.svn/wc.db`, `/.svn/format`), and
  `/wp-admin/phpinfo.php`. Three source IPs; one (34.97.248.170) accounts for
  205 hits.
- All hits landed on the dashboard site (`waf.asteriaxlabs.com`, shadow mode),
  so nothing was blocked and no legitimate request was affected. The 200
  responses are the SPA catch-all serving unknown paths; the 401s are probes
  that reached the admin API.
- False-positive candidates from the expected list — `wp-admin` login posts,
  `robots.txt`, `/.well-known/*` — did not occur in this window. No rule
  change is made. Patterns to watch if they ever appear: `/.well-known/*`
  (ACME HTTP-01 and security.txt traffic is legitimate; nginx serves the ACME
  location before the WAF on this host, but a site without such a location
  would expose it), plain `/wp-admin/` navigation, and `robots.txt`.
- Action: keep the rule as-is; revisit only if a legitimate path shows up in
  future logs.

## 2026-11-06 Seed data

- `make seed` writes `data/asterwaf-seed.db` with `seed=1` rows. The server warns loudly if it is started against a seed database. Synthetic data is deterministic from a fixed seed.

## 2026-11-06 Rule defaults and Stage A

- Default mode is `enforce`. The built-in rules are deliberately conservative: they target unambiguous syntax such as `union select`, `<script`, `../`, and known shell words rather than broad heuristics.
- Rule order is priority. The first enabled rule that matches any inspected target wins; shadow mode also stops at the first match.
- Stage A's real target, OWASP Juice Shop, is not available from npm (ETARGET for the pinned version). The gate ran against a committed 40 + 40 corpus through the actual proxy handler, not against Juice Shop. This is recorded as a known gap.

## 2026-11-06 Access list precedence

- Deny entries always win, even if the same address also matches an allow entry.
- An allow match skips the access list gate and rate limiting but never the rule engine.
- `allow_only` is a gate, not an implicit deny list: it only applies when no allow or deny entry matched.

## 2026-11-06 Rate limiting

- Buckets are in memory and reset on restart; this is acceptable for a single-node proxy and keeps the hot path lock-only.
- IPv6 clients share a bucket per /64. Per-address buckets would be bypassed by rotating the low 64 bits.
- The table is bounded at 100,000 entries; the least recently used bucket is evicted before insertion. A one-minute sweep removes buckets idle for ten minutes.

## 2026-11-06 Async request logging

- The request path only formats and enqueues a record into a 4096-slot channel. A single goroutine writes batches of up to 256 rows, one transaction per batch, at most one second apart. A full queue drops the row and increments `Dropped()`.
- `duration_ms` is captured before the request is handed to the reverse proxy, so it measures inspection only. The UI labels it "Inspection latency".
- Request headers and bodies are persisted only when a rule matched or the action is not `allow`; both copies are redacted and bounded (8 KiB headers, 2 KiB body). Blocked requests that never reached body inspection store empty bodies.
- `X-AsterWAF-Request-ID` is generated before routing and re-applied on `WriteHeader`, so an upstream cannot override it.

## 2026-11-06 Statistics rollups

- Minute rollups keep 12 fixed latency buckets (1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500 ms, overflow). p50/p95 are estimated from those buckets and must be labelled approximate.
- Log insert and all three rollup upserts share one transaction, so a dropped row never reaches aggregates.
- Retention runs hourly: raw logs 14 days, minute rollups 30 days, hourly rollups 180 days, audit log 365 days. Raw and audit deletes are batched 1000 rows at a time.
- `asterwaf seed` writes a separate `data/asterwaf-seed.db` with about 29,000 deterministic rows over 14 days. Timestamps are anchored to the run time, so the shape is reproducible but the absolute times move. The server warns loudly when seed rows are present.

## 2026-11-06 Admin authentication

- The admin listener is separate and defaults to `127.0.0.1:9090`. Only `GET /healthz` is unauthenticated.
- With an empty `auth.password_hash`, the first start generates a 20-character password in `first-run-admin-password` next to the database (mode 0600). Only the path is printed, to stderr; the password and hash never enter slog.
- Login attempts are limited to 5 per IP and 10 per username in a 15-minute fixed window. Counters are in memory and reset on success.
- Session tokens are 32 random bytes in an HttpOnly, SameSite=Strict cookie with a 12-hour absolute expiry. Restarts invalidate all sessions.
- Write requests with an `Origin` header must match the request Host. Requests without Origin are allowed for scripted clients.
- Health probes request each site's `health_path` with the site Host header every 5 seconds, do not follow redirects, and mark a site down after 3 consecutive failures (status < 500 is healthy).

## 2026-11-06 Database authority and rule versions

- On startup, YAML seeds only empty policy tables: access settings/entries, rate limit, and site modes. Existing rows are never overwritten. Site modes use `INSERT OR IGNORE`.
- Rules are seeded from the built-in set only when the rules table is empty. Every rule save, toggle, delete or rollback rewrites the ordered rule set, writes a full snapshot into `ruleset_versions`, writes the audit row, and commits in one transaction. The engine then installs that version so `/api/status` reports the database version.
- Editing a default rule marks it `custom`; `asterwaf rules import-defaults` updates only rows still marked `default`, keeps their enabled flag, inserts missing defaults, and leaves custom rules untouched. The CLI writes a new version; a running server needs a restart to pick it up.
- Rule order is the stored position and therefore priority. The first matching rule wins.

## 2026-11-06 Overview statistics API

- Buckets are aligned to their own boundaries; `auto` resolves to 1m up to one hour, 5m up to six hours, 15m up to a day, 1h up to three days, otherwise 6h. A bucket producing more than 2000 points is rejected.
- KPIs compare the selected window with the immediately preceding window of equal length. `delta_pct` is null when the previous value is zero.
- p50/p95 come from 12 histogram buckets and every response includes the note. Ranges over 24 hours mark unique IPs approximate and read them from `rollup_hour_ip`; shorter ranges count distinct IPs in raw logs.
- Top IP/path tables use raw logs up to 24 hours and hourly rollups beyond that.

## 2026-11-06 Security review outcomes

- The rate limiter reads its configuration under the same mutex that guards the bucket table; `PUT /api/rate-limit` can no longer race with live traffic.
- `scripts/lint.sh` prepends the pinned toolchain to `PATH` itself, so invoking it directly uses the same Go as the build.
- The struct-tag audit parses files individually instead of using the deprecated `parser.ParseDir`.
- `golang.org/x/crypto` v0.58.0 carries advisory GO-2026-5932 for the unused `openpgp` package. Nothing imports it; the only fix is an untagged pseudo-version, so it is left as debt to recheck on the next tagged release.
- `auth.secure_cookie` defaults to `false` because the admin listener is plain HTTP on loopback. Any deployment terminating TLS in front of the admin port must enable it.
