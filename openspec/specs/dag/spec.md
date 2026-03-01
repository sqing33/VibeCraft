# DAG 生成与校验

## Purpose

Master 节点通过 AI（Claude/OpenAI）输出结构化 DAG JSON，daemon 负责从输出中提取、校验、落库。DAG 定义了 worker 节点间的依赖关系和执行配置。

## Requirements

### JSON Extraction

The system MUST extract the first complete JSON object (`{...}`) from master node output. The system MUST support JSON embedded within natural language text, as AI may output explanatory text surrounding the JSON.

#### Scenario: Extract JSON from mixed output

- **WHEN** master output is "Here is the plan:\n{...valid JSON...}\nLet me know if changes needed"
- **THEN** the system extracts the first `{...}` JSON object
- **AND** proceeds to validate the extracted JSON

#### Scenario: Structured output via output_schema

- **WHEN** master expert has `output_schema: "dag_v1"` configured
- **THEN** the SDK call uses structured output mode
- **AND** the response is directly parsed as JSON

### DAG Schema Validation

The system MUST validate DAG JSON against the schema: nodes array (each with id, title, type, expert_id, prompt, and optional fields: fallback_expert_id, complexity, quality_tier, model, routing_reason) and edges array (each with from, to, type, and optional source_handle/target_handle).

#### Scenario: Valid DAG passes validation

- **WHEN** DAG JSON contains valid nodes with unique IDs, valid edges, and known expert_ids
- **THEN** validation passes
- **AND** the DAG is ready for persistence

### Integrity Checks

The system MUST verify: node list is non-empty, node IDs are unique within the DAG, all edge from/to references point to existing nodes, all expert_id values exist in the Expert registry, and the graph is acyclic (verified via Kahn's algorithm topological sort).

#### Scenario: Cycle detection

- **WHEN** DAG edges form a cycle (A→B→C→A)
- **THEN** validation fails with "cycle detected" error
- **AND** the DAG is rejected

#### Scenario: Unknown expert_id

- **WHEN** a DAG node references expert_id "nonexistent"
- **THEN** validation fails with "unknown expert_id" error

### DAG Persistence

The system MUST persist validated DAG by writing nodes and edges to SQLite. Node IDs MUST be rewritten to internal IDs with `nd_` prefix. The system MUST write a `dag.generated` audit event and broadcast it via WebSocket.

#### Scenario: DAG written to database

- **WHEN** DAG validation passes
- **THEN** worker nodes are created in the nodes table with `nd_` prefixed IDs
- **AND** edges are created in the edges table
- **AND** `dag.generated` event is broadcast via WebSocket

### Complexity Mapping

The system MUST map complexity to quality_tier: `low → fast`, `medium → balanced`, `high → deep`. The system MUST allow `workflow_title` in DAG to override the workflow title.

#### Scenario: Complexity auto-mapping

- **WHEN** a DAG node has `complexity: "high"` and no explicit `quality_tier`
- **THEN** the system sets `quality_tier: "deep"` automatically
