# Production Readiness Task Force: `cli_mate` vs `zero`

This folder contains a set of strictly defined, atomic tasks designed to elevate `cli_mate` to a production-ready state, surpassing the capabilities of the reference `zero` tool.

**Instructions for AI Agents:**
If you have been directed to this folder by a user, your goal is to select one of the task files below, read its requirements carefully, and implement the necessary changes in the `cli_mate` codebase (primarily in `internal/agent/tool_loop.go`).

## Task List

- `[x]` **Task 1: Core Loop Resilience** (`01-core-loop-resilience.md`)
  - Implement stream retry logic for stalled connections.
  - Implement graceful handling of multi-modal/image rejection errors.
- `[x]` **Task 2: Payload Parsing & Provider Strictness** (`02-json-and-provider-quirks.md`)
  - Implement `recoverableToolArguments` for concatenated JSON.
  - Fix strict provider replay by appending aborted tool results.
- `[x]` **Task 3: Agent Intelligence & Autonomy** (`03-agent-intelligence.md`)
  - Build headless completion gates and continuation nudges.
  - Add support for mid-run model escalation.
- `[x]` **Task 4: Performance & Token Optimization** (`04-performance-optimization.md`)
  - Implement batched post-edit self-correction.
  - Implement deferred tool loading via a `tool_search` mechanism.

## Definition of Done for each task:
1. The code changes are isolated and well-tested.
2. `make build` or `go build` passes without errors.
3. Code strictly follows the `cli_mate` Clean Architecture rules (as defined in `AGENTS.md`).
