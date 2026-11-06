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
| 11 fp-calibration | phase/11-fp-calibration | phase-11 | pending | |
| 12 security-review | phase/12-security-review | phase-12 | pending | |

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

## Open items

- None yet.

## Stage A notes

- Juice Shop is not published on npm: `npm install juice-shop@19.2.0` fails with ETARGET. The Stage A harness therefore ran the committed 40 normal / 40 attack corpus in-process behind the real proxy handler.
- Real Juice Shop integration remains a gap; the upstream path used by the harness is an in-process HTTP server.
