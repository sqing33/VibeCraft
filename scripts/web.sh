#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/.."
  pwd
)"

if [[ "${VIBE_TREE_SKIP_UI_BUILD:-}" == "1" ]]; then
  if [[ ! -f "${ROOT_DIR}/ui/dist/index.html" ]]; then
    echo "[web] VIBE_TREE_SKIP_UI_BUILD=1 but ui/dist/index.html not found"
    echo "[web] run: (cd ui && npm run build)  OR unset VIBE_TREE_SKIP_UI_BUILD"
    exit 1
  fi
  echo "[web] skipping UI build (VIBE_TREE_SKIP_UI_BUILD=1)"
else
  echo "[web] building UI..."

  if [[ ! -d "${ROOT_DIR}/ui/node_modules" ]]; then
    echo "[web] installing UI deps..."
    (cd "${ROOT_DIR}/ui" && npm install)
  fi

  (cd "${ROOT_DIR}/ui" && npm run build)
fi

echo "[web] starting daemon (serving ui/dist)..."
cd "${ROOT_DIR}/backend"
go run ./cmd/vibe-tree-daemon
