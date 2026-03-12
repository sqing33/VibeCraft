## Why

Repo Library currently models repository **snapshots** and **analysis runs** as separate selectable concepts, but in practice they are tightly coupled (often 1:1) and cannot be meaningfully separated in the product. This creates confusing UI (two selectors with overlapping meaning) and complicates downstream behaviors like “sync latest reply”.

Additionally, the current “sync latest reply” behavior re-sends the full original analysis prompt, causing unnecessary re-analysis and making it harder to converge on a format-valid report.

## What Changes

- Unify “snapshot” and “analysis run” into a single first-class entity: **analysis result** (one selector in UI, one ID carried through APIs/storage/search).
- **BREAKING**: Remove snapshot-centric Repo Library APIs/DTOs and replace them with analysis-result-centric APIs/DTOs.
- Persist and serve the formal report as the official report for an **analysis result** (not a snapshot).
- Update go-searchdb indexing and result references to key on analysis results (no snapshot ID in search payloads).
- Change “sync latest reply” behavior:
  - First validate/parse the latest assistant reply directly.
  - If validation passes, persist it immediately (no additional AI call).
  - If validation fails, ask the AI for a full corrected report using a **short format-repair prompt** that includes validator errors (do not resend the original large prompt).

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `repo-library-ingestion`: replace “snapshot + analysis run” persistence/linking with a single persisted analysis result.
- `repo-library-ai-analysis`: treat the formal report as the analysis result’s official report; adjust sync behavior to validate-latest-first.
- `repo-library-report-validation`: validation retries and sync repair prompts MUST be short and format-focused (no re-sending the initial full prompt).
- `repo-library-search`: search results MUST reference analysis results (not snapshots).
- `repo-library-searchdb`: index keys and rebuild semantics MUST use analysis results (no snapshot concept).
- `repo-library-knowledge`: APIs for detail/cards/report MUST be analysis-result-centric.
- `repo-library-detail-workspace`: UI selectors MUST collapse into a single analysis selector.
- `repo-library-ui`: repository detail UI MUST not surface snapshot/run as separate concepts.

## Impact

- Backend: SQLite schema, store queries, Repo Library service layer, API routes/DTOs, report validation/sync logic, searchdb indexing/rebuild.
- UI: repository detail page selectors, report rendering paths, search results links and filtering, overall detail layout expectations.
- Data migration: existing snapshot-linked data must be migrated or cleanly handled under the unified analysis result model.

