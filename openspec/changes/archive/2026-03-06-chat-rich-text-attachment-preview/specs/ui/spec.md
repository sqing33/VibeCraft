## MODIFIED Requirements

### Requirement: Chat history MUST render message attachment metadata
When a message includes attachments, the chat UI MUST display attachment metadata in the corresponding message bubble.

The initial rendering MUST include at least file name and attachment kind.

For preview-supported attachment types, the UI MUST provide a preview action.

#### Scenario: User message shows sent attachments
- **WHEN** a previously sent message has attachments
- **THEN** the chat history shows those attachments below the message content

#### Scenario: User previews an image attachment
- **WHEN** user clicks preview on an image attachment in chat history
- **THEN** the UI opens an in-app preview modal showing the image

#### Scenario: User previews a PDF attachment
- **WHEN** user clicks preview on a PDF attachment in chat history
- **THEN** the UI opens an in-app preview modal embedding the PDF document

#### Scenario: User previews a markdown attachment
- **WHEN** user clicks preview on a Markdown attachment in chat history
- **THEN** the UI opens an in-app preview modal rendering the Markdown content

#### Scenario: User previews a code attachment
- **WHEN** user clicks preview on a code or config attachment in chat history
- **THEN** the UI opens an in-app preview modal rendering the file with syntax highlighting and line numbers
