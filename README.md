# asterwaf

asterwaf is a reverse-proxy web application firewall with a small management UI. It routes requests by `Host` to one upstream per site, inspects requests with compiled RE2 rules, applies access lists and per-IP rate limits, and records decisions in SQLite.

The project is deliberately narrow: no TLS termination, no ACME, no cluster mode, no rule DSL, no bot scoring.

## Requirements

All toolchains are installed inside the repository. Do not use a system Go beyond the initial script.

```sh
make bootstrap   # download Go and Node, verified against toolchain.env
make tools       # fixed golangci-lint and govulncheck
make check       # formatting, vet, lint, vulnerability scan, race tests
make build       # build dashboard and bin/asterwaf
```

## Run

Copy `config.example.yaml` to `config.yaml`, set upstreams, then:

```sh
make run
```

The proxy listens on `listen` and the admin API on `admin_listen` (default `127.0.0.1:9090`). On first start, if no bcrypt hash is configured, a 20-character admin password is generated and written to `data/first-run-admin-password` with mode `0600`; its path is printed to stderr once.

```sh
./bin/asterwaf stats -config config.yaml
./bin/asterwaf seed -config config.yaml
./bin/asterwaf rules import-defaults -config config.yaml
```

## Storage

SQLite lives in `data/asterwaf.db` by default (WAL). Raw request logs are kept for 14 days, minute rollups for 30 days, hourly rollups for 180 days, and the audit log for 365 days. YAML configuration only seeds empty database tables; after first start the database is authoritative.

## Documentation

- `AGENTS.md` - repository conventions, including the Chinese identifier scheme.
- `docs/decisions.md` - decisions and technical debt.
- `docs/progress.md` - phase status.
- `docs/design-system.md` - dashboard tokens and components.
- `docs/stage-a.md` - Stage A smoke test result.
- `docs/calibration.md` - false-positive calibration result.
- `docs/security-review.md` - security review findings.
