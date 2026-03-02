---
name: github-feature-analyzer
description: Analyze how one or more features are implemented inside a GitHub repository, with principle-first explanations and evidence-backed conclusions. Use when users ask to read a github.com repository, explain implementation logic, trace runtime behavior, compare multiple feature implementations, or audit an unknown codebase by feature. Prefer multi-agent parallel analysis with a parent merge step, and fall back to single-agent mode when needed. Store artifacts under <project-root>/.github-feature-analyzer/{owner-repo}/ with a cumulative report.md.
---

# GitHub Feature Analyzer

## Overview
Analyze feature implementations in public github.com repositories with reproducible local workspaces and evidence-based report output.

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

Do not auto-clean downloaded repositories. Clean only when user explicitly asks.

## Workflow
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

4. Run analysis mode.

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
- `scripts/clean_downloads.py`: clean cached downloads on explicit request.

## Output Contract
Follow `references/report-schema.md`.

For each key conclusion:
- provide at least one `file:line` evidence item
- mark unsupported statements as `inference`
- prioritize mechanism explanation over path dump

## Failure Policy
Follow `references/failure-handling.md`.

Hard-stop conditions:
- invalid `repo_url` or non-github.com host
- private repository
- fetch failure on both retrieval paths
- empty `features`
