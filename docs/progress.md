# Progress

Status of each phase, with branch, tag and open items. Update at the end of every phase.

| Phase | Branch | Tag | Status | Notes |
| --- | --- | --- | --- | --- |
| 0 repo-bootstrap | phase/0-repo-bootstrap | phase-0 | complete | make check passes; standalone build works |
| 1 proxy-skeleton | phase/1-proxy-skeleton | phase-1 | complete | host routing, trusted client IP, graceful shutdown |
| 2 inspection-pipeline | phase/2-inspection-pipeline | phase-2 | complete | bounded body read, gzip/deflate copy, target extraction, redaction |
| 3 rule-engine | phase/3-rule-engine | phase-3 | complete | 23 default rules; Stage A: 0/40 false positives, 40/40 attacks blocked (fallback corpus) |
| 4 access-lists | phase/4-access-lists | phase-4 | complete | deny precedence, allow-only, allow bypasses ACL |
| 5 rate-limiting | phase/5-rate-limiting | phase-5 | complete | token bucket, IPv6 /64, cleanup, eviction, 429 |
| 6 async-logging | phase/6-async-logging | phase-6 | complete | SQLite WAL, embedded migration, 4096-slot queue, request IDs |
| 7 stats-rollups | phase/7-stats-rollups | phase-7 | complete | minute/hour rollups, retention, seed CLI (~28.7k rows), stats CLI |
| 8a api-foundation | phase/8a-api-foundation | phase-8a | complete | admin listener, bcrypt login, sessions, origin checks, audit, health probes |
| 8b api-management | phase/8b-api-management | phase-8b | complete | stats, logs, rule versions, ACL/rate/site persistence |
| 9 ui-shell-and-overview | phase/9-ui-shell-and-overview | phase-9 | complete | responsive shell, tokenized themes, Overview, 36 screenshots |
| 10 ui-management | phase/10-ui-management | phase-10 | complete | logs filters/drawer/row actions, rule editor/test/versions, ACL and rate forms, site modes |
| 11 fp-calibration | phase/11-fp-calibration | phase-11 | complete | 300 normal / 150 attack; 0.00% FP, 98.00% detection |
| 12 security-review | phase/12-security-review | phase-12 | complete | 3 fixed (rate limit race, lint toolchain, deprecated parser); 4 documented gaps |

## Phase 9 self-check

- Only design tokens are used for colors, font sizes, and spacing; `grep` finds no gradients, blur/backdrop filters, shadows, or emoji.
- Semantic states (blocked, recorded, rate limited, site health) always include a text label in addition to color.
- Contrast: every foreground/background pair used by the components was computed from `tokens.css`; the lowest ratio is 4.56:1 in dark mode and 5.40:1 in light mode, both above 4.5:1.
- Widths 320, 768, and 1440 were checked by script for `scrollWidth > clientWidth`; all pages and both themes pass.
- Cards and tables render loading, empty, and error states; the error state includes a Retry action.
- Copy is literal and lives in `web/src/copy.ts`; no marketing language.
- Screenshots for six pages, two themes, and three widths are in `docs/screenshots/`, produced by `scripts/screenshots.mjs` against the locally installed Chromium. The script adds no npm dependency.
- The screenshot script drives the UI through the real admin API with a session cookie; the local test database contains synthetic seed traffic.

## Phase 10 verification

- The four management pages were exercised through the real admin API with a browser session against a seed database:
  - Logs: action filter returned only blocked records; row click opened the drawer; Escape closed it; stored headers and body rendered as plain text (no `v-html`/`innerHTML` in `web/src`); "Allow this IP", "Deny this IP" and "Disable rule" created the corresponding access entries and rule change.
  - Rules: created a custom rule through the form, the payload test matched, save produced version 2 with 25 rows (defaults plus custom), the rule persisted across reloads, toggle and delete produced new versions, and rollback to an earlier version restored the deleted rule as a new version.
  - Access lists: added and deleted entries, including one created from a log row with `source_log_id` recorded; rate limit GET/PUT round-tripped.
  - Sites: mode selector saved `shadow` and the persisted value survived reloads; upstream stayed read-only.
- Screenshots for all six pages, both themes, and three widths were regenerated after the changes; no horizontal overflow was reported.

## Phase 11 numbers

