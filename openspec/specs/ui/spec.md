# 前端 UI

## Purpose

React SPA 提供 Kanban 列表页 + Workflow 详情页（DAG + Terminal Pool）交互，通过 HTTP/WebSocket 与 daemon 通信。
## Requirements
### Requirement: Technology Stack

The system MUST use React + TypeScript + Vite for the frontend build. The system MUST use Tailwind CSS + HeroUI (`@heroui/react`) for styling and UI components. The system MUST use Zustand for state management. The system MUST use React Flow + dagre for DAG visualization. The system MUST use xterm.js with fit addon for terminal rendering.

#### Scenario: Frontend builds successfully

- **WHEN** running `npm run build` in the ui/ directory
- **THEN** the build completes without errors
- **AND** produces static assets in ui/dist/

### Production hides demo-only tools by default

In production builds (`import.meta.env.DEV === false`), the UI MUST hide demo-only tools (e.g. oneshot demo execution) by default. In development builds, demo/diagnostic tools MAY be visible to aid debugging.

#### Scenario: Demo tools hidden in production

- **WHEN** the UI is served from `ui/dist/` by the daemon
- **THEN** demo-only actions are not visible by default

#### Scenario: Demo tools visible in development

- **WHEN** the UI is running under Vite dev server (`import.meta.env.DEV === true`)
- **THEN** demo/diagnostic actions MAY be visible

### Workflows-first information architecture

The UI MUST provide a primary navigation centered around Workflows. Connection settings and diagnostics MUST be accessible but MUST NOT dominate the default workflow-oriented UI.

#### Scenario: Opening the app lands on Workflows

- **WHEN** user opens the application
- **THEN** the Workflows Kanban is the primary visible content
- **AND** diagnostics/settings are reachable but not the main focus

### Settings contains connection and diagnostics

The UI MUST provide a Settings entry (e.g. drawer/modal) that contains daemon URL switching and diagnostics (version/paths/experts). Changing daemon URL MUST persist to local storage and MUST trigger a health check.

#### Scenario: Change daemon URL in Settings

- **WHEN** user updates the daemon URL in Settings and applies the change
- **THEN** the new URL is saved to local storage
- **AND** the UI performs `GET /api/v1/health` against the new URL
- **AND** the UI updates connection status (Health/WS)

### Kanban List Page

The system MUST render a Kanban board at route `/` with four columns: Todo, Running, Done, Failed. Each card MUST display: title, workspace, last updated time, run mode (Auto/Manual), running node count. The system MUST provide a `+ New` button to create workflows with fields: title, workspace, mode, master expert.

#### Scenario: Display workflow cards

- **WHEN** user opens the application
- **AND** health check passes
- **THEN** workflows are displayed in Kanban columns grouped by status

#### Scenario: Create new workflow

- **WHEN** user clicks `+ New` and fills in title, workspace, mode
- **THEN** a new workflow is created via POST /api/v1/workflows
- **AND** the card appears in the Todo column

### Detail Page Layout

The system MUST render workflow details at route `#/workflows/:id` with a left-right split layout. The top MUST include a run control bar with execution breakpoint toggle (auto ↔ manual) and `Approve all runnable` button (visible only in manual mode). The left pane MUST show DAG view. The right pane MUST show Terminal Pool.

#### Scenario: View running workflow details

- **WHEN** user clicks a running workflow
- **THEN** the detail page shows DAG on the left and terminal outputs on the right

### DAG View

The system MUST use React Flow with dagre auto-layout to render nodes and edges. Nodes MUST display title, expert, and status with color coding (green/yellow/red/gray). Clicking a node MUST scroll the right terminal to the corresponding pane and highlight it. Error nodes MUST show a red breathing animation (CSS).

#### Scenario: Node click triggers terminal focus

- **WHEN** user clicks a node in the DAG view
- **THEN** the right terminal scrolls to the corresponding pane
- **AND** the pane is visually highlighted

### Terminal Pool

The system MUST render terminal panes for running/selected nodes. Each pane MUST have minimum width `min-w-[420px]` with horizontal scroll container (`overflow-x-auto`). Each pane top bar MUST show node name, status, and Cancel/Retry buttons. Terminals MUST use xterm.js with fit addon, receiving WebSocket log chunks.

#### Scenario: Real-time log rendering

- **WHEN** WebSocket pushes `node.log` events with ANSI content
- **THEN** the corresponding xterm pane renders colored output in real-time

### Manual Mode Interaction

The system MUST allow clicking `Approve all runnable` to start all dependency-satisfied nodes. The system MUST support editing node expert (dropdown) and prompt (textarea). Saving edits MUST call `PATCH /api/v1/nodes/{id}`.

#### Scenario: Edit and approve nodes

- **WHEN** user edits a node's prompt and saves
- **THEN** PATCH /api/v1/nodes/{id} is called
- **AND** clicking Approve starts all runnable nodes

