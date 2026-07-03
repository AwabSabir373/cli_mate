package tools

import (
	"fmt"
	"strings"
)

const DefaultDeferThreshold = 20

// ToolRegistry manages tool registration with optional deferred loading.
// When the number of tools exceeds the threshold, full JSON schemas are
// withheld from the model and tool_search is provided instead.
type ToolRegistry struct {
	tools          map[string]Tool
	deferred       map[string]Tool // tools whose schemas are withheld
	threshold      int
	deferEnabled   bool
	allDefinitions []Definition // cached full definitions for eager tools
}

// NewToolRegistry creates a registry with the given threshold.
// If tool count exceeds threshold, extra tools become deferred.
func NewToolRegistry(threshold int) *ToolRegistry {
	if threshold <= 0 {
		threshold = DefaultDeferThreshold
	}
	return &ToolRegistry{
		tools:     make(map[string]Tool),
		deferred:  make(map[string]Tool),
		threshold: threshold,
	}
}

// Register adds a tool to the registry. If deferred mode is active and
// the count exceeds the threshold, the tool's schema is withheld.
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
	r.allDefinitions = nil // invalidate cache
}

// SetDeferred enables or disables deferred tool loading.
func (r *ToolRegistry) SetDeferred(enabled bool) {
	r.deferEnabled = enabled
	r.allDefinitions = nil
}

// ActiveTools returns the tools whose schemas should be sent to the model.
// In deferred mode, this excludes tools beyond the threshold.
func (r *ToolRegistry) ActiveTools() []Tool {
	if !r.deferEnabled {
		return r.allTools()
	}
	var active []Tool
	for _, tool := range r.tools {
		if _, isDeferred := r.deferred[tool.Name()]; !isDeferred {
			active = append(active, tool)
		}
	}
	return active
}

// DeferredTools returns tools whose schemas are withheld.
func (r *ToolRegistry) DeferredTools() []Tool {
	var deferred []Tool
	for _, tool := range r.deferred {
		deferred = append(deferred, tool)
	}
	return deferred
}

// EagerDefinitions returns tool definitions only for non-deferred tools.
// This is what gets sent in the API request.
func (r *ToolRegistry) EagerDefinitions() []Definition {
	if r.allDefinitions != nil {
		return r.allDefinitions
	}
	active := r.ActiveTools()
	defs := make([]Definition, 0, len(active))
	for _, tool := range active {
		defs = append(defs, tool.Definition())
	}
	r.allDefinitions = defs
	return defs
}

// GetTool returns a tool by name, searching both active and deferred.
func (r *ToolRegistry) GetTool(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// Search finds tools whose name or description contains the query.
func (r *ToolRegistry) Search(query string) []Tool {
	query = strings.ToLower(query)
	var matches []Tool
	for _, tool := range r.tools {
		def := tool.Definition()
		if strings.Contains(strings.ToLower(def.Name), query) ||
			strings.Contains(strings.ToLower(def.Description), query) {
			matches = append(matches, tool)
		}
	}
	return matches
}

// ListNames returns a comma-separated list of all tool names.
func (r *ToolRegistry) ListNames() string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// Count returns the total number of registered tools.
func (r *ToolRegistry) Count() int {
	return len(r.tools)
}

// PromoteTool moves a deferred tool to active (schema will be included).
func (r *ToolRegistry) PromoteTool(name string) error {
	tool, ok := r.deferred[name]
	if !ok {
		return fmt.Errorf("tool %q is not deferred", name)
	}
	delete(r.deferred, name)
	r.tools[name] = tool
	r.allDefinitions = nil
	return nil
}

// AllTools returns all registered tools (both active and deferred).
func (r *ToolRegistry) allTools() []Tool {
	all := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		all = append(all, tool)
	}
	return all
}

// Finalize splits tools into active and deferred based on the threshold.
// Call this after all tools are registered.
func (r *ToolRegistry) Finalize() {
	if !r.deferEnabled {
		return
	}
	all := r.allTools()
	if len(all) <= r.threshold {
		return
	}
	// Keep the first `threshold` tools as eager, defer the rest
	for i, tool := range all {
		if i >= r.threshold {
			r.deferred[tool.Name()] = tool
			delete(r.tools, tool.Name())
		}
	}
	r.allDefinitions = nil
}
