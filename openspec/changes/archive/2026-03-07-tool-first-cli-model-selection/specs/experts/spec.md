## MODIFIED Requirements

### Configuration Loading
The system MUST load expert configurations from `~/.config/vibecraft/config.json` under the `experts` array. Each expert MUST have a unique `id`. Each expert MUST be classifiable as one of:
- a process expert
- a primary CLI tool expert
- a helper SDK expert
- a custom persona expert

The system MUST NOT require `llm.models` to be mirrored as primary executable experts for chat/workflow/orchestration selection.

#### Scenario: Builtin CLI tool experts load without llm-model mirroring
- **WHEN** daemon starts with builtin `codex` and `claude` tool experts enabled
- **THEN** they are available as primary execution experts
- **AND** the system does not need to expose every `llm-model` as a primary executable expert

### Expert List API
The system MUST provide `GET /api/v1/experts` returning experts intended for execution-time selection, with metadata sufficient to distinguish helper-only entries from primary execution entries.

Helper-only model mirrors MAY remain internally available, but they MUST NOT dominate the primary user-facing execution selection flow.

#### Scenario: Public experts distinguish helper-only entries
- **WHEN** client requests `GET /api/v1/experts`
- **THEN** each returned item includes helper/runtime metadata
- **AND** UI clients can exclude helper-only entries from primary chat selection
