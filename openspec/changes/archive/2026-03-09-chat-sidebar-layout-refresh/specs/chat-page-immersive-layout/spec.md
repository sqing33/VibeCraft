## MODIFIED Requirements

### Requirement: Chat page MUST use an immersive workspace layout
The chat page MUST present an immersive workspace layout composed of an integrated left navigation rail and a dominant conversation canvas on the right.

The left navigation rail MUST merge with the page surface instead of appearing as an independently bordered card.

The left navigation rail MUST place page-level navigation at the top, session navigation in the middle, and system status plus utility actions at the bottom.

The conversation canvas MUST remain the visually dominant area and MUST continue to be rendered as a separate bordered panel on the right.

#### Scenario: User opens chat page
- **WHEN** user navigates to `#/chat`
- **THEN** the page shows a left navigation rail that visually blends into the page background
- **AND** the left rail contains top navigation, a session list region, and bottom system controls
- **AND** the conversation canvas remains the dominant bordered panel on the right

## ADDED Requirements

### Requirement: Conversation header MUST use a three-zone layout
The conversation canvas header MUST place session metadata on the left, the active conversation title in the visual center, and status plus conversation actions on the right.

The centered title MUST remain visually centered when the left and right zones have different content widths.

Long metadata or title text MUST truncate instead of forcing the header to grow vertically in the default desktop layout.

#### Scenario: Active session is displayed
- **WHEN** an active session is open in the conversation canvas
- **THEN** the header shows session identifier and model metadata on the left
- **AND** the conversation title is displayed in the center of the header
- **AND** the status chip and fork action are displayed on the right

#### Scenario: No active session is selected
- **WHEN** no chat session is currently active
- **THEN** the header still preserves the same three-zone layout
- **AND** the center area shows the empty-state title text
