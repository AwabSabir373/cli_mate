package reflection

import (
	"context"
	"strings"

	"cli_mate/internal/agent/agentloop"
)

type Engine struct {
	Retry agentloop.RetryPolicy
}

func New() *Engine {
	return &Engine{
		Retry: agentloop.RetryPolicy{
			MaxAttemptsPerTask: 3,
			MaxSameError:       2,
			RequiresNewInfo:    true,
		},
	}
}

func (e *Engine) Reflect(_ context.Context, state *agentloop.RunState, verification agentloop.VerificationResult) agentloop.ReflectionReport {
	task := state.ActiveTask()
	report := agentloop.ReflectionReport{Confidence: state.Confidence}

	if verification.Status == agentloop.VerificationPassed {
		report.ProgressDelta = 1
		report.Confidence = clamp(state.Confidence+0.35, 0, 1)
		report.Evidence = verification.Evidence
		return report
	}

	if verification.Status == agentloop.VerificationFailed {
		report.ProgressDelta = -0.2
		report.Confidence = clamp(state.Confidence-0.2, 0, 1)
		report.FailureReason = verification.Summary
		report.ReplanNeeded = true
	}

	if verification.Status == agentloop.VerificationUnknown {
		if lastToolSucceeded(state.Events, taskID(task)) {
			report.ProgressDelta = 0.1
			report.Confidence = clamp(state.Confidence+0.05, 0, 0.85)
		}
		if repeatedAction(state.Events, taskID(task), 3) {
			report.ProgressDelta = 0
			report.Confidence = clamp(state.Confidence-0.1, 0, 1)
			report.FailureReason = "same action repeated without increasing evidence"
			report.ReplanNeeded = true
		}
	}

	if task != nil {
		if task.Attempts >= max(1, e.Retry.MaxAttemptsPerTask) {
			report.StopRecommended = true
			report.StopReason = "task exceeded retry policy"
		}
		if repeatedFailure(state.Events, task.ID, max(1, e.Retry.MaxSameError)) {
			report.ReplanNeeded = true
			report.FailureReason = "same failure repeated without new evidence"
		}
	}

	if state.Iteration >= state.MaxIterations {
		report.StopRecommended = true
		report.StopReason = "iteration budget reached"
	}
	if state.Metrics.ConsecutiveNoProgressTurns >= 3 {
		report.StopRecommended = true
		report.StopReason = "no measurable progress across repeated turns"
	}
	if state.Metrics.ConsecutiveVerificationFail >= 2 {
		report.ReplanNeeded = true
		if report.FailureReason == "" {
			report.FailureReason = "verification failed repeatedly; changing strategy"
		}
	}

	if report.FailureReason == "" && state.LastError != "" {
		report.FailureReason = state.LastError
	}
	return report
}

func lastToolSucceeded(events []agentloop.Event, taskID string) bool {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.TaskID != taskID || event.Type != agentloop.EventToolCompleted {
			continue
		}
		return !strings.Contains(strings.ToLower(event.Summary), "failed")
	}
	return false
}

func repeatedAction(events []agentloop.Event, taskID string, threshold int) bool {
	if threshold <= 0 {
		return false
	}
	count := 0
	last := ""
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.TaskID != taskID || event.Type != agentloop.EventToolStarted {
			continue
		}
		sig := signature(event.Summary)
		if last == "" {
			last = sig
		}
		if sig != last {
			break
		}
		count++
		if count >= threshold {
			return true
		}
	}
	return false
}

func taskID(task *agentloop.TaskNode) string {
	if task == nil {
		return ""
	}
	return task.ID
}

func repeatedFailure(events []agentloop.Event, taskID string, threshold int) bool {
	if threshold <= 0 {
		return false
	}
	count := 0
	last := ""
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.TaskID != taskID || event.Type != agentloop.EventVerificationFailed {
			continue
		}
		sig := signature(event.Summary)
		if last == "" {
			last = sig
		}
		if sig != last {
			break
		}
		count++
		if count >= threshold {
			return true
		}
	}
	return false
}

func signature(text string) string {
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	if len(text) > 120 {
		return text[:120]
	}
	return text
}

func clamp(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
