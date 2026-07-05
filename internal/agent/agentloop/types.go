package agentloop

import (
	"strconv"
	"sync/atomic"
	"time"

	"cli_mate/internal/tools"
)

var idCounter uint64

type Phase string

const (
	PhaseIntake           Phase = "intake"
	PhaseContextGathering Phase = "context_gathering"
	PhasePlanning         Phase = "planning"
	PhaseActing           Phase = "acting"
	PhaseObserving        Phase = "observing"
	PhaseVerifying        Phase = "verifying"
	PhaseReflecting       Phase = "reflecting"
	PhaseReplanning       Phase = "replanning"
	PhaseDone             Phase = "done"
	PhaseStopped          Phase = "stopped"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskRunning    TaskStatus = "running"
	TaskBlocked    TaskStatus = "blocked"
	TaskVerified   TaskStatus = "verified"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
	TaskNeedsHuman TaskStatus = "needs_human"
)

type RiskLevel int

const (
	RiskReadOnly RiskLevel = iota
	RiskLocalEdit
	RiskCommand
	RiskNetwork
	RiskDependencyChange
	RiskDestructive
	RiskCredentialed
)

func (r RiskLevel) String() string {
	switch r {
	case RiskReadOnly:
		return "read_only"
	case RiskLocalEdit:
		return "local_edit"
	case RiskCommand:
		return "command"
	case RiskNetwork:
		return "network"
	case RiskDependencyChange:
		return "dependency_change"
	case RiskDestructive:
		return "destructive"
	case RiskCredentialed:
		return "credentialed"
	default:
		return "unknown"
	}
}

type ToolKind string

const (
	ToolFilesystem ToolKind = "filesystem"
	ToolTerminal   ToolKind = "terminal"
	ToolGit        ToolKind = "git"
	ToolBrowser    ToolKind = "browser"
	ToolSearch     ToolKind = "search"
	ToolMCP        ToolKind = "mcp"
	ToolCustom     ToolKind = "custom"
)

type EventType string

const (
	EventUserRequest         EventType = "user_request"
	EventPlanningStarted     EventType = "planning_started"
	EventPlanningCompleted   EventType = "planning_completed"
	EventContextCollected    EventType = "context_collected"
	EventPlanCreated         EventType = "plan_created"
	EventTaskStarted         EventType = "task_started"
	EventToolSelected        EventType = "tool_selected"
	EventToolStarted         EventType = "tool_started"
	EventToolCompleted       EventType = "tool_completed"
	EventToolFinished        EventType = "tool_finished" // Deprecated alias retained for compatibility.
	EventVerificationStarted EventType = "verification_started"
	EventVerificationPassed  EventType = "verification_passed"
	EventVerificationFailed  EventType = "verification_failed"
	EventReflectionStarted   EventType = "reflection_started"
	EventReflection          EventType = "reflection"
	EventRetryScheduled      EventType = "retry_scheduled"
	EventReplanRequested     EventType = "replan_requested"
	EventMemoryUpdated       EventType = "memory_updated"
	EventApprovalRequested   EventType = "approval_requested"
	EventApprovalDenied      EventType = "approval_denied"
	EventWaitingForUser      EventType = "waiting_for_user"
	EventStopped             EventType = "stopped"
	EventCompleted           EventType = "completed"
)

type HypothesisState string

const (
	HypothesisUnknown      HypothesisState = "unknown"
	HypothesisConfirmed    HypothesisState = "confirmed"
	HypothesisRejected     HypothesisState = "rejected"
	HypothesisWeakened     HypothesisState = "weakened"
	HypothesisStrengthened HypothesisState = "strengthened"
)

type Hypothesis struct {
	ID                   string
	Description          string
	SupportingEvidence   []string
	MissingEvidence      []string
	ContradictingEvidence []string
	Probability          float64
	VerificationCost     float64
	Risk                 RiskLevel
	InformationGain      float64
	VerificationMethod   string
	Confidence           float64
	State                HypothesisState
}

type EvidenceRef struct {
	Kind    string
	Source  string
	Summary string
	Line    int
}

type Event struct {
	ID        string
	Type      EventType
	TaskID    string
	Summary   string
	Data      map[string]any
	Evidence  []EvidenceRef
	CreatedAt time.Time
}

