package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli_mate/internal/agent/agentloop"
	"cli_mate/internal/agent/contextengine"
	"cli_mate/internal/agent/decision"
	"cli_mate/internal/agent/memory"
	"cli_mate/internal/agent/planner"
	"cli_mate/internal/agent/reflection"
	"cli_mate/internal/agent/toolrouter"
	"cli_mate/internal/agent/verification"
	"cli_mate/internal/providers"
	"cli_mate/internal/tools"
)

type autonomousRuntime struct {
	state            *agentloop.RunState
	planner          planner.Planner
	contextEngine    *contextengine.Engine
	router           *toolrouter.ToolRouter
	decider          *decision.Engine
	memory           memory.Store
	reflector        *reflection.Engine
	verifier         *verification.Engine
	workspaceRoot    string
	onStep           func(Step)
	readFiles        map[string]bool
	lastMemoryIndex  int
	lastDecision     agentloop.ActionDecision
	hypotheses       []agentloop.Hypothesis
	toolSignatures   map[string]int
	requiresMutation bool
	finalBlocks      int
}

func (r *CodingRunner) newAutonomousRuntime(opts RunOptions, maxIterations int) *autonomousRuntime {
	toolset := make([]tools.Tool, 0, len(r.Tools))
	for _, tool := range r.Tools {
		toolset = append(toolset, tool)
	}
	router := toolrouter.NewFromTools(toolset)
	budget := agentloop.TokenBudget{
		Total:          opts.MaxTokens,
		ReservedOutput: opts.ReserveTokens,
		ReservedTools:  1500,
	}
	if budget.Total <= 0 {
		budget.Total = 16000
	}
	store := r.Memory
	if store == nil {
		store = memory.NewInMemoryStore()
		r.Memory = store
	}
	ctxEngine := contextengine.New(budget)
	ctxEngine.Memory = store
	return &autonomousRuntime{
		state:            agentloop.NewRunState(agentloop.NewID("run"), opts.Prompt, maxIterations),
		planner:          planner.NewHeuristicPlanner(),
		contextEngine:    ctxEngine,
		router:           router,
		decider:          decision.New(router),
		memory:           store,
		reflector:        reflection.New(),
		verifier:         verification.New(r.WorkspaceRoot),
		workspaceRoot:    r.WorkspaceRoot,
		onStep:           opts.OnStep,
		readFiles:        map[string]bool{},
		toolSignatures:   map[string]int{},
		requiresMutation: promptRequestsMutation(opts.Prompt),
	}
}

func (rt *autonomousRuntime) events() []agentloop.Event {
	if rt == nil || rt.state == nil {
		return nil
	}
	out := make([]agentloop.Event, len(rt.state.Events))
	copy(out, rt.state.Events)
	return out
}

func (rt *autonomousRuntime) emit(event agentloop.Event) {
	if rt == nil || rt.state == nil {
		return
	}
	rt.state.Append(event)
}

func (rt *autonomousRuntime) emitStep(kind, text string) {
	if rt == nil || rt.onStep == nil || strings.TrimSpace(text) == "" {
		return
	}
	rt.onStep(Step{Kind: kind, Text: text})
}

func (rt *autonomousRuntime) beforeModelTurn(ctx context.Context, messages []providers.Message, iteration int, disabled bool) providers.Message {
	if rt == nil || disabled {
		return providers.Message{}
	}
	rt.state.Iteration = iteration + 1
	rt.state.Metrics.Iterations = rt.state.Iteration
	rt.state.SetPhase(agentloop.PhaseContextGathering)
	rt.contextEngine.Collectors = []contextengine.Collector{
		conversationCollector{messages: messages},
	}
	bundle, err := rt.contextEngine.Build(ctx, rt.state)
	if err != nil {
		rt.state.LastError = err.Error()
		rt.emit(agentloop.Event{Type: agentloop.EventContextCollected, Summary: "context collection failed: " + err.Error()})
	} else {
		rt.emit(agentloop.Event{
			Type:    agentloop.EventContextCollected,
			Summary: fmt.Sprintf("ranked %d context items with %d tokens remaining", len(bundle.Items), bundle.TokenBudget.Remaining()),
		})
	}

	if rt.state.Plan == nil || rt.state.Plan.Empty() {
		rt.state.SetPhase(agentloop.PhasePlanning)
		rt.emit(agentloop.Event{Type: agentloop.EventPlanningStarted, Summary: "creating task graph"})
		graph, planErr := rt.planner.Plan(ctx, bundle)
		if planErr != nil {
			rt.state.LastError = planErr.Error()
			rt.emit(agentloop.Event{Type: agentloop.EventStopped, Summary: "planning failed: " + planErr.Error()})
		} else {
			rt.state.Plan = graph
			rt.emit(agentloop.Event{Type: agentloop.EventPlanCreated, Summary: fmt.Sprintf("created task graph with %d nodes", len(graph.Nodes))})
			rt.emit(agentloop.Event{Type: agentloop.EventPlanningCompleted, Summary: "task graph ready"})
			rt.emitStep("system", "Planning completed: task graph ready")
		}
	}

	rt.ensureActiveTask()
	bundle.ActiveTask = rt.state.ActiveTask()
	decisionReport, decisionErr := rt.decider.Decide(ctx, rt.state, bundle)
	if decisionErr == nil {
		rt.lastDecision = decisionReport
		rt.hypotheses = decisionReport.ActiveHypotheses
		if decisionReport.SelectedAction.Tool.Name != "" {
			rt.emit(agentloop.Event{
				Type:   agentloop.EventToolSelected,
				TaskID: rt.state.ActiveTaskID,
				Summary: fmt.Sprintf("%s score=%.2f risk=%s info=%.2f unc=%.2f",
					decisionReport.SelectedAction.Tool.Name,
					decisionReport.SelectedAction.Score,
					decisionReport.SelectedAction.Tool.Risk.String(),
					decisionReport.InformationGainEstimate,
					decisionReport.RemainingUncertainty,
				),
				Data: map[string]any{"reason": decisionReport.Reason, "hypotheses": len(decisionReport.ActiveHypotheses)},
			})
		}
	} else {
		rt.state.LastError = decisionErr.Error()
	}

	return providers.Message{
		Role:    "user",
		Content: rt.loopPrompt(decisionReport),
	}
}

