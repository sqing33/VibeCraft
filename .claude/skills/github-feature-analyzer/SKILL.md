---
name: github-feature-analyzer
description: Analyze how one or more features are implemented inside a GitHub repository, with principle-first explanations and evidence-backed conclusions. Use when users ask to read a github.com repository, explain implementation logic, trace runtime behavior, compare multiple feature implementations, or audit an unknown codebase by feature. Prefer multi-agent parallel analysis with a parent merge step, and fall back to single-agent mode when needed. Store artifacts under <project-root>/.github-feature-analyzer/{owner-repo}/ with a cumulative report.md.
---

# GitHub Feature Analyzer

## Overview
Analyze feature implementations in public github.com repositories with reproducible local workspaces and evidence-based report output.

Default analysis now includes a **README-first repository pass**:
- read repository README before feature analysis
- extract project characteristics and signature capabilities
- explain how each characteristic is implemented (mechanism narrative + limited evidence refs)
- then run the feature-focused analysis requested by user

Default output is **principle-first**:
- runtime control flow
- data flow
- state/lifecycle
- failure/recovery
- concurrency/timing

Function/file mapping remains evidence, not the report center.

Load targeted references only when needed:
- `references/analysis-method.md`: analysis workflow and heuristics
- `references/report-schema.md`: report contract
- `references/failure-handling.md`: error classification and stop conditions
- `references/agent-output-schema.md`: sub-agent output JSON contract

## Required Inputs
Collect these fields before execution:
- `repo_url`: `https://github.com/{owner}/{repo}`
- `features`: one or more natural-language feature descriptions
- `ref` (optional): git reference, default `main`
- `depth` (optional): `standard` (default) or `deep`
- `language` (optional): `zh` (default) or `en`
- `agent_mode` (optional): `multi` (default), `single`, or `auto`
- `max_parallel_agents` (optional): `auto` (default) or positive integer
- `reference_lookup` (optional): when user asks to reuse historical analysis results
  - `enabled`: `true` / `false` (default `false`)
  - `query`: retrieval query (required when enabled)
  - `mode`: `compact` | `semi` (default) | `full`
  - `repo_filter`: optional list of `{owner-repo}` keys

Reject private repositories in this version.

## Path Conventions
Use fixed project-local paths:
- storage root: `<project-root>/.github-feature-analyzer/`
- project workspace: `<storage-root>/{owner-repo}/`
- source dir: `<workspace>/source/` (always overwritten with latest run)
- artifacts dir: `<workspace>/artifacts/` (always overwritten with latest run)
- sub-agent results: `<workspace>/artifacts/subagents/`
- merged sub-agent json: `<workspace>/artifacts/subagent_results.json`
- report file: `<workspace>/report.md` (append by run, never timestamped filename)
- retrieval index root: `<storage-root>/.knowledge-index/`
  - `manifest.json`
  - `chunks.jsonl`
  - `vectors.npy`
- retrieval runtime venv: `<skill-root>/.venv-reference/` (UV-managed Python 3.12, shared across repos)
- retrieval lock file: `<skill-root>/scripts/requirements-vector.lock.txt`

Do not auto-clean downloaded repositories. Clean only when user explicitly asks.

## Workflow
0. Optional historical reference retrieval (only when `reference_lookup.enabled=true`).
- Runtime bootstrap (Linux + macOS only, no Windows in current version):
```bash
bash scripts/ensure_uv_unix.sh
bash scripts/setup_reference_venv.sh
```
- `setup_reference_venv.sh` uses PyPI as default index + PyTorch CPU index (`https://download.pytorch.org/whl/cpu`) as extra index.
- Override with env when needed: `UV_TORCH_CPU_INDEX=<index-url>`.
- On low-memory machines, keep default serial install behavior (`UV_CONCURRENT_* = 1`) or set explicitly before running setup.
- `setup_reference_venv.sh` caches lock hash under `.venv-reference/.reference-lock.sha256`; unchanged lock skips `uv pip sync`.
- Force re-sync with `UV_REFERENCE_FORCE_SYNC=1`.
- Build/refresh local embedding index:
```bash
bash scripts/reference_retrieval_uv.sh build \
  --storage-root "${STORAGE_ROOT:-<project-root>/.github-feature-analyzer}" \
  --model "${REFERENCE_MODEL:-sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2}"
```
- Query historical reports before new analysis:
```bash
bash scripts/reference_retrieval_uv.sh query \
  --storage-root "${STORAGE_ROOT:-<project-root>/.github-feature-analyzer}" \
  --query "$REFERENCE_QUERY" \
  --mode "${REFERENCE_MODE:-semi}" \
  --format markdown \
  --refresh auto
```
- If `repo_filter` is provided, append repeated `--repo "<owner-repo>"`.
- Use retrieval output as prior context for downstream feature analysis.

1. Prepare workspace paths.
```bash
python3 scripts/prepare_workspace.py \
  --repo-url "$REPO_URL" \
  --ref "${REF:-main}"
```
Use the returned JSON values in subsequent commands.

2. Apply MCP-first retrieval policy.
- Use MCP calls (`search_code`, `get_file_contents`) first to verify repository accessibility and probe likely implementation files.
- Fetch local source with fallback behavior:
```bash
FETCH_JSON=$(python3 scripts/fetch_repo.py \
  --repo-url "$REPO_URL" \
  --ref "$REF" \
  --source-dir "$SOURCE_DIR" \
  --mode mcp-first \
  --public-only true)
```
- The script tries archive/API retrieval first, then automatically falls back to git clone.
- If both paths fail, stop immediately and report failure details.

