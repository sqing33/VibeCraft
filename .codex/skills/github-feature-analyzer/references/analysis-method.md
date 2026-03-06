# Analysis Method

## 1. Scope and Inputs
Use this method when task intent is:
- explain feature implementation logic
- explain runtime mechanism and boundaries
- compare implementation principles across features
- analyze project characteristics from README before feature deep-dive

Required inputs:
- `repo_url`
- `features` (one or more)

Optional inputs:
- `ref` (default `main`)
- `depth` (`standard` or `deep`)
- `agent_mode` (`multi`/`single`/`auto`)

## 2. Retrieval Strategy
Apply strict order:
1. Use MCP to verify repository access and quickly probe candidate files.
2. Fetch local source with `scripts/fetch_repo.py` (`mcp-first` mode).
3. If archive/API path fails, allow git fallback.
4. Stop only when both paths fail.

## 3. Multi-agent Topology (default)
When `agent_mode=multi` (or `auto` and available), run layered sub-agents:
1. `overview` agent: repository overview, entrypoints, module boundaries.
2. `architecture` agent: control plane, data boundaries, lifecycle surfaces.
3. `feature` agents: one per feature in parallel.

Parent agent responsibilities:
- enforce agent output schema contract
- deduplicate evidence by `path:line`
- merge conflicting claims with explicit conflict notes
- pass merged artifact to final report renderer

## 4. Principle-first Analysis Dimensions
For each feature, the final report must cover five dimensions:
1. Runtime Control Flow
2. Data Flow
3. State and Lifecycle
4. Failure and Recovery
5. Concurrency and Timing

Function or file mapping is supporting evidence, not the main narrative.

## 5. README-first Repository Characteristic Pass (default)
Before feature-specific analysis:
- read repository root README first (`README.md`/`README`/`readme*`)
- extract 3 project characteristics or signature capabilities
- for each characteristic, provide mechanism explanation and limited evidence refs
- if README is missing/weak, fallback to index/code inference and mark as `inference`

## 6. Two-phase Feature Process
1. Internal retrieval/indexing phase:
- normalize feature terms
- score files by path and line evidence
- filter weak/noisy matches

2. Final synthesis phase:
- answer each feature directly in principle language
- in Executive Principle Summary, use mechanism narrative (4-8 sentences) so readers can understand "how it works" without reading code
- explain mechanism along the five dimensions
- provide `path:line` evidence for each key conclusion
- list concrete risks and unknowns

## 7. Evidence Rules
Every key conclusion must include evidence:
- preferred format: `relative/path/to/file.ext:line`
- include short snippet when useful

If no direct evidence exists:
- mark as `inference`
- state what was searched and why confirmation is missing

## 8. Noise Control
Apply code-first filtering:
- prefer production runtime code over docs/comments-only files
- include docs only if code evidence is insufficient
- keep internal scoring artifacts out of final report

## 9. Multi-feature Handling
When multiple features are requested:
- analyze each feature in an independent section
- identify shared modules and coupling points
- report coupling risks only if overlap is evidence-backed

When feature text mentions runtime invocation form (SDK/CLI/sub-agent/working dir):
- classify path as `SDK`, `CLI`, or `Hybrid`
- explain working directory resolution basis (`working_dir`/`current_dir`/container mapping) with evidence or explicit `inference`

## 10. Depth Modes
`standard` mode:
- principle-first answer
- five-dimension mechanism summary
- concrete evidence and risks

`deep` mode:
- richer control/data/lifecycle sequence interpretation
- evidence-based boundary checklist
- evidence-based performance/concurrency signals
- evidence-based failure-path and test-signal notes

## 11. Quality Bar
A report is acceptable only if:
- project characteristic section is present and includes 3 items (or explicit fallback note)
- each feature has at least one evidence item, or explicit “no evidence found”
- each conclusion maps to concrete file/line references
- unresolved ambiguities are listed explicitly
- output follows `references/report-schema.md`
