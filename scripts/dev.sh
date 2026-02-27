#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/.."
  pwd
)"

cleanup() {
  if [[ -n "${BACKEND_PID:-}" ]]; then
    kill "${BACKEND_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "[dev] starting backend..."
(cd "${ROOT_DIR}/backend" && go run ./cmd/vibe-tree-daemon) &
BACKEND_PID="$!"

if [[ ! -d "${ROOT_DIR}/ui/node_modules" ]]; then
  echo "[dev] installing UI deps..."
  (cd "${ROOT_DIR}/ui" && npm install)
fi

echo "[dev] starting UI..."
cd "${ROOT_DIR}/ui"
npm run dev

