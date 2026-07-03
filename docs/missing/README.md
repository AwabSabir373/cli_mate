# Missing Features — Windsurf-Level Gap Analysis

> **Current score: ~82% of Windsurf-level.**  
> These are the remaining gaps keeping cli_mate from being a professional-grade AI coding editor CLI.

---

## 🚨 High Priority (Architectural)

| # | Feature | Effort | Why |
|---|---------|--------|-----|
| 1 | **Interactive tool approval** | High | When auto-approve is off, pause and ask "Allow tool X to run?" with yes/no/always prompts before executing each tool call. Needs an approval channel threaded through `tool_loop.go` → UI prompt → user response → resume. |
| 2 | **Project search indexing** | High | grep re-scans every time via `filepath.WalkDir`. On large projects (100K+ files), this is slow. Need a persistent TF-IDF or embedding-based search index for instant content search. |
| 3 | **Lint/error integration** | High | No parsing of compile/test errors. No inline problem display. No "fix this error" flow. Should parse `go build`/`go test` output and surface errors in the UI. |
| 4 | **Multi-file atomic edits** | High | Each tool call is independent. Can't "edit 3 files and run tests" as a single atomic operation with rollback on failure. |
| 5 | **Sandboxed shell execution** | High | Shell tool runs commands directly on the host. No container-level isolation. Risk of accidental damage. |

---

## ⚠️ Medium Priority (Deep Fixes)

| # | Feature | Effort | Why |
|---|---------|--------|-----|
| 6 | **Proper pre-edit undo** | Medium | Current `/undo` saves file content *after* the edit (post-edit), so first `/undo` is a no-op. Fix: `file_edit` tool must report old content in its result, or UI must read file *before* tool execution. |
| 7 | **Plugin/extension system** | High | All tools are hardcoded at compile time. No way for users to add custom tools without forking. MCP helps but requires running external servers. |
| 8 | **Vision/image support** | Medium | Provider implementations only support text. No image input for multimodal models (GPT-4o, Gemini, Claude). |
| 9 | **Multi-turn edit refinement** | Medium | No persistent edit context. Can't say "that edit was wrong, try a different approach" with the system remembering what was attempted. |

---

## 🔧 Low Priority (Quick Fixes)

| # | Feature | Effort | Fix |
|---|---------|--------|-----|
| 10 | **streamBuffer unbounded growth** | 15min | `app.go`: `a.streamBuffer += msg.token` accumulates every token ever streamed. Cap at 1000 chars. |
| 11 | **grepFileWithContext loads whole files** | 20min | `grep.go`: New code loads all lines into `[]string` instead of streaming via `bufio.Scanner`. Add a max-lines guard or revert to streaming. |
| 12 | **renderFinder double-spacing** | 5min | `view.go`: Non-selected items get `\n` from both `fmt.Sprintf` and the loop `WriteString`, creating blank lines. Remove the extra `WriteString("\n")`. |
| 13 | **grepFile dead code** | 2min | `grep.go`: Old line-by-line `grepFile` function is kept but never called. Remove it. |
| 14 | **Shell danger detection** | 30min | Current `rejectDangerousCommand` is a basic string-match (`"rm -rf"`). Easy to bypass (`rm -rf ./`). Use a proper allowlist of safe commands. |
| 15 | **Test coverage** | Ongoing | Only 4 tool tests, sparse UI tests. Many edge cases in new code (grep context_lines, file_list recursive, glob double-star) are untested. |

---

## 📊 Progress Tracker

```
Tools & File Operations    ████████████████░░░░  80%
Loading UX & Streaming     ████████████████████░  95%
Command History            ████████████████████░  95%
Undo System                █████████░░░░░░░░░░░  50%
Fuzzy File Finder          ██████████████████░░░  85%
Diff View                  ████████████████░░░░░  80%
Tool Approval              ██░░░░░░░░░░░░░░░░░░  10%
Search Indexing            ██░░░░░░░░░░░░░░░░░░  10%
Lint Integration           ██░░░░░░░░░░░░░░░░░░  10%
Plugin System              ██░░░░░░░░░░░░░░░░░░  10%

OVERALL                    ████████████████░░░░░  82%
```

---

## Quick Reference — Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Up` / `Down` | Cycle prompt history |
| `Alt+Up` / `Alt+Down` | Scroll console |
| `Ctrl+P` | Fuzzy file finder |
| `Ctrl+C` | Quit |
| `Esc` | Back / Close finder |
| `Tab` / `Enter` | Accept suggestion |

## Commands

| Command | Action |
|---------|--------|
| `/undo` | Undo last file edit |
| `/open <path>` | Preview a file |
| `/copy` | Copy last AI response |
| `/provider` | Choose provider |
| `/model` | Choose model |
| `/theme` | Choose theme |
| `/api-key` | Set API key |
| `/max-tokens` | Set context limit |
| `/base-url` | Set custom provider URL |
| `/connect` | Validate and connect |
| `/approve` | Toggle auto-approve |
| `/status` | Show configuration |
| `/clear` | Clear console |
| `/help` | Show this help |
