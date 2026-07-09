package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"cli_mate/internal/agent/agentloop"
	agentmemory "cli_mate/internal/agent/memory"
	"cli_mate/internal/notify"
	"cli_mate/internal/providers"
	"cli_mate/internal/redaction"
	"cli_mate/internal/specialist"
	"cli_mate/internal/tools"
	"cli_mate/pkg/tokenizer"
)

const defaultMaxToolIterations = 32

var toolBlockPattern = regexp.MustCompile("(?s)```(?:cli_mate-tool|goai-tool|tool|json)\\s*(.*?)\\s*```")
var openToolBlockPattern = regexp.MustCompile("(?s)```(?:cli_mate-tool|goai-tool|tool|json)\\s*(.*)$")

type CodingRunner struct {
	Provider      providers.Provider
	Tools         map[string]tools.Tool
	Instructions  string
	WorkspaceRoot string
	Style         ResponseStyle
	MaxIterations int
	Memory        agentmemory.Store
}

type RunOptions struct {
	Model                  string
	History                []providers.Message
	Prompt                 string
	MaxTokens              int
	ReserveTokens          int
	Temperature            float64
	Counter                tokenizer.Counter
	OnStep                 func(Step)
	OnContext              func(int)
	OnToken                func(string)
	ApproveTool            func(tools.Call) bool
	DisableTools           bool
	CompactionPreserveLast int
	OnUsage                func(providers.Usage)
	OnActivity             func()
	SelfCorrector          *SelfCorrector
	Hooks                  *HookDispatcher
	Style                  ResponseStyle
	CoreTools              []string
	ModelSwitcher          func(context.Context, string) (providers.Provider, error)
}

type RunResult struct {
	Messages []providers.Message
	Answer   string
	Steps    []Step
	Events   []agentloop.Event
}

type Step struct {
	Kind string
	Text string
}

func NewCodingRunner(provider providers.Provider, instruction string, toolset []tools.Tool, workspaceRoot string) *CodingRunner {
	indexed := make(map[string]tools.Tool, len(toolset))
	for _, tool := range toolset {
		indexed[tool.Name()] = tool
	}
	return &CodingRunner{
		Provider:      provider,
		Tools:         indexed,
		Instructions:  instruction,
		WorkspaceRoot: workspaceRoot,
		MaxIterations: defaultMaxToolIterations,
	}
}

// AddTools merges additional tools into the runner's tool map.
// Used to add MCP-discovered tools at startup.
func (r *CodingRunner) AddTools(extra []tools.Tool) {
	for _, tool := range extra {
		r.Tools[tool.Name()] = tool
	}
}

// EnableSpecialists adds the specialist task tool to the runner if a provider
// is available. The task tool allows the main agent to delegate work to
// specialist sub-agents (worker, explorer, code-review).
func (r *CodingRunner) EnableSpecialists() {
	if r.Provider == nil {
		return
	}
	registry := specialist.NewRegistry()
	toolList := make([]tools.Tool, 0, len(r.Tools))
	for _, tool := range r.Tools {
		toolList = append(toolList, tool)
	}
	taskTool := specialist.NewTaskTool(registry, r.Provider, toolList)
	r.Tools[taskTool.Name()] = taskTool
}

