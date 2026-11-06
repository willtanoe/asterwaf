# Security review

Scope: review and repair only. No new features. Every finding that could be fixed in this repository was fixed and covered by a test. Findings that are accepted risks or out of scope are listed separately.

Command used for the checks: `make check` (gofmt, vet, golangci-lint v2.15.0, govulncheck v1.9.0, `go test ./... -race`), plus `make calibrate` and the browser-driven UI checks described in `docs/progress.md`.

## Fixed

### 1. Data race when the rate limit configuration changes at runtime

`U限流器.U允许` read `限流器.配置` before taking the mutex, while `U修改配置` wrote the same field under the mutex. Saving a new rate limit through `PUT /api/rate-limit` while traffic was flowing was a real data race.

- Reproduced with a race-detector test that toggles the configuration while requests are checked.
- Fixed by reading the configuration inside the same mutex that guards the buckets and by ignoring a non-positive rate (which can only appear if a caller bypasses validation).
- Test: `internal/ratelimit/race_test.go` `Test并发修改配置不产生竞态`.

### 2. `scripts/lint.sh` used the system Go

Running `scripts/lint.sh` directly (rather than through `make`) resolved `go` from `PATH`. With `GOTOOLCHAIN=local` this either failed on the version check or, worse, could have analysed code with a different toolchain than the one used to build. The script now prepends `.toolchain/go/bin` and `.toolchain/bin` itself.

- Test: manual verification that `bash scripts/lint.sh` reports `0 issues.` without `make` setting `PATH`.

### 3. Deprecated parser call in the struct-tag audit

The tag audit used `go/parser.ParseDir`, which is deprecated because it ignores build tags. Replaced with explicit per-file parsing so the audit sees exactly the same files the compiler does.

- Test: `internal/store/tags_test.go` `Test序列化字段都有英文tag`, which inspects 171 serialized keys and fails on any non-ASCII JSON or YAML key.

## Verified, no change needed

### Resource consumption

- Oversized bodies are rejected from `Content-Length` before reading and capped with `http.MaxBytesReader` for chunked requests. Test: `Test大体积请求在读取前拒绝`.
- JSON deeper than 8 levels is skipped and recorded in `Skipped`. Test: `Test深层JSON被截断`.
- Compressed bodies are only decompressed for inspection with a 4 MiB ceiling; the original bytes are what is forwarded. Test: `Test高压缩比炸弹被丢弃`, and the proxy-level byte-for-byte test `Test压缩请求体原样转发`.
- Rule patterns are RE2; nested unbounded quantifiers cannot backtrack. A timing test evaluates eight 64 KiB adversarial inputs against every default rule in under a second (non-race build). Test: `internal/engine/redos_timing_test.go`.
- Rule patterns are capped at 1024 bytes, names at 120, IDs at 64 with `[a-z0-9._-]+`, enforced both in the compiler and in the API. Test: `Test超长模式与名称被拒绝`.

### Session handling

- Session identifiers are 32 random bytes from `crypto/rand`, base64url encoded, `HttpOnly`, `SameSite=Strict`, `Path=/`, with a 12-hour absolute expiry. `Secure` is configurable and tested in both states.
- The server always mints a new identifier on login; a client-supplied cookie is never adopted. Test: `Test登录不采用预置会话令牌`.
- Session tokens are never persisted; a database scan finds none. Test: `Test会话令牌不落库`.

### Origin checks

- Write requests carrying an `Origin` header must match the request `Host`; `null` and malformed origins are rejected, and requests without `Origin` are allowed for CLI clients. Tests: `Test来源校验`, `Test管理端口拒绝跨站写请求`.

### Login throttling

- 5 attempts per IP and 10 per username per 15 minutes, then `429` with `Retry-After`; counters clear on success. Tests: `Test按IP限速`, `Test按用户名限速`, `Test登录成功清除限速计数`.
- Because the client address comes from `clientip`, a forged `X-Forwarded-For` does not create new buckets unless the peer is a configured trusted proxy. Tests: `Test登录限速不受伪造XFF影响`, `Test登录限速忽略伪造XFF`, `Test访问列表不受伪造XFF影响`.