- Corpus: 300 normal, 150 labelled attacks (30 each for sqli, xss, traversal, rce, scanner), in-process through the real inspection pipeline.
- False positive rate: 0.00% (0 of 300). Per-rule false positives: none.
- Detection rate: 98.00% (147 of 150). Thresholds: FP <= 0.5%, detection >= 95%.
- One rule-adjustment round was needed (93.33% -> 98.00%). The three remaining misses are explained in `docs/calibration.md`; they are parser limits or deliberate non-goals, not silent gaps.
- `make calibrate` runs the gate. Rule changes for the round were applied through `asterwaf rules import-defaults`, which reported 3 inserted defaults, 0 updated defaults, and 22 custom rules skipped (the local test database had edited rules).

## UI overhaul (in progress)

- Screenshot loop: `scripts/screenshot.mjs` (`make shots`) seeds
  `data/asterwaf-seed.db`, starts a throwaway instance on 127.0.0.1:18180/18190,
  and captures all six pages in both themes at 1440/768/390 with console-error
  and horizontal-overflow checks. `make contrast` fails below 4.5:1 for every
  token pair used for text; `--text-faint` (dark) and `--ok` (light) were
  adjusted to pass.
- Shell fixes: the skip link is fixed, transform-hidden, and shown only on
  :focus-visible; the site filter is a themed listbox (keyboard: arrows, Home,
  End, Enter, Esc, typeahead); the sidebar becomes a top bar at 768px and
  controls reach 36px touch targets at 390px.

- Overview redesign: the gauge was replaced by a seven-card dense KPI row
  (value, signed delta, 96px sparkline; Recorded as a secondary figure on the
  Blocked card; histogram notes moved into tooltips; approximate flag on
  Unique IPs for rollup ranges). The traffic chart draws per-bucket bars for
  requests plus a stepped square-marker line for blocked, integer y axis from
  zero, bucket-start tooltips with block rate, no per-point animation. The
  status strip collapses to "N/N up" with an expandable list when healthy and
  shows only problem sites otherwise. Top tables are dense 32px rows with
  sortable headers (aria-sort), tabular numbers and copy-to-clipboard on IP
  and path. Polling is 15s overview / 10s latest blocked / 30s status, paused
  when hidden, with in-flight guards.

- Logs: the filter bar keeps a linkable URL (query params synced both ways,
  so back/forward restores the view), active filters render as removable
  chips, site and action use the themed listbox, rows truncate with tooltips
  at 32px height, seeded rows carry a visible "Seed" marker, payload sections
  in the drawer gained copy buttons, and the allow/deny/disable actions run
  behind a focus-trapped confirm dialog.

- Rules: the editor lives in a side drawer with live client validation
  (byte-length caps for name and pattern, id charset, client-side regex
  compile) plus inline server compile errors, a read-only Score with the
  reserved note, and a payload tester with a target selector that reports
  every rule that fires and the matched substring (new backend scope "all"
  on /api/rules/test backed by U匹配列表, which returns all hits instead of
  the first one; covered by Test匹配列表返回全部命中). The table shows
  enabled switches, updated time and version; delete and rollback run behind
  confirm dialogs.

- Access lists: deletes run behind a confirm dialog, the add form
  validates IPv4/IPv6 addresses and CIDR prefixes client-side, the table
  shows the created time, and a source log id links to /logs?log=<id>,
  which opens the request drawer directly. Sites gained a themed mode
  select that confirms when lowering away from enforce, a status dot with
  a text label instead of a repeated badge, and a last-check column
  (new last_check field on /api/status and /api/sites). The login screen
  shows the throttling retry window from the Retry-After header and never
  reveals which field failed.

- Access lists, sites, login: access deletes confirm, the add form
  validates IPv4/IPv6 CIDRs, entries show created time and deep-link their
  source log into the logs drawer (/logs?log=<id>). Sites use the themed
  mode select (confirm when lowering from enforce), a dot-plus-label health
  column and a last-check column (new last_check field on /api/status and
  /api/sites). Login shows the Retry-After window on throttling.
- Functional verification: docs/verification.md records the harness results —
  47/47 checks passed (auth, sessions, API shapes, overview-vs-stats
  equality, proxy enforcement, rate limiting, access entries, rule toggles
  and rollback, site mode persistence, upstream failure and recovery, the
  dropped-log counter under a forced writer stall, seed isolation). The
  keyboard walkthrough (skip link, focus rings, drawer Esc, minimum font
  size) also passes.

- UI polish round: per-site `skip_stats` (pointer flag, resolved by
  U跳过统计生效) stops the dashboard's own traffic from filling logs and
  rollups; the site whose upstream is the admin listener defaults to skip,
  explicit values win, existing rows are untouched. Covered by
  Test跳过统计生效 and Test跳过统计站点不落日志. The screenshots loop now
  also captures 1024px.

