# Report Schema

Use the following section order exactly. This schema is optimized for
principle-first explanation with strict evidence traceability.

## 1. Metadata
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

## 2. Repository Mental Model
Include:
- total file count
- indexed text file count
- top language distribution
- likely runtime entrypoints
- main module boundaries

## 3. Executive Principle Summary
Create one subsection per feature in input order.

Each feature summary must include:
- direct principle-first answer (what mechanism implements the feature)
- mechanism narrative in 4-8 sentences, explaining how control/data/state/failure/concurrency aspects work together
- prioritize explanatory language for human understanding; do not reduce this section to function/file list
- 2-5 key evidence references (`path:line`)
- confidence level (`high`/`medium`/`low`)

## 4. Feature Principle Analysis
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

## 5. Cross-feature Coupling and System Risks
Include:
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

## Formatting Rules
- use Markdown headings
- keep snippets short
- prefer relative paths in references
- mark non-evidence claims as `inference`
- avoid candidate-path dump sections
