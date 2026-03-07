## MODIFIED Requirements

### Requirement: Updated LLM settings take effect without daemon restart
After `PUT /api/v1/settings/llm` succeeds, the daemon MUST update the in-memory configuration so that model profiles are immediately available for:
- CLI tool model pools
- helper SDK tasks

The daemon MUST NOT require every saved model profile to appear as a primary executable expert in `GET /api/v1/experts`.

#### Scenario: New model is available in CLI tool pool
- **WHEN** client saves settings containing a new OpenAI-compatible model profile
- **THEN** that model becomes selectable under the `Codex CLI` tool configuration without daemon restart