type VerificationSpec struct {
	Commands       []CommandSpec
	FileChecks     []FileCheck
	DiffChecks     []DiffCheck
	RequiredPasses []string
	OptionalChecks []string
}

type CommandSpec struct {
	Name        string
	Command     string
	Workdir     string
	Timeout     time.Duration
	Required    bool
	Mutates     bool
	Retryable   bool
	FailureHint string
}

type FileCheck struct {
	Path        string
	MustExist   bool
	MustParse   bool
	Description string
}

type DiffCheck struct {
	Description        string
	AllowUnrelated     bool
	RequireUserChanges bool
}

type TaskNode struct {
	ID               string
	ParentID         string
	Goal             string
	Status           TaskStatus
	Priority         int
	Dependencies     []string
	Risk             RiskLevel
	Confidence       float64
	Evidence         []EvidenceRef
	CandidateTools   []string
	Verification     VerificationSpec
	SuccessCriteria  string
	FailureCriteria  string
	HypothesisIDs    []string
	Attempts         int
	MaxAttempts      int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TaskGraph struct {
	RootID string
	Nodes  map[string]*TaskNode
	Order  []string
}

func (g *TaskGraph) Empty() bool {
	return g == nil || len(g.Order) == 0
}

func (g *TaskGraph) NextRunnable() *TaskNode {
	if g == nil {
		return nil
	}
	for _, id := range g.Order {
		node := g.Nodes[id]
		if node == nil || node.Status != TaskPending {
			continue
		}
		if g.dependenciesVerified(node) {
			return node
		}
	}
	return nil
}

func (g *TaskGraph) dependenciesVerified(node *TaskNode) bool {
	for _, depID := range node.Dependencies {
		dep := g.Nodes[depID]
		if dep == nil || dep.Status != TaskVerified {
			return false
		}
	}
	return true
}

type ContextItem struct {
	ID        string
	Kind      string
	Source    string
	Content   string
	Tokens    int
	Score     float64
	Exact     bool
	CreatedAt time.Time
}

type TokenBudget struct {
	Total          int
	ReservedOutput int
	ReservedTools  int
	Used           int
}

func (b TokenBudget) Remaining() int {
	return b.Total - b.ReservedOutput - b.ReservedTools - b.Used
}

type ContextBundle struct {
	UserRequest  string
	ActiveTask   *TaskNode
	Items        []ContextItem
	Memories     []MemoryItem
	RecentEvents []Event
	TokenBudget  TokenBudget
}

type MemoryType string

const (
	WorkingMemory  MemoryType = "working"
	SessionMemory  MemoryType = "session"
	ProjectMemory  MemoryType = "project"
	ToolHistory    MemoryType = "tool_history"
	FailedAttempt  MemoryType = "failed_attempt"
	StrategyMemory MemoryType = "strategy"
)

type MemoryItem struct {
	ID         string
	Type       MemoryType
	Scope      string
	Key        string
	Value      string
	Confidence float64
	Evidence   []EvidenceRef
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ToolCost struct {
	Tokens  int
	Latency time.Duration
}

type ToolDescriptor struct {
	Name             string
	Kind             ToolKind
	Capabilities     []string
	Risk             RiskLevel
	Cost             ToolCost
	SupportsParallel bool
	Mutates          bool
	Reversible       bool
	Definition       tools.Definition
}

type ToolIntent struct {
	Kind             ToolKind
	Capability       string
	Goal             string
	Arguments        map[string]any
	RequiresMutation bool
	MaxRisk          RiskLevel
	Verification     VerificationSpec
}

type CandidateAction struct {
	Tool        ToolDescriptor
	Intent      ToolIntent
	Call        tools.Call
	Score       float64
	Reason      string
	ParallelKey string
	Evaluation  ActionEvaluation
}

type ActionEvaluation struct {
	ExpectedValue       float64
	Confidence          float64
	Risk                float64
	LatencyCost         float64
	TokenCost           float64
	ExecutionCost       float64
	FailureProbability  float64
	VerificationAbility float64
	ReversibilityBonus  float64
	InformationGain     float64
	Rejected            bool
	Review              string
}

type RouteOptions struct {
	MaxCandidates int
	AllowParallel bool
	MaxRisk       RiskLevel
}

type ToolCallResult struct {
	Action   CandidateAction
	Result   tools.Result
	Error    error
	Started  time.Time
	Finished time.Time
}

type VerificationStatus string

const (
	VerificationUnknown VerificationStatus = "unknown"
	VerificationPassed  VerificationStatus = "passed"
	VerificationFailed  VerificationStatus = "failed"
	VerificationSkipped VerificationStatus = "skipped"
)

type VerificationResult struct {
	Status   VerificationStatus
	Checks   []CheckResult
	Evidence []EvidenceRef
	Started  time.Time
	Finished time.Time
	Summary  string
}

type CheckResult struct {
	Name     string
	Command  string
	Passed   bool
	Output   string
	Error    string
	Duration time.Duration
}

type ReflectionReport struct {
	ProgressDelta      float64
	Confidence         float64
	FailureReason      string
	AlternativeActions []CandidateAction
	ReplanNeeded       bool
	StopRecommended    bool
	StopReason         string
	Evidence           []EvidenceRef
}

type RetryPolicy struct {
	MaxAttemptsPerTask int
	MaxSameError       int
	Backoff            time.Duration
	RequiresNewInfo    bool
}

type ActionDecision struct {
	KnowEnough              bool
	NeedMoreContext         bool
	CandidateActions        []CandidateAction
	SelectedAction          CandidateAction
	CanParallelize          bool
	CheapestSafeAction      CandidateAction
	RecoveryActions         []CandidateAction
	ActiveHypotheses        []Hypothesis
	PrimaryHypothesis       *Hypothesis
	RejectedHypotheses      []Hypothesis
	InformationGainEstimate float64
	RemainingUncertainty    float64
	VerificationReason      string
	ConfidenceDeltaPrediction float64
	VerificationPlan        VerificationSpec
	RequiresApproval        bool
	Reason                  string
	InternalConfidence      float64
	ReviewerNotes           []string
	RejectedActions         []CandidateAction
}

type RunState struct {
	ID            string
	UserRequest   string
	Phase         Phase
	Plan          *TaskGraph
	ActiveTaskID  string
	Iteration     int
	MaxIterations int
	Events        []Event
	LastError     string
	Confidence    float64
	StartedAt     time.Time
	UpdatedAt     time.Time
	Metrics       RunMetrics
}

type RunMetrics struct {
	Iterations                  int
	ToolCalls                   int
	ToolFailures                int
	VerificationRuns            int
	VerificationFailures        int
	Recoveries                  int
	ReflectionRuns              int
	RepeatedActionBlocks        int
	RejectedToolCalls           int
	AverageConfidence           float64
	AverageLatency              time.Duration
	EstimatedTokenUsage         int
	ReasoningEfficiency         float64
	LastProgressDelta           float64
	ConsecutiveNoProgressTurns  int
	ConsecutiveVerificationFail int
}

func NewRunState(id string, request string, maxIterations int) *RunState {
	if maxIterations <= 0 {
		maxIterations = 1024
	}
	now := time.Now().UTC()
	return &RunState{
		ID:            id,
		UserRequest:   request,
		Phase:         PhaseIntake,
		MaxIterations: maxIterations,
		Confidence:    0.2,
		StartedAt:     now,
		UpdatedAt:     now,
	}
}

func (s *RunState) Done() bool {
	if s == nil {
		return true
	}
	return s.Phase == PhaseDone || s.Phase == PhaseStopped || s.Iteration >= s.MaxIterations
}

func (s *RunState) Append(event Event) {
	if event.ID == "" {
		event.ID = NewID("evt")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	s.Events = append(s.Events, event)
	s.UpdatedAt = event.CreatedAt
}

func (s *RunState) ActiveTask() *TaskNode {
	if s == nil || s.Plan == nil || s.ActiveTaskID == "" {
		return nil
	}
	return s.Plan.Nodes[s.ActiveTaskID]
}

func (s *RunState) SetPhase(phase Phase) {
	s.Phase = phase
	s.UpdatedAt = time.Now().UTC()
}

func NewID(prefix string) string {
	seq := atomic.AddUint64(&idCounter, 1)
	return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000") + "-" + strconv.FormatUint(seq, 36)
}