func (r *CodingRunner) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	if r == nil || r.Provider == nil {
		return RunResult{}, fmt.Errorf("agent runner has no provider")
	}

	maxIterations := r.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxToolIterations
	}

	messages := append([]providers.Message{}, opts.History...)
	messages = append(messages, providers.Message{Role: "user", Content: opts.Prompt})
	steps := []Step{}
	auto := r.newAutonomousRuntime(opts, maxIterations)
	auto.emit(agentloop.Event{Type: agentloop.EventUserRequest, Summary: opts.Prompt})

	// Initialize compaction state for proactive context management.
	compaction := newCompactionState(opts.MaxTokens, opts.CompactionPreserveLast, opts.OnUsage)

	// Initialize guardrails state for runaway-loop protection.
	guard := newGuardState()

	maxStreamStallRetries := 2
	loadedTools := map[string]bool{}

	for iteration := 0; iteration < maxIterations; iteration++ {
		turnRequestedModel := ""

		var toolDefs []providers.ToolDefinition
		if !opts.DisableTools {
			for _, tool := range r.Tools {
				name := tool.Name()
				isCore := false
				for _, core := range opts.CoreTools {
					if name == core {
						isCore = true
						break
					}
				}
				if isCore || loadedTools[name] || len(opts.CoreTools) == 0 {
					d := tool.Definition()
					toolDefs = append(toolDefs, providers.ToolDefinition{
						Name:        d.Name,
						Description: d.Description,
						Schema:      d.Schema,
					})
				}
			}
		}

		// Proactive compaction: summarize old messages before they blow the context.
		messages = compaction.maybeCompact(ctx, r.Provider, messages, toolDefs)

		loopMsg := auto.beforeModelTurn(ctx, messages, iteration, opts.DisableTools)
		if result, handled := auto.runActiveVerification(ctx); handled {
			stepText := "Autonomous verification: " + result.Summary
			step := Step{Kind: "system", Text: stepText}
			steps = append(steps, step)
			if opts.OnStep != nil {
				opts.OnStep(step)
			}
			if diagnostics := verificationDiagnostics(result); diagnostics != "" {
				messages = append(messages, providers.Message{
					Role:    "user",
					Content: "Autonomous verification failed. Diagnose the root cause before retrying:\n\n" + diagnostics,
				})
			}
			continue
		}
		reqMessages := r.requestMessages(messages, opts)
		if strings.TrimSpace(loopMsg.Content) != "" {
			reqMessages = append(reqMessages, loopMsg)
		}
		if opts.OnContext != nil {
			total := 0
			for _, m := range reqMessages {
				total += opts.Counter.Count(m.Content)
			}
			opts.OnContext(total)
		}

		var answer string
		var nativeToolCalls []providers.ToolCall
		var err error

		for attempt := 0; attempt <= maxStreamStallRetries; attempt++ {
			events, streamErr := streamWithReconnect(ctx, r.Provider, providers.ChatRequest{
				Model:       opts.Model,
				Messages:    reqMessages,
				Tools:       toolDefs,
				Temperature: opts.Temperature,
				MaxTokens:   opts.MaxTokens,
			}, nil)

			if streamErr != nil {
				if isImageRejectionError(streamErr) {
					auto.stop(ctx, streamErr.Error())
					return RunResult{Messages: messages, Steps: steps, Events: auto.events()}, fmt.Errorf("model rejected image input: %v (try a vision-capable model)", streamErr)
				}
				// Reactive compaction: if the error looks like a context-limit error, compact and retry once.
				if compacted, retried, compactErr := compaction.recover(ctx, r.Provider, messages, toolDefs, streamErr.Error()); retried {
					if compactErr != nil {
						auto.stop(ctx, compactErr.Error())
						return RunResult{Messages: messages, Steps: steps, Events: auto.events()}, compactErr
					}
					messages = compacted
					continue // this will re-evaluate proactive compaction, which is fine
				}
				err = streamErr
				break
			}

			answer, nativeToolCalls, err = (StreamHandler{
				OnToken:    opts.OnToken,
				OnActivity: opts.OnActivity,
			}).Consume(ctx, events)

			if err != nil {
				if isImageRejectionError(err) {
					auto.stop(ctx, err.Error())
					return RunResult{Messages: messages, Steps: steps, Events: auto.events()}, fmt.Errorf("model rejected image input during stream: %v", err)
				}
				// If we got a stall error but no visible answer was forwarded, we can safely retry
				if attempt < maxStreamStallRetries && answer == "" && len(nativeToolCalls) == 0 {
					time.Sleep(1 * time.Second)
					continue
				}
				break
			}
			break // stream succeeded
		}

		if err != nil {
			auto.stop(ctx, err.Error())
			return RunResult{Messages: messages, Steps: steps, Events: auto.events()}, err
		}

		answer = strings.TrimSpace(answer)

		// Native tool calling path (OpenAI, Anthropic, etc.)
		if len(nativeToolCalls) > 0 {
			assistantMsg := answer
			if assistantMsg == "" {
				assistantMsg = fmt.Sprintf("I'll use the %s tool.", nativeToolCalls[0].Name)
			}
			messages = append(messages, providers.Message{
				Role:      "assistant",
				Content:   assistantMsg,
				ToolCalls: nativeToolCalls,
			})

			var mutatedThisTurn bool
			if opts.Hooks == nil && opts.ApproveTool == nil {
				if calls, ok := parseNativeToolCalls(ctx, nativeToolCalls); ok && auto.canRunToolCallsInParallel(calls) {
					results := r.executeToolBatchParallel(ctx, auto, calls)
					for i, call := range calls {
						result := results[i]
						for _, loaded := range result.LoadedTools {
							loadedTools[loaded] = true
						}
						if result.RequestedModel != "" {
							turnRequestedModel = result.RequestedModel
						}

						step := Step{Kind: "tool", Text: formatToolStep(call, result)}
						steps = append(steps, step)
						if opts.OnStep != nil {
							opts.OnStep(step)
						}
						redacted := redactToolResult(result)
						messages = append(messages, providers.Message{
							Role:       "tool",
							Name:       nativeToolCalls[i].Name,
							ToolCallID: nativeToolCalls[i].ID,
							Content:    formatToolResultContent(redacted),
						})

						failureOutcome := guard.observeToolResult(call.Name, result.Error != "", result.Error)
						if failureOutcome.Stop {
							for j := i + 1; j < len(nativeToolCalls); j++ {
								messages = append(messages, providers.Message{
									Role:       "tool",
									Name:       nativeToolCalls[j].Name,
									ToolCallID: nativeToolCalls[j].ID,
									Content:    "aborted: run halted by the repeated-failure guard",
								})
							}
							stopAnswer := toolFailureStopAnswer(call.Name, failureOutcome.Count)
							messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
							auto.stop(ctx, stopAnswer)
							return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
						}
						if failureOutcome.InjectHint {
							hint := toolFailureHint(call.Name, result.Error)
							messages = append(messages, providers.Message{Role: "user", Content: hint})
						}
					}

					callInfos := make([]toolCallInfo, 0, len(calls))
					for _, call := range calls {
						callInfos = append(callInfos, toolCallInfo{Name: call.Name})
					}
					if guard.observeTurn(answer, callInfos) {
						stopAnswer := noOutputStopAnswer(guard.emptyTurns)
						messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
						auto.stop(ctx, stopAnswer)
						return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
					}
					if reminder := guard.planReminder(iteration + 1); reminder != "" {
						messages = append(messages, providers.Message{Role: "user", Content: reminder})
					}
					if turnRequestedModel != "" && opts.ModelSwitcher != nil {
						newProvider, err := opts.ModelSwitcher(ctx, turnRequestedModel)
						if err != nil {
							messages = append(messages, providers.Message{
								Role:    "user",
								Content: "Note: could not switch to requested model: " + err.Error() + ". Continuing on " + opts.Model + ".",
							})
						} else if newProvider != nil {
							r.Provider = newProvider
							opts.Model = turnRequestedModel
						}
					}
					continue
				}
			}

			// Tool argument preparation with hard timeout: prevents the UI
			// from freezing permanently on "Preparing tool call" if the JSON
			// decoder or argument parser stalls on malformed stream output.
			// Each native tool call's argument parsing gets its own 30s timeout
			// to prevent a single malformed call from freezing the entire loop.
			for i, tc := range nativeToolCalls {
				var args map[string]any
				if tc.Arguments != "" {
					// Derive a timeout context so recoverableToolArguments
					// (which now checks ctx.Done() in its loop) can bail out
					// if argument parsing takes too long.
					parseCtx, parseCancel := context.WithTimeout(ctx, 30*time.Second)
					if first, ok := recoverableToolArguments(parseCtx, tc.Arguments); ok {
						json.Unmarshal([]byte(first), &args)
					} else {
						json.Unmarshal([]byte(tc.Arguments), &args)
					}
					parseCancel()
					if args == nil {
						args = map[string]any{}
					}
				}
				call := tools.Call{Name: tc.Name, Argument: args}

				// Lifecycle hook: beforeTool
				if opts.Hooks != nil {
					hookResults := opts.Hooks.Dispatch(ctx, HookBeforeTool, call.Name, nil)
					if ShouldBlock(hookResults) {
						blockedResult := tools.Result{Error: "tool call blocked by beforeTool hook"}
						step := Step{Kind: "system", Text: "Hook blocked: " + call.Name}
						steps = append(steps, step)
						if opts.OnStep != nil {
							opts.OnStep(step)
						}
						messages = append(messages, providers.Message{
							Role:       "tool",
							Name:       tc.Name,
							ToolCallID: tc.ID,
							Content:    blockedResult.Error,
						})
						continue
					}
				}

				blockedResult, blocked := auto.beforeTool(ctx, call)
				result := blockedResult
				if !blocked {
					result = r.executeToolIfApproved(ctx, call, opts)
				}
				auto.afterTool(ctx, call, result)

				for _, loaded := range result.LoadedTools {
					loadedTools[loaded] = true
				}
				if result.RequestedModel != "" {
					turnRequestedModel = result.RequestedModel
				}

				// Lifecycle hook: afterTool
				if opts.Hooks != nil {
					hookResults := opts.Hooks.Dispatch(ctx, HookAfterTool, call.Name, nil)
					if feedback := GetFeedback(hookResults); feedback != "" {
						step := Step{Kind: "system", Text: "Hook feedback: " + truncateToolText(feedback)}
						steps = append(steps, step)
						if opts.OnStep != nil {
							opts.OnStep(step)
						}
					}
				}

				step := Step{Kind: "tool", Text: formatToolStep(call, result)}
				steps = append(steps, step)
				if opts.OnStep != nil {
					opts.OnStep(step)
				}
				redacted := redactToolResult(result)
				messages = append(messages, providers.Message{
					Role:       "tool",
					Name:       tc.Name,
					ToolCallID: tc.ID,
					Content:    formatToolResultContent(redacted),
				})

				if isMutatingTool(call) && result.Error == "" {
					mutatedThisTurn = true
				}

				// Guardrails: track tool failures for this specific call
				failureOutcome := guard.observeToolResult(call.Name, result.Error != "", result.Error)
				if failureOutcome.Stop {
					// Append aborted results for remaining unexecuted tool calls
					for j := i + 1; j < len(nativeToolCalls); j++ {
						messages = append(messages, providers.Message{
							Role:       "tool",
							Name:       nativeToolCalls[j].Name,
							ToolCallID: nativeToolCalls[j].ID,
							Content:    "aborted: run halted by the repeated-failure guard",
						})
					}
					stopAnswer := toolFailureStopAnswer(call.Name, failureOutcome.Count)
					messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
					auto.stop(ctx, stopAnswer)
					return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
				}
				if failureOutcome.InjectHint {
					hint := toolFailureHint(call.Name, result.Error)
					messages = append(messages, providers.Message{Role: "user", Content: hint})
				}
			}

			// Self-correction: run verification AFTER ALL mutating tools in the batch
			if opts.SelfCorrector != nil && mutatedThisTurn {
				auto.beforeVerification("post-mutation verification")
				diagnostics, verifyErr := opts.SelfCorrector.VerifyAfterMutation(ctx)
				if verifyErr != nil {
					diagnostics = verifyErr.Error()
				}
				auto.afterVerification(ctx, diagnostics)
				if diagnostics != "" {
					step := Step{Kind: "system", Text: "Self-correction: verification found issues"}
					steps = append(steps, step)
					if opts.OnStep != nil {
						opts.OnStep(step)
					}
					messages = append(messages, providers.Message{
						Role:    "user",
						Content: "The last edit produced verification errors. Please fix these:\n\n" + diagnostics,
					})
				}
			}

			// Guardrails: track tool calls for empty turn detection
			var callInfos []toolCallInfo
			for _, tc := range nativeToolCalls {
				callInfos = append(callInfos, toolCallInfo{Name: tc.Name})
			}
			if guard.observeTurn(answer, callInfos) {
				stopAnswer := noOutputStopAnswer(guard.emptyTurns)
				messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
				auto.stop(ctx, stopAnswer)
				return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
			}
			if len(nativeToolCalls) == 0 {
				if strings.HasSuffix(answer, ":") || strings.Contains(answer, "Let me check") {
					messages = append(messages, providers.Message{Role: "user", Content: "Your message ended mid-step. Please continue."})
					continue
				}
			}

			// Inject plan reminder if needed
			if reminder := guard.planReminder(iteration + 1); reminder != "" {
				messages = append(messages, providers.Message{Role: "user", Content: reminder})
			}

			// Handle Model Escalation
			if turnRequestedModel != "" && opts.ModelSwitcher != nil {
				newProvider, err := opts.ModelSwitcher(ctx, turnRequestedModel)
				if err != nil {
					messages = append(messages, providers.Message{
						Role:    "user",
						Content: "Note: could not switch to requested model: " + err.Error() + ". Continuing on " + opts.Model + ".",
					})
				} else if newProvider != nil {
					r.Provider = newProvider
					opts.Model = turnRequestedModel
				}
			}

			continue
		}

		// Text-based tool calling fallback (Groq, Ollama, etc.)
		if answer == "" {
			answer = "(no response)"
		}
		if opts.DisableTools {
			messages = append(messages, providers.Message{Role: "assistant", Content: answer})
			auto.complete(ctx, answer)
			return RunResult{Messages: messages, Answer: answer, Steps: steps, Events: auto.events()}, nil
		}

		call, ok, err := parseToolCall(ctx, answer)
		if err != nil {
			errMsg := "Tool call parse error: " + err.Error()
			messages = append(messages,
				providers.Message{Role: "assistant", Content: answer},
				providers.Message{Role: "user", Content: errMsg + "\n\nTo call a tool, respond with exactly one JSON block:\n```cli_mate-tool\n{\"tool\":\"tool_name\",\"arguments\":{...}}\n```\nOr provide the final answer.\n\nAvailable tools:\n" + r.availableToolList()},
			)
			step := Step{Kind: "error", Text: "Invalid tool call: " + err.Error()}
			steps = append(steps, step)
			if opts.OnStep != nil {
				opts.OnStep(step)
			}
			continue
		}
		if !ok {
			if accepted, feedback := auto.acceptFinalAnswer(ctx, answer); !accepted {
				messages = append(messages,
					providers.Message{Role: "assistant", Content: answer},
					providers.Message{Role: "user", Content: feedback},
				)
				step := Step{Kind: "system", Text: truncateToolText(feedback)}
				steps = append(steps, step)
				if opts.OnStep != nil {
					opts.OnStep(step)
				}
				if auto.finalBlocks >= 3 {
					auto.stop(ctx, feedback)
					return RunResult{Messages: messages, Answer: feedback, Steps: steps, Events: auto.events()}, nil
				}
				continue
			}
			messages = append(messages, providers.Message{Role: "assistant", Content: answer})
			auto.complete(ctx, answer)
			return RunResult{Messages: messages, Answer: answer, Steps: steps, Events: auto.events()}, nil
		}

		// Lifecycle hook: beforeTool
		if opts.Hooks != nil {
			hookResults := opts.Hooks.Dispatch(ctx, HookBeforeTool, call.Name, nil)
			if ShouldBlock(hookResults) {
				blockedResult := tools.Result{Error: "tool call blocked by beforeTool hook"}
				step := Step{Kind: "system", Text: "Hook blocked: " + call.Name}
				steps = append(steps, step)
				if opts.OnStep != nil {
					opts.OnStep(step)
				}
				messages = append(messages,
					providers.Message{Role: "assistant", Content: answer},
					providers.Message{Role: "user", Content: blockedResult.Error},
				)
				continue
			}
		}

		blockedResult, blocked := auto.beforeTool(ctx, call)
		result := blockedResult
		if !blocked {
			result = r.executeToolIfApproved(ctx, call, opts)
		}
		auto.afterTool(ctx, call, result)

		for _, loaded := range result.LoadedTools {
			loadedTools[loaded] = true
		}
		if result.RequestedModel != "" {
			turnRequestedModel = result.RequestedModel
		}

		// Lifecycle hook: afterTool
		if opts.Hooks != nil {
			hookResults := opts.Hooks.Dispatch(ctx, HookAfterTool, call.Name, nil)
			if feedback := GetFeedback(hookResults); feedback != "" {
				step := Step{Kind: "system", Text: "Hook feedback: " + truncateToolText(feedback)}
				steps = append(steps, step)
				if opts.OnStep != nil {
					opts.OnStep(step)
				}
			}
		}

		step := Step{Kind: "tool", Text: formatToolStep(call, result)}
		steps = append(steps, step)
		if opts.OnStep != nil {
			opts.OnStep(step)
		}
		redacted := redactToolResult(result)
		messages = append(messages,
			providers.Message{Role: "assistant", Content: answer},
			providers.Message{Role: "user", Content: formatToolResult(call, redacted)},
		)

		// Self-correction: run verification after mutating tools
		if opts.SelfCorrector != nil && isMutatingTool(call) && result.Error == "" {
			auto.beforeVerification("post-mutation verification")
			diagnostics, verifyErr := opts.SelfCorrector.VerifyAfterMutation(ctx)
			if verifyErr != nil {
				diagnostics = verifyErr.Error()
			}
			auto.afterVerification(ctx, diagnostics)
			if diagnostics != "" {
				step := Step{Kind: "system", Text: "Self-correction: verification found issues"}
				steps = append(steps, step)
				if opts.OnStep != nil {
					opts.OnStep(step)
				}
				messages = append(messages, providers.Message{
					Role:    "user",
					Content: "The last edit produced verification errors. Please fix these:\n\n" + diagnostics,
				})
			}
		}

		// Guardrails: track turn and check for repeated tool failures
		callInfos := []toolCallInfo{{Name: call.Name}}
		if guard.observeTurn(answer, callInfos) {
			stopAnswer := noOutputStopAnswer(guard.emptyTurns)
			messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
			auto.stop(ctx, stopAnswer)
			return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
		}
		failureOutcome := guard.observeToolResult(call.Name, result.Error != "", result.Error)
		if failureOutcome.Stop {
			stopAnswer := toolFailureStopAnswer(call.Name, failureOutcome.Count)
			messages = append(messages, providers.Message{Role: "assistant", Content: stopAnswer})
			auto.stop(ctx, stopAnswer)
			return RunResult{Messages: messages, Answer: stopAnswer, Steps: steps, Events: auto.events()}, nil
		}
		if failureOutcome.InjectHint {
			hint := toolFailureHint(call.Name, result.Error)
			messages = append(messages, providers.Message{Role: "user", Content: hint})
		}
		// Inject plan or progress reminders if needed
		if reminder := guard.planReminder(iteration + 1); reminder != "" {
			messages = append(messages, providers.Message{Role: "user", Content: reminder})
		} else if progress := guard.progressReminder(); progress != "" {
			messages = append(messages, providers.Message{Role: "user", Content: progress})
		}
	}

	answer := fmt.Sprintf("Stopped after %d tool iterations. Narrow the request or inspect the last tool result.", maxIterations)
	messages = append(messages, providers.Message{Role: "assistant", Content: answer})
	auto.stop(ctx, answer)
	// Notify on long-running task completion
	if len(steps) > 5 {
		notify.SendIfSupported("cli_mate", "Long-running task completed after "+fmt.Sprintf("%d", len(steps))+" steps")
	}
	return RunResult{Messages: messages, Answer: answer, Steps: steps, Events: auto.events()}, nil
}

