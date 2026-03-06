## MODIFIED Requirements

### Requirement: Chat composer MUST support attachment selection and removal
The chat page MUST provide an attachment upload entry in the composer for the active session.

The user MUST be able to select one or more supported files, review the selected attachments before sending, and remove selected attachments before submission.

The composer MUST also support drag-and-drop file upload onto the composer region.

#### Scenario: User selects attachments before sending
- **WHEN** user chooses one or more supported files in the chat composer
- **THEN** the UI shows the selected attachments in the composer before sending

#### Scenario: User removes a selected attachment
- **WHEN** user clicks remove on a selected attachment chip
- **THEN** that attachment is removed from the pending send list

#### Scenario: User drags files into composer
- **WHEN** user drags supported files over the chat composer and drops them
- **THEN** the files are added to the pending attachment list
- **AND** the composer shows a visible drag-active state during the drop interaction

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
