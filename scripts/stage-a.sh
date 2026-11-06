#!/usr/bin/env bash
# Stage A：Juice Shop 冒烟测试。npm 上无可用包时回退到已提交语料。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLCHAIN="$ROOT/.toolchain"
STAGE="$ROOT/.stage-a"
JUICE_VERSION="19.2.0"

export PATH="$TOOLCHAIN/go/bin:$TOOLCHAIN/node/bin:$TOOLCHAIN/bin:$PATH"
export GOTOOLCHAIN=local
export GOCACHE="$TOOLCHAIN/cache/go-build"
export npm_config_cache="$TOOLCHAIN/cache/npm"

mkdir -p "$STAGE"

cleanup() {
  if [[ -n "${WAF_PID:-}" ]]; then
    kill "$WAF_PID" 2>/dev/null || true
    wait "$WAF_PID" 2>/dev/null || true
  fi
  if [[ -n "${SHOP_PID:-}" ]]; then
    kill "$SHOP_PID" 2>/dev/null || true
    wait "$SHOP_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

run_fallback() {
  local reason="$1"
  echo "stage-a: falling back to committed corpus: $reason" >&2
  ASTERWAF_STAGE_A_FALLBACK_REASON="$reason" \
    "$TOOLCHAIN/go/bin/go" test -tags stagea ./internal/engine -run TestStageA -v -count=1
}

installed=0
if [[ -d "$STAGE/node_modules/juice-shop" ]]; then
  installed=1
else
  if [[ ! -f "$STAGE/package.json" ]]; then
    printf '{"private":true}\n' >"$STAGE/package.json"
  fi
  echo "stage-a: installing juice-shop@$JUICE_VERSION from npm"
  if (cd "$STAGE" && "$TOOLCHAIN/node/bin/npm" install --no-audit --no-fund "juice-shop@$JUICE_VERSION" >npm-install.log 2>&1); then
    installed=1
  else
    detail="$(grep -m1 -E 'notarget|ERR!' "$STAGE/npm-install.log" | tr -d '\r' || true)"
    if [[ -z "$detail" ]]; then
      detail="$(grep -m1 'npm error' "$STAGE/npm-install.log" | tr -d '\r' || true)"
    fi
    if [[ -z "$detail" ]]; then
      detail="$(tail -1 "$STAGE/npm-install.log" | tr -d '\r')"
    fi
    run_fallback "npm install juice-shop@$JUICE_VERSION failed: $detail"
    exit $?
  fi
fi

if [[ "$installed" -ne 1 ]]; then
  run_fallback "juice-shop is not installed"
  exit $?
fi

echo "stage-a: starting Juice Shop on 127.0.0.1:3000"
(
  cd "$STAGE"
  PORT=3000 HOSTNAME=127.0.0.1 NODE_ENV=production \
    "$TOOLCHAIN/node/bin/node" node_modules/juice-shop/build/app.js >"$STAGE/juice-shop.log" 2>&1
) &
SHOP_PID=$!

shop_ready=0
for _ in $(seq 1 60); do
  if curl -fsS --max-time 1 -o /dev/null http://127.0.0.1:3000/ 2>/dev/null; then
    shop_ready=1
    break
  fi
  sleep 1
done
if [[ "$shop_ready" -ne 1 ]]; then
  run_fallback "Juice Shop did not start on 127.0.0.1:3000; see .stage-a/juice-shop.log"
  exit $?
fi

echo "stage-a: building asterwaf"
make -C "$ROOT" build >/dev/null

cat >"$STAGE/config.yaml" <<'YAML'
listen: 127.0.0.1:8080
admin_listen: 127.0.0.1:9090
mode: enforce
trusted_proxies: []
inspect_headers:
  - User-Agent
  - Referer
  - X-Forwarded-For
sites:
  - host: 127.0.0.1
    upstream: http://127.0.0.1:3000
    mode: enforce
    max_body_bytes: 1048576
    health_path: /
YAML

echo "stage-a: starting asterwaf"
"$ROOT/bin/asterwaf" -config "$STAGE/config.yaml" >"$STAGE/asterwaf.log" 2>&1 &
WAF_PID=$!

proxy_ready=0
for _ in $(seq 1 30); do
  code="$(curl -sS --max-time 1 -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/ 2>/dev/null || true)"
  if [[ "$code" != "000" && -n "$code" ]]; then
    proxy_ready=1
    break
  fi
  sleep 1
done
if [[ "$proxy_ready" -ne 1 ]]; then
  run_fallback "asterwaf did not start on 127.0.0.1:8080; see .stage-a/asterwaf.log"
  exit $?
fi

ASTERWAF_STAGE_A_BASE="http://127.0.0.1:8080" \
  ASTERWAF_STAGE_A_LOG="$STAGE/asterwaf.log" \
  "$TOOLCHAIN/go/bin/go" test -tags stagea ./internal/engine -run TestStageA -v -count=1
