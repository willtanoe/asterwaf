#!/usr/bin/env bash
# 安装固定版本的静态分析工具。只写入 .toolchain/bin。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN="$ROOT/.toolchain"
GO="$TOOLCHAIN/go/bin/go"

export GOTOOLCHAIN=local
export GOBIN="$TOOLCHAIN/bin"
export CGO_ENABLED=0

LINT_VERSION="v2.15.0"
VULN_VERSION="v1.9.0"
STATICCHECK_VERSION="v0.6.1"

if [[ ! -x "$GO" ]]; then
  echo "tools: run make bootstrap first" >&2
  exit 1
fi

mkdir -p "$TOOLCHAIN/bin"

lint_ready() {
  [[ -x "$TOOLCHAIN/bin/golangci-lint" ]] &&
    "$TOOLCHAIN/bin/golangci-lint" version 2>/dev/null | grep -q "${LINT_VERSION#v}"
}

if lint_ready; then
  echo "tools: golangci-lint $LINT_VERSION already installed"
else
  echo "tools: installing golangci-lint $LINT_VERSION"
  if ! "$GO" install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$LINT_VERSION"; then
    echo "tools: golangci-lint install failed; falling back to staticcheck $STATICCHECK_VERSION" >&2
    if ! "$GO" install "honnef.co/go/tools/cmd/staticcheck@$STATICCHECK_VERSION"; then
      echo "tools: staticcheck install also failed" >&2
      exit 1
    fi
    echo "staticcheck $STATICCHECK_VERSION" >"$TOOLCHAIN/.lint-fallback"
  fi
fi

vuln_ready() {
  [[ -x "$TOOLCHAIN/bin/govulncheck" ]] &&
    "$TOOLCHAIN/bin/govulncheck" -version 2>/dev/null | grep -q "govulncheck@$VULN_VERSION"
}

if vuln_ready; then
  echo "tools: govulncheck $VULN_VERSION already installed"
else
  echo "tools: installing govulncheck $VULN_VERSION"
  if ! "$GO" install "golang.org/x/vuln/cmd/govulncheck@$VULN_VERSION"; then
    echo "tools: govulncheck install failed; make check will report it and skip" >&2
  fi
fi
