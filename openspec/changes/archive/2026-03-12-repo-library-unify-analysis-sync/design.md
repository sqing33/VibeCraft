## Context

Repo Library currently persists two closely-related entities in `state.db`:

- `repo_snapshots`: repository ref/commit + storage layout (`storage_path`, `report_path`, etc.)
- `repo_analysis_runs`: analysis execution + chat linkage + status, referencing `repo_snapshot_id`

The UI exposes both as separate selectors (“快照” and “分析运行”), but the product does not support meaningfully separating them (typically one snapshot per analysis, and no first-class “rerun analysis on same snapshot” workflow).

The “sync latest reply” action currently re-triggers a heavy analysis-style prompt, which:

- wastes time/tokens,
- produces repeated analysis content instead of strict-format output,
- makes it harder to converge when the only issue is report format validation.

Constraints:

- Existing data must remain usable after migration (cards, reports, chat linkage).
- `search.db` is derived and rebuildable; schema can change freely.

## Goals / Non-Goals

**Goals:**

- Collapse “snapshot” and “analysis run” into a single persisted domain object: **analysis result**.
- Replace the two selectors with one analysis selector across API + store + UI.
- Make “sync latest reply” validate-and-parse the latest assistant reply first; only call AI to repair formatting when validation fails, using a short format-repair prompt.
- Remove snapshot/run duality from new codepaths (no “bind two concepts” only at UI layer).

**Non-Goals:**

- Supporting multiple analysis runs per frozen snapshot.
- Rewriting historical on-disk artifact directories (existing `storage_path` stays valid).
- Preserving backward-compatible Repo Library HTTP endpoints (we accept breaking API changes within the app).

## Decisions

1. **Single entity: `repo_analysis_results` in `state.db`.**
   - Introduce a new table `repo_analysis_results` that contains the union of fields needed for:
     - repository ref/commit + artifact storage paths (previously “snapshot” responsibility)
     - execution/chat linkage/status/features (previously “analysis run” responsibility)
   - Remove `repo_snapshots` and `repo_analysis_runs` usage in all codepaths.

2. **Stable IDs and minimal filesystem migration.**
   - Existing analysis results keep working by migrating rows and retaining the existing `storage_path` and `report_path`.
   - New analysis results are stored under `.../repositories/<repoKey>/analyses/<analysis_id>/...` (folder name changes only for new data).

3. **Knowledge cards key by analysis result only.**
   - Replace `repo_knowledge_cards(repo_snapshot_id, analysis_run_id)` with a single `analysis_result_id`.
   - Evidence remains linked to cards only (unchanged).

4. **Search/index is analysis-result-centric.**
   - Update go-searchdb chunk schema and filtering to use `analysis_result_id` (no snapshot/run IDs).
   - Because `search.db` is derived, we rely on rebuild rather than in-place migration.

5. **Sync latest reply = validate latest reply first, short repair prompt on failure.**
   - Sync flow:
     1. Load the latest assistant message content from the linked chat session.
     2. Run formal report validation + parsing against that content.
     3. If pass: persist report + refresh derived cards/search.
     4. If fail: send a short “format repair” prompt containing blocking validation errors, asking for a full corrected report; retry within a small budget.
   - The repair prompt MUST NOT re-send the original large “initial analysis” prompt.

## Risks / Trade-offs

- **[Risk] SQLite schema migration complexity (drop/rename columns/tables).** → Mitigation: create new tables, bulk-copy with `INSERT INTO ... SELECT ... JOIN ...`, then drop/rename atomically within a transaction.
- **[Risk] Existing artifacts live under `snapshots/<id>` while new ones use `analyses/<id>`.** → Mitigation: always rely on persisted `storage_path`/`report_path` when reading historical data; layout helper is used only for new creations.
- **[Risk] Some runs/snapshots might be partially created.** → Mitigation: migrate using `repo_analysis_runs` as the source-of-truth set and left-join snapshot fields; tolerate missing snapshot fields by keeping nullable columns and falling back to computed default paths when possible.
- **[Risk] Sync may be invoked when the latest reply is not intended as a full report.** → Mitigation: validation failure triggers a repair request that explicitly asks for a full formal report; the user can still keep free-form chat without syncing.

