package decision

import (
	"fmt"
	"strings"

	"cli_mate/internal/agent/agentloop"
)

// HypothesisEngine generates, scores, and updates competing hypotheses about
// the current problem. It is the core of the Hypothesis-Driven Reasoning Engine.
type HypothesisEngine struct{}

func NewHypothesisEngine() *HypothesisEngine {
	return &HypothesisEngine{}
}

// Generate creates 2-6 competing hypotheses from the context bundle and task goal.
// Each hypothesis explains the current problem from a different angle.
func (he *HypothesisEngine) Generate(task *agentloop.TaskNode, bundle agentloop.ContextBundle) []agentloop.Hypothesis {
	if task == nil || strings.TrimSpace(task.Goal) == "" {
		return nil
	}

	goal := strings.ToLower(task.Goal)
	knownFacts := extractKnownFacts(bundle)
	unknownFacts := extractUnknownFacts(bundle)

	// Generate between 2-6 hypotheses based on goal type and context richness.
	hypotheses := he.generateForGoal(goal, knownFacts, unknownFacts)

	// Score each hypothesis
	for i := range hypotheses {
		h := &hypotheses[i]
		h.ID = fmt.Sprintf("hyp-%d", i+1)
		h.InformationGain = heuristicInfoGain(h, unknownFacts)
		h.Confidence = clamp(h.Probability*0.6+h.InformationGain*0.4, 0, 1)
		if h.State == "" {
			h.State = agentloop.HypothesisUnknown
		}
	}

	return hypotheses
}

// generateForGoal returns competing hypotheses based on the task goal.
func (he *HypothesisEngine) generateForGoal(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	var hypotheses []agentloop.Hypothesis

	switch {
	case strings.Contains(goal, "fix"), strings.Contains(goal, "bug"), strings.Contains(goal, "error"), strings.Contains(goal, "fail"):
		hypotheses = he.bugHypotheses(goal, knownFacts, unknownFacts)
	case strings.Contains(goal, "implement"), strings.Contains(goal, "add"), strings.Contains(goal, "feature"):
		hypotheses = he.featureHypotheses(goal, knownFacts, unknownFacts)
	case strings.Contains(goal, "inspect"), strings.Contains(goal, "understand"), strings.Contains(goal, "diagnostic"):
		hypotheses = he.inspectHypotheses(goal, knownFacts, unknownFacts)
	case strings.Contains(goal, "refactor"), strings.Contains(goal, "change"), strings.Contains(goal, "edit"):
		hypotheses = he.changeHypotheses(goal, knownFacts, unknownFacts)
	default:
		hypotheses = he.generalHypotheses(goal, knownFacts, unknownFacts)
	}

	// Ensure at least 2 hypotheses and at most 6
	if len(hypotheses) < 2 {
		hypotheses = append(hypotheses, agentloop.Hypothesis{
			Description:        "The current understanding is incomplete; more context is needed before forming a specific hypothesis",
			Probability:        0.5,
			VerificationCost:   0.1,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "gather more context with read-only tools",
		})
	}
	if len(hypotheses) > 6 {
		hypotheses = hypotheses[:6]
	}

	return hypotheses
}

func (he *HypothesisEngine) bugHypotheses(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	return []agentloop.Hypothesis{
		{
			Description:        "A logic or runtime error in the application code is causing the failure",
			Probability:        0.35,
			VerificationCost:   0.3,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "inspect relevant source files and error stack traces",
			SupportingEvidence: knownFacts,
		},
		{
			Description:        "A configuration or environment issue is causing the failure",
			Probability:        0.25,
			VerificationCost:   0.2,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "inspect configuration files, environment variables, and dependencies",
		},
		{
			Description:        "A type or compilation error in the relevant code",
			Probability:        0.20,
			VerificationCost:   0.15,
			Risk:               agentloop.RiskCommand,
			VerificationMethod: "run the build or type checker on affected packages",
		},
		{
			Description:        "A dependency or integration issue with external libraries or services",
			Probability:        0.20,
			VerificationCost:   0.35,
			Risk:               agentloop.RiskNetwork,
			VerificationMethod: "check dependency versions, import paths, and API contracts",
		},
	}
}

func (he *HypothesisEngine) featureHypotheses(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	return []agentloop.Hypothesis{
		{
			Description:        "The feature can be implemented by extending existing patterns and structures in the codebase",
			Probability:        0.40,
			VerificationCost:   0.25,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "inspect existing similar features and their patterns",
		},
		{
			Description:        "The feature requires new abstractions, types, or modules to be created",
			Probability:        0.30,
			VerificationCost:   0.35,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "review architecture and determine if existing patterns fit",
		},
		{
			Description:        "The feature is partially already implemented or can use an existing library",
			Probability:        0.30,
			VerificationCost:   0.2,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "search codebase and dependencies for related functionality",
		},
	}
}

