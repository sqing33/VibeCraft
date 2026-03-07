#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${VIBE_TREE_ARTIFACT_DIR:-}"
prompt="${VIBE_TREE_PROMPT:-}"
system_prompt="${VIBE_TREE_SYSTEM_PROMPT:-}"
model="${VIBE_TREE_MODEL:-}"
model_id="${VIBE_TREE_MODEL_ID:-}"
workspace="${VIBE_TREE_WORKSPACE:-$PWD}"
cli_cmd="${VIBE_TREE_CLI_COMMAND_PATH:-claude}"
resume_session_id="${VIBE_TREE_RESUME_SESSION_ID:-}"
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

args=(-p --output-format stream-json --dangerously-skip-permissions --include-partial-messages)
if [[ -n "$model" ]]; then
  args+=(--model "$model")
fi
if [[ -n "$system_prompt" ]]; then
  args+=(--append-system-prompt "$system_prompt")
fi
if [[ -n "$resume_session_id" ]]; then
  args+=(-r "$resume_session_id")
fi

if ! command -v "$cli_cmd" >/dev/null 2>&1; then
  echo "CLI command not found: $cli_cmd" >"$raw_log"
  exit_code=127
else
  set +e
  (
    cd "$workspace"
    "$cli_cmd" "${args[@]}" "$prompt" 2>>"$raw_log"
  ) | tee -a "$raw_log"
  exit_code=$?
  set -e
fi

python3 - "$raw_log" "$final_file" "$session_file" <<'PY'
import json, sys
from pathlib import Path
raw_path = Path(sys.argv[1])
final_path = Path(sys.argv[2])
session_path = Path(sys.argv[3])
session_id = ''
parts = []
if raw_path.exists():
    for raw in raw_path.read_text(encoding='utf-8', errors='ignore').splitlines():
        raw = raw.strip()
        if not raw:
            continue
        try:
            obj = json.loads(raw)
        except Exception:
            continue
        sid = obj.get('session_id') or obj.get('sessionId')
        if isinstance(sid, str) and sid.strip() and not session_id:
            session_id = sid.strip()
        if obj.get('type') == 'assistant':
            msg = obj.get('message')
            if isinstance(msg, dict):
                for block in msg.get('content', []) or []:
                    if isinstance(block, dict) and block.get('type') == 'text' and isinstance(block.get('text'), str):
                        parts.append(block['text'])
        elif obj.get('type') == 'result' and isinstance(obj.get('result'), str):
            parts.append(obj['result'])
text = ''.join(parts).strip()
if text:
    final_path.write_text(text, encoding='utf-8')
if session_id:
    session_path.write_text(json.dumps({"tool_id": "claude", "session_id": session_id, "model": "", "resumed": False}, ensure_ascii=False), encoding='utf-8')
PY

session_id=""
if [[ -f "$session_file" ]]; then
  session_id="$(python3 - "$session_file" <<'PY'
import json, sys
from pathlib import Path
obj=json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
print(obj.get('session_id',''))
PY
)"
fi
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
  "title": "Claude Final Summary",
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
  "tool_id": "claude",
  "session_id": os.environ.get("session_id", ""),
  "model": os.environ.get("model", "") or os.environ.get("model_id", ""),
  "resumed": os.environ.get("resumed_flag", "false").lower() == "true",
}, ensure_ascii=False))
JSON
fi
exit $exit_code
