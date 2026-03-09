## MODIFIED Requirements

### Requirement: Chat runtime MUST support session-scoped warm Codex app-server reuse
For Codex-backed chat sessions, the daemon MUST support keeping a session-scoped app-server runtime alive across multiple turns within the same daemon lifecycle.

The daemon MUST evict or close warm runtimes when they become idle for too long, when the owning chat session is archived, or when the daemon shuts down.

#### Scenario: Idle runtime is evicted after timeout
- **WHEN** a warm Codex chat runtime remains unused past the configured idle timeout
- **THEN** the daemon closes the underlying app-server process
- **AND** removes the runtime from the in-memory pool

#### Scenario: Archiving a chat session releases its warm runtime
- **WHEN** the client archives a chat session that currently owns a warm Codex runtime
- **THEN** the daemon closes that runtime
- **AND** later follow-up turns require a fresh runtime or a cold `thread/resume`

#### Scenario: Daemon shutdown closes warm runtimes
- **WHEN** the daemon process begins shutdown
- **THEN** all warm Codex chat runtimes are closed before process exit
