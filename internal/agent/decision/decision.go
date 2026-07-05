package decision

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"cli_mate/internal/agent/agentloop"
	"cli_mate/internal/agent/toolrouter"
)

type Engine struct {
	Router       toolrouter.Router
	ApprovalRisk agentloop.RiskLevel
}

func New(router toolrouter.Router) *Engine {
	return &Engine{Router: router, ApprovalRisk: agentloop.RiskDependencyChange}
}

func (e *Engine) Decide(ctx context.Context, state *agentloop.RunState, bundle agentloop.ContextBundle) (agentloop.ActionDecision, error) {
	task := state.ActiveTask()
	intent := intentForTask(task)
	opts := agentloop.RouteOptions{
		MaxCandidates: 5,
		AllowParallel: true,
		MaxRisk:       intent.MaxRisk,
	}

	var candidates []agentloop.CandidateAction
	var err error
	if e.Router != nil {
		candidates, err = e.Router.Select(ctx, intent, opts)
		if err != nil {
			return agentloop.ActionDecision{}, err
		}
	}

	knowEnough := knowsEnough(task, bundle)
	needContext := !knowEnough && canGatherContext(candidates)
	canParallel := canParallelize(candidates)
	scored, rejected := evaluateCandidates(state, bundle, candidates, knowEnough)
	selected := first(scored)
	cheapest := cheapestSafe(scored)
	reviewerNotes := reviewSelection(task, knowEnough, selected, cheapest, rejected)
	approvalRisk := e.ApprovalRisk
	if approvalRisk == 0 {
		approvalRisk = agentloop.RiskDependencyChange
	}
	requiresApproval := selected.Tool.Risk >= approvalRisk || taskRisk(task) >= approvalRisk

	return agentloop.ActionDecision{
		KnowEnough:         knowEnough,
		NeedMoreContext:    needContext,
		CandidateActions:   scored,
		SelectedAction:     selected,
		CanParallelize:     canParallel,
		CheapestSafeAction: cheapest,
		VerificationPlan:   verificationPlan(task, intent),
		RequiresApproval:   requiresApproval,
		Reason:             reason(knowEnough, needContext, selected, cheapest, canParallel, requiresApproval, reviewerNotes),
		InternalConfidence: selected.Evaluation.Confidence,
		ReviewerNotes:      reviewerNotes,
		RejectedActions:    rejected,
	}, nil
}

func intentForTask(task *agentloop.TaskNode) agentloop.ToolIntent {
	if task == nil {
		return agentloop.ToolIntent{
			Kind:       agentloop.ToolFilesystem,
			Capability: "inspect",
			MaxRisk:    agentloop.RiskReadOnly,
		}
	}
	goal := strings.ToLower(task.Goal)
	intent := agentloop.ToolIntent{
		Goal:         task.Goal,
		MaxRisk:      task.Risk,
		Verification: task.Verification,
	}
	switch {
	case strings.Contains(goal, "inspect"), strings.Contains(goal, "context"), strings.Contains(goal, "diagnostic"):
		intent.Kind = agentloop.ToolFilesystem
		intent.Capability = "inspect"
		intent.MaxRisk = agentloop.RiskReadOnly
	case strings.Contains(goal, "verify"), strings.Contains(goal, "test"), strings.Contains(goal, "build"), strings.Contains(goal, "format"):
		intent.Kind = agentloop.ToolTerminal
		intent.Capability = "test"
		intent.MaxRisk = agentloop.RiskCommand
	case strings.Contains(goal, "edit"), strings.Contains(goal, "apply"), strings.Contains(goal, "implement"), strings.Contains(goal, "change"):
		intent.Kind = agentloop.ToolFilesystem
		intent.Capability = "edit"
		intent.RequiresMutation = true
		intent.MaxRisk = agentloop.RiskLocalEdit
	default:
		intent.Kind = agentloop.ToolFilesystem
		intent.Capability = "inspect"
		intent.MaxRisk = agentloop.RiskReadOnly
	}
	return intent
}

