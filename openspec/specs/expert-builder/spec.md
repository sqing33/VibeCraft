# Expert Builder

## Purpose

Expert Builder 定义“通过 AI 对话生成结构化专家草稿”的系统能力，供设置页中的 AI 创建专家流程复用。

## Requirements

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

### Requirement: Expert builder can derive drafts from persisted session history

The system MUST be able to regenerate the next expert draft from the full persisted builder conversation history of a session.

#### Scenario: Rebuild prompt from saved messages

- **WHEN** a builder session already contains historical user and assistant messages
- **THEN** the next generation round uses the ordered saved message history as context
- **AND** the returned draft becomes a new snapshot rather than replacing prior versions
