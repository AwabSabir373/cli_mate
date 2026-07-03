# cli_mate

Terminal-first AI coding agent. Chat with your codebase, edit files, run commands — all from one terminal.

## Install

```bash
# From source
go install ./cmd/cli_mate

# From release (download from GitHub Releases)
# Linux/macOS
curl -sSL https://github.com/cli_mate/cli_mate/releases/latest/download/cli_mate_linux_amd64.tar.gz | tar xz
sudo mv cli_mate /usr/local/bin/

# Windows (PowerShell)
# Download cli_mate_windows_amd64.zip from Releases, extract, add to PATH
```

## Quick Start

```bash
# Launch interactive TUI
cli_mate

# One-shot prompt
cli_mate run "explain the main function in cmd/cli_mate/main.go"

# Pipe input
cat main.go | cli_mate run "find bugs in this code"

# List past sessions
cli_mate sessions
```

## Provider Setup

Launch `cli_mate` and use slash commands:

1. `/provider` — choose a provider (openai, anthropic, openrouter, gemini, groq, ollama)
2. Paste your API key (not needed for local providers like Ollama)
3. `/model` — select a model
4. Start chatting

### Supported Providers

| Provider | API Key | Models |
|----------|---------|--------|
| OpenAI | Required | gpt-4.1, gpt-4.1-mini, gpt-4o, o3-mini |
| Anthropic | Required | claude-sonnet-4, claude-3.5-sonnet, claude-3.5-haiku |
| OpenRouter | Required | 100+ models via single key |
| Gemini | Required | gemini-2.5-flash, gemini-2.5-pro |
| Groq | Required | llama-3.3-70b, mixtral-8x7b |
| Ollama | Not needed | Any local model |

### Environment Variables

```bash
export CLI_MATE_PROFILES_DEFAULT_PROVIDER=openai
export CLI_MATE_PROFILES_DEFAULT_MODEL=gpt-4.1
export CLI_MATE_PROFILES_DEFAULT_APIKEY=sk-...
```

## Usage

### Interactive Mode

```bash
cli_mate                    # Launch TUI
cli_mate -p work            # Use "work" profile
cli_mate --config ./cli_mate.yaml  # Custom config file
```

### Commands

| Command | Description |
|---------|-------------|
| `/provider` | Choose provider |
| `/model` | Choose model |
| `/theme` | Choose theme (midnight, matrix, paper, mono) |
| `/max-tokens` | Set context limit |
| `/base-url` | Set provider URL (for Ollama) |
| `/connect` | Validate and connect |
| `/status` | Show configuration |
| `/clear` | Clear console |
| `/help` | Show commands |

### File Mentions

Type `@` followed by a filename to include file contents in your prompt:

```
@main.go explain this function
```

### Tools

The agent can:
- **Read files** — inspect your codebase
- **Write files** — create new files
- **Edit files** — make targeted changes
- **Run shell** — execute commands (tests, builds, formatting)
- **Glob** — search for files by pattern
- **Grep** — search file contents by regex

## Configuration

Config file: `~/.config/cli_mate/cli_mate.yaml`

```yaml
activeProfile: default
profiles:
  default:
    provider: openai
    model: gpt-4.1
    apiKey: sk-...
    maxTokens: 128000
    reserveTokens: 4096
    temperature: 0.2
  local:
    provider: ollama
    model: llama3.1
    baseURL: http://localhost:11434
log:
  level: info
storage:
  path: cli_mate.db
http:
  timeout: 30s
  retries: 3
```

## Build

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/cli_mate
```

## License

MIT
