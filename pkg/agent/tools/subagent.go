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
	Prompt       string   `json:"prompt" description:"The task or question for the subagent"`
	Name         string   `json:"name,omitempty" description:"Short identifier for this subagent (e.g. researcher, summarizer). Shown in tool-call output so the user can tell which subagent is acting."`
	SystemPrompt string   `json:"system_prompt,omitempty" description:"Optional system prompt for the subagent"`
	Model        string   `json:"model,omitempty" description:"Optional model override (e.g. openai/gpt-4o-mini)"`
	Tools        []string `json:"tools,omitempty" description:"Optional list of tool names the subagent can use. When omitted, inherits all parent tools except spawn_agent."`
}

type spawnAgentOutput struct {
	Result string `json:"result"`
}

// ProviderResolver resolves a provider.Provider from a model identifier.
type ProviderResolver func(ctx context.Context, model string) (provider.Provider, error)

// SubagentTools returns the spawn_agent tool. The tool uses defaultProvider for
// LLM calls unless a model override is given and resolve is non-nil.
// reg is the parent's tool registry; by default the child inherits all parent tools
// except spawn_agent, but the caller can pass an explicit tools list to restrict the set.
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

			// Build child tools: use explicit list if provided,
			// otherwise inherit all parent tools minus spawn_agent.
			var childReg *tool.Registry
			if reg != nil {
				if in.Tools != nil {
					childReg = reg.Subset(in.Tools)
				} else {
					var childNames []string
					for _, t := range reg.Tools() {
						if t.Name != spawnAgentName {
							childNames = append(childNames, t.Name)
						}
					}
					childReg = reg.Subset(childNames)
				}
			}

			var childToolDefs []provider.Tool
			var executor provider.ToolExecutor
			if childReg != nil {
				childToolDefs = childReg.Tools()
				if len(childToolDefs) > 0 {
					executor = childReg.Executor()
				}
			}

			// Extract parent's onToolCall from context and wrap it to
			// inject the agent name as the agent label.
			agentName := in.Name
			if agentName == "" {
				agentName = "subagent"
			}
			var childOnToolCall provider.ToolCallCallback
			if parentCb := provider.GetToolCallCallback(ctx); parentCb != nil {
				childOnToolCall = func(name, args, _ string) {
					parentCb(name, args, agentName)
				}
			}

			messages := []provider.Message{
				{Role: provider.RoleUser, Content: in.Prompt},
			}

			msgs, _, err := provider.RunAgentLoop(
				ctx, p, systemPrompt, messages, childToolDefs,
				executor, childOnToolCall, nil,
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
