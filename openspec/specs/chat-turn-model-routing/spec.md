# chat-turn-model-routing Specification

## Purpose
TBD - created by archiving change chat-per-message-model-routing. Update Purpose after archive.
## Requirements
### Requirement: Chat turns MUST support per-message expert override
The system MUST allow a chat turn request to specify an optional `expert_id` to override the session's current default expert for that single turn.

If `expert_id` is omitted, the system MUST use the session's current default `expert_id`.

If `expert_id` is provided, the system MUST validate it resolves to an SDK provider expert (OpenAI/Anthropic/Demo) before calling the provider.

#### Scenario: Turn overrides expert
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/turns` with `input` and `expert_id="X"`
- **THEN** the provider call uses expert `X` for that turn
- **AND** the session remains active and usable for subsequent turns

#### Scenario: Turn uses session default expert
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/turns` with `input` and no `expert_id`
- **THEN** the provider call uses the session's default expert

### Requirement: Chat messages MUST persist model identity metadata
The system MUST persist the following fields for each chat message:
- `expert_id`
- `provider`
- `model`

For messages created after this capability is enabled, these fields MUST be non-empty.
For historical messages created before this capability, these fields MAY be empty and clients MUST tolerate missing values.

#### Scenario: Message metadata is stored
- **WHEN** a successful turn completes
- **THEN** both the appended user message and assistant message include `expert_id`, `provider`, and `model` in `GET /api/v1/chat/sessions/:id/messages`

### Requirement: Anchor reuse MUST be scoped to provider+model
The system MUST NOT reuse provider anchors across different provider+model combinations.

For a turn whose provider+model matches the session's current default provider+model, the system MUST attempt to reuse the stored anchor.

For a turn whose provider+model does not match the session's current default provider+model, the system MUST bypass anchor reuse and use reconstructed local context (summary + recent messages) as the prompt input.

#### Scenario: Same provider+model reuses anchor
- **WHEN** the session has an existing provider anchor for its current default provider+model
- **AND** client sends a turn using the same expert (same provider+model)
- **THEN** the provider call includes the stored anchor

#### Scenario: Switching model bypasses anchor
- **WHEN** client sends a turn with an expert whose provider+model differs from the session's current default
- **THEN** the provider call does not include the prior anchor
- **AND** the input uses reconstructed local context from summary + recent messages

### Requirement: WebSocket chat events MUST include model identity metadata
The system MUST include `expert_id`, `provider`, and `model` in the payload of `chat.turn.started` for a turn.

#### Scenario: Started event includes model identity
- **WHEN** a turn starts
- **THEN** `chat.turn.started` payload contains `session_id`, `turn`, `expert_id`, `provider`, and `model`

