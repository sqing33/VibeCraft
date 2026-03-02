# UI (delta): migrate-ui-to-heroui

## MODIFIED Requirements

### Requirement: Technology Stack

The system MUST use React + TypeScript + Vite for the frontend build. The system MUST use Tailwind CSS + HeroUI (`@heroui/react`) for styling and UI components. The system MUST use Zustand for state management. The system MUST use React Flow + dagre for DAG visualization. The system MUST use xterm.js with fit addon for terminal rendering.

#### Scenario: Frontend builds successfully

- **WHEN** running `npm run build` in the ui/ directory
- **THEN** the build completes without errors
- **AND** produces static assets in ui/dist/