- Rule deletion: the API rejects DELETE on source=default (400, disable
  instead) and the rules table disables the Delete button for defaults with
  an explanatory tooltip; custom rules keep the confirm dialog.

- Sites: max body renders as MiB, the confirm-on-lower dialog from the
  earlier round stays, and tables cap their width at 1900px and wider so
  columns stop spreading into gaps.

- Navigation: pointer clicks blur the link after the route change
  (event.detail gates it, so keyboard activation keeps focus), which stops
  the focus ring from lingering on the active item after a mouse click.

- Logs: Allowed renders as muted text so Blocked and Recorded badges carry
  the visual weight, inspection latency under 1 ms shows "<1 ms", date
  inputs follow the theme (picker indicator inverted in dark mode), and the
  filter row is a compact wrapping bar instead of a four-column grid.

- Access lists: the add form sits on one 38px baseline (select, inputs,
  checkbox, button) and the YAML seed note moved under the page title.

- Latest blocked verification: rule ids and IPs truncate on one line at
  1440, 1024 and 768 in both themes (checked in the round's screenshots);
  no change was needed.

- Retheme: tokens now derive from the aster.svg palette (accent #207fff dark
  / #0d5ecb light, blue-tinted neutrals), semantic colors stay distinct from
  the accent, and the contrast gate extended to component boundaries and
  focus rings at 3:1 in both themes. The derivation is recorded in
  docs/design-system.md.

- Overview radial hero: an ActionDonut doughnut (72% cutout) shows the
  action split from breakdowns.actions with the request total and block rate
  in the center, a keyboard-focusable legend with label, count and percentage
  (zero-count entries stay in the legend and out of the ring), skeleton,
  empty and error states, and no refresh animation (initial render respects
  prefers-reduced-motion). It sits left of the KPI row at 300px and stacks
  above it under 768px. The screenshot loop grew the viewport to the document
  height before capturing because captureBeyondViewport blanked freshly drawn
  canvases.

- Density round, part A: the detections hero replaces the allowed-dominated
  donut (non-allowed decisions by category, thin action bar, compact 150px
  ring) with a 240px panel cap; KPI cards are fixed at 112px with nowrap
  values, "new"/"n/a" delta states, a sparkline on Unique IPs and an "Off"
  link on Rate limited when throttling is disabled; the sites table spans
  the full width again.

- Density round part B: the overview grew a system strip (ruleset, uptime,
  log DB size, retention windows, rate-limit state with active buckets,
  dropped logs), a second chart row (detections over time stacked by
  category, inspection latency p50/p95), status-class and method panels,
  throughput in/out, a per-site table with sparklines and click-to-filter,
  top rules and user agents, a weekday-hour heatmap, blocked-rate columns on
  the top tables, and a latest-events feed with All/Blocked/Recorded
  filters. Raw-log dimensions are marked and only served up to 24h.

- Access lists: range-scoped summary cards (entries, allow/deny counts,
  rate-limit state with active buckets, 429s in range with a series,
  denied requests), hits-in-range and last-hit columns on entries, and a
  recent top-offenders panel with one-click deny.

- Sites: the table gained per-site requests, blocked, block rate, p95,
  shadow would-block and a detail drawer per site (probe history ring,
  audit mode changes, recent blocked requests); host and upstream truncate
  instead of folding.

## Open items

- None yet.

## Stage A notes

- Juice Shop is not published on npm: `npm install juice-shop@19.2.0` fails with ETARGET. The Stage A harness therefore ran the committed 40 normal / 40 attack corpus in-process behind the real proxy handler.
- Real Juice Shop integration remains a gap; the upstream path used by the harness is an in-process HTTP server.

## Phase 12 summary

- Fixed: a real data race between `U限流器.U允许` and `U修改配置`; `scripts/lint.sh` resolving the system Go when invoked directly; a deprecated `go/parser.ParseDir` call in the tag audit.
- Verified with new tests: body/decompression/JSON-depth limits, RE2 timing on 64 KiB adversarial inputs, rule size caps, session minting and non-persistence, cookie attributes, Origin checks on writes, login throttling including spoofed `X-Forwarded-For`, password file permissions, loopback-only admin default, unauthenticated write rejection, SQL dimension allowlist, no credentials in logs/audit/responses, rollup consistency across drops and restarts, queue drain on shutdown, `import-defaults` preserving custom rules, static asset traversal, and no HTML rendering of log content.
- Not fixed and recorded in `docs/security-review.md`: `golang.org/x/crypto` openpgp advisory (package not imported; only an untagged pseudo-version fixes it), `secure_cookie` default depends on deployment, fixed per-site timeouts, in-memory session/rate state.
