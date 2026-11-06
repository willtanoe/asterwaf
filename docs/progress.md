# Progress

Status of each phase, with branch, tag and open items. Update at the end of every phase.

| Phase | Branch | Tag | Status | Notes |
| --- | --- | --- | --- | --- |
| 0 repo-bootstrap | phase/0-repo-bootstrap | phase-0 | complete | make check passes; standalone build works |
| 1 proxy-skeleton | phase/1-proxy-skeleton | phase-1 | complete | host routing, trusted client IP, graceful shutdown |
| 2 inspection-pipeline | phase/2-inspection-pipeline | phase-2 | complete | bounded body read, gzip/deflate copy, target extraction, redaction |
| 3 rule-engine | phase/3-rule-engine | phase-3 | complete | 23 default rules; Stage A: 0/40 false positives, 40/40 attacks blocked (fallback corpus) |
| 4 access-lists | phase/4-access-lists | phase-4 | complete | deny precedence, allow-only, allow bypasses ACL |
| 5 rate-limiting | phase/5-rate-limiting | phase-5 | pending | |
| 6 async-logging | phase/6-async-logging | phase-6 | pending | |
| 7 stats-rollups | phase/7-stats-rollups | phase-7 | pending | |
| 8a api-foundation | phase/8a-api-foundation | phase-8a | pending | |
| 8b api-management | phase/8b-api-management | phase-8b | pending | |
| 9 ui-shell-and-overview | phase/9-ui-shell-and-overview | phase-9 | pending | screenshot self-check |
| 10 ui-management | phase/10-ui-management | phase-10 | pending | |
| 11 fp-calibration | phase/11-fp-calibration | phase-11 | pending | |
| 12 security-review | phase/12-security-review | phase-12 | pending | |

## Open items

- None yet.

## Stage A notes

- Juice Shop is not published on npm: `npm install juice-shop@19.2.0` fails with ETARGET. The Stage A harness therefore ran the committed 40 normal / 40 attack corpus in-process behind the real proxy handler.
- Real Juice Shop integration remains a gap; the upstream path used by the harness is an in-process HTTP server.
