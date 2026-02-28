#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/.."
  pwd
)"

if [[ ! -f "${ROOT_DIR}/ui/dist/index.html" ]]; then
  echo "[web] ui/dist not found, building UI..."

  if [[ ! -d "${ROOT_DIR}/ui/node_modules" ]]; then
    echo "[web] installing UI deps..."
    (cd "${ROOT_DIR}/ui" && npm install)
  fi

  (cd "${ROOT_DIR}/ui" && npm run build)
fi

echo "[web] starting daemon (serving ui/dist)..."
cd "${ROOT_DIR}/backend"
go run ./cmd/vibe-tree-daemon

