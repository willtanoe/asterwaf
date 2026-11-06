#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:--l}"

mapfile -d '' FILES < <(find "$ROOT/cmd" "$ROOT/internal" -name '*.go' -type f -print0 2>/dev/null)
if [[ "${#FILES[@]}" -eq 0 ]]; then
  exit 0
fi

if [[ "$MODE" == "-w" ]]; then
  gofmt -w "${FILES[@]}"
  exit 0
fi

RESULT="$(gofmt -l "${FILES[@]}")"
if [[ -n "$RESULT" ]]; then
  echo "gofmt: the following files need formatting:" >&2
  echo "$RESULT" >&2
  exit 1
fi
