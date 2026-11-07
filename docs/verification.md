# Functional verification

Recorded from the automated harness run (`/tmp/verify.py`, throwaway instance on
127.0.0.1:18180/18190, seeded `data/asterwaf-seed.db`, three dummy upstreams on
18601-18603). Latest run: **47/47 checks passed**. Re-run by booting the same
layout and replaying the steps below.

## Auth and sessions

| Check | Result | Notes |
| --- | --- | --- |
| Login with correct credentials succeeds | PASS | `POST /api/login` 200, cookie set |
| Wrong password rejected | PASS | 401, message `invalid username or password` |
| Failed login stays field-ambiguous | PASS | identical message for unknown user and wrong password |
| Sixth bad login attempt throttled | PASS | per-IP window allows 5; 6th returns 429 with `Retry-After` |
| Session survives reload | PASS | cookie valid across requests |
| Logout clears the session | PASS | `/api/me` returns 401 afterwards |
| Cross-origin write rejected | PASS | `POST /api/status` with `Origin: https://evil.test` → 403 |

## API shapes

| Check | Result | Notes |
| --- | --- | --- |
| `/api/me`, `/api/status`, `/api/stats/overview`, `/api/logs`, `/api/rules`, `/api/rules/versions`, `/api/access-lists`, `/api/rate-limit`, `/api/sites` | PASS | required keys present, all 200 |

## Overview correctness

| Check | Result | Notes |
| --- | --- | --- |
| Counters move within one polling interval | PASS | 3 burst requests visible in ≤16 s |
| Totals equal `asterwaf stats` for 15m / 1h / 24h | PASS | exact equality after the async writer settles |
| Totals equal `asterwaf stats` for 7d | PASS | within one row: the API flags 7d as rollup-approximate (`approximate: true`) |

## Proxy enforcement

| Check | Result | Notes |
| --- | --- | --- |
| Benign request passes | PASS | 200 from dummy upstream |
| SQLi / XSS / traversal blocked on enforce site | PASS | 403 with rule ids recorded |
| Same attacks recorded (not blocked) on shadow site | PASS | 200, decisions logged as shadow |
| New traffic logged with rule ids | PASS | visible within the async writer flush window |
| Rate limit returns 429 with `Retry-After` | PASS | burst 5, sixth request in the bucket window throttled |
| Rate limit disable restores access | PASS | no restart needed |
| Deny entry blocks the next request without restart | PASS | 403 from the access list |
| Removing the deny entry restores access | PASS | |
| Allow-listed IP is still inspected by rules | PASS | XSS payload from an allow-listed IP still 403 |
| Rule toggle changes behavior immediately | PASS | disable → payload passes; re-enable → blocked |
| Custom rule blocks, rollback restores | PASS | rollback creates a new ruleset version |
| Site mode change applies immediately and persists across restart | PASS | mode re-read from the database after restart |

## Runtime behavior

| Check | Result | Notes |
| --- | --- | --- |
| Upstream down reflected within the probe window | PASS | 5 s probe interval, 3 failures to mark down |
| Upstream recovery reflected | PASS | |
| Dropped-log counter under overload | PASS | forced with an exclusive SQLite lock during a 20k-request flood; the in-process path is covered by `Test队列满丢弃并计数` |
| Seed data never appears in the real database view | PASS | production `/api/logs` contains no `seed` rows |

## UI checks

| Check | Result | Notes |
| --- | --- | --- |
| Console clean on every page/theme/width | PASS | `make shots` fails on console errors, warnings, uncaught exceptions and failed network requests |
| No horizontal page overflow at 1440/768/390 | PASS | tables scroll inside their own container |
| Keyboard walkthrough | PASS | skip link hidden until keyboard focus, Enter moves focus to `#main`, focus-visible rings on nav, Esc closes the log drawer and returns focus |
| No text below 12px | PASS | checked across all rendered text nodes |
| Token contrast ≥ 4.5:1 | PASS | `make contrast` gate on both themes |

## Known limits (recorded, not failures)

- The 7d overview range reads hourly rollups and is flagged `approximate` by
  the API; a one-row boundary difference against a raw-row count is expected.
- The live dropped-log demonstration depends on an external SQLite lock; the
  deterministic in-process path is the unit test `Test队列满丢弃并计数`.
