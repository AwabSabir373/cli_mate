# Task 1: Core Loop Resilience

**Goal:** Make the `cli_mate` execution loop indestructible against network blips, stalled AI provider streams, and model-specific capability errors.

## Context
Currently, in `internal/agent/tool_loop.go`, if the API stream disconnects or stalls, the tool surfaces the error and ends the loop. Furthermore, passing an image to a text-only model results in an unhandled HTTP 400 error loop. 

## Requirements

### 1. Stream Stalled Recovery
- **Problem:** Sometimes a model (like an Ollama local model or a slow API) will stream some reasoning and then freeze mid-tool-call without outputting any visible text.
- **Solution:** Wrap the `streamWithReconnect` or stream collection logic with a retry mechanism (`maxStreamStallRetries = 2`).
  - If the stream drops, check if *any visible text* was forwarded to the user.
  - If NO visible text was forwarded (e.g. it only generated a partial tool call), silently discard the partial tool call, backoff, and retry the request on a fresh connection.
  - Do NOT retry if visible text was already streamed (to avoid duplicating text for the user).

### 2. Image Rejection Handling
- **Problem:** Some providers return HTTP 400 when sent an image if the model is text-only.
- **Solution:** Intercept errors returning from the stream. Detect if the error matches image rejection keywords (`400`, `image`, `multimodal`, `vision`, `unsupported content type`).
  - If matched, gracefully abort the run and return a clean, user-friendly error instructing them to switch to a vision-capable model (like `claude-3-5-sonnet` or `gpt-4o`).
  - Do NOT attempt to compact and retry this error, as it will endlessly loop.

## Target Files
- `internal/agent/tool_loop.go` (or related stream handling files).
