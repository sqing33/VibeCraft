#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "ERROR: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  worktree-lite.sh init [--base <branch>] [--root <dir>] [--id <worktree-id>] [--topic <summary>]
  worktree-lite.sh review [--base <branch>]
  worktree-lite.sh merge-options [--target <branch>] [--source <branch>] [--format <plain|codex>]
  worktree-lite.sh propose-message [--base <branch>]
  worktree-lite.sh merge --target <branch> [--message "<commit title>"] [--source <branch>]

Commands:
  init             Create a new worktree and branch.
  review           Print review summary for changes against base branch.
  merge-options    Print merge decision options (plain 5-choice or codex-native payload).
  propose-message  Generate commit title candidates inferred from git history.
  merge            Squash merge source branch into target and commit.
EOF
}

repo_root() {
  git rev-parse --show-toplevel 2>/dev/null || die "not inside a git repository"
}

repo_common_root() {
  local common
  common="$(git rev-parse --git-common-dir 2>/dev/null)" || die "not inside a git repository"
  if [[ "$common" != /* ]]; then
    common="$(cd "$common" && pwd -P)"
  fi
  (cd "$common/.." && pwd -P)
}

current_branch() {
  local root="$1"
  local branch
  branch="$(git -C "$root" rev-parse --abbrev-ref HEAD)"
  [[ "$branch" != "HEAD" ]] || die "detached HEAD is not supported"
  echo "$branch"
}

ensure_branch_exists() {
  local root="$1"
  local branch="$2"
  git -C "$root" rev-parse --verify --quiet "$branch" >/dev/null || die "branch not found: $branch"
}

append_unique_line() {
  local file="$1"
  local line="$2"
  mkdir -p "$(dirname "$file")"
  touch "$file"
  grep -Fx -- "$line" "$file" >/dev/null 2>&1 || echo "$line" >>"$file"
}

resolve_container_path() {
  local common_root="$1"
  local root_arg="$2"
  if [[ "$root_arg" == /* ]]; then
    echo "$root_arg"
  else
    echo "$common_root/$root_arg"
  fi
}

sanitize_topic_slug() {
  local topic="$1"
  topic="$(printf '%s' "$topic" | tr '\n' ' ' | sed -E 's/^[[:space:]]+|[[:space:]]+$//g')"
  [[ -n "$topic" ]] || {
    echo ""
    return 0
  }

  if command -v python3 >/dev/null 2>&1; then
    python3 -c '
import re, sys, unicodedata
s = sys.argv[1].strip().lower()
buf = []
for ch in s:
    cat = unicodedata.category(ch)
    if cat[0] in ("L", "N"):
        buf.append(ch)
    elif ch in (" ", "-", "_", "/", "\\", "."):
        buf.append("-")
    else:
        buf.append("-")
s = "".join(buf)
s = re.sub(r"-+", "-", s).strip("-")
if len(s) > 24:
    s = s[:24]
s = s.strip("-")
print(s)
' "$topic" 2>/dev/null || true
    return 0
  fi

  printf '%s' "$topic" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g; s/-+/-/g; s/^(.{1,24}).*$/\1/; s/^-+|-+$//g'
}

allocate_worktree_id() {
  local container="$1"
  local topic_slug="${2:-}"
  local prefix
  prefix="$(date +%y%m%d)"
  local rand_hex i candidate
  for i in {1..64}; do
    rand_hex="$(od -An -N2 -tx1 /dev/urandom | tr -d ' \n')"
    if [[ -n "$topic_slug" ]]; then
      candidate="${prefix}-${topic_slug}-${rand_hex}"
    else
      candidate="${prefix}-${rand_hex}"
    fi
    if [[ ! -e "$container/$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done
  die "unable to allocate worktree id under $container"
}

write_meta() {
  local worktree_root="$1"
  local worktree_id="$2"
  local worktree_branch="$3"
  local base_branch="$4"
  cat >"$worktree_root/.worktree-lite-meta" <<EOF
WORKTREE_ID=$worktree_id
WORKTREE_BRANCH=$worktree_branch
BASE_BRANCH=$base_branch
CREATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
  local wt_exclude
  wt_exclude="$(git -C "$worktree_root" rev-parse --git-path info/exclude)"
  append_unique_line "$wt_exclude" "/.worktree-lite-meta"
}

meta_value() {
  local worktree_root="$1"
  local key="$2"
  local meta_file="$worktree_root/.worktree-lite-meta"
  [[ -f "$meta_file" ]] || return 1
  local line
  line="$(grep -E "^${key}=" "$meta_file" | head -n 1 || true)"
  [[ -n "$line" ]] || return 1
  echo "${line#*=}"
}

resolve_base_branch() {
  local worktree_root="$1"
  local explicit="${2:-}"
  local common_root="$3"

  if [[ -n "$explicit" ]]; then
    echo "$explicit"
    return 0
  fi

  local meta_base
  meta_base="$(meta_value "$worktree_root" "BASE_BRANCH" || true)"
  if [[ -n "$meta_base" ]]; then
    echo "$meta_base"
    return 0
  fi

  if git -C "$common_root" show-ref --verify --quiet refs/heads/main; then
    echo "main"
    return 0
  fi
  if git -C "$common_root" show-ref --verify --quiet refs/heads/master; then
    echo "master"
    return 0
  fi

  current_branch "$common_root"
}

count_non_empty() {
  local data="$1"
  printf '%s\n' "$data" | awk 'NF{c++} END{print c+0}'
}

build_subject() {
  local files="$1"
  local count
  count="$(count_non_empty "$files")"

  if [[ "$count" -eq 0 ]]; then
    echo "同步分支改动"
    return 0
  fi

  local first_file
  first_file="$(printf '%s\n' "$files" | awk 'NF{print; exit}')"
  if [[ "$count" -eq 1 ]]; then
    echo "调整 ${first_file} 相关逻辑"
    return 0
  fi

  local modules m1 m2
  modules="$(printf '%s\n' "$files" | awk -F/ 'NF{print $1}' | awk '!seen[$0]++')"
  m1="$(printf '%s\n' "$modules" | awk 'NF{print; exit}')"
  m2="$(printf '%s\n' "$modules" | awk 'NF{if (++n==2){print; exit}}')"

  if [[ -n "$m1" && -n "$m2" && "$m1" != "$m2" ]]; then
    echo "更新 ${m1} 与 ${m2} 相关改动"
    return 0
  fi
  if [[ -n "$m1" ]]; then
    echo "更新 ${m1} 模块相关改动"
    return 0
  fi
  echo "更新 ${count} 处文件改动"
}

history_recent_subjects() {
  local common_root="$1"
  local branch="$2"
  local n="${3:-40}"
  git -C "$common_root" log --format=%s -n "$n" "$branch" 2>/dev/null || true
}

title_style_stats() {
  local common_root="$1"
  local target_branch="$2"
  local sample total conventional action_cn
  sample="$(history_recent_subjects "$common_root" "$target_branch" 40)"
  total="$(count_non_empty "$sample")"
  conventional="$(
    printf '%s\n' "$sample" \
      | grep -E '^(feat|fix|docs|refactor|perf|test|build|ci|chore|revert)(\([^)]*\))?: .+' \
      | wc -l \
      | tr -d ' '
  )"
  action_cn="$(
    printf '%s\n' "$sample" \
      | grep -E '^[^[:space:]]+：.+' \
      | wc -l \
      | tr -d ' '
  )"
  printf '%s\t%s\t%s\n' "$total" "$conventional" "$action_cn"
}

detect_title_style() {
  local common_root="$1"
  local target_branch="$2"
  local total conventional action_cn
  IFS=$'\t' read -r total conventional action_cn <<<"$(title_style_stats "$common_root" "$target_branch")"
  if [[ "$total" -le 0 ]]; then
    echo "plain"
    return 0
  fi
  if [[ $((conventional * 100 / total)) -ge 60 ]]; then
    echo "conventional"
    return 0
  fi
  if [[ $((action_cn * 100 / total)) -ge 60 ]]; then
    echo "action_cn"
    return 0
  fi
  echo "plain"
}

history_default_conventional_type() {
  local common_root="$1"
  local target_branch="$2"
  local types
  types="$(
    history_recent_subjects "$common_root" "$target_branch" 100 \
      | sed -nE 's/^([a-z]+)(\([^)]*\))?: .+/\1/p'
  )"
  if [[ -z "$types" ]]; then
    echo "feat"
    return 0
  fi
  printf '%s\n' "$types" | awk '
NF { c[$0]++ }
END {
  best="feat"; max=0;
  for (t in c) { if (c[t] > max) { max=c[t]; best=t } }
  print best
}'
}

history_conventional_scopes() {
  local common_root="$1"
  local target_branch="$2"
  history_recent_subjects "$common_root" "$target_branch" 200 \
    | sed -nE 's/^[a-z]+\(([^)]+)\): .+/\1/p' \
    | awk 'NF && !seen[$0]++ { print }'
}

is_valid_conventional_type() {
  local t="$1"
  case "$t" in
    feat|fix|docs|refactor|perf|test|build|ci|chore|revert)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

type_to_action_cn() {
  local t="$1"
  case "$t" in
    fix)
      echo "修复"
      ;;
    feat)
      echo "新增"
      ;;
    perf)
      echo "优化"
      ;;
    *)
      echo "修改"
      ;;
  esac
}

parse_conventional_title() {
  local raw="$1"
  local re='^([a-z]+)(\(([^)]+)\))?:[[:space:]]*(.+)$'
  if [[ "$raw" =~ $re ]]; then
    printf '%s\t%s\t%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[3]}" "${BASH_REMATCH[4]}"
    return 0
  fi
  printf '\t\t\n'
}

parse_action_cn_title() {
  local raw="$1"
  local re='^([^：]+)：[[:space:]]*(.+)$'
  if [[ "$raw" =~ $re ]]; then
    printf '%s\t%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    return 0
  fi
  printf '\t\n'
}

strip_title_prefix() {
  local raw="$1"
  local t s sub
  IFS=$'\t' read -r t s sub <<<"$(parse_conventional_title "$raw")"
  if [[ -n "$sub" ]]; then
    echo "$sub"
    return 0
  fi
  IFS=$'\t' read -r _action sub <<<"$(parse_action_cn_title "$raw")"
  if [[ -n "$sub" ]]; then
    echo "$sub"
    return 0
  fi
  echo "$raw"
}

primary_range_commit_subject() {
  local common_root="$1"
  local base_branch="$2"
  local source_branch="$3"
  git -C "$common_root" log --format=%s -n 1 "${base_branch}..${source_branch}" 2>/dev/null || true
}

detect_scope_from_files() {
  local files="$1"
  local known_scopes="${2:-}"

  local modules mod_count first_module
  modules="$(printf '%s\n' "$files" | awk -F/ 'NF{print $1}' | awk '!seen[$0]++')"
  mod_count="$(count_non_empty "$modules")"
  first_module="$(printf '%s\n' "$modules" | awk 'NF{print; exit}')"

  if [[ "$mod_count" -eq 1 && -n "$first_module" && -n "$known_scopes" ]]; then
    if printf '%s\n' "$known_scopes" | grep -Fxq "$first_module"; then
      echo "$first_module"
      return 0
    fi
  fi

  if printf '%s\n' "$modules" | grep -Fxq "desktop"; then
    echo "desktop"
    return 0
  fi
  if printf '%s\n' "$modules" | grep -Fxq ".codex" || printf '%s\n' "$modules" | grep -Fxq "scripts"; then
    echo "dev"
    return 0
  fi
  echo "core"
}

detect_type_from_diff() {
  local common_root="$1"
  local range="$2"
  local statuses="$3"
  local count="$4"
  local default_type="$5"
  local files="$6"

  if [[ "$count" -eq 0 ]]; then
    echo "$default_type"
    return 0
  fi

  if [[ -n "$files" ]]; then
    if printf '%s\n' "$files" | awk 'NF && $0 !~ /^docs\// && $0 !~ /\.md$/ { exit 1 }'; then
      echo "docs"
      return 0
    fi
  fi

  local patch
  patch="$(git -C "$common_root" diff "$range" || true)"
  if printf '%s\n' "$patch" | grep -Eiq 'fix|bug|error|panic|crash|异常|错误|兼容|回归'; then
    echo "fix"
    return 0
  fi
  if printf '%s\n' "$patch" | grep -Eiq 'optimi|perf|cache|speed|性能|提速'; then
    echo "perf"
    return 0
  fi
  if printf '%s\n' "$statuses" | grep -Eq '^A[[:space:]]'; then
    echo "feat"
    return 0
  fi
  echo "$default_type"
}

build_message_candidates() {
  local common_root="$1"
  local base_branch="$2"
  local source_branch="$3"

  local range files statuses count fallback_subject
  range="${base_branch}...${source_branch}"
  files="$(git -C "$common_root" diff --name-only "$range" || true)"
  statuses="$(git -C "$common_root" diff --name-status "$range" || true)"
  count="$(count_non_empty "$files")"
  fallback_subject="$(build_subject "$files")"
  if [[ "$count" -eq 0 ]]; then
    fallback_subject="同步 ${source_branch} 到 ${base_branch}"
  fi

  local style default_type known_scopes scope type
  style="$(detect_title_style "$common_root" "$base_branch")"
  default_type="$(history_default_conventional_type "$common_root" "$base_branch")"
  known_scopes="$(history_conventional_scopes "$common_root" "$base_branch")"
  scope="$(detect_scope_from_files "$files" "$known_scopes")"
  type="$(detect_type_from_diff "$common_root" "$range" "$statuses" "$count" "$default_type" "$files")"

  local primary_raw
  primary_raw="$(primary_range_commit_subject "$common_root" "$base_branch" "$source_branch")"

  local subject
  subject="$(strip_title_prefix "$primary_raw")"
  subject="${subject:-$fallback_subject}"

  local rec alt1 alt2 reason
  case "$style" in
    conventional)
      local parsed_type parsed_scope parsed_subject
      IFS=$'\t' read -r parsed_type parsed_scope parsed_subject <<<"$(parse_conventional_title "$primary_raw")"

      local final_type="$type"
      local final_scope="$scope"
      if is_valid_conventional_type "$parsed_type"; then
        final_type="$parsed_type"
      fi
      if [[ -n "$parsed_scope" ]]; then
        if [[ -z "$known_scopes" ]] || printf '%s\n' "$known_scopes" | grep -Fxq "$parsed_scope"; then
          final_scope="$parsed_scope"
        fi
      fi
      if [[ -n "$parsed_subject" ]]; then
        subject="$parsed_subject"
      fi
      subject="${subject:-$fallback_subject}"

      rec="${final_type}(${final_scope}): ${subject}"
      if [[ -n "$fallback_subject" && "$fallback_subject" != "$subject" ]]; then
        alt1="${final_type}(${final_scope}): ${fallback_subject}"
      else
        alt1="${final_type}(${final_scope}): 处理 ${count} 处文件改动"
      fi
      if [[ "$final_type" == "feat" ]]; then
        alt2="chore(${final_scope}): ${subject}"
      else
        alt2="feat(${final_scope}): ${subject}"
      fi

      local total conventional action_cn
      IFS=$'\t' read -r total conventional action_cn <<<"$(title_style_stats "$common_root" "$base_branch")"
      reason="基于 ${base_branch} 最近 ${total} 条提交（conventional=${conventional}）推断格式为 conventional commits；并优先使用待合并分支最新提交标题作为摘要。"
      ;;
    action_cn)
      local commit_action commit_subject
      IFS=$'\t' read -r commit_action commit_subject <<<"$(parse_action_cn_title "$primary_raw")"
      local action
      action="$(type_to_action_cn "$type")"
      if [[ -n "$commit_action" ]]; then
        action="$commit_action"
      fi
      if [[ -n "$commit_subject" ]]; then
        subject="$commit_subject"
      fi
      subject="${subject:-$fallback_subject}"

      rec="${action}：${subject}"
      alt1="${action}：${fallback_subject}"
      alt2="修改：${subject}"
      reason="基于 ${base_branch} 最近提交推断格式为「动作：内容」，并优先使用待合并分支最新提交标题作为摘要。"
      ;;
    *)
      rec="$subject"
      alt1="$fallback_subject"
      alt2="更新 ${count} 处文件改动"
      reason="未检测到稳定的提交标题格式，回退为纯标题输出，并优先使用待合并分支最新提交标题作为摘要。"
      ;;
  esac

  printf '%s\t%s\t%s\t%s\n' "$rec" "$alt1" "$alt2" "$reason"
}

shell_quote() {
  printf '%q' "$1"
}

history_branch_token() {
  local branch="$1"
  printf '%s' "$branch" | sed -E 's#[^A-Za-z0-9._-]+#_#g'
}

source_series_key() {
  local source_branch="$1"
  if [[ "$source_branch" =~ ^worktree-lite/[0-9]{6}-(.+)-[0-9a-f]{4}$ ]]; then
    echo "${BASH_REMATCH[1]}"
    return 0
  fi
  echo "$source_branch"
}

history_meta_path() {
  local common_root="$1"
  local target_branch="$2"
  echo "$common_root/.worktree-lite/history/last-$(history_branch_token "$target_branch").meta"
}

history_files_path() {
  local common_root="$1"
  local target_branch="$2"
  echo "$common_root/.worktree-lite/history/last-$(history_branch_token "$target_branch").files"
}

history_meta_get() {
  local meta_file="$1"
  local key="$2"
  [[ -f "$meta_file" ]] || return 1
  local line
  line="$(grep -E "^${key}=" "$meta_file" | head -n 1 || true)"
  [[ -n "$line" ]] || return 1
  echo "${line#*=}"
}

collect_changed_files_range() {
  local common_root="$1"
  local range="$2"
  local out_file="$3"
  git -C "$common_root" diff --name-only "$range" | awk 'NF' | sort -u >"$out_file"
}

files_jaccard_percent() {
  local file_a="$1"
  local file_b="$2"
  local inter_count union_count

  inter_count="$(comm -12 "$file_a" "$file_b" | awk 'NF{c++} END{print c+0}')"
  union_count="$(
    {
      cat "$file_a"
      cat "$file_b"
    } | awk 'NF' | sort -u | awk 'NF{c++} END{print c+0}'
  )"

  if [[ "$union_count" -eq 0 ]]; then
    echo 100
    return 0
  fi
  echo $((inter_count * 100 / union_count))
}

reuse_last_message_decision() {
  local common_root="$1"
  local target_branch="$2"
  local source_branch="$3"
  local current_files_file="$4"

  local meta_file files_file last_source last_source_key current_source_key last_message sim_pct
  meta_file="$(history_meta_path "$common_root" "$target_branch")"
  files_file="$(history_files_path "$common_root" "$target_branch")"
  current_source_key="$(source_series_key "$source_branch")"

  [[ -s "$current_files_file" ]] || {
    printf '%s\t%s\t%s\n' "0" "" "0"
    return 0
  }
  [[ -f "$meta_file" && -f "$files_file" ]] || {
    printf '%s\t%s\t%s\n' "0" "" "0"
    return 0
  }

  last_source="$(history_meta_get "$meta_file" "SOURCE_BRANCH" || true)"
  last_source_key="$(history_meta_get "$meta_file" "SOURCE_KEY" || true)"
  last_message="$(history_meta_get "$meta_file" "MESSAGE" || true)"
  if [[ -z "$last_source" || -z "$last_message" ]]; then
    printf '%s\t%s\t%s\n' "0" "" "0"
    return 0
  fi
  if [[ -z "$last_source_key" ]]; then
    last_source_key="$last_source"
  fi
  if [[ "$last_source_key" != "$current_source_key" ]]; then
    printf '%s\t%s\t%s\n' "0" "" "0"
    return 0
  fi

  sim_pct="$(files_jaccard_percent "$current_files_file" "$files_file")"
  if [[ "$sim_pct" -ge 60 ]]; then
    printf '%s\t%s\t%s\n' "1" "$last_message" "$sim_pct"
    return 0
  fi
  printf '%s\t%s\t%s\n' "0" "" "$sim_pct"
}

record_merge_history() {
  local common_root="$1"
  local target_branch="$2"
  local source_branch="$3"
  local new_commit="$4"
  local message="$5"
  local merged_files_file="$6"

  local history_dir meta_file files_file
  local source_key
  history_dir="$common_root/.worktree-lite/history"
  mkdir -p "$history_dir"
  source_key="$(source_series_key "$source_branch")"

  meta_file="$(history_meta_path "$common_root" "$target_branch")"
  files_file="$(history_files_path "$common_root" "$target_branch")"
  cp "$merged_files_file" "$files_file"

  cat >"$meta_file" <<EOF
TARGET_BRANCH=$target_branch
SOURCE_BRANCH=$source_branch
SOURCE_KEY=$source_key
COMMIT=$new_commit
MESSAGE=$message
UPDATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
}

find_worktree_by_branch() {
  local common_root="$1"
  local branch="$2"
  git -C "$common_root" worktree list --porcelain \
    | awk -v b="$branch" '
      /^worktree / { wt=substr($0,10) }
      /^branch / {
        br=substr($0,8)
        sub(/^refs\/heads\//,"",br)
        if (br==b) { print wt; exit }
      }
    '
}

cleanup_source_worktree_after_merge() {
  local common_root="$1"
  local source_branch="$2"

  local source_worktree
  source_worktree="$(find_worktree_by_branch "$common_root" "$source_branch" || true)"
  if [[ -n "$source_worktree" && "$source_worktree" != "$common_root" ]]; then
    if (cd "$common_root" && git worktree remove --force "$source_worktree" >/dev/null 2>&1); then
      echo "CLEANUP_WORKTREE_REMOVED=$source_worktree"
    else
      echo "NOTICE=failed to remove source worktree: $source_worktree"
      echo "NOTICE=manual cleanup: git -C $common_root worktree remove --force $source_worktree"
    fi
  fi

  if [[ "$source_branch" == worktree-lite/* ]]; then
    if git -C "$common_root" show-ref --verify --quiet "refs/heads/$source_branch"; then
      if git -C "$common_root" branch -D "$source_branch" >/dev/null 2>&1; then
        echo "CLEANUP_BRANCH_DELETED=$source_branch"
      else
        echo "NOTICE=failed to delete source branch: $source_branch"
      fi
    fi
  fi
}

print_codex_auto_merge_payload() {
  local target_branch="$1"
  local source_branch="$2"
  local message="$3"
  local cmd="$4"
  local sim_pct="$5"

  if ! command -v python3 >/dev/null 2>&1; then
    echo "NOTICE=python3 not found; fallback to plain format."
    return 1
  fi

  python3 - "$target_branch" "$source_branch" "$message" "$cmd" "$sim_pct" <<'PY'
import json, sys

target, source, message, cmd, sim_pct = sys.argv[1:]
payload = {
    "auto_merge": True,
    "reason": f"与上次同分支改动相似度 {sim_pct}% ，复用上次提交标题并跳过提问。",
    "command": cmd,
    "context": {
        "target_branch": target,
        "source_branch": source,
        "message": message,
        "similarity_percent": int(sim_pct),
    },
}
print(json.dumps(payload, ensure_ascii=False, indent=2))
PY
}

print_codex_merge_options_payload() {
  local target_branch="$1"
  local source_branch="$2"
  local rec="$3"
  local alt1="$4"
  local alt2="$5"
  local cmd1="$6"
  local cmd2="$7"
  local cmd3="$8"

  if ! command -v python3 >/dev/null 2>&1; then
    echo "NOTICE=python3 not found; fallback to plain format."
    return 1
  fi

  python3 - "$target_branch" "$source_branch" "$rec" "$alt1" "$alt2" "$cmd1" "$cmd2" "$cmd3" <<'PY'
import json, sys

target, source, rec, alt1, alt2, cmd1, cmd2, cmd3 = sys.argv[1:]

payload = {
    "questions": [
        {
            "header": "合并选项",
            "id": "merge_choice",
            "question": f"选择合并标题（目标分支 {target}）。",
            "options": [
                {
                    "label": "合并推荐 (Recommended)",
                    "description": f"合并到 {target}，提交标题：{rec}",
                },
                {
                    "label": "合并备选1",
                    "description": f"合并到 {target}，提交标题：{alt1}",
                },
                {
                    "label": "合并备选2",
                    "description": f"合并到 {target}，提交标题：{alt2}",
                },
            ],
        },
        {
            "header": "暂不合并",
            "id": "hold_choice",
            "question": "若本次不合并，选择处理方式（仅在不合并时生效）。",
            "options": [
                {
                    "label": "继续修改 (Recommended)",
                    "description": "保留当前分支并继续修改。",
                },
                {
                    "label": "暂不合并",
                    "description": "保持现状，等待后续指令。",
                },
            ],
        },
    ],
    "selection_map": {
        "merge_choice": {
            "合并推荐 (Recommended)": cmd1,
            "合并备选1": cmd2,
            "合并备选2": cmd3,
        },
        "hold_choice": {
            "继续修改 (Recommended)": "NO_MERGE_CONTINUE_EDIT",
            "暂不合并": "NO_MERGE_HOLD",
        },
    },
    "context": {
        "target_branch": target,
        "source_branch": source,
        "messages": {
            "recommended": rec,
            "alternative_1": alt1,
            "alternative_2": alt2,
        },
    },
}

print(json.dumps(payload, ensure_ascii=False, indent=2))
PY
}

cmd_init() {
  local base_branch=""
  local root_arg=".worktree-lite"
  local worktree_id=""
  local topic=""
  local topic_slug=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base)
        [[ $# -ge 2 ]] || die "--base requires a value"
        base_branch="$2"
        shift 2
        ;;
      --root)
        [[ $# -ge 2 ]] || die "--root requires a value"
        root_arg="$2"
        shift 2
        ;;
      --id)
        [[ $# -ge 2 ]] || die "--id requires a value"
        worktree_id="$2"
        shift 2
        ;;
      --topic)
        [[ $# -ge 2 ]] || die "--topic requires a value"
        topic="$2"
        shift 2
        ;;
      *)
        die "unknown init option: $1"
        ;;
    esac
  done

  local common_root container branch worktree_root base_exclude
  common_root="$(repo_common_root)"
  if [[ -z "$base_branch" ]]; then
    base_branch="$(current_branch "$common_root")"
  fi
  ensure_branch_exists "$common_root" "$base_branch"

  container="$(resolve_container_path "$common_root" "$root_arg")"
  mkdir -p "$container"
  append_unique_line "$common_root/.git/info/exclude" "/.worktree-lite/"

  topic_slug="$(sanitize_topic_slug "$topic")"
  if [[ -z "$worktree_id" ]]; then
    worktree_id="$(allocate_worktree_id "$container" "$topic_slug")"
  fi
  worktree_root="$container/$worktree_id"
  [[ ! -e "$worktree_root" ]] || die "worktree path already exists: $worktree_root"

  branch="worktree-lite/$worktree_id"
  if git -C "$common_root" show-ref --verify --quiet "refs/heads/$branch"; then
    die "branch already exists: $branch"
  fi

  git -C "$common_root" worktree add -b "$branch" "$worktree_root" "$base_branch"
  write_meta "$worktree_root" "$worktree_id" "$branch" "$base_branch"

  base_exclude="$(git -C "$common_root" rev-parse --git-path info/exclude)"
  append_unique_line "$base_exclude" "/.worktree-lite/"

  echo "WORKTREE_ID=$worktree_id"
  echo "WORKTREE_ROOT=$worktree_root"
  echo "WORKTREE_BRANCH=$branch"
  echo "BASE_BRANCH=$base_branch"
  echo "NEXT=cd \"$worktree_root\""
}

cmd_review() {
  local base_branch=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base)
        [[ $# -ge 2 ]] || die "--base requires a value"
        base_branch="$2"
        shift 2
        ;;
      *)
        die "unknown review option: $1"
        ;;
    esac
  done

  local worktree_root common_root source_branch range
  worktree_root="$(repo_root)"
  common_root="$(repo_common_root)"
  source_branch="$(current_branch "$worktree_root")"
  base_branch="$(resolve_base_branch "$worktree_root" "$base_branch" "$common_root")"

  ensure_branch_exists "$common_root" "$base_branch"
  ensure_branch_exists "$common_root" "$source_branch"

  range="${base_branch}...${source_branch}"
  echo "SOURCE_BRANCH=$source_branch"
  echo "BASE_BRANCH=$base_branch"
  echo "RANGE=$range"
  echo
  echo "== Changed Files =="
  git -C "$common_root" diff --name-status "$range" || true
  echo
  echo "== Diff Stat =="
  git -C "$common_root" diff --stat "$range" || true
  echo
  echo "== Commits =="
  git -C "$common_root" log --oneline --no-decorate "$range" || true
  echo
  echo "== Working Tree Status =="
  git -C "$worktree_root" status --short
}

cmd_propose_message() {
  local base_branch=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base)
        [[ $# -ge 2 ]] || die "--base requires a value"
        base_branch="$2"
        shift 2
        ;;
      *)
        die "unknown propose-message option: $1"
        ;;
    esac
  done

  local worktree_root common_root source_branch rec alt1 alt2 reason
  worktree_root="$(repo_root)"
  common_root="$(repo_common_root)"
  source_branch="$(current_branch "$worktree_root")"
  base_branch="$(resolve_base_branch "$worktree_root" "$base_branch" "$common_root")"

  ensure_branch_exists "$common_root" "$base_branch"
  ensure_branch_exists "$common_root" "$source_branch"

  IFS=$'\t' read -r rec alt1 alt2 reason <<<"$(build_message_candidates "$common_root" "$base_branch" "$source_branch")"

  echo "推荐：$rec"
  echo "备选1：$alt1"
  echo "备选2：$alt2"
  echo "理由：$reason"
}

cmd_merge_options() {
  local target_branch=""
  local source_branch=""
  local format="plain"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --target)
        [[ $# -ge 2 ]] || die "--target requires a value"
        target_branch="$2"
        shift 2
        ;;
      --source)
        [[ $# -ge 2 ]] || die "--source requires a value"
        source_branch="$2"
        shift 2
        ;;
      --format)
        [[ $# -ge 2 ]] || die "--format requires a value"
        format="$2"
        shift 2
        ;;
      *)
        die "unknown merge-options option: $1"
        ;;
    esac
  done

  local worktree_root common_root rec alt1 alt2 reason
  local current_files_file auto_reuse auto_message sim_pct
  worktree_root="$(repo_root)"
  common_root="$(repo_common_root)"

  if [[ -z "$source_branch" ]]; then
    source_branch="$(current_branch "$worktree_root")"
  fi
  if [[ -z "$target_branch" ]]; then
    target_branch="$(resolve_base_branch "$worktree_root" "" "$common_root")"
  fi

  ensure_branch_exists "$common_root" "$target_branch"
  ensure_branch_exists "$common_root" "$source_branch"
  [[ "$target_branch" != "$source_branch" ]] || die "target and source branch must be different"

  IFS=$'\t' read -r rec alt1 alt2 reason <<<"$(build_message_candidates "$common_root" "$target_branch" "$source_branch")"
  local cmd1 cmd2 cmd3
  cmd1="bash .codex/skills/worktree-lite/scripts/worktree-lite.sh merge --target $(shell_quote "$target_branch") --source $(shell_quote "$source_branch") --message $(shell_quote "$rec")"
  cmd2="bash .codex/skills/worktree-lite/scripts/worktree-lite.sh merge --target $(shell_quote "$target_branch") --source $(shell_quote "$source_branch") --message $(shell_quote "$alt1")"
  cmd3="bash .codex/skills/worktree-lite/scripts/worktree-lite.sh merge --target $(shell_quote "$target_branch") --source $(shell_quote "$source_branch") --message $(shell_quote "$alt2")"

  current_files_file="$(mktemp)"
  collect_changed_files_range "$common_root" "${target_branch}...${source_branch}" "$current_files_file"
  IFS=$'\t' read -r auto_reuse auto_message sim_pct <<<"$(reuse_last_message_decision "$common_root" "$target_branch" "$source_branch" "$current_files_file")"
  rm -f "$current_files_file"

  if [[ "$auto_reuse" == "1" && -n "$auto_message" ]]; then
    local auto_cmd
    auto_cmd="bash .codex/skills/worktree-lite/scripts/worktree-lite.sh merge --target $(shell_quote "$target_branch") --source $(shell_quote "$source_branch") --message $(shell_quote "$auto_message")"
    case "$format" in
      plain)
        echo "AUTO_DECISION=MERGE_WITH_LAST_MESSAGE"
        echo "TARGET_BRANCH=$target_branch"
        echo "SOURCE_BRANCH=$source_branch"
        echo "AUTO_MESSAGE=$auto_message"
        echo "AUTO_SIMILARITY=${sim_pct}%"
        echo "AUTO_CMD=$auto_cmd"
        echo "AUTO_REASON=与上次同分支改动相似，直接复用上次提交标题并跳过提问。"
        return 0
        ;;
      codex)
        print_codex_auto_merge_payload "$target_branch" "$source_branch" "$auto_message" "$auto_cmd" "$sim_pct" && return 0
        echo "NOTICE=codex auto payload failed; fallback to plain auto decision."
        echo "AUTO_DECISION=MERGE_WITH_LAST_MESSAGE"
        echo "AUTO_MESSAGE=$auto_message"
        echo "AUTO_SIMILARITY=${sim_pct}%"
        echo "AUTO_CMD=$auto_cmd"
        return 0
        ;;
      *)
        die "--format only supports plain|codex"
        ;;
    esac
  fi

  case "$format" in
    plain)
      echo "TARGET_BRANCH=$target_branch"
      echo "SOURCE_BRANCH=$source_branch"
      echo "OPTION_1=合并到 ${target_branch}（推荐）：${rec}"
      echo "OPTION_2=合并到 ${target_branch}（备选1）：${alt1}"
      echo "OPTION_3=合并到 ${target_branch}（备选2）：${alt2}"
      echo "OPTION_4=继续修改（暂不合并）"
      echo "OPTION_5=暂不合并，等待后续指令"
      echo "OPTION_1_CMD=$cmd1"
      echo "OPTION_2_CMD=$cmd2"
      echo "OPTION_3_CMD=$cmd3"
      echo "理由：$reason"
      ;;
    codex)
      print_codex_merge_options_payload "$target_branch" "$source_branch" "$rec" "$alt1" "$alt2" "$cmd1" "$cmd2" "$cmd3" || {
        echo "OPTION_1=合并到 ${target_branch}（推荐）：${rec}"
        echo "OPTION_2=合并到 ${target_branch}（备选1）：${alt1}"
        echo "OPTION_3=合并到 ${target_branch}（备选2）：${alt2}"
        echo "OPTION_4=继续修改（暂不合并）"
        echo "OPTION_5=暂不合并，等待后续指令"
      }
      ;;
    *)
      die "--format only supports plain|codex"
      ;;
  esac
}

cmd_merge() {
  local target_branch=""
  local source_branch=""
  local message=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --target)
        [[ $# -ge 2 ]] || die "--target requires a value"
        target_branch="$2"
        shift 2
        ;;
      --source)
        [[ $# -ge 2 ]] || die "--source requires a value"
        source_branch="$2"
        shift 2
        ;;
      --message)
        [[ $# -ge 2 ]] || die "--message requires a value"
        message="$2"
        shift 2
        ;;
      *)
        die "unknown merge option: $1"
        ;;
    esac
  done

  [[ -n "$target_branch" ]] || die "--target is required"

  local worktree_root common_root current_main_branch action subject count
  local target_old target_new merge_tmp_root merge_tmp_id merge_worktree
  local main_dirty=0
  local overlap_skipped=0
  local should_fold=0
  local fold_notice=0
  local common_root_status=""
  local common_root_tracked_status=""
  local history_meta_file history_files_file last_source_branch last_source_key current_source_key last_target_commit last_message
  local merged_files_tmp=""
  local -a user_changed_paths=() merge_changed_paths=() sync_paths=()
  local -A user_changed_set=()
  worktree_root="$(repo_root)"
  common_root="$(repo_common_root)"
  if [[ -z "$source_branch" ]]; then
    source_branch="$(current_branch "$worktree_root")"
  fi

  ensure_branch_exists "$common_root" "$target_branch"
  ensure_branch_exists "$common_root" "$source_branch"
  [[ "$target_branch" != "$source_branch" ]] || die "target and source branch must be different"

  if [[ -n "$(git -C "$worktree_root" status --porcelain)" ]]; then
    die "current worktree is dirty; commit or clean before merge"
  fi

  current_main_branch="$(git -C "$common_root" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  common_root_status="$(git -C "$common_root" status --porcelain)"
  common_root_tracked_status="$(git -C "$common_root" status --porcelain --untracked-files=no)"
  if [[ "$current_main_branch" != "HEAD" && "$current_main_branch" == "$target_branch" ]]; then
    [[ -n "$common_root_status" ]] && main_dirty=1
  fi

  target_old="$(git -C "$common_root" rev-parse "$target_branch")"

  history_meta_file="$(history_meta_path "$common_root" "$target_branch")"
  history_files_file="$(history_files_path "$common_root" "$target_branch")"
  current_source_key="$(source_series_key "$source_branch")"
  last_source_branch="$(history_meta_get "$history_meta_file" "SOURCE_BRANCH" || true)"
  last_source_key="$(history_meta_get "$history_meta_file" "SOURCE_KEY" || true)"
  last_target_commit="$(history_meta_get "$history_meta_file" "COMMIT" || true)"
  last_message="$(history_meta_get "$history_meta_file" "MESSAGE" || true)"
  if [[ -z "$last_source_key" ]]; then
    last_source_key="$last_source_branch"
  fi
  if [[ -n "$last_source_key" && -n "$last_target_commit" ]]; then
    if [[ "$last_source_key" == "$current_source_key" && "$last_target_commit" == "$target_old" ]]; then
      if git -C "$common_root" rev-parse --verify --quiet "${target_old}^" >/dev/null; then
        should_fold=1
      fi
    fi
  fi

  if git -C "$common_root" diff --quiet "$target_branch...$source_branch"; then
    echo "INFO: no differences between $source_branch and $target_branch"
    return 0
  fi

  if [[ -z "$message" ]]; then
    if [[ "$should_fold" -eq 1 && -n "$last_message" ]]; then
      message="$last_message"
    else
      local rec alt1 alt2 reason
      IFS=$'\t' read -r rec alt1 alt2 reason <<<"$(build_message_candidates "$common_root" "$target_branch" "$source_branch")"
      message="$rec"
    fi
    echo "AUTO_MESSAGE=$message"
  fi

  # Snapshot user-local paths first; keep them untouched during post-merge alignment.
  if [[ "$current_main_branch" == "$target_branch" ]]; then
    while IFS= read -r -d '' path; do
      [[ -n "$path" ]] || continue
      if [[ -z "${user_changed_set[$path]+x}" ]]; then
        user_changed_set["$path"]=1
        user_changed_paths+=("$path")
      fi
    done < <(
      {
        git -C "$common_root" diff --name-only -z
        git -C "$common_root" diff --cached --name-only -z
        git -C "$common_root" ls-files --others --exclude-standard -z
      }
    )
  fi

  if [[ "$current_main_branch" == "$target_branch" && -z "$common_root_tracked_status" ]]; then
    if ! git -C "$common_root" merge --squash --no-commit "$source_branch"; then
      die "squash merge failed; resolve conflicts in $common_root and retry"
    fi
    if [[ "$should_fold" -eq 1 ]]; then
      git -C "$common_root" reset --soft HEAD~1
      fold_notice=1
    fi
    git -C "$common_root" commit -m "$message" >/dev/null
    target_new="$(git -C "$common_root" rev-parse "$target_branch")"

    merged_files_tmp="$(mktemp)"
    collect_changed_files_range "$common_root" "${target_old}..${target_new}" "$merged_files_tmp"
    record_merge_history "$common_root" "$target_branch" "$source_branch" "$target_new" "$message" "$merged_files_tmp"
    rm -f "$merged_files_tmp"

    cleanup_source_worktree_after_merge "$common_root" "$source_branch"

    echo "MERGED_FROM=$source_branch"
    echo "MERGED_INTO=$target_branch"
    echo "OLD_TARGET_COMMIT=$target_old"
    echo "NEW_TARGET_COMMIT=$target_new"
    echo "COMMIT_MESSAGE=$message"
    if [[ "$fold_notice" -eq 1 ]]; then
      echo "NOTICE=folded previous commit from same source branch into current merge commit."
    fi
    return 0
  fi

  merge_tmp_root="$(resolve_container_path "$common_root" ".worktree-lite/.merge-tmp")"
  mkdir -p "$merge_tmp_root"
  merge_tmp_id="$(date +%y%m%d-%H%M%S)-$(od -An -N2 -tx1 /dev/urandom | tr -d ' \n')"
  merge_worktree="$merge_tmp_root/$merge_tmp_id"

  git -C "$common_root" worktree add --detach "$merge_worktree" "$target_branch" >/dev/null

  if ! git -C "$merge_worktree" merge --squash --no-commit "$source_branch"; then
    echo "KEEP_MERGE_WORKTREE=$merge_worktree"
    die "squash merge failed in temp worktree; inspect and resolve there"
  fi
  if [[ "$should_fold" -eq 1 ]]; then
    git -C "$merge_worktree" reset --soft HEAD~1
    fold_notice=1
  fi
  git -C "$merge_worktree" commit -m "$message" >/dev/null
  target_new="$(git -C "$merge_worktree" rev-parse HEAD)"

  git -C "$common_root" update-ref "refs/heads/$target_branch" "$target_new" "$target_old"
  git -C "$common_root" worktree remove --force "$merge_worktree" >/dev/null

  # If target branch is checked out here, align merged paths not touched by user,
  # so the index/worktree won't show a full reverse-staged snapshot.
  if [[ "$current_main_branch" == "$target_branch" ]]; then
    while IFS= read -r -d '' path; do
      [[ -n "$path" ]] || continue
      merge_changed_paths+=("$path")
    done < <(git -C "$common_root" diff --name-only -z "$target_old..$target_new")

    if [[ "${#merge_changed_paths[@]}" -gt 0 ]]; then
      local mp
      for mp in "${merge_changed_paths[@]}"; do
        if [[ -n "${user_changed_set[$mp]+x}" ]]; then
          overlap_skipped=$((overlap_skipped + 1))
          continue
        fi
        sync_paths+=("$mp")
      done
    fi

    if [[ "${#sync_paths[@]}" -gt 0 ]]; then
      if git -C "$common_root" restore --source=HEAD --staged --worktree -- "${sync_paths[@]}"; then
        echo "NOTICE=auto-aligned ${#sync_paths[@]} merged path(s) in target worktree; local edits were kept."
      else
        echo "NOTICE=auto-align skipped due to path conflicts; local edits are still kept."
      fi
    fi

    if [[ "$overlap_skipped" -gt 0 ]]; then
      echo "NOTICE=skipped auto-align for ${overlap_skipped} merged path(s) because they overlap your local edits."
    fi
  fi

  merged_files_tmp="$(mktemp)"
  collect_changed_files_range "$common_root" "${target_old}..${target_new}" "$merged_files_tmp"
  record_merge_history "$common_root" "$target_branch" "$source_branch" "$target_new" "$message" "$merged_files_tmp"
  rm -f "$merged_files_tmp"

  cleanup_source_worktree_after_merge "$common_root" "$source_branch"

  echo "MERGED_FROM=$source_branch"
  echo "MERGED_INTO=$target_branch"
  echo "OLD_TARGET_COMMIT=$target_old"
  echo "NEW_TARGET_COMMIT=$target_new"
  echo "COMMIT_MESSAGE=$message"
  if [[ "$fold_notice" -eq 1 ]]; then
    echo "NOTICE=folded previous commit from same source branch into current merge commit."
  fi
  if [[ "$main_dirty" -eq 1 ]]; then
    echo "NOTICE=target worktree has local uncommitted changes; they were kept in place."
    echo "NOTICE=target branch pointer moved, so git status in target worktree may look different."
  fi
}

main() {
  [[ $# -gt 0 ]] || {
    usage
    exit 1
  }

  local cmd="$1"
  shift
  case "$cmd" in
    help|-h|--help)
      usage
      ;;
    init)
      cmd_init "$@"
      ;;
    review)
      cmd_review "$@"
      ;;
    merge-options)
      cmd_merge_options "$@"
      ;;
    propose-message)
      cmd_propose_message "$@"
      ;;
    merge)
      cmd_merge "$@"
      ;;
    *)
      die "unknown command: $cmd"
      ;;
  esac
}

main "$@"