func (rt *autonomousRuntime) ensureActiveTask() {
	if rt.state == nil || rt.state.Plan == nil {
		return
	}
	active := rt.state.ActiveTask()
	if active != nil && active.Status == agentloop.TaskRunning {
		return
	}
	for {
		next := rt.state.Plan.NextRunnable()
		if next == nil {
			return
		}
		if rt.resolveInternalTask(next) {
			continue
		}
		next.Status = agentloop.TaskRunning
		next.Attempts++
		next.UpdatedAt = time.Now().UTC()
		rt.state.ActiveTaskID = next.ID
		rt.emit(agentloop.Event{Type: agentloop.EventTaskStarted, TaskID: next.ID, Summary: next.Goal})
		return
	}
}

func (rt *autonomousRuntime) resolveInternalTask(task *agentloop.TaskNode) bool {
	if task == nil {
		return false
	}
	goal := strings.ToLower(task.Goal)
	if strings.Contains(goal, "task graph") || strings.Contains(goal, "explicit verification before edits") {
		task.Status = agentloop.TaskVerified
		task.Confidence = 0.95
		task.UpdatedAt = time.Now().UTC()
		rt.emit(agentloop.Event{Type: agentloop.EventReflection, TaskID: task.ID, Summary: "internal planning task satisfied by current task graph"})
		return true
	}
	return false
}

