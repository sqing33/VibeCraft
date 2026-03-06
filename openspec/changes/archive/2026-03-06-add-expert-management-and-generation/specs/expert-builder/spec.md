## ADDED Requirements

### Requirement: Settings can generate expert drafts via AI conversation
The system MUST provide an AI-assisted expert generation flow that accepts a conversation transcript, invokes an expert-creator skill, and returns a structured expert draft that can be published without manual schema translation.

#### Scenario: Generate draft from conversation
- **WHEN** client calls the expert generation API with a builder expert, conversation messages, available model ids, and available skills
- **THEN** the system returns an assistant reply plus a structured expert draft payload
- **AND** the draft payload contains normalized fields required by expert settings

#### Scenario: Demo builder works without external network
- **WHEN** the builder expert uses provider `demo`
- **THEN** the system still returns a deterministic structured expert draft
- **AND** the response can be used to validate the UI flow locally

### Requirement: Expert generation validates builder output
The system MUST validate generated expert drafts before returning them to the client.

#### Scenario: Builder returns invalid model reference
- **WHEN** generated draft references a model id that does not exist in LLM settings
- **THEN** the system rejects the generation request with a readable validation error

#### Scenario: Builder omits optional fallback model
- **WHEN** generated draft does not include a secondary model id
- **THEN** the system accepts the draft
- **AND** marks the expert as single-model without fallback
