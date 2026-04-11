package openai

import (
	"context"
	"encoding/json"
	"fmt"

	oai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"wildgecu/pkg/provider"
)

// Provider implements provider.Provider and provider.StreamProvider
// using the OpenAI Chat Completions API. It also works with any
// OpenAI-compatible endpoint (e.g. Ollama) via WithBaseURL.
type Provider struct {
	client *oai.Client
	model  string
}

// Option configures an OpenAI Provider.
type Option func(*options)

type options struct {
	baseURL string
}

// WithBaseURL overrides the API base URL.
// Use this for OpenAI-compatible endpoints such as Ollama.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// NewRegolo creates a provider for the Regolo OpenAI-compatible API.
func NewRegolo(apiKey, model string) *Provider {
	return New(apiKey, model, WithBaseURL("https://api.regolo.ai/v1"))
}

// NewMistral creates a provider for the Mistral OpenAI-compatible API.
func NewMistral(apiKey, model string) *Provider {
	return New(apiKey, model, WithBaseURL("https://api.mistral.ai/v1"))
}

// NewOllama creates a provider for a local Ollama instance.
// The default base URL is http://localhost:11434/v1; override with WithBaseURL.
func NewOllama(model string, opts ...Option) *Provider {
	opts = append([]Option{WithBaseURL("http://localhost:11434/v1")}, opts...)
	return New("", model, opts...)
}

// New creates an OpenAI provider. Pass an empty apiKey for endpoints
// that do not require authentication (e.g. local Ollama).
func New(apiKey, model string, opts ...Option) *Provider {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	clientOpts := []option.RequestOption{}
	if apiKey != "" {
		clientOpts = append(clientOpts, option.WithAPIKey(apiKey))
	} else {
		// Prevent the SDK from reading OPENAI_API_KEY from env.
		clientOpts = append(clientOpts, option.WithAPIKey("unused"))
	}
	if o.baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(o.baseURL))
	}
	client := oai.NewClient(clientOpts...)
	return &Provider{client: &client, model: model}
}

// Generate performs a non-streaming chat completion.
func (p *Provider) Generate(ctx context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	body := p.buildParams(params)

	resp, err := p.client.Chat.Completions.New(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("openai: generate: %w", err)
	}

	return toResponse(resp), nil
}

