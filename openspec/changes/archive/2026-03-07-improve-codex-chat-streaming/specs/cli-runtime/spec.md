## MODIFIED Requirements

### Requirement: Chat wrappers MUST emit normalized or parseable streaming events
CLI wrappers used for chat MUST expose a stream that the daemon can parse incrementally for assistant deltas, session updates, and final completion.

For Codex-backed chat turns, the system MAY satisfy this requirement via the official app-server JSON-RPC event stream instead of a shell wrapper, provided that the daemon still exposes normalized `chat.turn.*` events and preserves the artifact contract.

#### Scenario: Codex chat uses app-server event stream
- **WHEN** a Codex-backed chat turn starts through the app-server transport
- **THEN** the daemon consumes JSON-RPC notifications such as `item/agentMessage/delta` and `item/reasoning/*Delta`
- **AND** relays normalized chat events before turn completion

#### Scenario: Codex chat preserves artifact contract
- **WHEN** a Codex-backed chat turn completes through app-server
- **THEN** the system still writes chat runtime artifacts including `session.json` and `final_message.md`
- **AND** the stored session reference remains reusable by later turns
