# cli_mate Agent Guide

cli_mate is a terminal-first AI coding agent. The model should behave like a senior Go engineer working inside a real repository: inspect before editing, keep changes scoped, run verification, and explain only the important outcome.

## Priorities

1. Preserve user work. Never overwrite unrelated changes.
2. Prefer small, correct edits over broad rewrites.
3. Use project conventions before inventing new structure.
4. Verify with `gofmt`, `go test ./...`, `go vet ./...`, and `go build ./cmd/cli_mate` when code changes.
5. Ask only when the next step is genuinely ambiguous or risky.

## Architecture

This project follows Clean Architecture.

- `cmd/cli_mate`: CLI entrypoint only.
- `internal/agent`: orchestration, session state, context window, streaming.
- `internal/providers`: provider contracts and adapters.
- `internal/tools`: executable tools exposed to the agent.
- `internal/config`: configuration and profiles.
- `internal/storage`: persistence.
- `internal/ui`: Bubble Tea terminal UI.
- `pkg`: reusable infrastructure packages.

Dependencies should point inward toward contracts. Provider adapters should not import UI, storage, or agent packages.

## Coding Style

- Keep Go code simple and explicit.
- Prefer interfaces at boundaries, concrete types inside packages.
- Avoid global mutable state unless it is immutable registry metadata.
- Return clear errors with useful context.
- Add tests for interaction flows and regressions.

## Terminal UX

The terminal should feel guided, not command-heavy.

- `/` opens command suggestions.
- `@` opens file mentions.
- `Enter` commits highlighted suggestions.
- `Esc` moves back one setup step.
- `Ctrl+C` quits.

Provider setup should flow like this:

1. Choose provider.
2. Enter API key if required.
3. Choose model.
4. Connect.
5. Start chatting with file mentions and tools.

