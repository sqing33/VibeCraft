## MODIFIED Requirements

### Requirement: Chat page MUST use an immersive workspace layout
The chat page MUST render inside the shared workspace app shell.

Within that shell, the top of the left rail MUST show the global product navigation, the middle of the left rail MUST show chat session browsing and creation, and the bottom of the left rail MUST show shared status and utility controls.

The right conversation canvas MUST remain the visually dominant bordered panel on the right.

#### Scenario: User opens chat page
- **WHEN** user navigates to `#/chat`
- **THEN** the shared workspace shell is visible
- **AND** the chat lane is visually highlighted in the global navigation
- **AND** the middle left sidebar region shows the chat session list and new-session action
- **AND** the conversation canvas remains the dominant bordered panel on the right

#### Scenario: User switches back to chat from another product lane
- **WHEN** the user navigates from Orchestrations or Github 知识库 back to `#/chat`
- **THEN** the shared workspace shell remains mounted
- **AND** the middle left sidebar region swaps back to the chat session list
- **AND** the right content region swaps back to the conversation workspace