func (he *HypothesisEngine) inspectHypotheses(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	return []agentloop.Hypothesis{
		{
			Description:        "The relevant information is in the source files related to the goal keywords",
			Probability:        0.45,
			VerificationCost:   0.15,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "search and read source files matching the goal",
		},
		{
			Description:        "The relevant information is in configuration, documentation, or build files",
			Probability:        0.30,
			VerificationCost:   0.1,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "read configuration, docs, and project metadata files",
		},
		{
			Description:        "The relevant information requires running commands or tests to discover",
			Probability:        0.25,
			VerificationCost:   0.25,
			Risk:               agentloop.RiskCommand,
			VerificationMethod: "run diagnostic commands, tests, or build checks",
		},
	}
}

func (he *HypothesisEngine) changeHypotheses(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	return []agentloop.Hypothesis{
		{
			Description:        "The change can be applied through small, localized edits to existing code",
			Probability:        0.40,
			VerificationCost:   0.2,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "inspect the specific files that need changes",
		},
		{
			Description:        "The change requires understanding broader architecture before editing",
			Probability:        0.35,
			VerificationCost:   0.3,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "read related files and understand module interactions",
		},
		{
			Description:        "The change may have side effects on other parts of the codebase",
			Probability:        0.25,
			VerificationCost:   0.35,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "search for usages of affected symbols and test coverage",
		},
	}
}

func (he *HypothesisEngine) generalHypotheses(goal string, knownFacts, unknownFacts []string) []agentloop.Hypothesis {
	return []agentloop.Hypothesis{
		{
			Description:        "The goal can be achieved by first inspecting relevant files to build understanding",
			Probability:        0.50,
			VerificationCost:   0.15,
			Risk:               agentloop.RiskReadOnly,
			VerificationMethod: "read files matching the goal context",
		},
		{
			Description:        "The goal requires running tools or commands to gather diagnostics first",
			Probability:        0.30,
			VerificationCost:   0.2,
			Risk:               agentloop.RiskCommand,
			VerificationMethod: "run diagnostic or build commands",
		},
		{
			Description:        "The goal is straightforward and can be completed with direct action",
			Probability:        0.20,
			VerificationCost:   0.1,
			Risk:               agentloop.RiskLocalEdit,
			VerificationMethod: "apply changes directly and verify",
		},
	}
}

// EstimateInformationGainForTool predicts how much a candidate tool action
// will reduce hypothesis uncertainty. Returns 0-1 value.
func EstimateInformationGainForTool(candidate agentloop.CandidateAction, hypotheses []agentloop.Hypothesis) float64 {
	if len(hypotheses) == 0 {
		return 0
	}

	readTool := !candidate.Tool.Mutates && candidate.Tool.Risk <= agentloop.RiskReadOnly
	testTool := candidate.Tool.Kind == agentloop.ToolTerminal
	mutateTool := candidate.Tool.Mutates

	totalGain := 0.0
	for _, h := range hypotheses {
		if h.State == agentloop.HypothesisConfirmed || h.State == agentloop.HypothesisRejected {
			continue // already resolved
		}
		gain := hypothesisGainForTool(h, readTool, testTool, mutateTool)
		totalGain += gain * h.Probability
	}

	avgGain := totalGain / float64(len(hypotheses))
	return clamp(avgGain, 0, 1)
}

// hypothesisGainForTool estimates how much a single tool call resolves
// a specific hypothesis.
func hypothesisGainForTool(h agentloop.Hypothesis, readTool, testTool, mutateTool bool) float64 {
	method := strings.ToLower(h.VerificationMethod)

	// If the tool matches the verification method, high gain
	if readTool && (strings.Contains(method, "inspect") || strings.Contains(method, "read") || strings.Contains(method, "search")) {
		return 0.7
	}
	if testTool && (strings.Contains(method, "build") || strings.Contains(method, "test") || strings.Contains(method, "run")) {
		return 0.6
	}
	if mutateTool && (strings.Contains(method, "apply") || strings.Contains(method, "edit")) {
		return 0.3
	}

	// Generic estimation
	if readTool {
		return 0.4
	}
	if testTool {
		return 0.35
	}
	return 0.15
}

// UpdateHypothesesAfterTool updates each hypothesis based on a tool result.
// Returns updated hypotheses and number of hypotheses resolved.
func UpdateHypothesesAfterTool(hypotheses []agentloop.Hypothesis, callName string, resultContent, resultError string) ([]agentloop.Hypothesis, int) {
	resolved := 0
	content := strings.ToLower(resultContent)
	errMsg := strings.ToLower(resultError)
	hasError := resultError != ""

	updated := make([]agentloop.Hypothesis, len(hypotheses))
	copy(updated, hypotheses)

	for i, h := range updated {
		if h.State == agentloop.HypothesisConfirmed || h.State == agentloop.HypothesisRejected {
			continue
		}
		updated[i] = updateSingleHypothesis(h, content, errMsg, hasError)
		if updated[i].State == agentloop.HypothesisConfirmed || updated[i].State == agentloop.HypothesisRejected {
			resolved++
		}
	}

	return updated, resolved
}

