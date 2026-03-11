## MODIFIED Requirements

### Requirement: UI SHALL support multi-turn chat with streaming render

The chat page MUST continue to render existing local chat sessions and MUST additionally expose a frontend entry for importing Codex CLI history.

The import entry MUST be available from the chat session list area and MUST open a selection dialog that shows readable history titles returned by the backend.

The dialog MUST allow users to:
- search or filter the thread list by title
- select one or more history entries
- start the import

After a successful import, the UI MUST refresh the chat session list and SHOULD switch to the first newly imported session.

#### Scenario: Import Codex history from the chat page

- **WHEN** the user clicks the chat page import entry and confirms selected Codex history threads
- **THEN** the UI calls the backend import API
- **AND** the session list refreshes with the newly imported sessions
- **AND** imported sessions are shown with readable titles instead of raw thread ids
