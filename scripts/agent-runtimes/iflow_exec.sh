#!/usr/bin/env bash
set -euo pipefail

prompt="${VIBE_TREE_PROMPT:-}"
system_prompt="${VIBE_TREE_SYSTEM_PROMPT:-}"
model="${VIBE_TREE_MODEL:-${VIBE_TREE_MODEL_ID:-}}"
model_id="${VIBE_TREE_MODEL_ID:-}"
artifact_dir="${VIBE_TREE_ARTIFACT_DIR:-}"
workspace="${VIBE_TREE_WORKSPACE:-$PWD}"
cli_cmd="${VIBE_TREE_CLI_COMMAND_PATH:-iflow}"
resume_session_id="${VIBE_TREE_RESUME_SESSION_ID:-}"
iflow_home="${VIBE_TREE_IFLOW_HOME:-}"
iflow_auth_mode="${VIBE_TREE_IFLOW_AUTH_MODE:-browser}"
iflow_api_key="${VIBE_TREE_IFLOW_API_KEY:-}"
iflow_base_url="${VIBE_TREE_IFLOW_BASE_URL:-https://apis.iflow.cn/v1}"
iflow_allowed_mcp_servers="${VIBE_TREE_IFLOW_ALLOWED_MCP_SERVERS:-}"
iflow_mcp_servers_json="${VIBE_TREE_IFLOW_MCP_SERVERS_JSON:-}"
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
session_file="${artifact_dir:+$artifact_dir/session.json}"
raw_log="${artifact_dir:+$artifact_dir/raw_output.log}"
stdout_file="${artifact_dir:+$artifact_dir/stdout.log}"
exec_info_file="${artifact_dir:+$artifact_dir/iflow_execution.json}"
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
if [[ -z "$stdout_file" ]]; then
  stdout_file="$(mktemp)"
  cleanup_files+=("$stdout_file")
fi
if [[ -z "$exec_info_file" ]]; then
  exec_info_file="$(mktemp)"
  cleanup_files+=("$exec_info_file")
fi
trap 'for f in "${cleanup_files[@]:-}"; do [[ -n "$f" ]] && rm -f "$f"; done' EXIT

combined_prompt="$prompt"
if [[ -n "$system_prompt" ]]; then
  combined_prompt=$'System instructions:\n'"$system_prompt"$'\n\nUser request:\n'"$prompt"
fi

bootstrap_iflow_home() {
  local target_home="$1"
  [[ -z "$target_home" ]] && return 0
  mkdir -p "$target_home/.iflow"
  if [[ ! -f "$target_home/.iflow/settings.json" ]]; then
    cat > "$target_home/.iflow/settings.json" <<'JSON'
{
  "cna": "vibe-tree",
  "checkpointing": {
    "enabled": true
  },
  "bootAnimationShown": true,
  "hasViewedOfflineOutput": true
}
JSON
  fi
}

sync_iflow_mcp_servers() {
  [[ -z "$iflow_mcp_servers_json" ]] && return 0
  python3 - "$cli_cmd" "$workspace" "$iflow_mcp_servers_json" <<'PY'
import json
import os
import subprocess
import sys

cli_cmd, workspace, payload = sys.argv[1:4]
try:
    servers = json.loads(payload)
except Exception:
    raise SystemExit(0)
if not isinstance(servers, dict):
    raise SystemExit(0)
env = os.environ.copy()
for name, cfg in servers.items():
    if not isinstance(name, str) or not name.strip() or not isinstance(cfg, dict):
        continue
    subprocess.run(
        [cli_cmd, 'mcp', 'add-json', name.strip(), json.dumps(cfg, ensure_ascii=False), '--scope', 'project'],
        cwd=workspace,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
PY
}

if [[ -n "$iflow_home" ]]; then
  export HOME="$iflow_home"
  bootstrap_iflow_home "$HOME"
fi

export IFLOW_TRUST_WORKSPACE="1"
export IFLOW_selectedAuthType="iflow"
export IFLOW_SELECTED_AUTH_TYPE="iflow"
if [[ -n "$iflow_base_url" ]]; then
  export IFLOW_baseUrl="$iflow_base_url"
  export IFLOW_BASE_URL="$iflow_base_url"
fi
if [[ "$iflow_auth_mode" == "api_key" && -n "$iflow_api_key" ]]; then
  export IFLOW_apiKey="$iflow_api_key"
  export IFLOW_API_KEY="$iflow_api_key"
fi
if [[ -n "$model" ]]; then
  export IFLOW_modelName="$model"
  export IFLOW_MODEL_NAME="$model"
  export IFLOW_MODEL="$model"
fi

sync_iflow_mcp_servers || true

args=(--yolo --output-file "$exec_info_file")
if [[ -n "$model" ]]; then
  args+=(--model "$model")
fi
if [[ -n "$resume_session_id" ]]; then
  args+=(--resume "$resume_session_id")
fi
if [[ -n "$iflow_allowed_mcp_servers" ]]; then
  IFS=',' read -r -a __vibe_iflow_mcp_names <<< "$iflow_allowed_mcp_servers"
  for name in "${__vibe_iflow_mcp_names[@]}"; do
    name="${name##${name%%[![:space:]]*}}"
    name="${name%${name##*[![:space:]]}}"
    [[ -z "$name" ]] && continue
    args+=(--allowed-mcp-server-names "$name")
  done
fi

if ! command -v "$cli_cmd" >/dev/null 2>&1; then
  echo "CLI command not found: $cli_cmd" >"$raw_log"
  exit_code=127
else
  set +e
  (
    cd "$workspace"
    printf '%s' "$combined_prompt" | "$cli_cmd" "${args[@]}" 2>>"$raw_log"
  ) | tee "$stdout_file" | tee -a "$raw_log"
  exit_code=${PIPESTATUS[0]}
  set -e
fi

python3 - "$stdout_file" "$final_file" <<'INNER'
import re
import sys
from pathlib import Path
raw = Path(sys.argv[1]).read_text(encoding='utf-8', errors='ignore') if Path(sys.argv[1]).exists() else ''
clean = re.sub(r'\x1b\[[0-9;?]*[ -/]*[@-~]', '', raw)
clean = clean.replace('\r', '')
clean = clean.strip()
if clean:
    Path(sys.argv[2]).write_text(clean + '\n', encoding='utf-8')
INNER

session_id="$(python3 - "$session_file" "$exec_info_file" <<'INNER'
import json, sys
from pathlib import Path
for raw_path in sys.argv[1:]:
    path = Path(raw_path)
    if not path.exists():
        continue
    try:
        obj = json.loads(path.read_text(encoding='utf-8', errors='ignore') or '{}')
    except Exception:
        continue
    value = obj.get('session_id') or obj.get('session-id') or ''
    if isinstance(value, str) and value.strip():
        print(value.strip())
        raise SystemExit(0)
INNER
)"

