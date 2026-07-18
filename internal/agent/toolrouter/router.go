package toolrouter

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"cli_mate/internal/agent/agentloop"
	"cli_mate/internal/tools"
)

type Router interface {
	Register(tools.Tool, ...RegisterOption)
	Select(context.Context, agentloop.ToolIntent, agentloop.RouteOptions) ([]agentloop.CandidateAction, error)
	ExecuteOne(context.Context, agentloop.CandidateAction) agentloop.ToolCallResult
	ExecuteParallel(context.Context, []agentloop.CandidateAction) []agentloop.ToolCallResult
	Descriptors() []agentloop.ToolDescriptor
	Descriptor(string) (agentloop.ToolDescriptor, bool)
}

type RegisterOption func(*agentloop.ToolDescriptor)

type ToolRouter struct {
	mu          sync.RWMutex
	tools       map[string]tools.Tool
	descriptors map[string]agentloop.ToolDescriptor
}

func New() *ToolRouter {
	return &ToolRouter{
		tools:       map[string]tools.Tool{},
		descriptors: map[string]agentloop.ToolDescriptor{},
	}
}

func NewFromTools(toolset []tools.Tool) *ToolRouter {
	router := New()
	for _, tool := range toolset {
		router.Register(tool)
	}
	return router
}

func WithKind(kind agentloop.ToolKind) RegisterOption {
	return func(desc *agentloop.ToolDescriptor) { desc.Kind = kind }
}

func WithRisk(risk agentloop.RiskLevel) RegisterOption {
	return func(desc *agentloop.ToolDescriptor) { desc.Risk = risk }
}

func WithCapabilities(capabilities ...string) RegisterOption {
	return func(desc *agentloop.ToolDescriptor) { desc.Capabilities = append(desc.Capabilities, capabilities...) }
}

func WithParallel(enabled bool) RegisterOption {
	return func(desc *agentloop.ToolDescriptor) { desc.SupportsParallel = enabled }
}

func WithMutates(mutates bool) RegisterOption {
	return func(desc *agentloop.ToolDescriptor) { desc.Mutates = mutates }
}

func (r *ToolRouter) Register(tool tools.Tool, opts ...RegisterOption) {
	if tool == nil {
		return
	}
	def := tool.Definition()
	desc := inferDescriptor(def)
	for _, opt := range opts {
		opt(&desc)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
	r.descriptors[tool.Name()] = desc
}

func (r *ToolRouter) Descriptors() []agentloop.ToolDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]agentloop.ToolDescriptor, 0, len(r.descriptors))
	for _, desc := range r.descriptors {
		out = append(out, desc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *ToolRouter) Descriptor(name string) (agentloop.ToolDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	desc, ok := r.descriptors[name]
	return desc, ok
}

func (r *ToolRouter) Select(_ context.Context, intent agentloop.ToolIntent, opts agentloop.RouteOptions) ([]agentloop.CandidateAction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	maxRisk := opts.MaxRisk
	if maxRisk == 0 && intent.MaxRisk != 0 {
		maxRisk = intent.MaxRisk
	}
	if maxRisk == 0 {
		maxRisk = agentloop.RiskCredentialed
	}

	var candidates []agentloop.CandidateAction
	for name, desc := range r.descriptors {
		if desc.Risk > maxRisk {
			continue
		}
		score, reason := score(desc, intent)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, agentloop.CandidateAction{
			Tool:   desc,
			Intent: intent,
			Call: tools.Call{
				Name:     name,
				Argument: copyMap(intent.Arguments),
			},
			Score:       score,
			Reason:      reason,
			ParallelKey: parallelKey(desc),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Tool.Cost.Tokens < candidates[j].Tool.Cost.Tokens
		}
		return candidates[i].Score > candidates[j].Score
	})
	if opts.MaxCandidates > 0 && len(candidates) > opts.MaxCandidates {
		candidates = candidates[:opts.MaxCandidates]
	}
	return candidates, nil
}

