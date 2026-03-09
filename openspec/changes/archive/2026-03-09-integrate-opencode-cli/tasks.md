## 1. Spec and config foundation

- [x] 1.1 Write proposal, design, tasks, and delta specs for OpenCode CLI integration
- [x] 1.2 Extend CLI tool config and settings API to support `opencode` and multi-protocol compatibility
- [x] 1.3 Add or update tests covering CLI tool normalization and protocol compatibility checks

## 2. OpenCode runtime and chat integration

- [x] 2.1 Add `scripts/agent-runtimes/opencode_exec.sh` using official `opencode run` flags and standard artifacts
- [x] 2.2 Extend expert resolution/runtime env so OpenCode can consume selected OpenAI or Anthropic model source settings
- [x] 2.3 Add OpenCode-specific streaming parsing and CLI session resume handling in chat runtime
- [x] 2.4 Add focused tests for OpenCode wrapper/parser/runtime behavior where practical

## 3. UI and project integration

- [x] 3.1 Update CLI tool settings, chat runtime selection, and repo library model filtering for multi-protocol tools and `OpenCode CLI`
- [x] 3.2 Keep `reasoning_effort` strictly Codex-only in the chat UI and requests
- [x] 3.3 Update `PROJECT_STRUCTURE.md` for the new wrapper and runtime responsibilities

## 4. Validation and archive

- [x] 4.1 Run targeted backend tests and frontend build/lint relevant to the changed areas
- [x] 4.2 Review the final diff for regressions and sync completed task checkboxes
- [x] 4.3 Archive the completed OpenSpec change into `openspec/changes/archive/`
