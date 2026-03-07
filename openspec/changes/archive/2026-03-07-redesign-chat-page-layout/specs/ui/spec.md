## MODIFIED Requirements

### Requirement: UI SHALL provide chat session navigation and page
The UI MUST provide a navigation entry to a dedicated chat page. The chat page MUST support session list browsing and selecting one active session.

The chat page MUST use a two-region layout:
- a fixed-width left session rail for browsing, creating, and managing sessions,
- a dominant right conversation workspace for transcript reading and message composition.

Within the conversation workspace, the UI MUST provide:
- a lightweight header showing the current session context,
- a scrollable transcript region,
- a bottom-anchored composer region that remains stable while the transcript changes.

The visible transcript content SHOULD remain centered within a constrained readable column instead of spanning the full workspace width.

#### Scenario: Open chat page
- **WHEN** user clicks the chat entry in top navigation
- **THEN** the app navigates to `#/chat`
- **AND** the page displays the session rail and conversation workspace

#### Scenario: Switch sessions from the left rail
- **WHEN** user selects another session in the left session rail
- **THEN** the active conversation workspace updates to that session
- **AND** the composer remains in the bottom area of the workspace

#### Scenario: Read long transcript in wide viewport
- **WHEN** the chat page is opened on a desktop-sized viewport
- **THEN** the transcript content appears in a constrained readable column inside the conversation workspace
- **AND** the session rail remains narrower than the conversation workspace
