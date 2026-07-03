package agent

import (
	"strconv"
	"strings"
)

// Guardrail thresholds for the agent loop. These keep a runaway model from
// burning turns/tokens and nudge it toward synthesizing progress.
const (
	// maxEmptyTurns stops the run after this many consecutive turns that
	// produced no visible text AND no tool calls.
	maxEmptyTurns = 3

	// staleToolCallThreshold injects a one-shot reminder once this many tool
	// calls have executed since the last plan update.
	staleToolCallThreshold = 10

	// toolOnlyProgressReminderAt injects a one-shot progress nudge after this
	// many consecutive turns contain tool calls but no visible assistant text.
	toolOnlyProgressReminderAt = 6

	// planReminderTurn is the turn (1-based) by the end of which a multi-step
	// task should have called update_plan; if it hasn't, a one-time reminder
	// is injected.
	planReminderTurn = 3

	// planToolName is the planning tool the loop watches for by name.
	planToolName = "enter_plan_mode"

	// toolFailureHintAt injects a one-shot corrective hint after a tool fails
	// this many times in a row with the same error.
	toolFailureHintAt = 2

	// toolFailureStopAt halts the run after a tool fails this many times in a
	// row with the same error.
	toolFailureStopAt = 4
)

// planNotCalledReminderMarker is a stable substring for tests.
const planNotCalledReminderMarker = "you have not called update_plan"

// planStaleReminderMarker is a stable substring for tests.
const planStaleReminderMarker = "haven't updated the plan via update_plan"

// toolOnlyProgressReminderMarker is a stable substring for tests.
const toolOnlyProgressReminderMarker = "consecutive tool-only turns"

// toolFailureHintMarker is a stable substring for tests.
const toolFailureHintMarker = "kept failing with the same error"

// noOutputStopPrefix and related constants for the no-output stop answer.
const (
	noOutputStopPrefix = "Agent stopped after "
	noOutputStopMarker = "with no output (no visible text and no tool calls)"
	noOutputStopSuffix = "to avoid consuming tokens without making progress."
)

type toolFailureRecord struct {
	count     int
	errSig    string
	hintShown bool
}

type toolFailureOutcome struct {
	InjectHint bool
	Stop       bool
	Count      int
}

// guardState tracks the per-run signals the guardrails need.
type guardState struct {
	emptyTurns               int
	totalToolCalls           int
	toolCallsSincePlanUpdate int
	planEverCalled           bool
	notCalledReminderSent    bool
	staleReminderSent        bool
	toolOnlyTurns            int
	toolOnlyReminderSent     bool
	toolFailures             map[string]*toolFailureRecord
}

func newGuardState() *guardState {
	return &guardState{toolFailures: map[string]*toolFailureRecord{}}
}

// observeToolResult tracks repeated identical failures of a tool. A successful
// result clears that tool's failure streak.
func (state *guardState) observeToolResult(name string, failed bool, output string) toolFailureOutcome {
	if state.toolFailures == nil {
		state.toolFailures = map[string]*toolFailureRecord{}
	}
	if !failed {
		delete(state.toolFailures, name)
		return toolFailureOutcome{}
	}
	sig := errorSignature(output)
	record := state.toolFailures[name]
	if record == nil || record.errSig != sig {
		record = &toolFailureRecord{count: 1, errSig: sig}
		state.toolFailures[name] = record
	} else {
		record.count++
	}
	outcome := toolFailureOutcome{Count: record.count}
	if record.count >= toolFailureStopAt {
		outcome.Stop = true
		return outcome
	}
	if record.count >= toolFailureHintAt && !record.hintShown {
		record.hintShown = true
		outcome.InjectHint = true
	}
	return outcome
}

