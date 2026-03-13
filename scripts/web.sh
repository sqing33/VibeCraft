#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/.."
  pwd
)"

if [[ "${VIBECRAFT_SKIP_UI_BUILD:-}" == "1" ]]; then
  if [[ ! -f "${ROOT_DIR}/ui/dist/index.html" ]]; then
    echo "[web] VIBECRAFT_SKIP_UI_BUILD=1 but ui/dist/index.html not found"
    echo "[web] run: (cd ui && pnpm build)  OR unset VIBECRAFT_SKIP_UI_BUILD"
    exit 1
  fi
  echo "[web] skipping UI build (VIBECRAFT_SKIP_UI_BUILD=1)"
else
  echo "[web] building UI..."

  if [[ ! -d "${ROOT_DIR}/ui/node_modules" ]]; then
    echo "[web] installing UI deps..."
    (cd "${ROOT_DIR}/ui" && pnpm install)
  fi

  (cd "${ROOT_DIR}/ui" && pnpm build)
fi

echo "[web] starting daemon (serving ui/dist)..."
cd "${ROOT_DIR}/backend"
go run ./cmd/vibecraft-daemon