func knowsEnough(task *agentloop.TaskNode, bundle agentloop.ContextBundle) bool {
	if task == nil {
		return false
	}
	goal := strings.ToLower(task.Goal)
	if strings.Contains(goal, "summarize") {
		return len(bundle.RecentEvents) > 0
	}
	if strings.Contains(goal, "verify") {
		return len(task.Verification.Commands) > 0 || len(task.Verification.FileChecks) > 0
	}
	for _, item := range bundle.Items {
		if item.Exact || item.Score >= 1 {
			return true
		}
	}
	return len(bundle.Memories) > 0 && task.Risk == agentloop.RiskReadOnly
}

func canGatherContext(candidates []agentloop.CandidateAction) bool {
	for _, candidate := range candidates {
		if !candidate.Tool.Mutates && candidate.Tool.Risk <= agentloop.RiskReadOnly {
			return true
		}
	}
	return false
}

func canParallelize(candidates []agentloop.CandidateAction) bool {
	parallel := 0
	for _, candidate := range candidates {
		if candidate.Tool.SupportsParallel && !candidate.Tool.Mutates {
			parallel++
		}
	}
	return parallel > 1
}

func cheapestSafe(candidates []agentloop.CandidateAction) agentloop.CandidateAction {
	var best agentloop.CandidateAction
	ok := false
	for _, candidate := range candidates {
		if candidate.Tool.Mutates || candidate.Tool.Risk > agentloop.RiskCommand {
			continue
		}
		if !ok || candidate.Tool.Cost.Tokens < best.Tool.Cost.Tokens {
			best = candidate
			ok = true
		}
	}
	return best
}

func first(candidates []agentloop.CandidateAction) agentloop.CandidateAction {
	if len(candidates) == 0 {
		return agentloop.CandidateAction{}
	}
	return candidates[0]
}

func evaluateCandidates(state *agentloop.RunState, bundle agentloop.ContextBundle, candidates []agentloop.CandidateAction, knowEnough bool) ([]agentloop.CandidateAction, []agentloop.CandidateAction) {
	if len(candidates) == 0 {
		return nil, nil
	}
	stats := toolStats(state.Events)
	scored := make([]agentloop.CandidateAction, 0, len(candidates))
	rejected := make([]agentloop.CandidateAction, 0)
	for _, candidate := range candidates {
		evaluation := evaluateCandidate(state, bundle, stats, candidate, knowEnough)
		candidate.Evaluation = evaluation
		candidate.Score = compositeScore(candidate.Score, evaluation)
		candidate.Reason = enrichReason(candidate.Reason, evaluation)
		if evaluation.Rejected {
			rejected = append(rejected, candidate)
			continue
		}
		scored = append(scored, candidate)
	}
	sortCandidates(scored)
	sortCandidates(rejected)
	if len(scored) == 0 {
		for _, candidate := range rejected {
			if !candidate.Tool.Mutates && candidate.Tool.Risk <= agentloop.RiskReadOnly {
				candidate.Evaluation.Rejected = false
				candidate.Evaluation.Review = "accepted as the least risky recovery option after reviewer rejection"
				scored = append(scored, candidate)
				break
			}
		}
	}
	return scored, rejected
}

