package provider

import "context"

type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
	RoleTool  Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

// ToolCall represents a function call requested by the model.
type ToolCall struct {
	Args             map[string]any
	ID               string
	Name             string
	ThoughtSignature []byte // Opaque signature for thinking models (e.g. Gemini)
}

// Tool defines a function the model can call.
type Tool struct {
	Parameters  map[string]any
	Name        string
	Description string
}

// GenerateParams holds everything needed for a single generation request.
type GenerateParams struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []Tool
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is what comes back from a generation call.
type Response struct {
	Message Message
	Usage   Usage
}

// Provider is the interface every LLM backend must implement.
type Provider interface {
	Generate(ctx context.Context, params *GenerateParams) (*Response, error)
}

// StreamChunk is a partial text chunk from a streaming response.
type StreamChunk struct {
	Content   string
	ToolCalls []ToolCall // populated in the last chunk if the model made tool calls
	Usage     Usage      // populated on last chunk
}

// StreamProvider extends Provider with streaming support.
type StreamProvider interface {
	Provider
	GenerateStream(ctx context.Context, params *GenerateParams) (<-chan StreamChunk, <-chan error)
}

// StreamCallback is called for each text chunk during a streaming response.
type StreamCallback func(chunk string)

// InformFunc is called when the agent wants to display a status message to the user.
type InformFunc func(message string)

type informKeyType struct{}

var informKey = informKeyType{}

// WithInformFunc returns a context carrying the given InformFunc.
func WithInformFunc(ctx context.Context, fn InformFunc) context.Context {
	return context.WithValue(ctx, informKey, fn)
}

// GetInformFunc extracts the InformFunc from the context, or nil.
func GetInformFunc(ctx context.Context) InformFunc {
	fn, _ := ctx.Value(informKey).(InformFunc)
	return fn
}

// ToolCallCallback is invoked before each tool execution with the tool name,
// formatted arguments, and an optional agent label (empty for the parent agent).
type ToolCallCallback func(name, args, agent string)

type toolCallCallbackKeyType struct{}

var toolCallCallbackKey = toolCallCallbackKeyType{}

// WithToolCallCallback returns a context carrying the given ToolCallCallback.
func WithToolCallCallback(ctx context.Context, fn ToolCallCallback) context.Context {
	return context.WithValue(ctx, toolCallCallbackKey, fn)
}

// GetToolCallCallback extracts the ToolCallCallback from the context, or nil.
func GetToolCallCallback(ctx context.Context) ToolCallCallback {
	fn, _ := ctx.Value(toolCallCallbackKey).(ToolCallCallback)
	return fn
}