func (r *ToolRouter) ExecuteOne(ctx context.Context, action agentloop.CandidateAction) agentloop.ToolCallResult {
	started := time.Now().UTC()
	r.mu.RLock()
	tool := r.tools[action.Tool.Name]
	r.mu.RUnlock()
	if tool == nil {
		return agentloop.ToolCallResult{
			Action:   action,
			Result:   tools.Result{Error: "tool is not registered: " + action.Tool.Name},
			Started:  started,
			Finished: time.Now().UTC(),
		}
	}
	result, err := tool.Execute(ctx, action.Call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	return agentloop.ToolCallResult{
		Action:   action,
		Result:   result,
		Error:    err,
		Started:  started,
		Finished: time.Now().UTC(),
	}
}

func (r *ToolRouter) ExecuteParallel(ctx context.Context, actions []agentloop.CandidateAction) []agentloop.ToolCallResult {
	results := make([]agentloop.ToolCallResult, len(actions))
	var wg sync.WaitGroup
	for i, action := range actions {
		if !action.Tool.SupportsParallel || action.Tool.Mutates {
			results[i] = r.ExecuteOne(ctx, action)
			continue
		}
		wg.Add(1)
		go func(i int, action agentloop.CandidateAction) {
			defer wg.Done()
			results[i] = r.ExecuteOne(ctx, action)
		}(i, action)
	}
	wg.Wait()
	return results
}

func inferDescriptor(def tools.Definition) agentloop.ToolDescriptor {
	name := strings.ToLower(def.Name)
	desc := agentloop.ToolDescriptor{
		Name:             def.Name,
		Kind:             agentloop.ToolCustom,
		Capabilities:     []string{def.Name},
		Risk:             agentloop.RiskReadOnly,
		Cost:             agentloop.ToolCost{Tokens: 250, Latency: 250 * time.Millisecond},
		SupportsParallel: true,
		Reversible:       true,
		Definition:       def,
	}
	switch {
	case strings.Contains(name, "file") || strings.Contains(name, "grep") || strings.Contains(name, "glob") || strings.Contains(name, "subtree"):
		desc.Kind = agentloop.ToolFilesystem
		desc.Capabilities = append(desc.Capabilities, "read", "search", "inspect")
	case strings.Contains(name, "shell") || strings.Contains(name, "background"):
		desc.Kind = agentloop.ToolTerminal
		desc.Capabilities = append(desc.Capabilities, "command", "test", "build", "format")
		desc.Risk = agentloop.RiskCommand
		desc.Cost = agentloop.ToolCost{Tokens: 500, Latency: 2 * time.Second}
		desc.SupportsParallel = false
		desc.Reversible = false
	case strings.Contains(name, "web"):
		desc.Kind = agentloop.ToolSearch
		desc.Capabilities = append(desc.Capabilities, "search", "fetch", "docs")
		desc.Risk = agentloop.RiskNetwork
		desc.Reversible = false
	case strings.Contains(name, "mcp"):
		desc.Kind = agentloop.ToolMCP
		desc.Capabilities = append(desc.Capabilities, "remote_tool", "dynamic")
		desc.Risk = agentloop.RiskNetwork
		desc.Reversible = false
	case strings.Contains(name, "diff") || strings.Contains(name, "commit") || strings.Contains(name, "worktree"):
		desc.Kind = agentloop.ToolGit
		desc.Capabilities = append(desc.Capabilities, "git", "diff", "checkpoint")
		desc.Risk = agentloop.RiskCommand
	}
	if strings.Contains(name, "edit") || strings.Contains(name, "write") || strings.Contains(name, "patch") || strings.Contains(name, "replace") || strings.Contains(name, "rename") || strings.Contains(name, "delete") || strings.Contains(name, "commit") {
		desc.Mutates = true
		desc.Risk = maxRisk(desc.Risk, agentloop.RiskLocalEdit)
		desc.SupportsParallel = false
		// file_edit with patch is reversible; file_write overwrites entirely
		if strings.Contains(name, "write") || strings.Contains(name, "commit") {
			desc.Reversible = false
		}
	}
	return desc
}

func score(desc agentloop.ToolDescriptor, intent agentloop.ToolIntent) (float64, string) {
	value := 0.0
	reasons := []string{}
	if intent.Kind != "" && desc.Kind == intent.Kind {
		value += 5
		reasons = append(reasons, "kind match")
	}
	if intent.Capability != "" && hasCapability(desc, intent.Capability) {
		value += 4
		reasons = append(reasons, "capability match")
	}
	if intent.RequiresMutation == desc.Mutates {
		value += 1
	}
	value -= float64(desc.Cost.Tokens) / 1000
	value -= float64(desc.Risk) * 0.35
	if value <= 0 && intent.Kind == "" && intent.Capability == "" {
		value = 0.5 - float64(desc.Risk)*0.1
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "fallback candidate")
	}
	return value, strings.Join(reasons, ", ")
}

func hasCapability(desc agentloop.ToolDescriptor, capability string) bool {
	capability = strings.ToLower(capability)
	for _, item := range desc.Capabilities {
		if strings.Contains(strings.ToLower(item), capability) || strings.Contains(capability, strings.ToLower(item)) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(desc.Definition.Description), capability)
}

func parallelKey(desc agentloop.ToolDescriptor) string {
	if !desc.SupportsParallel || desc.Mutates {
		return desc.Name
	}
	return string(desc.Kind)
}

func copyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func maxRisk(a, b agentloop.RiskLevel) agentloop.RiskLevel {
	if a > b {
		return a
	}
	return b
}