func (r *CodingRunner) availableToolList() string {
	var b strings.Builder
	for _, tool := range r.Tools {
		def := tool.Definition()
		b.WriteString("- ")
		b.WriteString(def.Name)
		b.WriteString(": ")
		b.WriteString(def.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func (r *CodingRunner) requestMessages(messages []providers.Message, opts RunOptions) []providers.Message {
	sysMsg := providers.Message{Role: "system", Content: r.systemPrompt(opts.DisableTools)}

	if opts.MaxTokens <= 0 {
		return append([]providers.Message{sysMsg}, messages...)
	}

	counter := opts.Counter
	if counter == nil {
		counter = tokenizer.NewApproxCounter()
	}

	sysCost := counter.Count(sysMsg.Content)
	// Create a window that accounts for the system prompt's size
	window := NewContextWindow(opts.MaxTokens-sysCost, opts.ReserveTokens, counter)

	agentMessages := make([]Message, len(messages))
	for i, message := range messages {
		agentMessages[i] = Message{
			Role:       Role(message.Role),
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallID: message.ToolCallID,
			Name:       message.Name,
		}
	}

	trimmed := window.Trim(agentMessages)

	out := make([]providers.Message, 0, len(trimmed)+1)
	out = append(out, sysMsg)
	for _, message := range trimmed {
		out = append(out, providers.Message{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallID: message.ToolCallID,
			Name:       message.Name,
		})
	}
	return out
}

func (r *CodingRunner) toolDefinitions() []providers.ToolDefinition {
	defs := make([]providers.ToolDefinition, 0, len(r.Tools))
	for _, tool := range r.Tools {
		d := tool.Definition()
		defs = append(defs, providers.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
			Schema:      d.Schema,
		})
	}
	return defs
}

func (r *CodingRunner) systemPrompt(disableTools bool) string {
	if disableTools {
		return "You are cli_mate, a concise terminal assistant. Answer the user directly. Do not inspect files, run commands, or use tools for casual conversation."
	}

	var b strings.Builder
	if strings.TrimSpace(r.Instructions) != "" {
		b.WriteString(strings.TrimSpace(r.Instructions))
		b.WriteString("\n\n")
	}
	// Add workspace context if available
	if wsCtx := BuildWorkspaceContext(r.WorkspaceRoot); wsCtx != "" {
		b.WriteString("## Workspace Context\n\n")
		b.WriteString(wsCtx)
		b.WriteString("\n\n")
	}
	// Add response style if set
	if stylePrompt := StylePrompt(r.Style); stylePrompt != "" {
		b.WriteString(stylePrompt)
		b.WriteString("\n\n")
	}
	b.WriteString("## cli_mate tool protocol\n\n")
	b.WriteString("You are an AI coding agent inside a terminal. You can inspect and modify the project using tools.\n\n")
	b.WriteString("### Tool rules\n")
	b.WriteString("- Always read a file BEFORE editing it.\n")
	b.WriteString("- Use file_edit for small, targeted changes (replace exact text).\n")
	b.WriteString("- Use file_write only for creating new files or replacing an entire file.\n")
	b.WriteString("- Use shell for running tests, builds, formatting, and inspection commands.\n")
	b.WriteString("- Use glob to discover files by pattern (e.g. '**/*.go').\n")
	b.WriteString("- Use grep to search file contents by regex pattern.\n")
	b.WriteString("- Use file_list to list files and directories.\n")
	b.WriteString("- Use read_subtree to see the structure of a directory and parsed variable names.\n")
	b.WriteString("- After Go edits, run `gofmt -s -w .` and relevant tests.\n\n")
	b.WriteString("### Final answer rules\n\n")
	b.WriteString("- When a coding task is complete, end with readable details of what you did.\n")
	b.WriteString("- Include concise sections for Summary, Files changed, Verification, and Notes when relevant.\n")
	b.WriteString("- In Verification, list the commands you ran and whether they passed. If you could not run verification, say why.\n")
	b.WriteString("- Keep the final answer easy to scan and do not paste raw tool logs unless the user asks for them.\n\n")
	b.WriteString("### Tool call format\n\n")
	b.WriteString("When you need to use a tool, respond with exactly one fenced JSON block:\n\n")
	b.WriteString("```cli_mate-tool\n{\"tool\":\"file_read\",\"arguments\":{\"path\":\"internal/example.go\"}}\n```\n\n")
	b.WriteString("After receiving a tool result, continue with another tool call or provide the final answer.\n\n")
	b.WriteString("### Available tools\n\n")
	for _, tool := range r.Tools {
		def := tool.Definition()
		b.WriteString("- **")
		b.WriteString(def.Name)
		b.WriteString("**: ")
		b.WriteString(def.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func (r *CodingRunner) executeTool(ctx context.Context, call tools.Call) tools.Result {
	tool, ok := r.Tools[call.Name]
	if !ok {
		return tools.Result{Error: fmt.Sprintf("unknown tool %q. Available tools: %s", call.Name, r.availableToolList())}
	}
	result, err := tool.Execute(ctx, call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	return result
}

func (r *CodingRunner) executeToolIfApproved(ctx context.Context, call tools.Call, opts RunOptions) tools.Result {
	if opts.ApproveTool != nil && !opts.ApproveTool(call) {
		return tools.Result{Error: fmt.Sprintf("tool %q was denied by the user", call.Name)}
	}
	return r.executeTool(ctx, call)
}

func parseNativeToolCalls(ctx context.Context, native []providers.ToolCall) ([]tools.Call, bool) {
	calls := make([]tools.Call, 0, len(native))
	for _, tc := range native {
		select {
		case <-ctx.Done():
			return nil, false
		default:
		}
		var args map[string]any
		if strings.TrimSpace(tc.Arguments) != "" {
			payload := tc.Arguments
			if first, ok := recoverableToolArguments(ctx, payload); ok {
				payload = first
			}
			if err := json.Unmarshal([]byte(payload), &args); err != nil {
				return nil, false
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		calls = append(calls, tools.Call{Name: tc.Name, Argument: args})
	}
	return calls, true
}

func (r *CodingRunner) executeToolBatchParallel(ctx context.Context, auto *autonomousRuntime, calls []tools.Call) []tools.Result {
	results := make([]tools.Result, len(calls))
	run := make([]bool, len(calls))
	for i, call := range calls {
		if blockedResult, blocked := auto.beforeTool(ctx, call); blocked {
			results[i] = blockedResult
			continue
		}
		run[i] = true
	}

	var wg sync.WaitGroup
	for i, call := range calls {
		if !run[i] {
			continue
		}
		wg.Add(1)
		go func(i int, call tools.Call) {
			defer wg.Done()
			results[i] = r.executeTool(ctx, call)
		}(i, call)
	}
	wg.Wait()

	for i, call := range calls {
		auto.afterTool(ctx, call, results[i])
	}
	return results
}

func recoverableToolArguments(ctx context.Context, arguments string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(arguments))
	var head json.RawMessage
	if err := dec.Decode(&head); err != nil {
		return "", false
	}
	// Safety breaker: prevent infinite loop if the decoder stalls
	// on malformed input. 100 values at the top level is more than
	// enough for any valid tool call. The context is checked on each
	// iteration so cancellation (e.g. user Ctrl+C or outer timeout)
	// breaks out immediately.
	const maxTopLevelValues = 100
	for i := 0; i < maxTopLevelValues; i++ {
		select {
		case <-ctx.Done():
			return "", false
		default:
		}
		var rest json.RawMessage
		if err := dec.Decode(&rest); err != nil {
			if err == io.EOF {
				return strings.TrimSpace(string(head)), true
			}
			return "", false
		}
	}
	// Safety: exceeded max top-level values, treat as malformed.
	return "", false
}

func isImageRejectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "400") {
		return false
	}
	for _, keyword := range []string{"image", "vision", "multimodal", "unsupported content type", "does not support"} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func parseToolCall(ctx context.Context, text string) (tools.Call, bool, error) {
	payload := strings.TrimSpace(text)
	if match := toolBlockPattern.FindStringSubmatch(text); len(match) == 2 {
		payload = strings.TrimSpace(match[1])
	} else if match := openToolBlockPattern.FindStringSubmatch(text); len(match) == 2 {
		payload = strings.TrimSpace(match[1])
	} else if !strings.HasPrefix(payload, "{") {
		return tools.Call{}, false, nil
	}

	var raw struct {
		Tool      string          `json:"tool"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Args      json.RawMessage `json:"args"`
	}

	// Use a timeout context so recoverableToolArguments can bail out
	// if the JSON decoder stalls on malformed input.
	parseCtx, parseCancel := context.WithTimeout(ctx, 30*time.Second)
	defer parseCancel()
	if first, ok := recoverableToolArguments(parseCtx, payload); ok {
		payload = first
	} else {
		if parseCtx.Err() != nil {
			return tools.Call{}, true, fmt.Errorf("tool call parsing failed: %v", parseCtx.Err())
		}
		return tools.Call{}, true, fmt.Errorf("malformed or incomplete JSON payload")
	}

	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return tools.Call{}, true, err
	}
	name := strings.TrimSpace(raw.Tool)
	if name == "" {
		name = strings.TrimSpace(raw.Name)
	}
	if name == "" {
		return tools.Call{}, true, fmt.Errorf("tool name is required in JSON block %q", truncateToolText(payload))
	}
	var args map[string]any
	if len(raw.Arguments) > 0 {
		json.Unmarshal(raw.Arguments, &args)
	} else if len(raw.Args) > 0 {
		json.Unmarshal(raw.Args, &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	return tools.Call{Name: name, Argument: args}, true, nil
}

func formatToolResult(call tools.Call, result tools.Result) string {
	payload := map[string]string{
		"tool":    call.Name,
		"content": truncateToolText(result.Content),
	}
	if result.Error != "" {
		payload["error"] = result.Error
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Sprintf("Tool result for %s:\n%s\n%s", call.Name, result.Content, result.Error)
	}
	return "Tool result:\n```json\n" + string(data) + "\n```"
}

func formatToolResultContent(result tools.Result) string {
	if result.Error != "" {
		if strings.TrimSpace(result.Content) == "" {
			return "Error: " + result.Error
		}
		return result.Content + "\nError: " + result.Error
	}
	return result.Content
}

func formatToolStep(call tools.Call, result tools.Result) string {
	path, _ := call.Argument["path"].(string)
	label := call.Name
	if path != "" {
		label += " " + path
	}
	if result.Error != "" {
		return label + " failed: " + result.Error
	}
	content := strings.TrimSpace(result.Content)
	if content == "" {
		return label + " completed"
	}
	return label + ": " + truncateToolText(content)
}

func truncateToolText(text string) string {
	if len(text) <= maxToolTextBytes {
		return text
	}
	return text[:maxToolTextBytes] + "\n... truncated ..."
}

const maxToolTextBytes = 12000

// redactToolResult scrubs secrets from tool output before it enters the
// conversation history, preventing API keys and tokens from leaking into
// the model's context window or stored sessions.
func redactToolResult(result tools.Result) tools.Result {
	if result.Content == "" && result.Error == "" {
		return result
	}
	opts := redaction.Options{}
	return tools.Result{
		Content: redaction.RedactString(result.Content, opts),
		Error:   redaction.RedactString(result.Error, opts),
	}
}
