# Report Schema

Use the following **three-part top-level structure** exactly. This schema is optimized for human readability first, while preserving AI-grade mechanism traceability.

## 1. 项目参数与结构解析 (Project Parameters and Structure)
Include:
- repository URL
- requested ref
- resolved ref
- resolved commit (if available)
- generation timestamp
- analysis depth
- analysis mode (`multi-agent` or `single-agent`)
- source directory
- index file path
- sub-agent result path (if used)

And include repository structure overview:
- total file count
- indexed text file count
- top language distribution
- likely runtime entrypoints
- main module boundaries
- README-first note / fallback note
- exactly 3 project characteristics with mechanism explanation, confidence, and evidence refs

## 2. 面向人的功能说明 (Human-readable Feature Explanation)
Create one subsection per feature in input order.

Each feature summary must include:
- 功能作用（优先 direct_answer，用“用途 + 价值”表达）
- 特殊功能（优先失败恢复/并发时序等高信号能力）
- 实现想法（结合控制流 + 数据流 + 状态生命周期解释“为什么这样做”）
- confidence (`high`/`medium`/`low`)
- 2-5 key evidence references (`path:line`)

## 3. 面向 AI 的实现细节与证据链 (AI-facing Mechanism Details and Evidence)
Create one subsection per feature in input order.

Each feature subsection must include:
- `Runtime Control Flow`
- `Data Flow`
- `State and Lifecycle`
- `Failure and Recovery`
- `Concurrency and Timing`
- `Key Evidence`
- `Inference and Unknowns`

### Runtime Control Flow
Describe trigger -> dispatch -> execution -> completion flow.
If uncertain, mark uncertain links as `inference`.

### Data Flow
Describe important inputs/outputs and transformation boundaries.
Avoid generic architecture text.

### State and Lifecycle
Describe key state transitions, ownership, and lifecycle events.

### Failure and Recovery
Describe explicit failure handling paths and recovery behavior.
If no evidence, state that as a gap.

### Concurrency and Timing
Describe observed parallelism/ordering/queue behavior and timing-sensitive points.
If not evident, state low-confidence explicitly.

### Key Evidence
For each evidence item:
- `path:line`
- short snippet

### Inference and Unknowns
List uncertainties and missing verification points.

### Conditional Block: Invocation Path Classification
Add this block only when feature intent involves SDK/CLI/sub-agent/runtime invocation routing.

Block content:
- invocation path type: `SDK` | `CLI` | `Hybrid` | `inference`
- working directory resolution notes (`working_dir` / `current_dir` / container mapping)
- if no direct evidence, explicitly mark `inference`

### Global tail block under Part 3
After all features, include global block:
- `Cross-feature Coupling and System Risks`
- shared-module coupling risks across features (if present)
- global blind spots (generated code, binary-only, truncated scans)
- sections with weak evidence confidence

## Deep Mode Additions
When depth is `deep`, add `Deep Audit` under each feature with:
- detailed boundary checklist
- performance/concurrency risk hints
- failure-branch signals
- test coverage signals for involved modules

Do not emit generic deep-audit filler text.

## Hard Rules (unchanged)
- Keep `path:line` traceability for key conclusions.
- Keep `confidence` and `inference` mechanisms.
- Mark non-evidence claims as `inference`.
- Prefer mechanism explanation over file/function dump.
- Keep `Run N` append behavior unchanged.

## Migration Note
Older reports may use six top-level sections. New reports use three top-level parts while preserving the same mechanism dimensions and evidence contract in Part 3.