### First-run password file

- Created with `O_EXCL` and mode `0600` inside a `0700` data directory, path only printed to stderr, password and hash never logged. Tests: `Test首次运行密码文件权限`, `Test首次运行密码文件`, `Test配置哈希优先`.

### Admin surface

- Default listener is `127.0.0.1:9090`; only `GET /healthz` is unauthenticated. Test: `Test默认管理监听仅限回环` plus the unauthenticated sweep of every API path.
- Every mutating route requires a session; `Test未认证的写请求被拒绝` covers rules, access lists, rate limit and site mode.

### SQL

- All values are bound parameters. The only interpolated fragment is the distribution column, which comes from a fixed switch of `category`, `rule`, `action`, `site`. Injection-shaped dimension strings are rejected and the `logs` table survives. Test: `Test分布维度白名单`.
- Log filtering builds its predicate from literal fragments only; user values are always parameters (`internal/store/logs.go`).

### Logs and audit

- No password, token, or session value is written to the database. Tests: `Test审计不记录密码`, `Test响应不包含凭据`, `Test会话令牌不落库`.
- Stored header and body copies are redacted (`Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`; sensitive names in query and body) and bounded. Tests: `Test脱敏头与查询`, `Test脱敏体`, `Test请求日志落库与脱敏`.
- Admin responses never echo credentials or bcrypt hashes.

### Rollup consistency

- Log insert and the three rollup upserts share one transaction, so a dropped row never reaches aggregates. Tests: `Test丢弃的行不进聚合`, `Test重启后聚合一致`.
- Shutdown drains the queue even for a partial batch. Tests: `Test关闭时排空未满批次`, `Test关闭后队列不再接收`.

### `import-defaults` protection

- Only rows still marked `source=default` are refreshed; edited defaults (now `custom`) and custom rules are untouched, and missing defaults are inserted. Test: `cmd/asterwaf/rules_test.go` `Test导入默认规则保护自定义规则`.

### Frontend rendering

- No `v-html`, `innerHTML`, `outerHTML`, or `insertAdjacentHTML` in `web/src`. Stored headers, bodies and matched targets render inside `<pre>` as text, verified in the browser against a record whose payload contains markup (no `<img>`, `<script>` or `<iframe>` nodes appear). Log content is attacker-controlled and stays text.

### Serialization keys

- No JSON or YAML tag and no SQL identifier contains non-ASCII characters; the audit covers 171 keys and the migration files.

## Not fixed

### 1. `golang.org/x/crypto` v0.58.0 advisory GO-2026-5932

`govulncheck` reports that the module contains the unmaintained `openpgp` package. Nothing in asterwaf imports it (`go list -deps` shows no `openpgp`), and govulncheck reports the code as affected by zero vulnerabilities. The fix is only published as the pseudo-version `v0.58.1-0.20261009181718-2f1f834330b2`. Pinning an unreleased commit for a package we do not call is a worse trade than waiting for the next release, so this stays as debt and must be re-checked when a tagged release exists.

### 2. No secure default for `auth.secure_cookie`

The default is `false` because the admin listener is HTTP on loopback with no TLS in scope. Deployments that terminate TLS in front of the admin port must set `auth.secure_cookie: true` themselves. A wrong value only weakens the cookie, never the login checks, but it is a configuration hazard rather than something the code can infer.

### 3. Per-site timeouts and body limits for streaming

The listener has fixed read/write timeouts and `max_body_bytes` is a hard cap. Large uploads and streaming responses that legitimately exceed those limits will fail. Recorded as technical debt in `docs/decisions.md`; changing it needs per-site server configuration, which is out of scope for this build.

### 4. Rate limit and session state are not shared or persisted

Both are in memory. A restart clears them, and a future multi-instance deployment would need shared state. Single-node scope is explicit in the design.
