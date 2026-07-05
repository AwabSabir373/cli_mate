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
	Hypotheses   *HypothesisEngine
}

func New(router toolrouter.Router) *Engine {
	return &Engine{Router: router, ApprovalRisk: agentloop.RiskDependencyChange, Hypotheses: NewHypothesisEngine()}
}

func (e *Engine) Decide(ctx context.Context, state *agentloop.RunState, bundle agentloop.ContextBundle) (agentloop.ActionDecision, error) {
	task := state.ActiveTask()
	intent := intentForTask(task)
	opts := agentloop.RouteOptions{
		MaxCandidates: 5,
		AllowParallel: true,
		MaxRisk:       intent.MaxRisk,
	}

	// Stage 1: Generate competing hypotheses before selecting tools.
	activeHypotheses := e.Hypotheses.Generate(task, bundle)

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

	// Stage 2: Evaluate candidates with hypothesis-derived information gain.
	scored, rejected := evaluateCandidatesWithHypotheses(state, bundle, candidates, activeHypotheses, knowEnough)
	selected := first(scored)
	cheapest := cheapestSafe(scored)
	recovery := recoveryActions(scored, rejected, selected)

	// Stage 3: Mutation rule — reject mutation while multiple high-probability hypotheses remain unresolved.
	remainingUncertainty := RemainingUncertainty(activeHypotheses)
	primaryHyp := PrimaryHypothesis(activeHypotheses)
	infoGain := 0.0
	if selected.Tool.Name != "" {
		infoGain = EstimateInformationGainForTool(selected, activeHypotheses)
	}

	// Enforce mutation rule when uncertainty is high (unless user explicitly requires mutation)
	if selected.Tool.Mutates && !intent.RequiresMutation && remainingUncertainty > 0.35 && primaryHyp != nil && primaryHyp.Probability < 0.7 {
		rejected = append(rejected, selected)
		// Try to find a read-only inspection tool from remaining scored candidates
		foundNonMutating := false
		for _, c := range scored[1:] {
			if !c.Tool.Mutates && c.Tool.Risk <= agentloop.RiskReadOnly {
				selected = c
				foundNonMutating = true
				break
			}
		}
		// Fallback: prefer cheapest safe tool
		if !foundNonMutating && cheapest.Tool.Name != "" {
			selected = cheapest
			foundNonMutating = true
		}
		// Last resort: search rejected or recovery for any read-only action
		if !foundNonMutating {
			for _, c := range rejected {
				if !c.Tool.Mutates && c.Tool.Risk <= agentloop.RiskReadOnly {
					selected = c
					foundNonMutating = true
					break
				}
			}
		}
		if !foundNonMutating && len(recovery) > 0 {
			for _, c := range recovery {
				if !c.Tool.Mutates && c.Tool.Risk <= agentloop.RiskReadOnly {
					selected = c
					break
				}
			}
		}
	}

	reviewerNotes := selfCritic(task, knowEnough, selected, cheapest, rejected, recovery)
	approvalRisk := e.ApprovalRisk
	if approvalRisk == 0 {
		approvalRisk = agentloop.RiskDependencyChange
	}
	requiresApproval := selected.Tool.Risk >= approvalRisk || taskRisk(task) >= approvalRisk

	// Stage 4: Build hypothesis-informed decision.
	var rejectedHyps []agentloop.Hypothesis
	for _, h := range activeHypotheses {
		if h.State == agentloop.HypothesisRejected {
			rejectedHyps = append(rejectedHyps, h)
		}
	}
	confidenceDelta := infoGain * (1 - remainingUncertainty)

	return agentloop.ActionDecision{
		KnowEnough:              knowEnough,
		NeedMoreContext:         needContext,
		CandidateActions:        scored,
		SelectedAction:          selected,
		CanParallelize:          canParallel,
		CheapestSafeAction:      cheapest,
		RecoveryActions:         recovery,
		ActiveHypotheses:        activeHypotheses,
		PrimaryHypothesis:       primaryHyp,
		RejectedHypotheses:      rejectedHyps,
		InformationGainEstimate: infoGain,
		RemainingUncertainty:    remainingUncertainty,
		VerificationReason:      verificationReason(selected, activeHypotheses),
		ConfidenceDeltaPrediction: confidenceDelta,
		VerificationPlan:        verificationPlan(task, intent),
		RequiresApproval:        requiresApproval,
		Reason:                  reason(knowEnough, needContext, selected, cheapest, recovery, canParallel, requiresApproval, reviewerNotes),
		InternalConfidence:      selected.Evaluation.Confidence,
		ReviewerNotes:           reviewerNotes,
		RejectedActions:         rejected,
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

// recoveryActions selects 1-2 fallback actions for the recovery plan.
// It picks safe, reversible alternatives that scored well but weren't selected.
func recoveryActions(scored, rejected []agentloop.CandidateAction, selected agentloop.CandidateAction) []agentloop.CandidateAction {
	var recovery []agentloop.CandidateAction
	seen := map[string]bool{}
	if selected.Tool.Name != "" {
		seen[selected.Tool.Name] = true
	}

	// First pass: pick from scored candidates (excluding selected)
	for _, candidate := range scored {
		if len(recovery) >= 2 {
			break
		}
		if seen[candidate.Tool.Name] {
			continue
		}
		if candidate.Tool.Mutates && candidate.Tool.Risk > agentloop.RiskLocalEdit {
			continue
		}
		seen[candidate.Tool.Name] = true
		recovery = append(recovery, candidate)
	}

	// Second pass: pick safe read-only actions from rejected as last resort
	if len(recovery) == 0 {
		for _, candidate := range rejected {
			if len(recovery) >= 1 {
				break
			}
			if seen[candidate.Tool.Name] {
				continue
			}
			if !candidate.Tool.Mutates && candidate.Tool.Risk <= agentloop.RiskReadOnly {
				seen[candidate.Tool.Name] = true
				recovery = append(recovery, candidate)
			}
		}
	}

	return recovery
}

func evaluateCandidatesWithHypotheses(state *agentloop.RunState, bundle agentloop.ContextBundle, candidates []agentloop.CandidateAction, hypotheses []agentloop.Hypothesis, knowEnough bool) ([]agentloop.CandidateAction, []agentloop.CandidateAction) {
	if len(candidates) == 0 {
		return nil, nil
	}
	// Nil guard for safe nil-state access.
	var events []agentloop.Event
	if state != nil {
		events = state.Events
	}
	stats := toolStats(events)
	scored := make([]agentloop.CandidateAction, 0, len(candidates))
	rejected := make([]agentloop.CandidateAction, 0)
	for _, candidate := range candidates {
		evaluation := evaluateCandidate(state, bundle, stats, candidate, knowEnough)
		// Augment with hypothesis-driven information gain
		if len(hypotheses) > 0 {
			infoGain := EstimateInformationGainForTool(candidate, hypotheses)
			evaluation.InformationGain = infoGain
		}
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

	// Reversibility bonus: reversible tools are safer and should be preferred
	reversibilityBonus := 0.0
	if candidate.Tool.Reversible {
		reversibilityBonus = 0.12
	}

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
		ReversibilityBonus:  reversibilityBonus,
	}
}

func compositeScore(routerScore float64, evaluation agentloop.ActionEvaluation) float64 {
	score := routerScore
	score += evaluation.ExpectedValue * 4
	score += evaluation.Confidence * 2
	score += evaluation.VerificationAbility
	score += evaluation.InformationGain * 5  // Highest priority: information gain
	score += evaluation.ReversibilityBonus * 3
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
	if evaluation.InformationGain > 0 {
		parts = append(parts, fmt.Sprintf("info=%.2f", evaluation.InformationGain))
	}
	if evaluation.ReversibilityBonus > 0 {
		parts = append(parts, fmt.Sprintf("reversible=%.2f", evaluation.ReversibilityBonus))
	}
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

// selfCritic implements the DIE 7-question structured self-critic.
// Each question produces a note if the condition is detected.
func selfCritic(task *agentloop.TaskNode, knowEnough bool, selected, cheapest agentloop.CandidateAction, rejected []agentloop.CandidateAction, recovery []agentloop.CandidateAction) []string {
	var notes []string

	if selected.Tool.Name == "" {
		return []string{"self-critic: no acceptable tool candidate; ask the model to reason from context or gather safer evidence"}
	}

	// Q1: Is this premature?
	if selected.Tool.Mutates && !knowEnough {
		notes = append(notes, "self-critic Q1: action may be premature without sufficient evidence")
	}

	// Q2: Am I guessing?
	if selected.Evaluation.Confidence < 0.3 && selected.Tool.Risk > agentloop.RiskReadOnly {
		notes = append(notes, "self-critic Q2: scoring relies on inference, not confirmed evidence (confidence < 0.3)")
	}

	// Q3: Am I skipping inspection?
	// Build a combined candidate set from rejected, selected, and recovery to check all alternatives.
	combined := make([]agentloop.CandidateAction, 0, len(rejected)+len(recovery)+1)
	combined = append(combined, rejected...)
	combined = append(combined, recovery...)
	if selected.Tool.Name != "" {
		combined = append(combined, selected)
	}
	if selected.Tool.Mutates && canGatherContext(combined) {
		notes = append(notes, "self-critic Q3: cheaper inspection alternatives available before mutation")
	}

	// Q4: Is there a cheaper action?
	if cheapest.Tool.Name != "" && cheapest.Tool.Name != selected.Tool.Name && cheapest.Score >= selected.Score-0.75 {
		notes = append(notes, "self-critic Q4: cheaper safe alternative is close in value: "+cheapest.Tool.Name)
	}

	// Q5: Is there a safer action?
	if len(recovery) > 0 && recovery[0].Tool.Name != selected.Tool.Name && recovery[0].Tool.Risk < selected.Tool.Risk && recovery[0].Score >= selected.Score-1.0 {
		notes = append(notes, "self-critic Q5: safer alternative with similar value available: "+recovery[0].Tool.Name)
	}

	// Q6: Would an expert engineer do this?
	if selected.Tool.Mutates && (task == nil || task.Confidence < 0.4) {
		notes = append(notes, "self-critic Q6: expert would confirm key assumptions before mutating (task confidence < 0.4)")
	}

	// Q7: Would this scale?
	if selected.Tool.Risk >= agentloop.RiskNetwork && !selected.Tool.Reversible {
		notes = append(notes, "self-critic Q7: narrow, contextual approach preferred over broad irreversible action")
	}

	if len(notes) == 0 {
		notes = append(notes, "self-critic: all 7 checks passed — the selected action is the simplest safe step")
	}
	return notes
}

func verificationReason(selected agentloop.CandidateAction, hypotheses []agentloop.Hypothesis) string {
	if len(hypotheses) == 0 {
		return "no hypotheses to verify"
	}
	var b strings.Builder
	hypCount := 0
	for _, h := range hypotheses {
		if h.State != agentloop.HypothesisConfirmed && h.State != agentloop.HypothesisRejected {
			hypCount++
		}
	}
	fmt.Fprintf(&b, "%d/%d unresolved hypotheses remain. ", hypCount, len(hypotheses))
	gain := EstimateInformationGainForTool(selected, hypotheses)
	fmt.Fprintf(&b, "Selected action expected info gain: %.2f. ", gain)
	if selected.Tool.Mutates && hypCount > 1 {
		b.WriteString("Mutation with active unresolved hypotheses — verify before finalizing.")
	} else if !selected.Tool.Mutates {
		b.WriteString("Inspection action that helps resolve hypotheses.")
	}
	return b.String()
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

func reason(knowEnough, needContext bool, selected, cheapest agentloop.CandidateAction, recovery []agentloop.CandidateAction, parallel, approval bool, reviewerNotes []string) string {
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
	if len(recovery) > 0 {
		names := make([]string, len(recovery))
		for i, r := range recovery {
			names[i] = r.Tool.Name
		}
		parts = append(parts, "recovery plan if selected action fails: "+strings.Join(names, ", "))
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
