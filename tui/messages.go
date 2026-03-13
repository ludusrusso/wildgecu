package tui

import "gonesis/provider"

// agentResponseMsg is sent when the agent completes a turn successfully.
type agentResponseMsg struct {
	response *provider.Response
	messages []provider.Message
}

// agentErrorMsg is sent when the agent encounters an error.
type agentErrorMsg struct {
	err error
}

// agentDoneMsg is sent when the executor signals provider.ErrDone.
type agentDoneMsg struct {
	messages []provider.Message
}

// initialTurnMsg triggers the first model turn for pre-seeded messages.
type initialTurnMsg struct{}

// streamChunkMsg carries a partial text chunk from the streaming response.
type streamChunkMsg struct {
	content string
}

// streamDoneMsg signals streaming is complete with the final results.
type streamDoneMsg struct {
	response *provider.Response
	messages []provider.Message
}

// toolCallMsg is sent when the agent starts executing a tool.
type toolCallMsg struct {
	name string
	args string
}
