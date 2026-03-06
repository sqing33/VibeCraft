## MODIFIED Requirements

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

The system MUST provide an API to read persisted attachment content for a specific chat session attachment so that the UI can preview supported attachment types.

#### Scenario: Message history includes attachments
- **WHEN** a user sends a turn with attachments and later calls `GET /api/v1/chat/sessions/:id/messages`
- **THEN** the returned user message contains its attachment metadata
- **AND** the metadata is stable across daemon restart

#### Scenario: Read persisted attachment content
- **WHEN** client calls the chat attachment content API with a valid `session_id` and `attachment_id`
- **THEN** the daemon returns the stored attachment content with the correct content type
- **AND** the UI can use that response for preview
