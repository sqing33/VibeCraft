## ADDED Requirements

### Requirement: Chat turns MUST accept multimodal attachments
The system MUST allow `POST /api/v1/chat/sessions/:id/turns` to accept attachments together with an optional text input.

The endpoint MUST continue to accept the existing JSON request shape for pure-text turns.

When attachments are present, the endpoint MUST accept `multipart/form-data` with fields:
- `input` (optional text)
- `expert_id` (optional text)
- one or more `files` parts

The system MUST reject a turn only when both the trimmed text input is empty and the attachment list is empty.

#### Scenario: Send text with attachments
- **WHEN** client sends `multipart/form-data` containing `input="请看这张图"` and one or more `files`
- **THEN** the turn is accepted
- **AND** the provider call uses both the text input and the attachments

#### Scenario: Send attachments without text
- **WHEN** client sends `multipart/form-data` containing only `files`
- **THEN** the turn is accepted
- **AND** the system generates a provider input that tells the model to read the attachments first

#### Scenario: Reject empty turn
- **WHEN** client sends a turn request with no non-empty `input` and no files
- **THEN** the API returns a validation error

### Requirement: Chat attachments MUST be persisted with message metadata
For every accepted turn with attachments, the system MUST persist each attachment as message-linked metadata and local file content.

Each persisted attachment MUST include at least:
- stable `attachment_id`
- `message_id`
- `session_id`
- `file_name`
- `mime_type`
- `kind`
- `size_bytes`
- creation timestamp

Message query APIs MUST return the attachments associated with each message.

#### Scenario: Message history includes attachments
- **WHEN** a user sends a turn with attachments and later calls `GET /api/v1/chat/sessions/:id/messages`
- **THEN** the returned user message contains its attachment metadata
- **AND** the metadata is stable across daemon restart

### Requirement: Providers MUST receive provider-native multimodal input
The system MUST translate persisted attachments into the native input structure supported by the selected provider SDK.

- For OpenAI Responses, images MUST be sent as image inputs, PDFs MUST be sent as file inputs, and text/code files MUST be added as text content.
- For Anthropic Messages, images MUST be sent as image blocks, PDFs MUST be sent as document blocks, and text/code files MUST be added as text blocks.

The system MUST NOT rely on local file path references alone for provider ingestion.

#### Scenario: OpenAI receives image and pdf input blocks
- **WHEN** an OpenAI-backed chat turn includes an image and a PDF attachment
- **THEN** the OpenAI SDK request includes native image/file input items for those attachments

#### Scenario: Anthropic receives image and document blocks
- **WHEN** an Anthropic-backed chat turn includes an image and a PDF attachment
- **THEN** the Anthropic SDK request includes native image/document content blocks for those attachments

### Requirement: Attachment validation MUST enforce supported types and size limits
The system MUST validate attachments before provider execution.

The system MUST reject unsupported file types, too many files, oversized files, or oversized total payloads with user-readable validation errors.

The initial supported types MUST include:
- common image formats
- PDF
- plain text and code files

#### Scenario: Unsupported file type is rejected
- **WHEN** client uploads an unsupported binary file type
- **THEN** the API returns a validation error before any provider call is made

#### Scenario: Oversized payload is rejected
- **WHEN** the uploaded files exceed configured size or count limits
- **THEN** the API returns a validation error
- **AND** no attachment metadata is persisted
