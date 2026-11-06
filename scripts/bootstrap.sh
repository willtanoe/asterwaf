#!/usr/bin/env bash
# 固定 Go 与 Node 的引导脚本。只允许写入 .toolchain/。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN="$ROOT/.toolchain"
DOWNLOADS="$TOOLCHAIN/downloads"

# shellcheck source=../toolchain.env
source "$ROOT/toolchain.env"

if [[ "$(id -u)" -eq 0 ]]; then
  echo "bootstrap: refusing to run as root; toolchains must be user-owned" >&2
  exit 1
fi

mkdir -p "$DOWNLOADS" "$TOOLCHAIN/bin"

fail() {
  echo "bootstrap: $*" >&2
  exit 1
}

go_installed() {
  [[ -x "$TOOLCHAIN/go/bin/go" ]] &&
    [[ "$(cat "$TOOLCHAIN/go/VERSION" 2>/dev/null)" == "go$GO_VERSION" ]]
}

node_installed() {
  [[ -x "$TOOLCHAIN/node/bin/node" ]] &&
    [[ "$("$TOOLCHAIN/node/bin/node" --version 2>/dev/null)" == "v$NODE_VERSION" ]]
}

download() {
  local url="$1" target="$2" label="$3"
  if [[ -f "$target" ]]; then
    return 0
  fi
  echo "bootstrap: downloading $label from $url"
  local tmp="$target.part"
  rm -f "$tmp"
  if ! curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 -o "$tmp" "$url"; then
    rm -f "$tmp"
    fail "download failed for $label ($url)"
  fi
  mv "$tmp" "$target"
}

verify() {
  local file="$1" expected="$2" label="$3"
  local actual
  actual="$(sha256sum "$file" | cut -d' ' -f1)"
  if [[ "$actual" != "$expected" ]]; then
    fail "checksum mismatch for $label: expected $expected, got $actual"
  fi
}

install_go() {
  local archive="$DOWNLOADS/go-$GO_VERSION.tar.gz"
  download "$GO_URL" "$archive" "Go $GO_VERSION"
  verify "$archive" "$GO_SHA256" "Go $GO_VERSION"
  local tmp
  tmp="$(mktemp -d "$TOOLCHAIN/.go-extract.XXXXXX")"
  trap 'rm -rf "$tmp"' RETURN
  tar -xzf "$archive" -C "$tmp"
  [[ -x "$tmp/go/bin/go" ]] || fail "Go archive did not contain go/bin/go"
  rm -rf "$TOOLCHAIN/go"
  mv "$tmp/go" "$TOOLCHAIN/go"
  date -u +%FT%TZ >"$TOOLCHAIN/.go-$GO_VERSION.stamp"
}

install_node() {
  local archive="$DOWNLOADS/node-v$NODE_VERSION-linux-x64.tar.xz"
  download "$NODE_URL" "$archive" "Node $NODE_VERSION"
  verify "$archive" "$NODE_SHA256" "Node $NODE_VERSION"
  local tmp
  tmp="$(mktemp -d "$TOOLCHAIN/.node-extract.XXXXXX")"
  trap 'rm -rf "$tmp"' RETURN
  tar -xJf "$archive" -C "$tmp"
  local extracted="$tmp/node-v$NODE_VERSION-linux-x64"
  [[ -x "$extracted/bin/node" ]] || fail "Node archive did not contain bin/node"
  rm -rf "$TOOLCHAIN/node-v$NODE_VERSION-linux-x64"
  mv "$extracted" "$TOOLCHAIN/node-v$NODE_VERSION-linux-x64"
  ln -sfn "node-v$NODE_VERSION-linux-x64" "$TOOLCHAIN/node"
  date -u +%FT%TZ >"$TOOLCHAIN/.node-$NODE_VERSION.stamp"
}

if go_installed; then
  echo "bootstrap: Go $GO_VERSION already installed"
else
  install_go
fi

if node_installed; then
  echo "bootstrap: Node $NODE_VERSION already installed"
else
  install_node
fi

echo "bootstrap: Go $("$TOOLCHAIN/go/bin/go" version)"
echo "bootstrap: Node $("$TOOLCHAIN/node/bin/node" --version)"