3. Build repository index (internal artifact).
```bash
python3 scripts/build_code_index.py \
  --source-dir "$SOURCE_DIR" \
  --output "$ARTIFACTS_DIR/code_index.json"
```

4. Run README-first + feature analysis mode.
- Always read repository README (if present) first.
- Always generate exactly 3 project characteristics and explain implementation mechanisms.
- If README is missing or too weak, fallback to index/code inference and mark this as `inference`.

### Multi-agent mode (`agent_mode=multi` or `agent_mode=auto` and available)
Use a parent-agent + layered sub-agent topology:
- 1 `overview` sub-agent: repository overview, entrypoints, boundaries
- 1 `architecture` sub-agent: runtime structure and cross-module dependencies
- N `feature` sub-agents: one sub-agent per feature

Parallel policy:
- default cap: `min(len(features) + 2, 8)`
- if `max_parallel_agents` is numeric, use `min(requested, features+2)`
- run in batches when needed

Each sub-agent must return JSON conforming to `references/agent-output-schema.md`.
Store each JSON in `$ARTIFACTS_DIR/subagents/*.json`.

Merge all sub-agent outputs:
```bash
python3 scripts/merge_agent_results.py \
  --input "$ARTIFACTS_DIR/subagents" \
  --output "$ARTIFACTS_DIR/subagent_results.json"
```

Render final report with merged sub-agent results:
```bash
python3 scripts/render_report.py \
  --repo-url "$REPO_URL" \
  --ref "$REF" \
  --resolved-ref "$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("resolved_ref") or "")' <<< "$FETCH_JSON")" \
  --commit-sha "$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("commit_sha") or "")' <<< "$FETCH_JSON")" \
  --source-dir "$SOURCE_DIR" \
  --index-json "$ARTIFACTS_DIR/code_index.json" \
  --subagent-results "$ARTIFACTS_DIR/subagent_results.json" \
  --output "$REPORT_PATH" \
  --depth "${DEPTH:-standard}" \
  --language "${LANGUAGE:-zh}" \
  --feature "feature A" \
  --feature "feature B"
```

### Single-agent mode (`agent_mode=single`)
Skip sub-agent merge step and run `render_report.py` directly (without `--subagent-results`).

### Unavailable multi-agent behavior (`agent_mode=auto` or `multi`)
If multi-agent cannot run, ask user before proceeding:
- continue in single-agent mode
- stop and wait for multi-agent enablement
- retry after environment adjustment

5. Return summary:
- resolved ref and commit (if available)
- README-first project characteristics and implementation mechanism summary
- per-feature mechanism-first explanation + line-level evidence
- boundaries/risks and unknowns
- final report path

## Depth Policy
Use `standard` unless user explicitly asks deeper auditing.

Switch to `deep` when user intent contains terms like:
- `超深度`
- `深度审计`
- `全面审计`
- `deep dive`

## Scripts
- `scripts/prepare_workspace.py`: create deterministic local paths and emit workspace metadata.
- `scripts/fetch_repo.py`: execute retrieval (`mcp-first` archive path with git fallback) and emit fetch metadata.
- `scripts/build_code_index.py`: scan source tree and produce a machine-readable index.
- `scripts/merge_agent_results.py`: merge sub-agent outputs into one normalized JSON artifact.
- `scripts/render_report.py`: convert indexed data (+ optional merged sub-agent results) into final Markdown report.
- `scripts/ensure_uv_unix.sh`: bootstrap UV on Linux/macOS via official installer.
- `scripts/setup_reference_venv.sh`: enforce UV-managed Python 3.12 and sync vector dependencies into fixed venv.
- `scripts/reference_retrieval_uv.sh`: UV-only wrapper for `reference_retrieval.py` build/query.
- `scripts/reference_retrieval.py`: build/query local embedding index over historical `report.md` + `subagent_results.json`.
- `scripts/requirements-vector.txt`: python dependencies required by `reference_retrieval.py`.
- `scripts/requirements-vector.lock.txt`: locked dependency set for `uv pip sync`.
- `scripts/clean_downloads.py`: clean cached downloads on explicit request.

## Output Contract
Follow `references/report-schema.md`.

Top-level structure is now **three-part**:
1) 项目参数与结构解析
2) 面向人的功能说明
3) 面向 AI 的实现细节与证据链（内含五维机制分析）

For each key conclusion:
- provide at least one `file:line` evidence item
- mark unsupported statements as `inference`
- keep `confidence` / `inference` mechanism unchanged
- prioritize mechanism explanation over path dump

Invocation-path rule (on-demand):
- when a feature mentions agent/sub-agent/SDK/CLI/runtime invocation path, include explicit SDK vs CLI vs Hybrid classification
- include working-directory resolution notes (`working_dir`/`current_dir`/`cwd`/container mapping); if no direct evidence, mark as `inference`

## Failure Policy
Follow `references/failure-handling.md`.

Hard-stop conditions:
- invalid `repo_url` or non-github.com host
- private repository
- fetch failure on both retrieval paths
- empty `features`
- when `reference_lookup.enabled=true`: missing vector dependencies / embedding model load failure / retrieval index corruption
- when `reference_lookup.enabled=true`: UV bootstrap failure / UV-managed Python install failure / locked dependency sync failure
