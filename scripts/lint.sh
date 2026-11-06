#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN="$ROOT/.toolchain"

export GOTOOLCHAIN=local
export GOLANGCI_LINT_CACHE="$TOOLCHAIN/cache/golangci-lint"
export GOCACHE="$TOOLCHAIN/cache/go-build"
mkdir -p "$GOLANGCI_LINT_CACHE" "$GOCACHE"

if [[ -f "$TOOLCHAIN/.lint-fallback" ]]; then
  echo "lint: using staticcheck fallback ($(cat "$TOOLCHAIN/.lint-fallback"))" >&2
  exec "$TOOLCHAIN/bin/staticcheck" ./...
fi

if [[ ! -x "$TOOLCHAIN/bin/golangci-lint" ]]; then
  echo "lint: golangci-lint is not installed; run make tools" >&2
  exit 1
fi

exec "$TOOLCHAIN/bin/golangci-lint" run ./...
