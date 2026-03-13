package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
	"gonesis/provider"
)

// Provider implements provider.Provider using the Google Gemini API.
type Provider struct {
	client *genai.Client
	model  string
}

// New creates a Gemini provider with the given API key and model name.
func New(ctx context.Context, apiKey, model string) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}
	return &Provider{client: client, model: model}, nil
}

func (p *Provider) GenerateStream(ctx context.Context, params *provider.GenerateParams) (<-chan provider.StreamChunk, <-chan error) {
	chunks := make(chan provider.StreamChunk)
	errCh := make(chan error, 1)

	model := p.model
	if params.Model != "" {
		model = params.Model
	}
	contents := toContents(params.Messages)
	config := &genai.GenerateContentConfig{}
	if params.SystemPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(params.SystemPrompt, genai.RoleUser)
	}
	if len(params.Tools) > 0 {
		config.Tools = toTools(params.Tools)
	}

	go func() {
		defer close(chunks)
		defer close(errCh)

		for resp, err := range p.client.Models.GenerateContentStream(ctx, model, contents, config) {
			if err != nil {
				errCh <- fmt.Errorf("gemini: stream: %w", err)
				return
			}
			chunk := provider.StreamChunk{}
			if resp.UsageMetadata != nil {
				chunk.Usage = provider.Usage{
					InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
					OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
				}
			}
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					if part.Text != "" {
						chunk.Content += part.Text
					}
					if part.FunctionCall != nil {
						chunk.ToolCalls = append(chunk.ToolCalls, provider.ToolCall{
							ID:               part.FunctionCall.Name,
							Name:             part.FunctionCall.Name,
							Args:             part.FunctionCall.Args,
							ThoughtSignature: part.ThoughtSignature,
						})
					}
				}
			}
			chunks <- chunk
		}
	}()

	return chunks, errCh
}

func (p *Provider) Generate(ctx context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	model := p.model
	if params.Model != "" {
		model = params.Model
	}

	contents := toContents(params.Messages)
	config := &genai.GenerateContentConfig{}

	if params.SystemPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(params.SystemPrompt, genai.RoleUser)
	}

	if len(params.Tools) > 0 {
		config.Tools = toTools(params.Tools)
	}

	resp, err := p.client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini: generate: %w", err)
	}

	return toResponse(resp), nil
}

// toContents converts our messages to Gemini Content objects.
func toContents(messages []provider.Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, msg := range messages {
		contents = append(contents, toContent(msg))
	}
	return contents
}

func toContent(msg provider.Message) *genai.Content {
	switch msg.Role {
	case provider.RoleTool:
		result := map[string]any{"result": msg.Content}
		return genai.NewContentFromFunctionResponse(msg.ToolCallID, result, genai.RoleUser)

	case provider.RoleModel:
		if len(msg.ToolCalls) > 0 {
			parts := make([]*genai.Part, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				p := genai.NewPartFromFunctionCall(tc.Name, tc.Args)
				if len(tc.ThoughtSignature) > 0 {
					p.ThoughtSignature = tc.ThoughtSignature
				}
				parts = append(parts, p)
			}
			if msg.Content != "" {
				parts = append([]*genai.Part{genai.NewPartFromText(msg.Content)}, parts...)
			}
			return genai.NewContentFromParts(parts, genai.RoleModel)
		}
		return genai.NewContentFromText(msg.Content, genai.RoleModel)

	default: // user
		return genai.NewContentFromText(msg.Content, genai.RoleUser)
	}
}

// toTools converts our tool definitions to Gemini Tool objects.
func toTools(tools []provider.Tool) []*genai.Tool {
	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  toSchema(t.Parameters),
		})
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// toSchema converts a JSON Schema map to a genai.Schema.
func toSchema(params map[string]any) *genai.Schema {
	if params == nil {
		return nil
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	if props, ok := params["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, raw := range props {
			propMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			schema.Properties[name] = toPropertySchema(propMap)
		}
	}

	if required, ok := params["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	return schema
}

func toPropertySchema(prop map[string]any) *genai.Schema {
	s := &genai.Schema{}

	if t, ok := prop["type"].(string); ok {
		switch t {
		case "string":
			s.Type = genai.TypeString
		case "number":
			s.Type = genai.TypeNumber
		case "integer":
			s.Type = genai.TypeInteger
		case "boolean":
			s.Type = genai.TypeBoolean
		case "array":
			s.Type = genai.TypeArray
			if items, ok := prop["items"].(map[string]any); ok {
				s.Items = toPropertySchema(items)
			}
		case "object":
			s.Type = genai.TypeObject
			if props, ok := prop["properties"].(map[string]any); ok {
				s.Properties = make(map[string]*genai.Schema)
				for name, raw := range props {
					if pm, ok := raw.(map[string]any); ok {
						s.Properties[name] = toPropertySchema(pm)
					}
				}
			}
		}
	}

	if desc, ok := prop["description"].(string); ok {
		s.Description = desc
	}

	if enum, ok := prop["enum"].([]any); ok {
		for _, e := range enum {
			if es, ok := e.(string); ok {
				s.Enum = append(s.Enum, es)
			}
		}
	}

	return s
}

// toResponse converts a Gemini response to our Response type.
func toResponse(resp *genai.GenerateContentResponse) *provider.Response {
	r := &provider.Response{}

	if resp.UsageMetadata != nil {
		r.Usage = provider.Usage{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		}
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return r
	}

	content := resp.Candidates[0].Content
	r.Message.Role = provider.RoleModel

	for _, part := range content.Parts {
		if part.Text != "" {
			r.Message.Content += part.Text
		}
		if part.FunctionCall != nil {
			r.Message.ToolCalls = append(r.Message.ToolCalls, provider.ToolCall{
				ID:               part.FunctionCall.Name, // Gemini uses name as ID
				Name:             part.FunctionCall.Name,
				Args:             part.FunctionCall.Args,
				ThoughtSignature: part.ThoughtSignature,
			})
		}
	}

	return r
}
