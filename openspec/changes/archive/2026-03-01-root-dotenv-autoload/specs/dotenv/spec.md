## ADDED Requirements

### Requirement: Dotenv is loaded at daemon startup
The system MUST attempt to load dotenv before loading runtime configuration, so that `${ENV_VAR}` substitution can use variables from dotenv.

#### Scenario: Load dotenv before config load
- **WHEN** the daemon process starts
- **THEN** dotenv loading is attempted before config is loaded and experts are resolved

### Requirement: Dotenv default path is repo root .env
If no explicit dotenv path is configured, the system MUST attempt to locate the repository root by searching parent directories for a `.git` directory, and then attempt to load `<repo_root>/.env`.

#### Scenario: Locate repo root from backend working directory
- **WHEN** the daemon is started with current working directory inside the repository (e.g., `<repo_root>/backend`)
- **THEN** the system finds `<repo_root>` by searching parent directories for `.git`
- **AND** the system attempts to load `<repo_root>/.env`

#### Scenario: Repo root cannot be located
- **WHEN** the daemon is started in a directory tree that does not contain a `.git` directory
- **THEN** the system MUST NOT attempt to load a default `.env` path

### Requirement: Dotenv path can be explicitly configured
If `VIBE_TREE_DOTENV_PATH` is set, the system MUST attempt to load dotenv from that path.

#### Scenario: Load dotenv from explicit path
- **WHEN** `VIBE_TREE_DOTENV_PATH` is set to an existing dotenv file path
- **THEN** the system attempts to load dotenv from that path

### Requirement: Dotenv loading can be disabled
If `VIBE_TREE_DOTENV` is set to `"0"`, the system MUST skip dotenv loading entirely.

#### Scenario: Disable dotenv loading
- **WHEN** `VIBE_TREE_DOTENV` is `"0"`
- **THEN** the system does not attempt to load dotenv (default or explicit path)

### Requirement: Dotenv variables override existing process environment
When dotenv is loaded, each key/value pair MUST be written to the process environment and MUST override any existing environment variable with the same key.

#### Scenario: Override existing environment variable
- **WHEN** the process environment contains `ANTHROPIC_API_KEY="old"`
- **AND** the loaded dotenv file contains `ANTHROPIC_API_KEY="new"`
- **THEN** the process environment after dotenv loading contains `ANTHROPIC_API_KEY="new"`

### Requirement: Dotenv failures do not prevent startup
If dotenv loading fails due to missing file, read error, or parse error, the system MUST continue starting the daemon.

#### Scenario: Missing .env does not block startup
- **WHEN** the system attempts to load a dotenv path and the file does not exist
- **THEN** the daemon continues startup

#### Scenario: Parse error does not block startup
- **WHEN** the system attempts to load a dotenv file and parsing fails
- **THEN** the daemon continues startup

### Requirement: Logging MUST NOT leak dotenv values
Startup logs about dotenv loading MUST NOT include any dotenv values.

#### Scenario: Log does not contain secrets
- **WHEN** dotenv is loaded successfully
- **THEN** logs MAY include the dotenv path and the number of keys loaded
- **AND** logs MUST NOT include any dotenv value content
