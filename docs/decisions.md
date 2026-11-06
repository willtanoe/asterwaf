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

## 2026-11-06 Seed data

- `make seed` writes `data/asterwaf-seed.db` with `seed=1` rows. The server warns loudly if it is started against a seed database. Synthetic data is deterministic from a fixed seed.

## 2026-11-06 Rule defaults and Stage A

- Default mode is `enforce`. The built-in rules are deliberately conservative: they target unambiguous syntax such as `union select`, `<script`, `../`, and known shell words rather than broad heuristics.
- Rule order is priority. The first enabled rule that matches any inspected target wins; shadow mode also stops at the first match.
- Stage A's real target, OWASP Juice Shop, is not available from npm (ETARGET for the pinned version). The gate ran against a committed 40 + 40 corpus through the actual proxy handler, not against Juice Shop. This is recorded as a known gap.
