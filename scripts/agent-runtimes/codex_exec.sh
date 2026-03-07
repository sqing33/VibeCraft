#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${VIBE_TREE_ARTIFACT_DIR:-}"
prompt="${VIBE_TREE_PROMPT:-}"
system_prompt="${VIBE_TREE_SYSTEM_PROMPT:-}"
model="${VIBE_TREE_MODEL:-}"
workspace="${VIBE_TREE_WORKSPACE:-$PWD}"
status="ok"
summary_text=""
next_action=""
modified_code="false"

if [[ -n "$artifact_dir" ]]; then
  mkdir -p "$artifact_dir"
fi
final_file="${artifact_dir:+$artifact_dir/final_message.md}"
summary_file="${artifact_dir:+$artifact_dir/summary.json}"
artifacts_file="${artifact_dir:+$artifact_dir/artifacts.json}"
raw_log="${artifact_dir:+$artifact_dir/raw_output.log}"
patch_file="${artifact_dir:+$artifact_dir/patch.diff}"

cleanup_files=()
if [[ -z "$final_file" ]]; then
  final_file="$(mktemp)"
  cleanup_files+=("$final_file")
fi
if [[ -z "$raw_log" ]]; then
  raw_log="$(mktemp)"
  cleanup_files+=("$raw_log")
fi
trap 'for f in "${cleanup_files[@]:-}"; do [[ -n "$f" ]] && rm -f "$f"; done' EXIT

combined_prompt="$prompt"
if [[ -n "$system_prompt" ]]; then
  combined_prompt=$'System instructions:
'"$system_prompt"$'

User request:
'"$prompt"
fi

if command -v codex >/dev/null 2>&1; then
  set +e
  printf '%s' "$combined_prompt" | codex exec --color never --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox -C "$workspace" ${model:+--model "$model"} -o "$final_file" >"${raw_log:-/dev/null}" 2>&1
  exit_code=$?
  set -e
else
  echo "codex CLI not found" >"${raw_log:-/dev/stderr}"
  exit_code=127
fi

if [[ -f "$final_file" ]]; then
  cat "$final_file"
  summary_text="$(tr -d '
' < "$final_file" | tail -n 12 | tr '
' ' ' | sed 's/[[:space:]]\+/ /g' | sed 's/^ //; s/ $//')"
fi
if [[ -z "$summary_text" && -n "$raw_log" && -f "$raw_log" ]]; then
  summary_text="$(tail -n 20 "$raw_log" | tr '
' ' ' | sed 's/[[:space:]]\+/ /g' | sed 's/^ //; s/ $//')"
fi
if [[ -z "$summary_text" ]]; then
  summary_text="CLI run finished"
fi
if [[ $exit_code -ne 0 ]]; then
  status="error"
  next_action="Inspect raw_output.log for failure details"
fi
if git -C "$workspace" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if git -C "$workspace" diff --quiet --ignore-submodules --exit-code; then
    modified_code="false"
  else
    modified_code="true"
    git -C "$workspace" diff --no-ext-diff > "$patch_file" || true
  fi
fi
if [[ -n "$summary_file" ]]; then
  export status summary_text modified_code next_action
  python3 - <<'JSON' > "$summary_file"
import json, os
print(json.dumps({
  "status": os.environ.get("status", "ok"),
  "summary": os.environ.get("summary_text", ""),
  "modified_code": os.environ.get("modified_code", "false").lower() == "true",
  "next_action": os.environ.get("next_action", ""),
  "key_files": [],
}, ensure_ascii=False))
JSON
fi
if [[ -n "$artifacts_file" ]]; then
  export summary_text
  python3 - <<'JSON' > "$artifacts_file"
import json, os
print(json.dumps({"artifacts": [{
  "kind": "cli_session_summary",
  "title": "Codex Final Summary",
  "summary": os.environ.get("summary_text", ""),
}]}, ensure_ascii=False))
JSON
fi
exit $exit_code
