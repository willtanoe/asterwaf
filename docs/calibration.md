# Calibration

This corpus is a regression gate, not a measurement of production accuracy. It has 300 normal and 150 labelled attack requests, is small, and was written by the same person who wrote the rules, so the numbers below are optimistic.

## Thresholds

| Metric | Threshold | Result |
| --- | --- | --- |
| False positive rate | <= 0.5% | 0.00% (0 of 300) |
| Detection rate | >= 95% | 98.00% (147 of 150) |
| Rules evaluated | n/a | 25 |

## Detection by category

| Category | Blocked | Total | Rate |
| --- | --- | --- | --- |
| rce | 29 | 30 | 96.7% |
| scanner | 30 | 30 | 100.0% |
| sqli | 30 | 30 | 100.0% |
| traversal | 28 | 30 | 93.3% |
| xss | 30 | 30 | 100.0% |

## Missed attacks

| ID | Category | Request | Note |
| --- | --- | --- | --- |
| a072 | traversal | `GET /download?file=..;/..;/etc/passwd` | semicolon |
| a082 | traversal | `GET /download?file=..%c0%af..%c0%afetc%c0%afpasswd` | overlong utf8 |
| a103 | rce | `GET /upload?name=shell.php` | php upload name |

Miss reasons:

- `a072` sends `;` inside the query value. Go's `url.ParseQuery` rejects the semicolon form, so the pipeline records `invalid query encoding` and has no decoded query targets to test. `traversal.dotdot` would match the value if it were parsed; the limitation is in query parsing, not in the pattern.
- `a082` uses overlong UTF-8 (`%c0%af`). Decoding produces bytes that are not a valid `/`, so the decoded target contains no encoded separator and the pattern deliberately does not guess at illegal UTF-8. A backend that accepts overlong sequences is out of scope for these rules.
- `a103` is only a file name (`name=shell.php`). Blocking every `.php` name would flag legitimate uploads, so no rule targets it; the request is recorded and the upstream must validate uploads.

## False positives

None.

## Per-rule false positives

No rule produced a false positive.

## Before pointing it at a real site

1. Run asterwaf in `shadow` mode for several days. Nothing is blocked; every rule decision is recorded.
2. Review the logs page filtered by action `Recorded`, grouped by rule and path. Investigate each rule that fires on legitimate traffic.
3. Adjust or disable those rules through the UI, then roll back if the change makes things worse. Each change creates a new ruleset version.
4. Move a single site to `enforce` only after its shadow decisions have been reviewed for a full business cycle, and keep the access list ready for fast exceptions.
