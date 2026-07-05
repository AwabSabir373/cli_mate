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

func (e *Engine) Reflect(ctx context.Context, state *agentloop.RunState, verification agentloop.VerificationResult, hypotheses []agentloop.Hypothesis) agentloop.ReflectionReport {
	task := state.ActiveTask()
	report := agentloop.ReflectionReport{Confidence: state.Confidence}

	// Hypothesis-aware reflection: check which hypotheses are resolved.
	resolvedCount := 0
	totalActive := 0
	for _, h := range hypotheses {
		if h.State == agentloop.HypothesisConfirmed || h.State == agentloop.HypothesisRejected {
			resolvedCount++
		} else {
			totalActive++
		}
	}
	if len(hypotheses) > 0 {
		resolutionRate := float64(resolvedCount) / float64(len(hypotheses))
		if resolutionRate > 0.5 {
			report.ProgressDelta += 0.15
			report.Confidence = clamp(report.Confidence+0.1, 0, 1)
		}
		if resolutionRate >= 1.0 {
			report.ProgressDelta += 0.2
			report.Confidence = clamp(report.Confidence+0.15, 0, 1)
		}
		// If no hypotheses were resolved and we've been running, suggest replan
		if resolutionRate == 0 && state.Iteration > 3 {
			report.FailureReason = "no hypotheses resolved after multiple iterations; replanning needed"
			report.ReplanNeeded = true
		}
	}

	if verification.Status == agentloop.VerificationPassed {
		report.ProgressDelta = clamp(report.ProgressDelta+0.85, 0, 1)
		report.Confidence = clamp(report.Confidence+0.25, 0, 1)
		report.Evidence = verification.Evidence
		return report
	}

	if verification.Status == agentloop.VerificationFailed {
		report.ProgressDelta = clamp(report.ProgressDelta-0.2, -1, 1)
		report.Confidence = clamp(report.Confidence-0.2, 0, 1)
		report.FailureReason = verification.Summary
		report.ReplanNeeded = true
	}

	if verification.Status == agentloop.VerificationUnknown {
		if lastToolSucceeded(state.Events, taskID(task)) {
			report.ProgressDelta = clamp(report.ProgressDelta+0.1, 0, 1)
			report.Confidence = clamp(report.Confidence+0.05, 0, 0.85)
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
