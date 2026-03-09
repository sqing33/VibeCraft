#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${VIBE_TREE_ARTIFACT_DIR:-}"
prompt="${VIBE_TREE_PROMPT:-}"
system_prompt="${VIBE_TREE_SYSTEM_PROMPT:-}"
model="${VIBE_TREE_MODEL:-}"
model_id="${VIBE_TREE_MODEL_ID:-}"
protocol_family="${VIBE_TREE_PROTOCOL_FAMILY:-}"
workspace="${VIBE_TREE_WORKSPACE:-$PWD}"
cli_cmd="${VIBE_TREE_CLI_COMMAND_PATH:-opencode}"
resume_session_id="${VIBE_TREE_RESUME_SESSION_ID:-}"
openai_api_key="${OPENAI_API_KEY:-}"
openai_base_url="${VIBE_TREE_BASE_URL:-${OPENAI_BASE_URL:-}}"
anthropic_api_key="${ANTHROPIC_API_KEY:-}"
anthropic_base_url="${VIBE_TREE_BASE_URL:-${ANTHROPIC_BASE_URL:-}}"
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
exec_info_file="${artifact_dir:+$artifact_dir/opencode_execution.json}"
patch_file="${artifact_dir:+$artifact_dir/patch.diff}"

cleanup_files=()
cleanup_dirs=()
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
trap 'for f in "${cleanup_files[@]:-}"; do [[ -n "$f" ]] && rm -f "$f"; done; for d in "${cleanup_dirs[@]:-}"; do [[ -n "$d" ]] && rm -rf "$d"; done' EXIT

combined_prompt="$prompt"
if [[ -n "$system_prompt" ]]; then
  combined_prompt=$'System instructions:\n'"$system_prompt"$'\n\nUser request:\n'"$prompt"
fi

