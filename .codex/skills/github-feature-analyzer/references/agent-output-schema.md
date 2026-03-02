# Agent Output Schema

Each sub-agent must output one JSON object.

## Required top-level fields
- `agent_role`: `overview` | `architecture` | `feature`
- `status`: `ok` | `partial` | `failed`
- `confidence`: `high` | `medium` | `low`
- `summary`: mechanism-oriented natural-language conclusion (prefer 4-8 explanatory sentences for feature role)
- `evidence`: array of evidence items
- `notes`: array of warnings or caveats

## Required for `feature` role
- `feature`: exact input feature string
- `principles`: object with five required keys:
  - `runtime_control_flow`
  - `data_flow`
  - `state_lifecycle`
  - `failure_recovery`
  - `concurrency_timing`

Each principle value is an object:
- `conclusion`: string
- `confidence`: `high` | `medium` | `low`
- `inference`: boolean

## Evidence item schema
Each `evidence` item must contain:
- `path`: relative file path
- `line`: positive integer
- `snippet`: short code snippet
- `for_dimension`: one of
  - `runtime_control_flow`
  - `data_flow`
  - `state_lifecycle`
  - `failure_recovery`
  - `concurrency_timing`
  - `overview`
  - `architecture`

## Optional fields
- `conflicts`: array of conflicting claims
- `unknowns`: array of unverified assumptions
- `raw_files_scanned`: number

## Validity Rules
- `evidence` can be empty only when `status` is `failed`
- if any principle conclusion is unsupported, set `inference=true`
- keep snippets concise and avoid full-file dumps
