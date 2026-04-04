package tui

// streamChunkMsg carries a partial text chunk from the streaming response.
type streamChunkMsg struct {
	content string
}

// streamDoneMsg signals streaming is complete with the final content.
type streamDoneMsg struct {
	content string
}

// toolCallMsg is sent when the agent starts executing a tool.
type toolCallMsg struct {
	name string
	args string
}

// agentErrorMsg is sent when the agent encounters an error.
type agentErrorMsg struct {
	err error
}

// sessionCreatedMsg signals that a session was successfully created.
type sessionCreatedMsg struct {
	sessionID string
	welcome   string
}

// sessionErrorMsg signals that session creation failed.
type sessionErrorMsg struct {
	err error
}
