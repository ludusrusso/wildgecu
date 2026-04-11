package tool

import (
	"context"
	"fmt"

	"wildgecu/pkg/provider"
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
	r.Add(tools)
	return r
}

// Add appends tools to the registry.
func (r *Registry) Add(tools []Tool) {
	for _, t := range tools {
		name := t.Definition().Name
		r.tools[name] = t
		r.order = append(r.order, name)
	}
}

// Tools returns provider.Tool definitions for all registered tools.
func (r *Registry) Tools() []provider.Tool {
	out := make([]provider.Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name].Definition())
	}
	return out
}

// Subset returns a new Registry containing only the named tools,
// preserving their original insertion order. Unknown names are silently ignored.
func (r *Registry) Subset(names []string) *Registry {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	sub := &Registry{tools: make(map[string]Tool)}
	for _, name := range r.order {
		if _, ok := want[name]; ok {
			sub.tools[name] = r.tools[name]
			sub.order = append(sub.order, name)
		}
	}
	return sub
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