func (rt *autonomousRuntime) loopPrompt(decisionReport agentloop.ActionDecision) string {
	task := rt.state.ActiveTask()
	goal := rt.state.UserRequest
	if task != nil && strings.TrimSpace(task.Goal) != "" {
		goal = task.Goal
	}
	var b strings.Builder
	b.WriteString("Autonomous loop state for CLI Mate. Use this internally; do not quote it unless relevant.\n")
	b.WriteString("- Current goal: ")
	b.WriteString(singleLine(goal, 220))
	b.WriteString("\n")
	if task != nil {
		fmt.Fprintf(&b, "- Task: %s, attempt %d/%d, risk %s, confidence %.2f\n",
			task.Status, task.Attempts, maxInt(task.MaxAttempts, 1), task.Risk.String(), task.Confidence)
	}
	b.WriteString("- Decision: ")
	if decisionReport.Reason != "" {
		b.WriteString(singleLine(decisionReport.Reason, 300))
	} else {
		b.WriteString("gather evidence, choose the cheapest safe tool, verify before final answer")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- Knows enough: %t; needs more context: %t; parallel safe reads: %t; approval needed: %t\n",
		decisionReport.KnowEnough, decisionReport.NeedMoreContext, decisionReport.CanParallelize, decisionReport.RequiresApproval)
	if decisionReport.InternalConfidence > 0 {
		fmt.Fprintf(&b, "- Internal confidence: %.2f\n", decisionReport.InternalConfidence)
	}
	if decisionReport.SelectedAction.Tool.Name != "" {
		fmt.Fprintf(&b, "- Recommended next tool: %s (score %.2f). Alternative cheapest safe tool: %s.\n",
			decisionReport.SelectedAction.Tool.Name,
			decisionReport.SelectedAction.Score,
			emptyDefault(decisionReport.CheapestSafeAction.Tool.Name, "none"))
	}
	if len(decisionReport.ReviewerNotes) > 0 {
		b.WriteString("- Self-critic: ")
		b.WriteString(singleLine(strings.Join(decisionReport.ReviewerNotes, "; "), 500))
		b.WriteString("\n")
	}
	if len(decisionReport.RecoveryActions) > 0 {
		b.WriteString("- Recovery plan if selected action fails: ")
		names := make([]string, 0, len(decisionReport.RecoveryActions))
		for _, rec := range decisionReport.RecoveryActions {
			names = append(names, rec.Tool.Name)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}
	if len(decisionReport.RejectedActions) > 0 {
		b.WriteString("- Rejected actions: ")
		names := make([]string, 0, len(decisionReport.RejectedActions))
		for _, rejected := range decisionReport.RejectedActions {
			names = append(names, rejected.Tool.Name)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "- Metrics: iterations=%d tool_success=%.2f verification_success=%.2f avg_confidence=%.2f no_progress=%d\n",
		rt.state.Metrics.Iterations,
		rt.toolSuccessRate(),
		rt.verificationSuccessRate(),
		rt.state.Metrics.AverageConfidence,
		rt.state.Metrics.ConsecutiveNoProgressTurns,
	)
	if len(decisionReport.VerificationPlan.Commands) > 0 {
		b.WriteString("- Verification required before finish: ")
		names := make([]string, 0, len(decisionReport.VerificationPlan.Commands))
		for _, command := range decisionReport.VerificationPlan.Commands {
			if command.Required {
				names = append(names, command.Name)
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}
	if task != nil {
		if task.SuccessCriteria != "" {
			fmt.Fprintf(&b, "- Success criteria: %s\n", singleLine(task.SuccessCriteria, 160))
		}
		if task.FailureCriteria != "" {
			fmt.Fprintf(&b, "- Failure criteria: %s\n", singleLine(task.FailureCriteria, 160))
		}
	}
	// Hypothesis display
	if len(decisionReport.ActiveHypotheses) > 0 {
		b.WriteString(fmt.Sprintf("- Hypotheses: %d active, %.0f%% uncertainty\n", len(decisionReport.ActiveHypotheses), decisionReport.RemainingUncertainty*100))
		if decisionReport.PrimaryHypothesis != nil {
			b.WriteString(fmt.Sprintf("- Primary hypothesis: %s (prob=%.0f%% method=%s)\n",
				singleLine(decisionReport.PrimaryHypothesis.Description, 100),
				decisionReport.PrimaryHypothesis.Probability*100,
				decisionReport.PrimaryHypothesis.VerificationMethod))
		}
		if decisionReport.InformationGainEstimate > 0 {
			b.WriteString(fmt.Sprintf("- Selected action info gain: %.0f%%\n", decisionReport.InformationGainEstimate*100))
		}
		if len(decisionReport.RejectedHypotheses) > 0 {
			b.WriteString(fmt.Sprintf("- %d rejected hypotheses\n", len(decisionReport.RejectedHypotheses)))
		}
	}
	b.WriteString("Continue the loop: inspect only what is missing, edit only after evidence, recover from failures, and finish only after verification.")
	return b.String()
}

func (rt *autonomousRuntime) beforeTool(ctx context.Context, call tools.Call) (tools.Result, bool) {
	if rt == nil {
		return tools.Result{}, false
	}
	if call.Argument == nil {
		call.Argument = map[string]any{}
	}
	if desc, ok := rt.router.Descriptor(call.Name); ok {
		rt.emit(agentloop.Event{
			Type:    agentloop.EventToolSelected,
			TaskID:  rt.state.ActiveTaskID,
			Summary: fmt.Sprintf("%s risk=%s mutates=%t", call.Name, desc.Risk.String(), desc.Mutates),
			Data:    map[string]any{"capabilities": desc.Capabilities},
		})
	}
	if reason := rt.reviewToolCall(call); reason != "" {
		rt.state.Metrics.RejectedToolCalls++
		result := tools.Result{Error: reason}
		rt.emit(agentloop.Event{Type: agentloop.EventWaitingForUser, TaskID: rt.state.ActiveTaskID, Summary: reason})
		return result, true
	}
	if err := rt.validateToolCall(ctx, call); err != nil {
		rt.state.Metrics.RejectedToolCalls++
		result := tools.Result{Error: err.Error()}
		rt.emit(agentloop.Event{Type: agentloop.EventWaitingForUser, TaskID: rt.state.ActiveTaskID, Summary: err.Error()})
		return result, true
	}
	rt.state.SetPhase(agentloop.PhaseActing)
	rt.trackToolSignature(call)
	rt.emit(agentloop.Event{Type: agentloop.EventToolStarted, TaskID: rt.state.ActiveTaskID, Summary: call.Name, Data: map[string]any{"args": safeToolArgs(call)}})
	return tools.Result{}, false
}

func (rt *autonomousRuntime) reviewToolCall(call tools.Call) string {
	if rt == nil {
		return ""
	}
	for _, rejected := range rt.lastDecision.RejectedActions {
		if rejected.Tool.Name == call.Name {
			return fmt.Sprintf("self-critic rejected %s: %s", call.Name, rejected.Evaluation.Review)
		}
	}
	if rt.lastDecision.NeedMoreContext && isMutatingTool(call) {
		return fmt.Sprintf("refusing %s because current decision says more context is needed before mutation", call.Name)
	}
	if rt.lastDecision.RequiresApproval && !isReadOnlyTool(call, rt.router) {
		return fmt.Sprintf("refusing %s because the decision policy requires approval for this risk level", call.Name)
	}
	signature := rt.toolSignature(call)
	if rt.toolSignatures[signature] >= 3 {
		rt.state.Metrics.RepeatedActionBlocks++
		return fmt.Sprintf("refusing repeated action %s; choose a different strategy or gather new evidence first", call.Name)
	}
	task := rt.state.ActiveTask()
	if task != nil && task.Attempts > maxInt(task.MaxAttempts, 1) {
		return fmt.Sprintf("refusing %s because task retry budget is exhausted", call.Name)
	}
	return ""
}

func (rt *autonomousRuntime) trackToolSignature(call tools.Call) {
	if rt == nil {
		return
	}
	rt.toolSignatures[rt.toolSignature(call)]++
}

func (rt *autonomousRuntime) toolSignature(call tools.Call) string {
	data, err := json.Marshal(safeToolArgs(call))
	if err != nil {
		return call.Name
	}
	return call.Name + ":" + string(data)
}

func isReadOnlyTool(call tools.Call, router *toolrouter.ToolRouter) bool {
	if router == nil {
		return !isMutatingTool(call)
	}
	desc, ok := router.Descriptor(call.Name)
	if !ok {
		return !isMutatingTool(call)
	}
	return !desc.Mutates && desc.Risk <= agentloop.RiskReadOnly
}

func (rt *autonomousRuntime) afterTool(ctx context.Context, call tools.Call, result tools.Result) agentloop.ReflectionReport {
	if rt == nil {
		return agentloop.ReflectionReport{}
	}
	rt.state.Metrics.ToolCalls++
	if result.Error != "" {
		rt.state.Metrics.ToolFailures++
	}
	rt.state.SetPhase(agentloop.PhaseObserving)
	status := "completed"
	if result.Error != "" {
		status = "failed: " + result.Error
		rt.state.LastError = result.Error
	}
	rt.emit(agentloop.Event{
		Type:    agentloop.EventToolCompleted,
		TaskID:  rt.state.ActiveTaskID,
		Summary: call.Name + " " + status,
		Data: map[string]any{
			"tool":             call.Name,
			"mutates":          isMutatingTool(call),
			"mutation_applied": toolMutationApplied(call, result),
			"success":          result.Error == "",
		},
		Evidence: []agentloop.EvidenceRef{{
			Kind:    "tool",
			Source:  call.Name,
			Summary: truncateToolText(result.Content + "\n" + result.Error),
		}},
	})
	rt.observeToolEffects(call, result)
	rt.advanceTaskAfterTool(call, result)
	// Update hypotheses based on tool result
	if len(rt.hypotheses) > 0 {
		updated, _ := decision.UpdateHypothesesAfterTool(rt.hypotheses, call.Name, result.Content, result.Error)
		rt.hypotheses = updated
	}
	return rt.reflectAfterObservation(ctx, result)
}

func (rt *autonomousRuntime) beforeVerification(summary string) {
	if rt == nil {
		return
	}
	rt.state.SetPhase(agentloop.PhaseVerifying)
	rt.emit(agentloop.Event{Type: agentloop.EventVerificationStarted, TaskID: rt.state.ActiveTaskID, Summary: summary})
}

func (rt *autonomousRuntime) runActiveVerification(ctx context.Context) (agentloop.VerificationResult, bool) {
	if rt == nil || rt.verifier == nil {
		return agentloop.VerificationResult{}, false
	}
	task := rt.state.ActiveTask()
	if task == nil || !isVerificationTask(task) {
		return agentloop.VerificationResult{}, false
	}
	spec := task.Verification
	if len(spec.Commands)+len(spec.FileChecks)+len(spec.DiffChecks) == 0 {
		return agentloop.VerificationResult{}, false
	}
	rt.beforeVerification("running verification without another model call")
	result := rt.verifier.Verify(ctx, spec)
	rt.afterVerificationResult(ctx, result)
	return result, true
}

func isVerificationTask(task *agentloop.TaskNode) bool {
	if task == nil {
		return false
	}
	goal := strings.ToLower(task.Goal)
	return strings.Contains(goal, "verify") ||
		strings.Contains(goal, "test") ||
		strings.Contains(goal, "build") ||
		strings.Contains(goal, "format") ||
		len(task.Verification.Commands)+len(task.Verification.FileChecks)+len(task.Verification.DiffChecks) > 0
}

func verificationDiagnostics(result agentloop.VerificationResult) string {
	if result.Status != agentloop.VerificationFailed {
		return ""
	}
	var b strings.Builder
	if strings.TrimSpace(result.Summary) != "" {
		b.WriteString(result.Summary)
		b.WriteString("\n")
	}
	for _, check := range result.Checks {
		if check.Passed {
			continue
		}
		fmt.Fprintf(&b, "\n%s failed: %s\n", check.Name, check.Error)
		if strings.TrimSpace(check.Output) != "" {
			b.WriteString(check.Output)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func (rt *autonomousRuntime) afterVerification(ctx context.Context, diagnostics string) {
	if rt == nil {
		return
	}
	result := agentloop.VerificationResult{
		Status:  agentloop.VerificationPassed,
		Summary: "verification passed",
		Evidence: []agentloop.EvidenceRef{{
			Kind:    "verification",
			Source:  "self_corrector",
			Summary: "post-mutation checks passed",
		}},
	}
	if strings.TrimSpace(diagnostics) != "" {
		result.Status = agentloop.VerificationFailed
		result.Summary = truncateToolText(diagnostics)
		result.Evidence[0].Summary = result.Summary
	}
	rt.afterVerificationResult(ctx, result)
}

func (rt *autonomousRuntime) afterVerificationResult(ctx context.Context, result agentloop.VerificationResult) {
	if rt == nil {
		return
	}
	rt.state.Metrics.VerificationRuns++
	if len(result.Evidence) == 0 {
		result.Evidence = []agentloop.EvidenceRef{{
			Kind:    "verification",
			Source:  "verifier",
			Summary: result.Summary,
		}}
	}
	if result.Status == agentloop.VerificationFailed {
		rt.state.Metrics.VerificationFailures++
		rt.state.Metrics.ConsecutiveVerificationFail++
		rt.emit(agentloop.Event{Type: agentloop.EventVerificationFailed, TaskID: rt.state.ActiveTaskID, Summary: result.Summary, Evidence: result.Evidence})
	} else {
		rt.state.Metrics.ConsecutiveVerificationFail = 0
		rt.emit(agentloop.Event{Type: agentloop.EventVerificationPassed, TaskID: rt.state.ActiveTaskID, Summary: result.Summary, Evidence: result.Evidence})
		if task := rt.state.ActiveTask(); task != nil {
			task.Status = agentloop.TaskVerified
			task.Confidence = 0.95
		}
	}
	rt.reflectWithVerification(ctx, result)
	rt.updateMemory(ctx)
}

func (rt *autonomousRuntime) complete(ctx context.Context, answer string) {
	if rt == nil {
		return
	}
	rt.state.SetPhase(agentloop.PhaseDone)
	rt.emit(agentloop.Event{Type: agentloop.EventCompleted, Summary: singleLine(answer, 300)})
	rt.updateMemory(ctx)
}

func (rt *autonomousRuntime) acceptFinalAnswer(ctx context.Context, answer string) (bool, string) {
	if rt == nil {
		return true, ""
	}
	if strings.TrimSpace(answer) == "" || strings.TrimSpace(answer) == "(no response)" {
		rt.finalBlocks++
		return false, "Final answer rejected: no substantive answer was produced."
	}
	if len(rt.router.Descriptors()) == 0 {
		return true, ""
	}
	if rt.hasMutationWithoutVerification() {
		spec := rt.completionVerificationSpec()
		if len(spec.Commands)+len(spec.FileChecks)+len(spec.DiffChecks) == 0 {
			rt.finalBlocks++
			return false, "Final answer rejected: files changed but no verification plan is available."
		}
		rt.beforeVerification("completion verification before final answer")
		result := rt.verifier.Verify(ctx, spec)
		rt.afterVerificationResult(ctx, result)
		if result.Status != agentloop.VerificationPassed {
			rt.finalBlocks++
			return false, "Final answer rejected: completion verification failed. Fix the diagnostics before finishing:\n\n" + verificationDiagnostics(result)
		}
	}
	if rt.requiresMutation && !rt.hasSuccessfulMutationEvidence() && !finalAnswerExplainsNoMutation(answer) {
		rt.finalBlocks++
		return false, "Final answer rejected: this looks like a change request, but no successful file edit/write/patch tool has run. Find the correct file and apply the edit, or clearly say why no edit was possible."
	}
	if rt.hasCompletionEvidence() {
		return true, ""
	}
	rt.finalBlocks++
	if rt.finalBlocks >= 3 {
		return false, "Final answer rejected repeatedly because there is still no evidence of completion; stop or ask for missing information instead of repeating the same conclusion."
	}
	return false, "Final answer rejected: completion requires evidence such as successful verification, successful relevant tool output, or confirmed requested file changes."
}

func (rt *autonomousRuntime) hasSuccessfulMutationEvidence() bool {
	if rt == nil || rt.state == nil {
		return false
	}
	for _, event := range rt.state.Events {
		if event.Type != agentloop.EventToolCompleted {
			continue
		}
		if eventBool(event.Data, "mutation_applied") && eventBool(event.Data, "success") {
			return true
		}
	}
	return false
}

func (rt *autonomousRuntime) hasCompletionEvidence() bool {
	if rt == nil || rt.state == nil {
		return false
	}
	if rt.allPlannedWorkVerified() {
		return true
	}
	for _, event := range rt.state.Events {
		switch event.Type {
		case agentloop.EventVerificationPassed:
			return true
		case agentloop.EventToolCompleted:
			if !strings.Contains(strings.ToLower(event.Summary), "failed") {
				return true
			}
		}
	}
	return false
}

func eventBool(data map[string]any, key string) bool {
	if data == nil {
		return false
	}
	value, ok := data[key]
	if !ok {
		return false
	}
	b, ok := value.(bool)
	return ok && b
}

func promptRequestsMutation(prompt string) bool {
	text := strings.ToLower(prompt)
	mutationWords := []string{
		"add ", "change", "create", "delete", "edit", "fix", "implement",
		"modify", "patch", "refactor", "remove", "rename", "repair",
		"replace", "update", "write",
	}
	for _, word := range mutationWords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func finalAnswerExplainsNoMutation(answer string) bool {
	text := strings.ToLower(answer)
	explanations := []string{
		"could not modify", "couldn't modify", "unable to modify",
		"did not modify", "no file was modified", "no files were modified",
		"no edit was possible", "no changes were made", "i could not edit",
		"i couldn't edit", "i was unable to edit",
	}
	for _, explanation := range explanations {
		if strings.Contains(text, explanation) {
			return true
		}
	}
	return false
}

func toolMutationApplied(call tools.Call, result tools.Result) bool {
	if !isMutatingTool(call) || result.Error != "" {
		return false
	}
	preview, _ := call.Argument["preview"].(bool)
	if preview {
		return false
	}
	switch call.Name {
	case "apply_patch":
		for _, line := range strings.Split(result.Content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "OK ") {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func (rt *autonomousRuntime) allPlannedWorkVerified() bool {
	if rt == nil || rt.state == nil || rt.state.Plan == nil {
		return false
	}
	checked := 0
	for _, id := range rt.state.Plan.Order {
		node := rt.state.Plan.Nodes[id]
		if node == nil || id == rt.state.Plan.RootID {
			continue
		}
		checked++
		if node.Status != agentloop.TaskVerified && node.Status != agentloop.TaskSkipped {
			return false
		}
	}
	return checked > 0
}

func (rt *autonomousRuntime) hasMutationWithoutVerification() bool {
	if rt == nil || rt.state == nil {
		return false
	}
	seenMutation := false
	verifiedAfterMutation := false
	for _, event := range rt.state.Events {
		if event.Type == agentloop.EventToolCompleted && event.Data != nil {
			if eventBool(event.Data, "mutation_applied") && !strings.Contains(strings.ToLower(event.Summary), "failed") {
				seenMutation = true
				verifiedAfterMutation = false
			}
		}
		if event.Type == agentloop.EventVerificationPassed && seenMutation {
			verifiedAfterMutation = true
		}
	}
	return seenMutation && !verifiedAfterMutation
}

func (rt *autonomousRuntime) completionVerificationSpec() agentloop.VerificationSpec {
	if rt == nil || rt.state == nil || rt.state.Plan == nil {
		return agentloop.VerificationSpec{}
	}
	for _, id := range rt.state.Plan.Order {
		node := rt.state.Plan.Nodes[id]
		if node == nil {
			continue
		}
		if len(node.Verification.Commands)+len(node.Verification.FileChecks)+len(node.Verification.DiffChecks) > 0 {
			return node.Verification
		}
	}
	return agentloop.VerificationSpec{}
}

func (rt *autonomousRuntime) stop(ctx context.Context, reason string) {
	if rt == nil {
		return
	}
	rt.state.SetPhase(agentloop.PhaseStopped)
	rt.emit(agentloop.Event{Type: agentloop.EventStopped, Summary: singleLine(reason, 300)})
	rt.updateMemory(ctx)
}

func (rt *autonomousRuntime) reflectAfterObservation(ctx context.Context, result tools.Result) agentloop.ReflectionReport {
	status := agentloop.VerificationUnknown
	summary := "tool completed"
	if result.Error != "" {
		status = agentloop.VerificationFailed
		summary = "tool failed: " + result.Error
	}
	return rt.reflectWithVerification(ctx, agentloop.VerificationResult{Status: status, Summary: summary})
}

func (rt *autonomousRuntime) reflectWithVerification(ctx context.Context, verificationResult agentloop.VerificationResult) agentloop.ReflectionReport {
	rt.state.SetPhase(agentloop.PhaseReflecting)
	rt.state.Metrics.ReflectionRuns++
	rt.emit(agentloop.Event{Type: agentloop.EventReflectionStarted, TaskID: rt.state.ActiveTaskID, Summary: "evaluating progress"})
	report := rt.reflector.Reflect(ctx, rt.state, verificationResult, rt.hypotheses)
	rt.state.Confidence = report.Confidence
	rt.updateReflectionMetrics(report)
	rt.emit(agentloop.Event{Type: agentloop.EventReflection, TaskID: rt.state.ActiveTaskID, Summary: reflectionSummary(report), Evidence: report.Evidence})
	if report.ReplanNeeded {
		rt.emit(agentloop.Event{Type: agentloop.EventReplanRequested, TaskID: rt.state.ActiveTaskID, Summary: emptyDefault(report.FailureReason, "reflection requested replanning")})
		rt.replan(ctx, report)
	} else if verificationResult.Status == agentloop.VerificationFailed && !report.StopRecommended {
		rt.emit(agentloop.Event{Type: agentloop.EventRetryScheduled, TaskID: rt.state.ActiveTaskID, Summary: "retry with new evidence or a different tool"})
	}
	if report.StopRecommended {
		rt.stop(ctx, report.StopReason)
	}
	rt.updateMemory(ctx)
	return report
}

func (rt *autonomousRuntime) replan(ctx context.Context, report agentloop.ReflectionReport) {
	rt.state.SetPhase(agentloop.PhaseReplanning)
	rt.state.Metrics.Recoveries++
	graph, err := rt.planner.Replan(ctx, rt.state, report)
	if err != nil {
		rt.state.LastError = err.Error()
		rt.emit(agentloop.Event{Type: agentloop.EventStopped, Summary: "replanning failed: " + err.Error()})
		return
	}
	rt.state.Plan = graph
	rt.state.ActiveTaskID = ""
	rt.emit(agentloop.Event{Type: agentloop.EventPlanningCompleted, Summary: fmt.Sprintf("replanned task graph with %d nodes", len(graph.Nodes))})
}

func (rt *autonomousRuntime) updateReflectionMetrics(report agentloop.ReflectionReport) {
	if rt == nil || rt.state == nil {
		return
	}
	metrics := &rt.state.Metrics
	metrics.LastProgressDelta = report.ProgressDelta
	if report.ProgressDelta <= 0 {
		metrics.ConsecutiveNoProgressTurns++
	} else {
		metrics.ConsecutiveNoProgressTurns = 0
	}
	if metrics.ReflectionRuns > 0 {
		previous := metrics.AverageConfidence * float64(metrics.ReflectionRuns-1)
		metrics.AverageConfidence = (previous + report.Confidence) / float64(metrics.ReflectionRuns)
	}
	if metrics.ToolCalls > 0 {
		metrics.ReasoningEfficiency = clampFloat(metrics.AverageConfidence/(float64(metrics.ToolCalls)+1), 0, 1)
	}
}

func (rt *autonomousRuntime) toolSuccessRate() float64 {
	if rt == nil || rt.state.Metrics.ToolCalls == 0 {
		return 1
	}
	return 1 - float64(rt.state.Metrics.ToolFailures)/float64(rt.state.Metrics.ToolCalls)
}

func (rt *autonomousRuntime) verificationSuccessRate() float64 {
	if rt == nil || rt.state.Metrics.VerificationRuns == 0 {
		return 1
	}
	return 1 - float64(rt.state.Metrics.VerificationFailures)/float64(rt.state.Metrics.VerificationRuns)
}

func (rt *autonomousRuntime) updateMemory(ctx context.Context) {
	if rt == nil || rt.memory == nil || rt.lastMemoryIndex >= len(rt.state.Events) {
		return
	}
	raw := rt.state.Events[rt.lastMemoryIndex:]
	events := make([]agentloop.Event, 0, len(raw))
	for _, event := range raw {
		if event.Type != agentloop.EventMemoryUpdated {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		rt.lastMemoryIndex = len(rt.state.Events)
		return
	}
	if err := rt.memory.UpdateFromEvents(ctx, rt.memoryScope(), events); err == nil {
		rt.emit(agentloop.Event{Type: agentloop.EventMemoryUpdated, Summary: fmt.Sprintf("stored %d event-derived memories", len(events))})
		rt.lastMemoryIndex = len(rt.state.Events)
	}
}

func (rt *autonomousRuntime) memoryScope() string {
	if rt == nil || strings.TrimSpace(rt.workspaceRoot) == "" {
		return "project"
	}
	return "project:" + filepath.ToSlash(strings.ToLower(rt.workspaceRoot))
}

func (rt *autonomousRuntime) observeToolEffects(call tools.Call, result tools.Result) {
	if result.Error != "" {
		return
	}
	if isMutatingTool(call) {
		rt.invalidatePostMutationTasks()
	}
	path := toolPath(call)
	if path == "" {
		return
	}
	normalized := rt.normalizePath(path)
	switch call.Name {
	case "file_read":
		rt.readFiles[normalized] = true
	case "file_write", "file_edit", "apply_patch":
		rt.readFiles[normalized] = true
	}
}

func (rt *autonomousRuntime) invalidatePostMutationTasks() {
	if rt == nil || rt.state == nil || rt.state.Plan == nil {
		return
	}
	activeID := rt.state.ActiveTaskID
	for _, id := range rt.state.Plan.Order {
		if id == activeID {
			continue
		}
		node := rt.state.Plan.Nodes[id]
		if node == nil || node.Status != agentloop.TaskVerified {
			continue
		}
		if isVerificationTask(node) || strings.Contains(strings.ToLower(node.Goal), "summarize") {
			node.Status = agentloop.TaskPending
			node.Confidence = minFloat(node.Confidence, 0.5)
			node.UpdatedAt = time.Now().UTC()
			rt.emit(agentloop.Event{Type: agentloop.EventReplanRequested, TaskID: node.ID, Summary: "post-mutation reality changed; verification/summary must run again"})
		}
	}
}

func (rt *autonomousRuntime) advanceTaskAfterTool(call tools.Call, result tools.Result) {
	task := rt.state.ActiveTask()
	if task == nil || result.Error != "" {
		return
	}
	goal := strings.ToLower(task.Goal)
	switch {
	case strings.Contains(goal, "inspect") && !isMutatingTool(call):
		task.Status = agentloop.TaskVerified
		task.Confidence = minFloat(task.Confidence+0.4, 0.95)
	case strings.Contains(goal, "apply") || strings.Contains(goal, "implement") || strings.Contains(goal, "change"):
		if isMutatingTool(call) {
			task.Status = agentloop.TaskVerified
			task.Confidence = minFloat(task.Confidence+0.35, 0.9)
		}
	}
	task.UpdatedAt = time.Now().UTC()
}

func (rt *autonomousRuntime) validateToolCall(_ context.Context, call tools.Call) error {
	switch call.Name {
	case "file_edit":
		preview, _ := call.Argument["preview"].(bool)
		if preview {
			return nil
		}
		path := toolPath(call)
		if path == "" {
			return fmt.Errorf("file_edit requires a path")
		}
		if !rt.readFiles[rt.normalizePath(path)] {
			return fmt.Errorf("refusing file_edit on %s before file_read; inspect the existing file first", path)
		}
	case "file_write":
		path := toolPath(call)
		if path == "" {
			return fmt.Errorf("file_write requires a path")
		}
		if rt.existingFile(path) && !rt.readFiles[rt.normalizePath(path)] {
			return fmt.Errorf("refusing to overwrite %s before file_read; use file_edit for small changes", path)
		}
	}
	return nil
}

func (rt *autonomousRuntime) canRunToolCallsInParallel(calls []tools.Call) bool {
	if rt == nil || len(calls) < 2 {
		return false
	}
	for _, call := range calls {
		desc, ok := rt.router.Descriptor(call.Name)
		if !ok || desc.Mutates || !desc.SupportsParallel {
			return false
		}
		if call.Name == "shell" || call.Name == "background_run" {
			return false
		}
		if err := rt.validateToolCall(context.Background(), call); err != nil {
			return false
		}
	}
	return true
}

func (rt *autonomousRuntime) existingFile(path string) bool {
	resolved := rt.resolvePath(path)
	if resolved == "" {
		return false
	}
	info, err := os.Stat(resolved)
	return err == nil && !info.IsDir()
}

func (rt *autonomousRuntime) normalizePath(path string) string {
	resolved := rt.resolvePath(path)
	if resolved == "" {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return strings.ToLower(filepath.ToSlash(resolved))
}

func (rt *autonomousRuntime) resolvePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	root := rt.workspaceRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	resolved := filepath.FromSlash(path)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(absRoot, resolved)
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ""
	}
	return resolved
}

type conversationCollector struct {
	messages []providers.Message
}

func (c conversationCollector) Collect(_ context.Context, _ *agentloop.RunState) ([]agentloop.ContextItem, error) {
	const maxMessages = 12
	start := len(c.messages) - maxMessages
	if start < 0 {
		start = 0
	}
	items := make([]agentloop.ContextItem, 0, len(c.messages)-start)
	for i, message := range c.messages[start:] {
		content := strings.TrimSpace(message.Content)
		if content == "" && len(message.ToolCalls) == 0 {
			continue
		}
		if len(content) > 2000 {
			content = content[:1400] + "\n... [conversation item compressed] ...\n" + content[len(content)-400:]
		}
		if len(message.ToolCalls) > 0 {
			names := make([]string, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				names = append(names, call.Name)
			}
			content += "\nTool calls: " + strings.Join(names, ", ")
		}
		items = append(items, agentloop.ContextItem{
			ID:        agentloop.NewID("ctx"),
			Kind:      "conversation",
			Source:    fmt.Sprintf("%s:%d", message.Role, start+i),
			Content:   content,
			Tokens:    ApproxTextTokens(content),
			Score:     scoreMessage(message),
			CreatedAt: time.Now().UTC(),
		})
	}
	return items, nil
}

func scoreMessage(message providers.Message) float64 {
	switch message.Role {
	case "tool":
		return 0.8
	case "assistant":
		return 0.6
	case "user":
		return 0.9
	default:
		return 0.4
	}
}

func toolPath(call tools.Call) string {
	path, _ := call.Argument["path"].(string)
	if path == "" {
		path, _ = call.Argument["file_path"].(string)
	}
	return strings.TrimSpace(path)
}

func safeToolArgs(call tools.Call) map[string]any {
	out := map[string]any{}
	for key, value := range call.Argument {
		lowered := strings.ToLower(key)
		if strings.Contains(lowered, "key") || strings.Contains(lowered, "token") || strings.Contains(lowered, "secret") || strings.Contains(lowered, "password") {
			out[key] = "[redacted]"
			continue
		}
		if text, ok := value.(string); ok && len(text) > 500 {
			out[key] = text[:500] + "... truncated ..."
			continue
		}
		out[key] = value
	}
	return out
}

func reflectionSummary(report agentloop.ReflectionReport) string {
	if report.StopRecommended {
		return "stop recommended: " + report.StopReason
	}
	if report.ReplanNeeded {
		return "replan needed: " + report.FailureReason
	}
	return fmt.Sprintf("progress %.2f confidence %.2f", report.ProgressDelta, report.Confidence)
}

func singleLine(text string, maxBytes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(text) <= maxBytes {
		return text
	}
	return text[:maxBytes] + "..."
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
