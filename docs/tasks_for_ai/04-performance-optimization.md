# Task 4: Performance & Token Optimization

**Goal:** Reduce token consumption and increase speed by batching operations and lazily loading tools.

## Context
`cli_mate` evaluates self-correction after every single file edit and loads all tools into the system prompt simultaneously. This is expensive and slow.

## Requirements

### 1. Batched Post-Edit Self-Correction
- **Problem:** Currently, if an AI outputs three `file_edit` tool calls in one response, `opts.SelfCorrector.VerifyAfterMutation` is called three times sequentially.
- **Solution:** 
  - Inside the turn's tool execution loop, collect the names of all files modified by successful mutating tools into a slice (`changedFilesThisBatch`).
  - Run the `SelfCorrector.VerifyAfterMutation` logic **only once** at the end of the turn, passing the unique union of all changed files.
  - Append the resulting verification feedback to the message history so the model can fix any issues in the next turn.

### 2. Deferred Tool Loading
- **Problem:** Pumping 20+ tool definitions into the system prompt wastes tokens and confuses smaller models.
- **Solution:** 
  - Create a "Tool Discovery" or `tool_search` capability.
  - By default, only expose core tools (`shell`, `file_read`, `file_edit`, `tool_search`) in the prompt.
  - If the agent needs to do something specialized (e.g., database inspection, specialized linting), it calls `tool_search`.
  - The agent loop tracks loaded tools (`loaded[name] = true`) and dynamically includes their full schemas in subsequent API requests.
