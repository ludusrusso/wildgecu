package tool

import (
	"context"
	"fmt"

	"gonesis/provider"
)

// Registry collects tools and provides a single ToolExecutor.
type Registry struct {
	tools map[string]Tool
	order []string // preserves insertion order
}

// NewRegistry creates a registry from the given tools.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{
		tools: make(map[string]Tool, len(tools)),
	}
	for _, t := range tools {
		name := t.Definition().Name
		r.tools[name] = t
		r.order = append(r.order, name)
	}
	return r
}

// Tools returns provider.Tool definitions for all registered tools.
func (r *Registry) Tools() []provider.Tool {
	out := make([]provider.Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name].Definition())
	}
	return out
}

// Executor returns a provider.ToolExecutor that dispatches by tool name.
func (r *Registry) Executor() provider.ToolExecutor {
	return func(ctx context.Context, tc provider.ToolCall) (string, error) {
		t, ok := r.tools[tc.Name]
		if !ok {
			return fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Name), nil
		}
		return t.Execute(ctx, tc.Args)
	}
}