// GenerateStream returns channels that emit partial chunks and a final error.
func (p *Provider) GenerateStream(ctx context.Context, params *provider.GenerateParams) (<-chan provider.StreamChunk, <-chan error) {
	chunks := make(chan provider.StreamChunk)
	errCh := make(chan error, 1)

	body := p.buildParams(params)
	body.StreamOptions = oai.ChatCompletionStreamOptionsParam{
		IncludeUsage: oai.Opt(true),
	}

	go func() {
		defer close(chunks)
		defer close(errCh)

		stream := p.client.Chat.Completions.NewStreaming(ctx, body)
		defer func() { _ = stream.Close() }()

		// Accumulate tool call deltas by index.
		type toolAcc struct {
			id   string
			name string
			args string
		}
		var toolCalls []toolAcc

		for stream.Next() {
			raw := stream.Current()

			chunk := provider.StreamChunk{}

			// Usage (populated on the last chunk when include_usage is set).
			if raw.Usage.PromptTokens > 0 || raw.Usage.CompletionTokens > 0 {
				chunk.Usage = provider.Usage{
					InputTokens:  int(raw.Usage.PromptTokens),
					OutputTokens: int(raw.Usage.CompletionTokens),
				}
			}

			if len(raw.Choices) > 0 {
				delta := raw.Choices[0].Delta

				// Text content.
				if delta.Content != "" {
					chunk.Content = delta.Content
				}

				// Accumulate tool call deltas.
				for i := range delta.ToolCalls {
					idx := int(delta.ToolCalls[i].Index)
					for len(toolCalls) <= idx {
						toolCalls = append(toolCalls, toolAcc{})
					}
					if delta.ToolCalls[i].ID != "" {
						toolCalls[idx].id = delta.ToolCalls[i].ID
					}
					if delta.ToolCalls[i].Function.Name != "" {
						toolCalls[idx].name = delta.ToolCalls[i].Function.Name
					}
					toolCalls[idx].args += delta.ToolCalls[i].Function.Arguments
				}

				// When the model finishes tool_calls, emit them.
				if raw.Choices[0].FinishReason == "tool_calls" {
					for _, tc := range toolCalls {
						args, _ := parseArgs(tc.args)
						chunk.ToolCalls = append(chunk.ToolCalls, provider.ToolCall{
							ID:   tc.id,
							Name: tc.name,
							Args: args,
						})
					}
					toolCalls = nil
				}
			}

			select {
			case chunks <- chunk:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if err := stream.Err(); err != nil {
			errCh <- fmt.Errorf("openai: stream: %w", err)
		}
	}()

	return chunks, errCh
}

// buildParams constructs the ChatCompletionNewParams from our internal types.
func (p *Provider) buildParams(params *provider.GenerateParams) oai.ChatCompletionNewParams {
	model := p.model
	if params.Model != "" {
		model = params.Model
	}
	body := oai.ChatCompletionNewParams{
		Model:    model,
		Messages: toMessages(params.SystemPrompt, params.Messages),
	}
	if len(params.Tools) > 0 {
		body.Tools = toTools(params.Tools)
	}
	return body
}

// toMessages converts internal messages to OpenAI message params.
// The system prompt is prepended as a system message.
func toMessages(systemPrompt string, messages []provider.Message) []oai.ChatCompletionMessageParamUnion {
	out := make([]oai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	if systemPrompt != "" {
		out = append(out, oai.ChatCompletionMessageParamUnion{
			OfSystem: &oai.ChatCompletionSystemMessageParam{
				Content: oai.ChatCompletionSystemMessageParamContentUnion{
					OfString: oai.Opt(systemPrompt),
				},
			},
		})
	}

	for _, msg := range messages {
		out = append(out, toMessage(msg))
	}
	return out
}

func toMessage(msg provider.Message) oai.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case provider.RoleTool:
		return oai.ChatCompletionMessageParamUnion{
			OfTool: &oai.ChatCompletionToolMessageParam{
				ToolCallID: msg.ToolCallID,
				Content: oai.ChatCompletionToolMessageParamContentUnion{
					OfString: oai.Opt(msg.Content),
				},
			},
		}

	case provider.RoleModel:
		asst := &oai.ChatCompletionAssistantMessageParam{}
		if msg.Content != "" {
			asst.Content = oai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: oai.Opt(msg.Content),
			}
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Args)
				asst.ToolCalls = append(asst.ToolCalls, oai.ChatCompletionMessageToolCallParam{
					ID: tc.ID,
					Function: oai.ChatCompletionMessageToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
		return oai.ChatCompletionMessageParamUnion{OfAssistant: asst}

	default: // user
		return oai.ChatCompletionMessageParamUnion{
			OfUser: &oai.ChatCompletionUserMessageParam{
				Content: oai.ChatCompletionUserMessageParamContentUnion{
					OfString: oai.Opt(msg.Content),
				},
			},
		}
	}
}

// toTools converts internal tool definitions to OpenAI tool params.
func toTools(tools []provider.Tool) []oai.ChatCompletionToolParam {
	out := make([]oai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, oai.ChatCompletionToolParam{
			Function: oai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: oai.Opt(t.Description),
				Parameters:  oai.FunctionParameters(t.Parameters),
			},
		})
	}
	return out
}

// toResponse converts an OpenAI ChatCompletion to our internal Response.
func toResponse(resp *oai.ChatCompletion) *provider.Response {
	r := &provider.Response{}

	r.Usage = provider.Usage{
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}

	if len(resp.Choices) == 0 {
		return r
	}

	msg := resp.Choices[0].Message
	r.Message.Role = provider.RoleModel
	r.Message.Content = msg.Content

	for i := range msg.ToolCalls {
		args, _ := parseArgs(msg.ToolCalls[i].Function.Arguments)
		r.Message.ToolCalls = append(r.Message.ToolCalls, provider.ToolCall{
			ID:   msg.ToolCalls[i].ID,
			Name: msg.ToolCalls[i].Function.Name,
			Args: args,
		})
	}

	return r
}

// parseArgs parses a JSON string into a map[string]any.
func parseArgs(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var args map[string]any
	err := json.Unmarshal([]byte(raw), &args)
	return args, err
}
