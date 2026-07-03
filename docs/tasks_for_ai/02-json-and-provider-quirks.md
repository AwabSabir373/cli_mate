# Task 2: Payload Parsing & Provider Strictness

**Goal:** Ensure the agent never crashes due to weak models outputting bad JSON, and ensure strict compliance with provider message APIs (like Anthropic).

## Context
Small models or overloaded models occasionally hallucinate bad JSON, specifically by outputting two concatenated JSON objects for a single tool call (e.g., `{"tool":"edit"}{"tool":"edit"}`). Anthropic also enforces a strict alternating message history: every `tool_use` must have exactly one corresponding `tool_result`.

## Requirements

### 1. Robust Concatenated JSON Decoding (`recoverableToolArguments`)
- **Problem:** `parseToolCall` uses basic JSON unmarshaling, which throws an "invalid character '{' after top-level value" error if a model concatenates JSON.
- **Solution:** 
  - Write a robust parser that uses `json.NewDecoder`. 
  - Decode the *first* valid JSON object. 
  - If the first object is valid but there is trailing data, verify if the trailing data consists of whole JSON objects. If it does, ignore them and return the first object. If it is garbage text, return a parse error.
  - If a tool call is genuinely malformed and dropped, inject a user-role message into the context: *"Your previous tool call was malformed... Re-issue the tool call"* rather than silently skipping it.

### 2. Strict Provider Replay (Aborted Tool Calls)
- **Problem:** If a turn contains multiple tool calls, but the execution guard halts the loop on the first one (e.g., due to repeated failures), the remaining tool calls are never executed. When the history is sent to the provider on the next turn, Anthropic will reject the request because there are `tool_use` blocks without `tool_result` blocks.
- **Solution:**
  - Before returning an error or halting the loop mid-turn, append a dummy `tool_result` message for every unexecuted tool call.
  - The content of this dummy result should be: `"aborted: run halted by the repeated-failure guard"`.
