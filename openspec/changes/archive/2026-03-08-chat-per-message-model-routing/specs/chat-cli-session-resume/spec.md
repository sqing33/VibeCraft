## ADDED Requirements

### Requirement: Codex CLI session resume MUST restore reasoning effort defaults
When a chat session stores a Codex thread id and a last-used `reasoning_effort`, the system MUST include that effort as `config.model_reasoning_effort` during Codex thread start or resume.

#### Scenario: Codex resume restores reasoning effort default
- **WHEN** a chat session already stores a Codex thread id and `reasoning_effort`
- **THEN** the next Codex thread start or resume includes `config.model_reasoning_effort`
- **AND** only the current turn input is sent in `turn/start`
