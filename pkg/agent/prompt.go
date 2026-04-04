package agent

import _ "embed"

//go:embed AGENT.md
var agentPrompt string

//go:embed BOOTSTRAP.md
var bootstrapPrompt string

//go:embed MEMORY_AGENT.md
var memoryAgentPrompt string

//go:embed CODE_AGENT.md
var codeAgentPrompt string
