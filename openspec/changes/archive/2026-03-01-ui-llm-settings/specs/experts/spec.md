# Experts (delta): ui-llm-settings

## ADDED Requirements

### Requirement: Expert registry supports runtime reload

The system MUST support reloading the in-memory expert registry at runtime after configuration updates (e.g. after saving LLM settings), without requiring a daemon restart.

#### Scenario: Reload updates listExperts API

- **WHEN** the daemon accepts an LLM settings update that changes model profiles
- **THEN** subsequent `GET /api/v1/experts` responses reflect the updated expert set