if [[ -n "$session_id" && -n "$session_file" ]]; then
  export session_id model model_id resumed_flag="false"
  if [[ -n "$resume_session_id" ]]; then export resumed_flag="true"; fi
  python3 - <<'INNER' > "$session_file"
import json, os
print(json.dumps({
  "tool_id": "iflow",
  "session_id": os.environ.get("session_id", ""),
  "model": os.environ.get("model", "") or os.environ.get("model_id", ""),
  "resumed": os.environ.get("resumed_flag", "false").lower() == "true",
}, ensure_ascii=False))
INNER
fi

if [[ -f "$final_file" ]]; then
  summary_text="$(python3 - "$final_file" <<'INNER'
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding='utf-8', errors='ignore').strip()
print(' '.join(text.splitlines()[-12:]).strip())
INNER
)"
fi
if [[ -z "$summary_text" && -f "$raw_log" ]]; then
  summary_text="$(python3 - "$raw_log" <<'INNER'
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding='utf-8', errors='ignore').strip().splitlines()
print(' '.join(text[-20:]).strip())
INNER
)"
fi
if [[ -z "$summary_text" ]]; then
  summary_text="CLI run finished"
fi
if grep -qi "Invalid API key provided" "$raw_log" "$final_file" 2>/dev/null; then
  status="error"
  summary_text="iFlow 官方登录态已失效，请到 Settings → CLI 工具 → iFlow CLI 重新网页登录，或填写新的官方 API Key。"
  next_action="Re-authenticate iFlow in Settings → CLI Tools"
  printf '%s
' "$summary_text" > "$final_file"
  if [[ $exit_code -eq 0 ]]; then exit_code=1; fi
elif grep -qi "Error when talking to iFlow API" "$raw_log" "$final_file" 2>/dev/null; then
  status="error"
  summary_text="iFlow CLI 调用失败，请检查 Settings → CLI 工具 → iFlow CLI 中的网页登录、API Key 与模型配置。"
  next_action="Check iFlow auth and model settings"
  printf '%s
' "$summary_text" > "$final_file"
  if [[ $exit_code -eq 0 ]]; then exit_code=1; fi
elif [[ $exit_code -ne 0 ]]; then
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
  python3 - <<'INNER' > "$summary_file"
import json, os
print(json.dumps({
  "status": os.environ.get("status", "ok"),
  "summary": os.environ.get("summary_text", ""),
  "modified_code": os.environ.get("modified_code", "false").lower() == "true",
  "next_action": os.environ.get("next_action", ""),
  "key_files": [],
}, ensure_ascii=False))
INNER
fi
if [[ -n "$artifacts_file" ]]; then
  export summary_text
  python3 - "$exec_info_file" <<'INNER' > "$artifacts_file"
import json, os, sys
from pathlib import Path
payload = None
path = Path(sys.argv[1])
if path.exists():
    try:
        payload = json.loads(path.read_text(encoding='utf-8', errors='ignore') or '{}')
    except Exception:
        payload = None
print(json.dumps({"artifacts": [{
  "kind": "cli_session_summary",
  "title": "iFlow Final Summary",
  "summary": os.environ.get("summary_text", ""),
  "payload": payload,
}]}, ensure_ascii=False))
INNER
fi
exit $exit_code