### WebSocket Subscription

The system MUST connect to `GET /api/v1/ws` for real-time event subscription. The system MUST handle events: `workflow.updated`, `dag.generated`, `node.updated`, `execution.started`, `execution.exited`, `node.log`. `node.log` chunks MUST be written directly to the corresponding xterm instance.

#### Scenario: WebSocket reconnection

- **WHEN** WebSocket connection is lost
- **THEN** the client automatically attempts reconnection
- **AND** resumes receiving events after reconnection

### Daemon Communication

The system MUST encapsulate all HTTP API calls in `daemon.ts`. The system MUST support daemon URL configuration via `VITE_DAEMON_URL` environment variable or runtime switching. The system MUST perform health checks via `GET /api/v1/health`.

#### Scenario: Health check on startup

- **WHEN** the application starts
- **THEN** it calls GET /api/v1/health
- **AND** displays connection status to the user

### UI text is Chinese by default

The UI MUST present user-facing copy in Simplified Chinese across the primary workflow path, including navigation labels, action buttons, form labels, status text, empty states, and error prompts.

#### Scenario: User opens workflows page

- **WHEN** user opens the application
- **THEN** primary page titles and action buttons are displayed in Simplified Chinese

#### Scenario: User sees error or empty state

- **WHEN** workflow list or detail page enters error/empty/loading states
- **THEN** the corresponding visible prompts are displayed in Simplified Chinese

### UI supports light and dark theme toggle

The UI MUST provide a user-accessible theme switch for light and dark modes. Theme selection MUST be persisted in local storage and restored on next load.

#### Scenario: User toggles theme

- **WHEN** user switches between light and dark theme in UI settings/topbar
- **THEN** the entire UI theme updates immediately
- **AND** selected theme is saved to local storage

#### Scenario: User reloads app after selecting theme

- **WHEN** user reloads the app after previously selecting dark or light theme
- **THEN** the UI restores the saved theme from local storage

### Default theme is light

When no persisted theme value exists, the UI MUST initialize with light theme.

#### Scenario: First-time visitor

- **WHEN** local storage does not contain a saved theme key
- **THEN** the UI loads in light theme

### Requirement: Settings uses tab navigation and includes LLM configuration

The UI MUST present the existing System Settings as a tabbed view.
The UI MUST provide at least two tabs:

- `连接与诊断`: contains daemon URL switching and diagnostics (version/paths/experts).
- `模型`: contains LLM Sources / Model Profiles configuration.

#### Scenario: User switches settings tabs

- **WHEN** user opens System Settings
- **THEN** the UI shows multiple tabs including `连接与诊断` and `模型`
- **AND** switching tabs updates the visible settings content

### Requirement: UI can edit and save LLM settings

In the `模型` tab, the UI MUST provide two sections:

- **Sources**: manage API sources (base URL + API key) without binding a model.
- **Models**: manage model profiles (model name, selected source, SDK provider: `codex(openai)` or `claudecode(anthropic)`).

The UI MUST save changes by calling `PUT /api/v1/settings/llm`.
After saving succeeds, the UI MUST refresh experts by calling `GET /api/v1/experts` so that workflow/node dropdowns can use the latest models.

#### Scenario: User adds a source and model and saves

- **WHEN** user creates a new Source and a new Model Profile referencing it
- **AND** user clicks Save
- **THEN** the UI calls `PUT /api/v1/settings/llm`
- **AND** the UI shows a success toast
- **AND** the UI refreshes the experts list via `GET /api/v1/experts`

### Requirement: Model profiles can be tested from the settings UI

In the `模型` settings tab, each model profile card MUST provide a `测试` button located to the left of the delete button.

When clicked, the UI MUST call `POST /api/v1/settings/llm/test` using the model card's current draft provider/model/base_url/api_key values.

The UI MUST show success or failure feedback to the user (e.g. toast).

#### Scenario: User tests a model profile

- **WHEN** user clicks `测试` on a model card with complete configuration
- **THEN** the UI calls `POST /api/v1/settings/llm/test`
- **AND** the UI displays the result to the user

### Requirement: LLM model profiles require a valid Source

When at least one LLM Source exists, the UI MUST ensure each model profile is bound to a non-empty, valid Source ID. The UI MUST prevent saving or testing LLM settings when any model profile has an empty Source selection.

#### Scenario: User cannot clear Source selection

- **WHEN** a model profile has at least one available Source option
- **AND** the user attempts to clear the Source selection in the UI
- **THEN** the UI keeps a non-empty Source selection (either the previous value or a default)

#### Scenario: Saving is blocked when Source is missing

- **WHEN** the user clicks Save while any model profile has an empty Source selection
- **THEN** the UI shows an error toast describing the missing Source
- **AND** does not submit the settings update request

