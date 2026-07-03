# Repository Architecture

## Structure

- `cmd/cli_mate`: CLI entrypoint (Cobra commands)
- `internal/agent`: Orchestration, session state, context window, streaming
- `internal/providers`: Provider contracts and adapters
- `internal/tools`: Executable tools exposed to the AI agent
- `internal/config`: Configuration and profiles
- `internal/storage`: SQLite persistence for sessions
- `internal/ui`: Bubble Tea terminal UI
- `pkg`: Reusable infrastructure packages

## Conventions

- Clean Architecture — dependencies point inward toward contracts
- Interfaces at boundaries, concrete types inside packages
- No global mutable state except immutable registry metadata
- Clear errors with useful context