resolved_provider="${protocol_family,,}"
resolved_model="$model"
if [[ "$resolved_model" == */* ]]; then
  resolved_provider="${resolved_model%%/*}"
  resolved_model="${resolved_model#*/}"
fi
if [[ -z "$resolved_provider" ]]; then
  resolved_provider="openai"
fi
full_model="$model"
if [[ -z "$full_model" ]]; then
  full_model="$model_id"
fi
if [[ -n "$full_model" && "$full_model" != */* ]]; then
  full_model="$resolved_provider/$full_model"
fi
if [[ -z "$full_model" && -n "$resolved_model" ]]; then
  full_model="$resolved_provider/$resolved_model"
fi
if [[ "$full_model" == */* ]]; then
  resolved_provider="${full_model%%/*}"
  resolved_model="${full_model#*/}"
fi
resolved_provider="${resolved_provider,,}"

base_xdg_config="${XDG_CONFIG_HOME:-}"
base_config_file=""
if [[ -n "$base_xdg_config" && -f "$base_xdg_config/opencode/opencode.json" ]]; then
  base_config_file="$base_xdg_config/opencode/opencode.json"
elif [[ -n "${HOME:-}" && -f "$HOME/.config/opencode/opencode.json" ]]; then
  base_config_file="$HOME/.config/opencode/opencode.json"
fi

state_root=""
if [[ -n "$artifact_dir" ]]; then
  session_scope="$artifact_dir"
  artifact_parent="$(dirname "$artifact_dir")"
  artifact_grandparent="$(dirname "$artifact_parent")"
  if [[ "$(basename "$artifact_grandparent")" == "chat" ]]; then
    session_scope="$artifact_parent"
  fi
  state_root="$session_scope/.opencode"
else
  state_root="$(mktemp -d)"
  cleanup_dirs+=("$state_root")
fi
config_root="$state_root/config"
data_root="$state_root/data"
cache_root="$state_root/cache"
state_home_root="$state_root/state"
mkdir -p "$config_root/opencode" "$data_root" "$cache_root" "$state_home_root"
export XDG_CONFIG_HOME="$config_root"
export XDG_DATA_HOME="$data_root"
export XDG_CACHE_HOME="$cache_root"
export XDG_STATE_HOME="$state_home_root"
export VIBE_TREE_OPENCODE_PROVIDER="$resolved_provider"
export VIBE_TREE_OPENCODE_MODEL_NAME="$resolved_model"
export VIBE_TREE_OPENCODE_OPENAI_API_KEY="$openai_api_key"
export VIBE_TREE_OPENCODE_OPENAI_BASE_URL="$openai_base_url"
export VIBE_TREE_OPENCODE_ANTHROPIC_API_KEY="$anthropic_api_key"
export VIBE_TREE_OPENCODE_ANTHROPIC_BASE_URL="$anthropic_base_url"
python3 - "$base_config_file" "$config_root/opencode/opencode.json" <<'PY'
import json
import os
import sys
from pathlib import Path

base_path = Path(sys.argv[1]) if len(sys.argv) > 1 and sys.argv[1] else None
out_path = Path(sys.argv[2])
config = {}
if base_path and base_path.exists():
    try:
        loaded = json.loads(base_path.read_text(encoding='utf-8', errors='ignore') or '{}')
        if isinstance(loaded, dict):
            config = loaded
    except Exception:
        config = {}

provider_id = (os.environ.get('VIBE_TREE_OPENCODE_PROVIDER') or 'openai').strip().lower() or 'openai'
model_name = (os.environ.get('VIBE_TREE_OPENCODE_MODEL_NAME') or '').strip()
provider_defaults = {
    'openai': {'npm': '@ai-sdk/openai', 'name': 'OpenAI'},
    'anthropic': {'npm': '@ai-sdk/anthropic', 'name': 'Anthropic'},
}
defs = provider_defaults.get(provider_id, {'npm': '', 'name': provider_id})
config.setdefault('$schema', 'https://opencode.ai/config.json')
providers = config.setdefault('provider', {})
if not isinstance(providers, dict):
    providers = {}
    config['provider'] = providers
entry = providers.get(provider_id)
if not isinstance(entry, dict):
    entry = {}
if defs.get('npm') and not str(entry.get('npm') or '').strip():
    entry['npm'] = defs['npm']
if defs.get('name') and not str(entry.get('name') or '').strip():
    entry['name'] = defs['name']
options = entry.get('options')
if not isinstance(options, dict):
    options = {}
api_key = ''
base_url = ''
if provider_id == 'openai':
    api_key = os.environ.get('VIBE_TREE_OPENCODE_OPENAI_API_KEY', '')
    base_url = os.environ.get('VIBE_TREE_OPENCODE_OPENAI_BASE_URL', '')
elif provider_id == 'anthropic':
    api_key = os.environ.get('VIBE_TREE_OPENCODE_ANTHROPIC_API_KEY', '')
    base_url = os.environ.get('VIBE_TREE_OPENCODE_ANTHROPIC_BASE_URL', '')
if api_key.strip():
    options['apiKey'] = api_key.strip()
if base_url.strip():
    options['baseURL'] = base_url.strip()
if options:
    entry['options'] = options
models = entry.get('models')
if not isinstance(models, dict):
    models = {}
if model_name:
    existing = models.get(model_name)
    if not isinstance(existing, dict):
        models[model_name] = {}
if models:
    entry['models'] = models
providers[provider_id] = entry
out_path.write_text(json.dumps(config, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
PY

args=(run --format json --thinking --dir "$workspace")
if [[ -n "$full_model" ]]; then
  args+=(--model "$full_model")
fi
if [[ -n "$resume_session_id" ]]; then
  args+=(--session "$resume_session_id")
fi
args+=("$combined_prompt")

if ! command -v "$cli_cmd" >/dev/null 2>&1; then
  printf 'CLI command not found: %s\n' "$cli_cmd" >"$raw_log"
  printf 'CLI command not found: %s\n' "$cli_cmd" >"$final_file"
  exit_code=127
else
  set +e
  (
    cd "$workspace"
    "$cli_cmd" "${args[@]}" 2>>"$raw_log"
  ) | tee "$stdout_file" | tee -a "$raw_log"
  exit_code=${PIPESTATUS[0]}
  set -e
fi

python3 - "$stdout_file" "$exec_info_file" "$final_file" <<'PY'
import json
import sys
from collections import OrderedDict
from pathlib import Path

stdout_path = Path(sys.argv[1])
exec_info_path = Path(sys.argv[2])
final_path = Path(sys.argv[3])

part_types = {}
text_parts = OrderedDict()
reasoning_parts = OrderedDict()
non_json_lines = []
session_id = ''
error_message = ''
event_count = 0

def as_dict(value):
    return value if isinstance(value, dict) else {}

def as_list(value):
    return value if isinstance(value, list) else []

def first_non_empty(*values):
    for value in values:
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ''

def nested(obj, *path):
    cur = obj
    for key in path:
        if isinstance(cur, dict):
            cur = cur.get(key)
        else:
            return None
    return cur

lines = stdout_path.read_text(encoding='utf-8', errors='ignore').splitlines() if stdout_path.exists() else []
for raw in lines:
    line = raw.strip()
    if not line:
        continue
    try:
        obj = json.loads(line)
    except Exception:
        non_json_lines.append(raw)
        continue
    event_count += 1
    session_id = first_non_empty(
        str(obj.get('sessionID') or ''),
        str(obj.get('session_id') or ''),
        str(nested(obj, 'properties', 'sessionID') or ''),
        str(nested(obj, 'properties', 'part', 'sessionID') or ''),
        str(nested(obj, 'properties', 'info', 'id') or ''),
        session_id,
    )
    event_type = first_non_empty(str(obj.get('type') or ''))
    if event_type == 'message.part.updated':
        part = as_dict(nested(obj, 'properties', 'part'))
        part_id = first_non_empty(str(part.get('id') or '')) or f"part-{len(part_types)+1}"
        part_type = first_non_empty(str(part.get('type') or ''))
        if part_type:
            part_types[part_id] = part_type
        if part_type == 'text':
            text_parts[part_id] = str(part.get('text') or '')
        elif part_type == 'reasoning':
            reasoning_parts[part_id] = str(part.get('text') or '')
    elif event_type in ('text', 'reasoning', 'tool', 'patch', 'agent', 'subtask', 'retry', 'compaction', 'step_start', 'step_finish'):
        part = as_dict(obj.get('part'))
        part_id = first_non_empty(str(part.get('id') or '')) or f"part-{len(part_types)+1}"
        part_type = first_non_empty(str(part.get('type') or ''))
        if part_type:
            part_types[part_id] = part_type
        if part_type == 'text':
            text_parts[part_id] = str(part.get('text') or '')
        elif part_type == 'reasoning':
            reasoning_parts[part_id] = str(part.get('text') or '')
    elif event_type == 'message.part.delta':
        props = as_dict(obj.get('properties'))
        part_id = first_non_empty(str(props.get('partID') or '')) or f"part-{len(part_types)+1}"
        field = first_non_empty(str(props.get('field') or ''))
        delta = str(props.get('delta') or '')
        if delta and (not field or field == 'text' or field.endswith('.text')):
            part_type = part_types.get(part_id, 'text')
            if part_type == 'reasoning':
                reasoning_parts[part_id] = reasoning_parts.get(part_id, '') + delta
            else:
                text_parts[part_id] = text_parts.get(part_id, '') + delta
    elif event_type in ('error', 'session.error'):
        error_message = first_non_empty(
            str(nested(obj, 'error', 'data', 'message') or ''),
            str(nested(obj, 'error', 'message') or ''),
            str(nested(obj, 'properties', 'error', 'data', 'message') or ''),
            str(nested(obj, 'properties', 'error', 'message') or ''),
            error_message,
        )

final_text = '\n\n'.join(text.strip() for text in text_parts.values() if str(text).strip())
reasoning_text = '\n\n'.join(text.strip() for text in reasoning_parts.values() if str(text).strip())
if not final_text and error_message:
    final_text = error_message.strip()
if not final_text and non_json_lines:
    final_text = '\n'.join(line.rstrip() for line in non_json_lines if line.strip()).strip()
if final_text:
    final_path.write_text(final_text.rstrip() + '\n', encoding='utf-8')
exec_info_path.write_text(json.dumps({
    'session_id': session_id,
    'final_text': final_text,
    'reasoning_text': reasoning_text,
    'error_message': error_message,
    'event_count': event_count,
}, ensure_ascii=False), encoding='utf-8')
PY

session_id="$(python3 - "$exec_info_file" <<'PY'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
if path.exists():
    try:
        payload = json.loads(path.read_text(encoding='utf-8', errors='ignore') or '{}')
    except Exception:
        payload = {}
    value = payload.get('session_id') or ''
    if isinstance(value, str) and value.strip():
        print(value.strip())
PY
)"

if [[ -n "$session_id" && -n "$session_file" ]]; then
  export session_id full_model resumed_flag="false"
  if [[ -n "$resume_session_id" ]]; then export resumed_flag="true"; fi
  python3 - <<'PY' > "$session_file"
import json
import os
print(json.dumps({
  'tool_id': 'opencode',
  'session_id': os.environ.get('session_id', ''),
  'model': os.environ.get('full_model', ''),
  'resumed': os.environ.get('resumed_flag', 'false').lower() == 'true',
}, ensure_ascii=False))
PY
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
  python3 - <<'PY' > "$summary_file"
import json
import os
print(json.dumps({
  'status': os.environ.get('status', 'ok'),
  'summary': os.environ.get('summary_text', ''),
  'modified_code': os.environ.get('modified_code', 'false').lower() == 'true',
  'next_action': os.environ.get('next_action', ''),
  'key_files': [],
}, ensure_ascii=False))
PY
fi
if [[ -n "$artifacts_file" ]]; then
  export summary_text
  python3 - "$exec_info_file" <<'PY' > "$artifacts_file"
import json
import os
import sys
from pathlib import Path
payload = None
path = Path(sys.argv[1])
if path.exists():
    try:
        payload = json.loads(path.read_text(encoding='utf-8', errors='ignore') or '{}')
    except Exception:
        payload = None
print(json.dumps({'artifacts': [{
  'kind': 'cli_session_summary',
  'title': 'OpenCode Final Summary',
  'summary': os.environ.get('summary_text', ''),
  'payload': payload,
}]}, ensure_ascii=False))
PY
fi
exit $exit_code
