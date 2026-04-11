package tools

import (
	"context"
	"fmt"

	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/tool"
)

const spawnAgentName = "spawn_agent"

const defaultSubagentSystemPrompt = "You are a helpful assistant. Complete the given task and provide a clear response."

type spawnAgentInput struct {
	Prompt       string `json:"prompt" description:"The task or question for the subagent"`
	SystemPrompt string `json:"system_prompt,omitempty" description:"Optional system prompt for the subagent"`
	Model        string `json:"model,omitempty" description:"Optional model override (e.g. openai/gpt-4o-mini)"`
}

type spawnAgentOutput struct {
	Result string `json:"result"`
}

// ProviderResolver resolves a provider.Provider from a model identifier.
type ProviderResolver func(ctx context.Context, model string) (provider.Provider, error)

// SubagentTools returns the spawn_agent tool. The tool uses defaultProvider for
// LLM calls unless a model override is given and resolve is non-nil.
// reg is the parent's tool registry; the child inherits all tools except spawn_agent.
func SubagentTools(defaultProvider provider.Provider, reg *tool.Registry, resolve ProviderResolver) []tool.Tool {
	return []tool.Tool{newSpawnAgentTool(defaultProvider, reg, resolve)}
}

func newSpawnAgentTool(defaultProvider provider.Provider, reg *tool.Registry, resolve ProviderResolver) tool.Tool {
	return tool.NewTool(spawnAgentName,
		"Spawn an ephemeral subagent to handle a task. The subagent runs in isolation with its own context and returns only the final text response.",
		func(ctx context.Context, in spawnAgentInput) (spawnAgentOutput, error) {
			p := defaultProvider
			if in.Model != "" && resolve != nil {
				resolved, err := resolve(ctx, in.Model)
				if err != nil {
					return spawnAgentOutput{}, fmt.Errorf("resolve model %q: %w", in.Model, err)
				}
				p = resolved
			}

			systemPrompt := defaultSubagentSystemPrompt
			if in.SystemPrompt != "" {
				systemPrompt = in.SystemPrompt
			}

			// Build child tools: all parent tools minus spawn_agent.
			var childToolDefs []provider.Tool
			var childNames []string
			if reg != nil {
				for _, t := range reg.Tools() {
					if t.Name != spawnAgentName {
						childToolDefs = append(childToolDefs, t)
						childNames = append(childNames, t.Name)
					}
				}
			}

			var executor provider.ToolExecutor
			if reg != nil && len(childNames) > 0 {
				executor = reg.Subset(childNames).Executor()
			}

			messages := []provider.Message{
				{Role: provider.RoleUser, Content: in.Prompt},
			}

			msgs, _, err := provider.RunAgentLoop(
				ctx, p, systemPrompt, messages, childToolDefs,
				executor, nil, nil,
			)
			if err != nil {
				return spawnAgentOutput{}, err
			}

			// Extract final text from last model message.
			var result string
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == provider.RoleModel && msgs[i].Content != "" {
					result = msgs[i].Content
					break
				}
			}

			return spawnAgentOutput{Result: result}, nil
		},
	)
}