func evaluateCandidate(state *agentloop.RunState, bundle agentloop.ContextBundle, stats map[string]toolHistory, candidate agentloop.CandidateAction, knowEnough bool) agentloop.ActionEvaluation {
	history := stats[candidate.Tool.Name]
	failureProbability := 0.15
	if history.Total > 0 {
		failureProbability = clamp(float64(history.Failures)/float64(history.Total), 0.05, 0.9)
	}
	if history.RecentSameFailure >= 2 {
		failureProbability = clamp(failureProbability+0.35, 0, 0.95)
	}
	contextConfidence := 0.25
	if knowEnough {
		contextConfidence += 0.35
	}
	if len(bundle.Memories) > 0 {
		contextConfidence += 0.1
	}
	for _, item := range bundle.Items {
		if item.Exact || item.Score >= 1 {
			contextConfidence += 0.1
			break
		}
	}
	if state != nil {
		contextConfidence += state.Confidence * 0.25
	}

	riskCost := float64(candidate.Tool.Risk) / float64(agentloop.RiskCredentialed+1)
	tokenCost := clamp(float64(candidate.Tool.Cost.Tokens)/2500, 0, 1)
	latency := candidate.Tool.Cost.Latency
	if latency <= 0 {
		latency = 250 * time.Millisecond
	}
	latencyCost := clamp(float64(latency)/float64(10*time.Second), 0, 1)
	executionCost := clamp((tokenCost+latencyCost+riskCost)/3, 0, 1)
	verificationAbility := verificationAbility(candidate)
	expectedValue := clamp((candidate.Score/10)+verificationAbility+(contextConfidence*0.4)-(failureProbability*0.5)-(riskCost*0.35), 0, 1)
	confidence := clamp(contextConfidence+(expectedValue*0.25)-(failureProbability*0.25)-(riskCost*0.15), 0, 1)

	rejected := false
	review := "passes self-critic"
	if candidate.Tool.Mutates && !knowEnough {
		rejected = true
		review = "rejected: mutation before enough evidence"
	}
	if history.RecentSameFailure >= 2 && verificationAbility < 0.3 {
		rejected = true
		review = "rejected: repeated failure without a stronger verification path"
	}
	if confidence < 0.18 && candidate.Tool.Risk > agentloop.RiskReadOnly {
		rejected = true
		review = "rejected: low confidence for non-readonly action"
	}

	return agentloop.ActionEvaluation{
		ExpectedValue:       expectedValue,
		Confidence:          confidence,
		Risk:                riskCost,
		LatencyCost:         latencyCost,
		TokenCost:           tokenCost,
		ExecutionCost:       executionCost,
		FailureProbability:  failureProbability,
		VerificationAbility: verificationAbility,
		Rejected:            rejected,
		Review:              review,
	}
}

func compositeScore(routerScore float64, evaluation agentloop.ActionEvaluation) float64 {
	score := routerScore
	score += evaluation.ExpectedValue * 4
	score += evaluation.Confidence * 2
	score += evaluation.VerificationAbility
	score -= evaluation.Risk * 1.5
	score -= evaluation.TokenCost
	score -= evaluation.LatencyCost
	score -= evaluation.FailureProbability * 2
	if evaluation.Rejected {
		score -= 100
	}
	return score
}

func enrichReason(reason string, evaluation agentloop.ActionEvaluation) string {
	parts := []string{strings.TrimSpace(reason)}
	parts = append(parts,
		fmt.Sprintf("ev=%.2f", evaluation.ExpectedValue),
		fmt.Sprintf("confidence=%.2f", evaluation.Confidence),
		fmt.Sprintf("risk=%.2f", evaluation.Risk),
		fmt.Sprintf("failure=%.2f", evaluation.FailureProbability),
		fmt.Sprintf("verify=%.2f", evaluation.VerificationAbility),
	)
	if evaluation.Review != "" {
		parts = append(parts, evaluation.Review)
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ", ")
}

