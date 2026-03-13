## 1. Dotenv Core

- [x] 1.1 Add `backend/internal/dotenv` module with repo-root `.env` discovery, `VIBECRAFT_DOTENV_PATH`, and `VIBECRAFT_DOTENV=0`
- [x] 1.2 Integrate dotenv loading into `backend/cmd/vibecraft-daemon/main.go` before `config.Load()` with safe logging (no values)
- [x] 1.3 Add Go dependency for dotenv parsing (`github.com/joho/godotenv`)

## 2. Tests

- [x] 2.1 Add unit tests for disable switch, explicit path, repo root discovery, and override semantics
- [x] 2.2 Run `cd backend && go test ./...` and ensure tests pass

## 3. Repo Hygiene & Docs

- [x] 3.1 Add root `.gitignore` to ignore `.env`
- [x] 3.2 Add root `.env.example` documenting required keys (Anthropic/OpenAI)
- [x] 3.3 Update `README.md` with dotenv usage and the new environment variables
- [x] 3.4 Update `PROJECT_STRUCTURE.md` to index the new dotenv module and related keywords
