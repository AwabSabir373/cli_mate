# cli_mate

`cli_mate` is a terminal-first AI coding agent written in Go. It provides a guided Bubble Tea interface for understanding repositories, editing files, running commands, reviewing changes, and connecting to MCP tools without turning normal use into a configuration-heavy workflow.

## Highlights

- Guided provider setup: choose a provider, enter credentials when required, choose a model, and start working.
- Coding-focused terminal UI with a teal visual system, streaming responses, tool activity, file changes, inline diffs, and interrupt support.
- File mentions with `@` and discoverable slash commands with `/`.
- Targeted file reads, writes, edits, search, shell execution, and verification tools.
- Built-in MCP server plus support for external MCP servers.
- Semantic repository tools for Go and multiple other languages.
- Persistent sessions, profiles, themes, permissions, context compaction, and diagnostics.
- One-shot operation for scripts and non-interactive workflows.

## Install

### GitHub release

Linux amd64:

```bash
curl -sSL https://github.com/AwabSabir373/cli_mate/releases/latest/download/cli_mate_linux_amd64.tar.gz | tar xz
sudo install cli_mate /usr/local/bin/cli_mate
```

macOS Apple Silicon:

```bash
curl -sSL https://github.com/AwabSabir373/cli_mate/releases/latest/download/cli_mate_darwin_arm64.tar.gz | tar xz
sudo install cli_mate /usr/local/bin/cli_mate
```

Windows amd64: download `cli_mate_windows_amd64.zip` from [GitHub Releases](https://github.com/AwabSabir373/cli_mate/releases), extract `cli_mate.exe`, and add its directory to `PATH`.

### Build from source

Go 1.26.5 or newer is required.

```bash
git clone https://github.com/AwabSabir373/cli_mate.git
cd cli_mate
go build -o cli_mate ./cmd/cli_mate
```

## Quick start

```bash
# Open the interactive coding interface in the current directory
cli_mate

# Work in a specific repository
cli_mate --workspace /path/to/project

# Run a single prompt without opening the TUI
cli_mate run "explain cmd/cli_mate/main.go"

# Supply piped context
cat main.go | cli_mate run "review this code for correctness"

# List saved sessions
cli_mate sessions
```

On first launch:

1. Choose a provider.
2. Enter an API key if the provider requires one.
3. Choose a model.
4. Connect and start chatting.

Use `@filename` to mention repository files. Type `/` to browse commands, press `Enter` to accept the highlighted suggestion, press `Esc` to step back or interrupt, and press `Ctrl+C` to quit.

## Providers

`cli_mate` includes adapters for OpenAI, Anthropic, OpenRouter, Gemini, Groq, Mistral, DeepSeek, xAI, Ollama, LM Studio, and OpenAI-compatible endpoints. Available model names are loaded through the selected provider rather than being hardcoded in this document.

Provider credentials and model selection are handled by the guided setup flow:

```text
/setup
/provider
/model
```

## Main commands

| Command | Purpose |
| --- | --- |
| `/setup` | Open guided provider setup |
| `/provider` | Choose the active provider |
| `/model` | Choose the active model |
| `/open` | Open or mention a repository file |
| `/mcp` | Manage MCP servers and connections |
| `/resume` | Resume a saved session |
| `/permissions` | Review tool approval behavior |
| `/theme` | Select a terminal theme |
| `/doctor` | Diagnose configuration and connectivity |
| `/compact` | Compact the active conversation context |
| `/status` | Show current runtime status |
| `/help` | Show all available commands and keys |
| `/exit` | Exit `cli_mate` |

## MCP

The built-in MCP server communicates over standard input/output and can be started with:

```bash
cli_mate mcp-server
```

Example client configuration:

```json
{
  "mcpServers": {
    "cli_mate": {
      "command": "cli_mate",
      "args": ["mcp-server"]
    }
  }
}
```

The server supports repository navigation, text and semantic search, symbol inspection, targeted edits, and other coding-agent operations. Responses are designed to return focused context instead of unnecessarily large file dumps.

External MCP servers can be added and managed from `/mcp` inside the TUI.

## Development and verification

The repository follows Clean Architecture. CLI entrypoints live under `cmd`, while agent orchestration, providers, tools, storage, configuration, and UI implementations live in their respective `internal` packages.

Run the complete local verification gate before submitting changes:

```bash
gofmt -w .
go test -count=1 ./...
go vet ./...
go build ./cmd/cli_mate ./cmd/cli_mcp
go mod verify
```

Release validation additionally runs race-enabled tests, GolangCI-Lint, Go vulnerability scanning, and GoReleaser configuration checks in CI.

## Release process

Tags matching `v*` trigger the release workflow. A successful release publishes Linux and macOS archives, a Windows zip, checksums, and multi-architecture container images to GitHub Container Registry.

```bash
git add -A
git commit -m "release: prepare cli_mate v0.1.0"
git push origin main
git tag v0.1.0
git push origin v0.1.0
```

Before tagging, confirm the working tree contains only the intended release changes and that CI is green.

## License

MIT