func sortCandidates(candidates []agentloop.CandidateAction) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if nearlyEqual(candidates[i].Score, candidates[j].Score) {
			return candidates[i].Tool.Cost.Tokens < candidates[j].Tool.Cost.Tokens
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func verificationAbility(candidate agentloop.CandidateAction) float64 {
	if len(candidate.Intent.Verification.Commands)+len(candidate.Intent.Verification.FileChecks)+len(candidate.Intent.Verification.DiffChecks) > 0 {
		return 0.4
	}
	if candidate.Tool.Kind == agentloop.ToolTerminal && hasCapability(candidate.Tool, "test") {
		return 0.45
	}
	if !candidate.Tool.Mutates && candidate.Tool.Risk == agentloop.RiskReadOnly {
		return 0.25
	}
	if candidate.Tool.Mutates {
		return 0.2
	}
	return 0.1
}

func hasCapability(desc agentloop.ToolDescriptor, capability string) bool {
	capability = strings.ToLower(capability)
	for _, item := range desc.Capabilities {
		item = strings.ToLower(item)
		if strings.Contains(item, capability) || strings.Contains(capability, item) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(desc.Definition.Description), capability)
}

type toolHistory struct {
	Total             int
	Failures          int
	RecentSameFailure int
}

func toolStats(events []agentloop.Event) map[string]toolHistory {
	stats := map[string]toolHistory{}
	lastFailureByTool := map[string]string{}
	for _, event := range events {
		if event.Type != agentloop.EventToolCompleted && event.Type != agentloop.EventToolFinished {
			continue
		}
		name := strings.Fields(event.Summary)
		if len(name) == 0 {
			continue
		}
		key := name[0]
		history := stats[key]
		history.Total++
		if strings.Contains(strings.ToLower(event.Summary), "failed") {
			history.Failures++
			sig := signature(event.Summary)
			if lastFailureByTool[key] == sig {
				history.RecentSameFailure++
			} else {
				history.RecentSameFailure = 1
			}
			lastFailureByTool[key] = sig
		} else {
			history.RecentSameFailure = 0
			lastFailureByTool[key] = ""
		}
		stats[key] = history
	}
	return stats
}

func reviewSelection(task *agentloop.TaskNode, knowEnough bool, selected, cheapest agentloop.CandidateAction, rejected []agentloop.CandidateAction) []string {
	var notes []string
	if selected.Tool.Name == "" {
		return []string{"no acceptable tool candidate; ask the model to reason from context or gather safer evidence"}
	}
	if selected.Tool.Mutates && !knowEnough {
		notes = append(notes, "self-critic: mutation needs more evidence first")
	}
	if cheapest.Tool.Name != "" && cheapest.Tool.Name != selected.Tool.Name && cheapest.Score >= selected.Score-0.75 {
		notes = append(notes, "self-critic: cheaper safe alternative is close in value: "+cheapest.Tool.Name)
	}
	if selected.Evaluation.VerificationAbility < 0.2 && task != nil && len(task.Verification.Commands) > 0 {
		notes = append(notes, "self-critic: selected action has weak verification leverage for a task with required checks")
	}
	if len(rejected) > 0 {
		notes = append(notes, fmt.Sprintf("self-critic rejected %d weaker candidate(s)", len(rejected)))
	}
	if len(notes) == 0 {
		notes = append(notes, "self-critic accepted the selected action as the simplest safe step")
	}
	return notes
}

func taskRisk(task *agentloop.TaskNode) agentloop.RiskLevel {
	if task == nil {
		return agentloop.RiskReadOnly
	}
	return task.Risk
}

func verificationPlan(task *agentloop.TaskNode, intent agentloop.ToolIntent) agentloop.VerificationSpec {
	if task != nil && len(task.Verification.Commands)+len(task.Verification.FileChecks)+len(task.Verification.DiffChecks) > 0 {
		return task.Verification
	}
	return intent.Verification
}

func reason(knowEnough, needContext bool, selected, cheapest agentloop.CandidateAction, parallel, approval bool, reviewerNotes []string) string {
	parts := []string{}
	if knowEnough {
		parts = append(parts, "known context is sufficient for the active task")
	} else {
		parts = append(parts, "more evidence is needed before high-confidence action")
	}
	if needContext {
		parts = append(parts, "readonly context gathering is available")
	}
	if selected.Tool.Name != "" {
		parts = append(parts, "selected "+selected.Tool.Name+" because "+selected.Reason)
		parts = append(parts, fmt.Sprintf("selected score %.2f beats alternatives by expected value, confidence, risk, cost, failure probability, and verification ability", selected.Score))
	}
	if cheapest.Tool.Name != "" && cheapest.Tool.Name != selected.Tool.Name {
		parts = append(parts, "cheapest safe alternative is "+cheapest.Tool.Name)
	}
	if parallel {
		parts = append(parts, "independent readonly tools can run in parallel")
	}
	if approval {
		parts = append(parts, "approval is required by risk policy")
	}
	parts = append(parts, reviewerNotes...)
	return strings.Join(parts, "; ")
}

func signature(text string) string {
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	if len(text) > 120 {
		return text[:120]
	}
	return text
}

func clamp(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}
