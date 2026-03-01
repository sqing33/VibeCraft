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

ensure_air() {
  if command -v air >/dev/null 2>&1; then
    return 0
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "[dev] air not found and go is not available; falling back to go run"
    return 1
  fi

  local gobin
  local gopath
  gobin="$(go env GOBIN 2>/dev/null || true)"
  gopath="$(go env GOPATH 2>/dev/null || true)"

  if [[ -n "${gobin}" ]]; then
    export PATH="${gobin}:${PATH}"
  elif [[ -n "${gopath}" ]]; then
    export PATH="${gopath}/bin:${PATH}"
  fi

  echo "[dev] air not found, auto-installing (go install github.com/air-verse/air@latest)..."
  if ! go install github.com/air-verse/air@latest; then
    echo "[dev] air auto-install failed; falling back to go run"
    return 1
  fi

  if ! command -v air >/dev/null 2>&1; then
    echo "[dev] air installed but not found in PATH; falling back to go run"
    echo "[dev] hint: ensure $(go env GOPATH)/bin or $(go env GOBIN) is in PATH"
    return 1
  fi

  return 0
}

echo "[dev] starting backend..."
if [[ "${VIBE_TREE_NO_AIR:-}" == "1" ]]; then
  echo "[dev] VIBE_TREE_NO_AIR=1, starting backend with go run..."
  (cd "${ROOT_DIR}/backend" && go run ./cmd/vibe-tree-daemon) &
elif ensure_air; then
  echo "[dev] detected air, starting backend with hot reload..."
  (cd "${ROOT_DIR}/backend" && air -c .air.toml) &
else
  echo "[dev] starting backend with go run..."
  (cd "${ROOT_DIR}/backend" && go run ./cmd/vibe-tree-daemon) &
fi
BACKEND_PID="$!"

if [[ ! -d "${ROOT_DIR}/ui/node_modules" ]]; then
  echo "[dev] installing UI deps..."
  (cd "${ROOT_DIR}/ui" && npm install)
fi

echo "[dev] starting UI..."
cd "${ROOT_DIR}/ui"
npm run dev
