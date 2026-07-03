# Provider & Model Guidance

## Supported Providers

| Provider | API Key | Notes |
|----------|---------|-------|
| OpenAI | Required | gpt-4.1, gpt-4.1-mini, gpt-4o, o3-mini |
| Anthropic | Required | claude-sonnet-4, claude-3.5-sonnet, claude-3.5-haiku |
| OpenRouter | Required | 100+ models via single key |
| Gemini | Required | gemini-2.5-flash, gemini-2.5-pro |
| Groq | Required | llama-3.3-70b, mixtral-8x7b |
| Ollama | Not needed | llama3.1, qwen2.5-coder, deepseek-coder |

## Configuration

API keys are encrypted at rest using machine-bound AES-256-GCM keys derived via PBKDF2.

## Usage

Configure via the TUI (`/provider`, `/model`, `/api-key`) or environment variables with the `CLI_MATE_` prefix.