// observeTurn updates counters from a turn's collected output. Returns whether
// the no-output guard should stop the run.
func (state *guardState) observeTurn(answer string, toolCalls []toolCallInfo) bool {
	hasToolCalls := len(toolCalls) > 0
	hasVisibleText := strings.TrimSpace(answer) != ""

	if hasToolCalls || hasVisibleText {
		state.emptyTurns = 0
	} else {
		state.emptyTurns++
	}
	if hasToolCalls && !hasVisibleText {
		state.toolOnlyTurns++
	} else {
		state.toolOnlyTurns = 0
		state.toolOnlyReminderSent = false
	}

	for _, tc := range toolCalls {
		state.totalToolCalls++
		if tc.Name == planToolName {
			state.planEverCalled = true
			state.toolCallsSincePlanUpdate = 0
			state.staleReminderSent = false
		} else {
			state.toolCallsSincePlanUpdate++
		}
	}

	return state.emptyTurns >= maxEmptyTurns
}

// planReminder returns a one-shot reminder message to inject, or empty string.
func (state *guardState) planReminder(turn int) string {
	if state.planEverCalled &&
		!state.staleReminderSent &&
		state.toolCallsSincePlanUpdate >= staleToolCallThreshold {
		state.staleReminderSent = true
		return planStaleReminder(state.toolCallsSincePlanUpdate)
	}

	if !state.notCalledReminderSent &&
		!state.planEverCalled &&
		turn >= planReminderTurn &&
		state.totalToolCalls >= 1 {
		state.notCalledReminderSent = true
		return planNotCalledReminder()
	}

	return ""
}

// progressReminder returns a one-shot nudge for tool-only turns, or empty string.
func (state *guardState) progressReminder() string {
	if state.toolOnlyReminderSent || state.toolOnlyTurns < toolOnlyProgressReminderAt {
		return ""
	}
	state.toolOnlyReminderSent = true
	return toolOnlyProgressReminder(state.toolOnlyTurns)
}

// errorSignature normalizes a tool error to a short, comparable signature.
func errorSignature(output string) string {
	s := strings.ToLower(strings.Join(strings.Fields(output), " "))
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// planNotCalledReminder nudges the model to track a multi-step task.
func planNotCalledReminder() string {
	return "Reminder: this looks like a multi-step task and " + planNotCalledReminderMarker +
		". Use the enter_plan_mode tool to record the steps and keep progress visible. " +
		"Continue with your work after updating the plan."
}

// planStaleReminder nudges the model to refresh the plan after a stretch of
// tool calls without a plan update.
func planStaleReminder(callsSinceUpdate int) string {
	return "Reminder: you've made " + strconv.Itoa(callsSinceUpdate) +
		" tool calls but " + planStaleReminderMarker +
		" in a while. Update the plan to reflect completed and remaining steps, then continue."
}

// toolOnlyProgressReminder nudges the model to synthesize before spending more turns.
func toolOnlyProgressReminder(turns int) string {
	return "Reminder: you've made " + strconv.Itoa(turns) + " " + toolOnlyProgressReminderMarker +
		" without visible progress. Before calling more tools, summarize what you already know, state the next concrete step, and finish if you have enough information."
}

// toolFailureHint tells the model exactly how a tool's arguments must look.
func toolFailureHint(toolName, errOutput string) string {
	return "Your calls to the `" + toolName + "` tool " + toolFailureHintMarker + ":\n" +
		strings.TrimSpace(errOutput) +
		"\n\nFix the arguments and try once more, or take a different approach."
}

// toolFailureStopAnswer is the final answer when the repeated-failure guard halts.
func toolFailureStopAnswer(toolName string, count int) string {
	return "Agent stopped: the `" + toolName + "` tool failed " + strconv.Itoa(count) +
		" times in a row with the same error, so I halted instead of looping further. " +
		"Please check the request or adjust the tool arguments."
}

// noOutputStopAnswer is the final answer returned when the no-output guard stops.
func noOutputStopAnswer(turns int) string {
	return noOutputStopPrefix + strconv.Itoa(turns) + " turns " + noOutputStopMarker + " " + noOutputStopSuffix
}

// toolCallInfo is a lightweight summary of a tool call for guardrail tracking.
type toolCallInfo struct {
	Name string
}
