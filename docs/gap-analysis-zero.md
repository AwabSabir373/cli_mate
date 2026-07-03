# cli_mate vs Zero — Gap Analysis & Implementation Plan

**Date:** 2026-07-03  
**Reference Project:** `C:\Users\awabs\OneDrive\Documents\cli\zero`  
**Target Project:** `D:\back_end\GolandProjects\cli_mate`

---

## Executive Summary

Zero is a mature AI coding agent with 25+ providers, cross-platform sandboxing, multi-agent swarm, and 12+ themes. cli_mate covers the core well but is missing significant features in providers, UI polish, safety architecture, and tooling. This document lists every identified gap organized into implementable tasks.

---

## Priority 1: High-Impact Core Features

### T1: Context Compaction (Proactive + Reactive) ✅ DONE

**Gap:** Zero proactively summarizes old messages before hitting ~80% context window usage (`agent/compaction.go`). cli_mate only trims messages by token count in `context_window.go`, losing information.

**Implementation:**
- [x] Add `internal/agent/compaction.go` with proactive compaction logic
- [x] Trigger compaction when context usage exceeds 80% of `MaxTokens - ReserveTokens`
- [x] Use LLM to summarize older messages, preserving key decisions and code references
- [x] Add reactive compaction: catch context-limit errors, summarize, retry
- [x] Preserve trailing N messages verbatim (don't summarize recent context)
- [x] Add `/compact` slash command that triggers manual compaction (replace current temp-file-clean behavior)

**Zero reference:** `internal/agent/compaction.go` (proactive + reactive paths)  
**cli_mate reference:** `internal/agent/context_window.go:19-48` (current trim-only logic)

---

### T2: Self-Correcting Edits ✅ DONE

**Gap:** Zero runs LSP diagnostics and project tests after every mutating tool call, feeding failures back to the model for automatic fix attempts (`agent/selfcorrect.go`). cli_mate has no such loop.

**Implementation:**
- [x] Add `internal/agent/selfcorrect.go`
- [x] After file write/edit, run `go vet ./...` and `go build ./cmd/cli_mate`
- [x] If errors found, append error output to context and re-run the tool loop
- [x] Bound by max attempt ceiling (default 3)
- [x] Gate behind autonomy level (don't auto-fix if user is in ask mode)
- [x] Add `/selfcorrect` toggle command
- [x] Generalize: detect project type (Go, Python, TS) and run appropriate linter/test

**Zero reference:** `internal/agent/selfcorrect.go`  
**cli_mate reference:** `internal/ui/chat.go` (tool loop execution point)

---

### T3: Lifecycle Hooks ✅ DONE

**Gap:** Zero dispatches 6 lifecycle events around tool calls with optional tool-name matchers. Non-zero exit codes gate execution. cli_mate has no hook system.

**Implementation:**
- [x] Add `internal/agent/hooks.go` with hook dispatcher
- [x] Define events: `beforeTool`, `afterTool`, `beforePrompt`, `afterPrompt`, `onError`, `onSessionStart`
- [x] Each hook has: event name, optional tool-name filter, shell command to run
- [x] Non-zero exit on `beforeTool` blocks the tool call
- [x] Non-zero exit on `afterTool` surfaces feedback to model
- [ ] Add hooks config section to `config.go` (pending config integration)
- [ ] Register hooks from config at session start (pending config integration)

**Zero reference:** `AGENTS.MD` lines 174-185 (hook spec)

---

### T4: Apply Patch Tool ✅ DONE

**Gap:** Zero uses unified diffs (`apply_patch.go`) instead of full file rewrites. More token-efficient and safer.

**Implementation:**
- [x] Add `internal/tools/apply_patch.go`
- [x] Accept unified diff format as input
- [x] Parse and validate the diff against actual file contents
- [x] Apply hunks, report conflicts
- [x] Register as agent tool alongside existing `file_edit`

**Zero reference:** `internal/tools/apply_patch.go`  
**cli_mate reference:** `internal/tools/file_edit.go` (current search-replace approach)

---

### T5: Tool Search (Deferred Loading) ✅ DONE

**Gap:** Zero delays loading full tool schemas when too many tools are registered (`tool_search.go` + `deferred.go`). cli_mate always sends all schemas.

**Implementation:**
- [x] Add `internal/tools/tool_search.go` — compact search over tool descriptions
- [x] Add `internal/tools/registry.go` — tracks deferred vs eager tools with threshold
- [x] When tool count exceeds threshold (default 20), withhold full JSON schemas
- [x] Model uses `tool_search` to discover tools, then requests specific schemas
- [ ] Add `tools.deferThreshold` to config (pending config integration)

**Zero reference:** `internal/tools/tool_search.go`, `internal/tools/deferred.go`  
**cli_mate reference:** `internal/tools/tool.go` (current all-at-once registration)

---

## Priority 2: Providers & Models

### T6: Add Missing Providers (First Batch) ✅ DONE

**Gap:** cli_mate supports 7 providers. Zero supports 25+. Most missing ones are OpenAI-compatible, so they can share `openai_compat/shared.go`.

**Implementation (first batch):**
- [x] **DeepSeek** — `internal/providers/deepseek/client.go` (OpenAI-compatible)
- [x] **Mistral** — `internal/providers/mistral/client.go` (OpenAI-compatible)
- [x] **LM Studio** — `internal/providers/lmstudio/client.go` (OpenAI-compatible, local)
- [x] **xAI/Grok** — `internal/providers/xai/client.go` (OpenAI-compatible)
- [ ] **GitHub Models** — `internal/providers/github/client.go` (OpenAI-compatible)
- [ ] **Together AI** — `internal/providers/together/client.go` (OpenAI-compatible)
- [ ] **Hugging Face** — `internal/providers/huggingface/client.go` (OpenAI-compatible)
- [ ] **NVIDIA NIM** — `internal/providers/nvidia/client.go` (OpenAI-compatible)
- [ ] **MiniMax** — `internal/providers/minimax/client.go` (OpenAI-compatible)
- [ ] **Moonshot/Kimi** — `internal/providers/moonshot/client.go` (OpenAI-compatible)
- [ ] **DashScope** — `internal/providers/dashscope/client.go` (OpenAI-compatible)
- [ ] Register each in `internal/providers/registry/registry.go`

**Zero reference:** `internal/providercatalog/catalog.go:89-150`  
**cli_mate reference:** `internal/providers/registry/registry.go:26-69`

---

### T7: Model Registry with Pricing Metadata

**Gap:** Zero has 13 curated models with cost-per-million-token breakdowns, reasoning levels, and deprecation rules. cli_mate has a flat model list.

**Implementation:**
- [ ] Add `internal/providers/registry/models.go` with model metadata struct:
  ```go
  type ModelSpec struct {
      ID                string
      Provider          string
      ContextWindow     int
      Pricing           TokenPricing  // input/output per million tokens
      ReasoningSupport  bool
      MaxOutputTokens   int
      DeprecationDate   *time.Time
      UpgradeTarget     string  // suggest better model
  }
  ```
- [ ] Define specs for top 13 models (GPT-4.1 family, Claude Opus/Sonnet/Haiku, Gemini 2.5 family)
- [ ] Add `/cost` command to estimate prompt cost before sending
- [ ] Show cost in response footer

---

### T8: Auto-Detect Thinking Models

**Gap:** Zero auto-detects think-tag-emitting models (DeepSeek R1, Qwen3, QwQ, Kimi K2 Thinking, Magistral) and toggles reasoning block parsing.

**Implementation:**
- [ ] Add thinking model list to `internal/providers/registry/models.go`
- [ ] When model matches, enable `<thinking>` tag parsing in stream handler
- [ ] Render thinking blocks separately (collapsible or muted style)
- [ ] Add `ModelThinking` flag to provider config

**Zero reference:** `internal/providers/factory.go:103-135`

---

## Priority 3: UI/UX Enhancements

### T9: Theme Expansion ✅ DONE

**Gap:** Zero has 12+ themes with auto-detection. cli_mate has 4.

**Implementation:**
- [x] Added 8 new themes to `internal/ui/styles.go`:
  - [x] Catppuccin Mocha
  - [x] Dracula
  - [x] Nord
  - [x] Gruvbox Dark
  - [x] Tokyo Night
  - [x] Rose Pine
  - [x] Solarized Dark
  - [x] One Dark
- [ ] Add auto-detection: check `TERM_PROGRAM`, `COLORTERM`, background luminance
- [ ] Add `--theme` CLI flag
- [ ] Persist theme choice in config
- [ ] Add `ZERO_THEME` env var support

**Zero reference:** `internal/tui/theme_palettes.go`

---

### T10: Streaming Fade Animation

**Gap:** Zero fades in streaming text. cli_mate shows tokens instantly.

**Implementation:**
- [ ] Add fade animation to `internal/ui/view.go` rendering
- [ ] Animate opacity from 50% to 100% over ~200ms per token chunk
- [ ] Add `ZERO_NO_FADE=1` env var to disable
- [ ] Add `preferences.fadeAnimation` config toggle

---

### T11: Permission Card UX Overhaul

**Gap:** Zero has rich permission cards with amber borders, badges, and 5 decision types. cli_mate uses y/n/a keys.

**Implementation:**
- [ ] Redesign permission prompt in `internal/ui/view.go`
- [ ] Add card-style rendering with border and badge
- [ ] Expand decisions: Allow, Allow for Session, Always Allow, Allow Prefix, Deny, Cancel
- [ ] Add Shift+Tab cycling between auto and ask modes
- [ ] Show tool name, file path, and preview in the card

**Zero reference:** `internal/agent/types.go:51-60`, `internal/tui/model.go`

---

### T12: Double-Press Safety Guards

**Gap:** Zero guards Ctrl+C and Esc with a 3-second double-press window. cli_mate quits immediately.

**Implementation:**
- [ ] Add timestamp tracking in `internal/ui/app.go` for Ctrl+C and Esc
- [ ] First press: show "Press again to quit/cancel" warning
- [ ] Second press within 3s: execute action
- [ ] Reset timer after 3s

---

### T13: Context Utilization Gauge

**Gap:** Zero shows a visual context usage gauge. cli_mate shows a raw token count.

**Implementation:**
- [ ] Replace token count in header with a bar gauge in `internal/ui/view.go`
- [ ] Color code: green (<60%), yellow (60-80%), red (>80%)
- [ ] Show percentage and bar: `[████████░░] 78%`
- [ ] Trigger proactive compaction warning at 80%

---

### T14: Doctor Diagnostics View

**Gap:** Zero has a diagnostic view for checking system health.

**Implementation:**
- [ ] Add `/doctor` command
- [ ] Check: Go version, provider connectivity, API key validity, MCP server status, config file validity
- [ ] Render results as a checklist with pass/fail indicators
- [ ] Add to `internal/ui/commands.go`

---

## Priority 4: Safety & Architecture

### T15: OS-Level Sandboxing

**Land:** Zero has Landlock+seccomp (Linux), Seatbelt (macOS), restricted-token ACLs (Windows). cli_mate has basic path sandboxing.

**Implementation:**
- [ ] Add `internal/sandbox/` package
- [ ] Windows: Implement restricted-token ACLs (self-dispatching subcommand pattern)
- [ ] Linux: Landlock LSM for file/network restrictions
- [ ] macOS: Seatbelt profiles
- [ ] Runtime detection via `sandbox.SelectBackend()`
- [ ] Integrate into shell and file tools

**Note:** This is a large effort. Consider phased rollout starting with Windows ACLs.

---

### T16: Secret Scrubbing in Tool Output

**Gap:** Zero scrubs API keys and secrets from all tool output. cli_mate doesn't.

**Implementation:**
- [ ] Add `internal/tools/scrub.go`
- [ ] Define scrub patterns: API key formats, env var values, encrypted strings
- [ ] Apply scrubbing after every tool execution before returning to model
- [ ] Register in tool execution pipeline

**Zero reference:** `internal/tools/registry.go:206-232`

---

### T17: File Mutation Tracking

**Gap:** Zero tracks which files were mutated and validates them post-edit. cli_mate doesn't.

**Implementation:**
- [ ] Add `internal/tools/file_tracker.go`
- [ ] Record file path + pre-edit hash after each write/edit
- [ ] Add `mutation_targets` tool that returns list of modified files
- [ ] Useful for commit workflows and self-correction

**Zero reference:** `internal/tools/file_tracker.go`, `internal/tools/mutation_targets.go`

---

### T18: Escalate Model on Failure

**Gap:** Zero has `escalate_model.go` that auto-escalates to a stronger model when the current one fails repeatedly.

**Implementation:**
- [ ] Add `internal/tools/escalate_model.go`
- [ ] Define escalation chain per provider (e.g., gpt-4.1-mini → gpt-4.1 → gpt-4.1)
- [ ] Trigger after N consecutive tool failures
- [ ] Notify user before escalating

**Zero reference:** `internal/tools/escalate_model.go`

---

### T19: LSP Navigation Tool

**Gap:** Zero has `lsp_navigate.go` for go-to-definition and find-references. cli_mate reads entire files instead.

**Implementation:**
- [ ] Add `internal/tools/lsp_navigate.go`
- [ ] Use `golang.org/x/tools/gopls` or spawn `gopls` as subprocess
- [ ] Support: go-to-definition, find-references, document-symbols, workspace-symbols
- [ ] Register as agent tool

**Zero reference:** `internal/tools/lsp_navigate.go`

---

## Priority 5: Commands & Config

### T20: Missing Slash Commands

**Commands to add:**
- [ ] `/rewind` — step back to previous conversation checkpoint
- [ ] `/selfcorrect` — toggle self-correction (T2)
- [ ] `!` shell escape — run raw shell command without tool layer
- [ ] `/doctor` — system diagnostics (T14)
- [ ] `/cost` — show session token usage and estimated cost (T7)
- [ ] `/hooks` — list/configure lifecycle hooks (T3)

**Zero reference:** `internal/tui/commands.go:48-55`

---

### T21: Enhanced Config System

**Gap:** Zero has 4-layer config with notification mode, local control, and project-level settings.

**Implementation:**
- [ ] Add project-level config: `./cli_mate.yaml` already exists, but add validation and merge priority
- [ ] Add `notifications` config section (mode: none/desktop/terminal)
- [ ] Add `localControl` config section for browser/desktop helpers
- [ ] Add `additionalWriteRoots` to sandbox config (with security guard rejecting project-config overrides)
- [ ] Add `swarm.maxTeamSize` when swarm is implemented

---

## Priority 6: Advanced Features (Large Effort)

### T22: Multi-Agent Swarm

**Gap:** Zero has a full swarm package with specialist sub-agents, mailbox communication, and scheduling.

**Implementation:**
- [ ] Add `internal/swarm/` package
- [ ] Define `Team`, `Member`, `Coordinator`, `Scheduler` types
- [ ] Implement mailbox-based inter-member communication
- [ ] Add concurrency capping (default 8)
- [ ] Add `swarm` tool for agent to spawn specialists
- [ ] Add `/swarm` command for manual orchestration

**Note:** Major feature. Plan separately after core gaps are closed.

---

### T23: ACP (Agent Client Protocol)

**Gap:** Zero has a JSON-RPC server for editor integrations.

**Implementation:**
- [ ] Add `internal/acp/` package
- [ ] Implement JSON-RPC server over stdio
- [ ] Wrap agent runtime with workspace-scoped tool registries
- [ ] Handle editor authentication externally

**Note:** Only implement if editor integration is a priority.

---

### T24: Local Control (Browser/Desktop/Terminal)

**Gap:** Zero can control browser, capture screen, and drive external terminals.

**Implementation:**
- [ ] Add `internal/tools/local_browser.go` — Playwright/Puppeteer integration
- [ ] Add `internal/tools/local_capture.go` — screenshot via Playwright
- [ ] Add `internal/tools/local_terminal.go` — external terminal control
- [ ] Add `internal/tools/local_desktop.go` — desktop automation

**Note:** Requires external dependencies. Consider as plugin system.

---

## Implementation Order

| Phase | Tasks | Effort | Impact |
|-------|-------|--------|--------|
| **Phase 1** (Core) | T1, T2, T3 | ~3 days | High — fixes session longevity and code quality |
| **Phase 2** (Tools) | T4, T5, T16, T17 | ~2 days | Medium — better tooling and safety |
| **Phase 3** (Providers) | T6, T7, T8 | ~2 days | High — broader model support |
| **Phase 4** (UI) | T9, T10, T11, T12, T13 | ~3 days | Medium — polished experience |
| **Phase 5** (Safety) | T15, T18, T19 | ~3 days | Medium — production hardening |
| **Phase 6** (Commands) | T14, T20, T21 | ~1 day | Low — nice-to-haves |
| **Phase 7** (Advanced) | T22, T23, T24 | ~5+ days | High — major features |

**Total estimated effort:** ~20 days

---

*This file is the source of truth for gap implementation. Check off items as they're completed.*
