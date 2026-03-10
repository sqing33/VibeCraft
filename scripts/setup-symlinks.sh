#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
用法:
  bash scripts/setup-symlinks.sh [--force] [--no-backup]

说明:
  - 生成/修复以下软链：
    1) .claude/skills -> ../.codex/skills
    2) CLAUDE.md -> AGENTS.md
  - 默认会在目标路径已存在时创建备份（.bak.<timestamp>）。

选项:
  --force     若软链路径已存在，直接删除后重建（不备份）。
  --no-backup 若软链路径已存在，不备份，直接报错退出。
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

FORCE=false
BACKUP=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      FORCE=true
      BACKUP=false
      ;;
    --no-backup)
      BACKUP=false
      ;;
    *)
      echo "[setup-symlinks] error: unknown option '$1'" >&2
      usage
      exit 2
      ;;
  esac
  shift
done

link_path() {
  local link_rel="$1"
  local target_rel="$2"

  local link_abs="$ROOT_DIR/$link_rel"
  local link_dir
  link_dir="$(dirname "$link_abs")"

  local target_abs
  target_abs="$(realpath -m "$link_dir/$target_rel")"

  if [[ ! -e "$target_abs" ]]; then
    echo "[setup-symlinks] error: target does not exist: $target_abs" >&2
    return 1
  fi

  if [[ -L "$link_abs" ]]; then
    local current_target
    current_target="$(readlink "$link_abs" || true)"
    if [[ "$current_target" == "$target_rel" ]]; then
      echo "[setup-symlinks] already linked: $link_rel -> $target_rel"
      return 0
    fi
  fi

  mkdir -p "$link_dir"

  if [[ -e "$link_abs" || -L "$link_abs" ]]; then
    if [[ "$FORCE" == "true" ]]; then
      rm -rf "$link_abs"
      echo "[setup-symlinks] removed existing path: $link_rel"
    elif [[ "$BACKUP" == "true" ]]; then
      local ts
      ts="$(date +%Y%m%d-%H%M%S)"
      local backup_abs="$link_abs.bak.$ts"
      mv "$link_abs" "$backup_abs"
      echo "[setup-symlinks] backup created: ${backup_abs#$ROOT_DIR/}"
    else
      echo "[setup-symlinks] error: link path already exists: $link_rel" >&2
      echo "[setup-symlinks] tip: use --force or remove it manually." >&2
      return 1
    fi
  fi

  ln -s "$target_rel" "$link_abs"
  echo "[setup-symlinks] created symlink: $link_rel -> $target_rel"
  ls -l "$link_abs" >/dev/null
}

echo "[setup-symlinks] repo root: $ROOT_DIR"

link_path ".claude/skills" "../.codex/skills"
link_path "CLAUDE.md" "AGENTS.md"

echo "[setup-symlinks] done"
