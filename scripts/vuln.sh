#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN="$ROOT/.toolchain"

export GOTOOLCHAIN=local
export GOCACHE="$TOOLCHAIN/cache/go-build"

if [[ ! -x "$TOOLCHAIN/bin/govulncheck" ]]; then
  echo "govulncheck: not installed (tools unavailable); skipping vulnerability scan" >&2
  exit 0
fi

exec "$TOOLCHAIN/bin/govulncheck" ./...
