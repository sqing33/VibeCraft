# Failure Handling

## Core Rule
Stop immediately only for hard-stop failures. Otherwise continue with fallback and capture diagnostics.

## Hard-stop Failures
Stop and report actionable error when:
- `repo_url` is invalid
- host is not `github.com`
- repository is private (current scope is public-only)
- feature list is empty
- both retrieval paths fail
- report rendering fails after all fallbacks

## Retrieval Fallback Rules
Fallback order:
1. archive/API path
2. git clone fallback

If step 1 fails:
- continue to step 2
- include step-1 error summary in metadata

If step 2 fails:
- stop and report both errors

## Multi-agent Failure Rules
If multi-agent mode is requested:
- if multi-agent is unavailable, ask user whether to continue in single-agent mode
- if some sub-agents fail, continue with successful results and mark gaps explicitly
- if all sub-agents fail and single-agent fallback is not approved, stop

## Merge-stage Failure Rules
If sub-agent outputs are partially invalid:
- skip invalid JSON files
- keep valid files
- emit `merge_notes` with skipped file reasons

If all outputs are invalid:
- fail merge stage
- ask for rerun or fallback to single-agent analysis

## Error Reporting Contract
Always include:
- failing step
- command or endpoint attempted
- concise root-cause message
- suggested next action

Examples of next actions:
- verify repo URL and ref
- retry with a different ref
- check network access
- confirm repository visibility
- rerun failed sub-agent shards
- continue with single-agent fallback

## Partial-success Behavior
If source fetch succeeds but analysis signals are weak:
- still generate report
- explicitly mark low-confidence sections
- include recommended follow-up probes
