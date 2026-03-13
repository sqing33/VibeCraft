#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${VIBECRAFT_ARTIFACT_DIR:-}"
prompt="${VIBECRAFT_PROMPT:-}"
system_prompt="${VIBECRAFT_SYSTEM_PROMPT:-}"
model="${VIBECRAFT_MODEL:-}"
model_id="${VIBECRAFT_MODEL_ID:-}"
workspace="${VIBECRAFT_WORKSPACE:-$PWD}"
cli_cmd="${VIBECRAFT_CLI_COMMAND_PATH:-codex}"
resume_session_id="${VIBECRAFT_RESUME_SESSION_ID:-}"
cli_tool_id="${VIBECRAFT_CLI_TOOL_ID:-codex}"
openai_api_key="${OPENAI_API_KEY:-}"
openai_base_url="${OPENAI_BASE_URL:-${VIBECRAFT_BASE_URL:-}}"
codex_home="${CODEX_HOME:-}"
status="ok"
summary_text=""
next_action=""
modified_code="false"

mkdir -p "${artifact_dir:-$(mktemp -d)}"
final_file="${artifact_dir:+$artifact_dir/final_message.md}"
summary_file="${artifact_dir:+$artifact_dir/summary.json}"
artifacts_file="${artifact_dir:+$artifact_dir/artifacts.json}"
session_file="${artifact_dir:+$artifact_dir/session.json}"
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

ensure_codex_home() {
  local tool_id="$1"
  local base_url="$2"
  local target_home="$3"
  if [[ -z "$target_home" ]]; then
    local xdg_data_home="${XDG_DATA_HOME:-}"
    if [[ -z "$xdg_data_home" ]]; then
      xdg_data_home="$HOME/.local/share"
    fi
    target_home="$xdg_data_home/vibecraft/managed-clis/${tool_id:-codex}"
  fi
  mkdir -p "$target_home"
  if [[ -z "$base_url" ]]; then
    base_url="https://api.openai.com/v1"
  fi
  cat > "$target_home/config.toml" <<TOML
model_provider = "vibecraft"

[model_providers.vibecraft]
name = "vibecraft-managed"
base_url = "$base_url"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
TOML
  chmod 600 "$target_home/config.toml"
  printf '%s' "$target_home"
}

if [[ -n "$openai_api_key" || -n "$openai_base_url" ]]; then
  codex_home="$(ensure_codex_home "$cli_tool_id" "$openai_base_url" "$codex_home")"
  export CODEX_HOME="$codex_home"
fi

combined_prompt="$prompt"
if [[ -n "$system_prompt" ]]; then
  combined_prompt=$'System instructions:\n'"$system_prompt"$'\n\nUser request:\n'"$prompt"
fi

if ! command -v "$cli_cmd" >/dev/null 2>&1; then
  echo "CLI command not found: $cli_cmd" >"$raw_log"
  exit_code=127
else
  set +e
  if [[ -n "$resume_session_id" ]]; then
    (
      cd "$workspace"
      printf '%s' "$combined_prompt" | "$cli_cmd" exec resume --json --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox ${model:+--model "$model"} -o "$final_file" "$resume_session_id" - 2>>"$raw_log"
    ) | tee -a "$raw_log"
  else
    (
      cd "$workspace"
      printf '%s' "$combined_prompt" | "$cli_cmd" exec --json --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox ${model:+--model "$model"} -o "$final_file" - 2>>"$raw_log"
    ) | tee -a "$raw_log"
  fi
  exit_code=$?
  set -e
fi

session_id="$(python3 - "$raw_log" <<'PY'
import json, sys
from pathlib import Path

def walk(obj):
    if isinstance(obj, dict):
        if isinstance(obj.get('thread_id'), str) and obj['thread_id'].strip():
            return obj['thread_id'].strip()
        thread = obj.get('thread')
        if isinstance(thread, dict) and isinstance(thread.get('id'), str) and thread['id'].strip():
            return thread['id'].strip()
        if isinstance(obj.get('session_id'), str) and obj['session_id'].strip():
            return obj['session_id'].strip()
        for v in obj.values():
            got = walk(v)
            if got:
                return got
    elif isinstance(obj, list):
        for item in obj:
            got = walk(item)
            if got:
                return got
    return ''

path = Path(sys.argv[1])
if path.exists():
    for raw in path.read_text(encoding='utf-8', errors='ignore').splitlines():
        raw = raw.strip()
        if not raw:
            continue
        try:
            obj = json.loads(raw)
        except Exception:
            continue
        got = walk(obj)
        if got:
            print(got)
            break
PY
)"

if [[ -f "$final_file" ]]; then
  summary_text="$(python3 - "$final_file" <<'PY'
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding='utf-8', errors='ignore').strip()
print(' '.join(text.splitlines()[-12:]).strip())
PY
)"
fi
if [[ -z "$summary_text" && -f "$raw_log" ]]; then
  summary_text="$(python3 - "$raw_log" <<'PY'
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding='utf-8', errors='ignore').strip().splitlines()
print(' '.join(text[-20:]).strip())
PY
)"
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
    if [[ -n "$patch_file" ]]; then git -C "$workspace" diff --no-ext-diff > "$patch_file" || true; fi
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
if [[ -n "$session_file" && -n "$session_id" ]]; then
  export session_id model resumed_flag="false"
  if [[ -n "$resume_session_id" ]]; then export resumed_flag="true"; fi
  python3 - <<'JSON' > "$session_file"
import json, os
print(json.dumps({
  "tool_id": "codex",
  "session_id": os.environ.get("session_id", ""),
  "model": os.environ.get("model", "") or os.environ.get("model_id", ""),
  "resumed": os.environ.get("resumed_flag", "false").lower() == "true",
}, ensure_ascii=False))
JSON
fi
exit $exit_code