func updateSingleHypothesis(h agentloop.Hypothesis, content, errMsg string, hasError bool) agentloop.Hypothesis {
	method := strings.ToLower(h.VerificationMethod)
	methodWords := strings.Fields(method)
	isRelevant := hasError
	if len(methodWords) > 0 {
		isRelevant = isRelevant || strings.Contains(content, methodWords[0])
	}

	if !isRelevant {
		// No new evidence; slightly weaken if probability was high
		if h.Probability > 0.7 {
			h.Probability = clamp(h.Probability-0.05, 0, 1)
			h.State = agentloop.HypothesisWeakened
		}
		return h
	}

	if hasError {
		// Tool failed — hypothesis is weakened unless the error confirms it
		h.ContradictingEvidence = append(h.ContradictingEvidence, "tool error: "+errMsg)
		h.Probability = clamp(h.Probability-0.15, 0, 1)
		h.State = agentloop.HypothesisWeakened
		return h
	}

	// Tool succeeded and content is relevant — hypothesis is strengthened
	h.SupportingEvidence = append(h.SupportingEvidence, "evidence from tool output")
	h.Probability = clamp(h.Probability+0.2, 0, 1)
	h.State = agentloop.HypothesisStrengthened

	if h.Probability >= 0.85 {
		h.State = agentloop.HypothesisConfirmed
	}
	return h
}

// RemainingUncertainty computes how much uncertainty remains across hypotheses.
// Returns 0-1 where 0 = fully resolved, 1 = completely uncertain.
func RemainingUncertainty(hypotheses []agentloop.Hypothesis) float64 {
	if len(hypotheses) == 0 {
		return 1
	}
	resolved := 0
	totalProb := 0.0
	for _, h := range hypotheses {
		if h.State == agentloop.HypothesisConfirmed || h.State == agentloop.HypothesisRejected {
			resolved++
		}
		totalProb += h.Probability
	}
	resolution := float64(resolved) / float64(len(hypotheses))
	avgProb := totalProb / float64(len(hypotheses))
	// Uncertainty is high when few hypotheses are resolved and probabilities are spread
	uncertainty := 1.0 - (resolution*0.6 + avgProb*0.4)
	return clamp(uncertainty, 0, 1)
}

// PrimaryHypothesis returns the hypothesis with the highest probability.
func PrimaryHypothesis(hypotheses []agentloop.Hypothesis) *agentloop.Hypothesis {
	if len(hypotheses) == 0 {
		return nil
	}
	best := &hypotheses[0]
	for i := range hypotheses[1:] {
		if hypotheses[i+1].Probability > best.Probability {
			best = &hypotheses[i+1]
		}
	}
	return best
}

// heuristicInfoGain estimates how much information a hypothesis provides
// based on its probability, how many unknowns it addresses, and its verification cost.
func heuristicInfoGain(h *agentloop.Hypothesis, unknownFacts []string) float64 {
	if h == nil {
		return 0
	}
	baseGain := h.Probability * 0.5
	// Hypotheses that are cheap to verify have higher info gain
	costBonus := (1 - h.VerificationCost) * 0.3
	// Hypotheses addressing more unknowns have higher gain
	unknownBonus := 0.0
	if len(unknownFacts) > 0 {
		unknownBonus = clamp(float64(len(unknownFacts))*0.05, 0, 0.2)
	}
	return clamp(baseGain+costBonus+unknownBonus, 0, 1)
}

// extractKnownFacts pulls known facts from the context bundle.
func extractKnownFacts(bundle agentloop.ContextBundle) []string {
	var facts []string
	for _, item := range bundle.Items {
		if item.Score >= 0.8 || item.Exact {
			summary := strings.TrimSpace(item.Content)
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			if summary != "" {
				facts = append(facts, item.Kind+": "+summary)
			}
		}
	}
	if len(facts) == 0 {
		facts = append(facts, "no high-confidence facts available yet")
	}
	return facts
}

// extractUnknownFacts identifies what is still unknown based on goal and context.
func extractUnknownFacts(bundle agentloop.ContextBundle) []string {
	var unknowns []string
	// If there's no high-confidence context, everything is unknown
	hasGoodContext := false
	for _, item := range bundle.Items {
		if item.Score >= 0.8 || item.Exact {
			hasGoodContext = true
			break
		}
	}
	if !hasGoodContext {
		unknowns = append(unknowns, "no high-confidence context collected yet")
		unknowns = append(unknowns, "the current state of relevant files is unknown")
		unknowns = append(unknowns, "which files are affected by this task is unknown")
	}
	return unknowns
}
