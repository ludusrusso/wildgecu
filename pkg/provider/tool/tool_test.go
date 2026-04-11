package tool

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"wildgecu/pkg/provider"
)

// --- Schema generation ---

func TestGenerateSchema(t *testing.T) {
	t.Run("SimpleStruct", func(t *testing.T) {
		type Input struct {
			Name string `json:"name" description:"The user name"`
			Age  int    `json:"age" description:"The user age"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))

		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatal("expected properties map")
		}
		if len(props) != 2 {
			t.Fatalf("expected 2 properties, got %d", len(props))
		}

		nameProp := props["name"].(map[string]any)
		if nameProp["type"] != "string" {
			t.Fatalf("name type = %v, want string", nameProp["type"])
		}
		if nameProp["description"] != "The user name" {
			t.Fatalf("name description = %v", nameProp["description"])
		}

		ageProp := props["age"].(map[string]any)
		if ageProp["type"] != "number" {
			t.Fatalf("age type = %v, want number", ageProp["type"])
		}

		required := schema["required"].([]any)
		if len(required) != 2 {
			t.Fatalf("expected 2 required, got %d", len(required))
		}
	})

	t.Run("Omitempty", func(t *testing.T) {
		type Input struct {
			Name     string `json:"name"`
			Optional string `json:"optional,omitempty"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))

		required := schema["required"].([]any)
		if len(required) != 1 {
			t.Fatalf("expected 1 required, got %d", len(required))
		}
		if required[0] != "name" {
			t.Fatalf("required[0] = %v, want name", required[0])
		}
	})

	t.Run("SkipDash", func(t *testing.T) {
		type Input struct {
			Name    string `json:"name"`
			Skipped string `json:"-"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))
		props := schema["properties"].(map[string]any)
		if len(props) != 1 {
			t.Fatalf("expected 1 property, got %d", len(props))
		}
	})

	t.Run("NestedStruct", func(t *testing.T) {
		type Address struct {
			City string `json:"city"`
		}
		type Input struct {
			Addr Address `json:"addr"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))
		props := schema["properties"].(map[string]any)
		addrProp := props["addr"].(map[string]any)
		innerProps := addrProp["properties"].(map[string]any)
		if _, ok := innerProps["city"]; !ok {
			t.Fatal("expected nested city property")
		}
	})

	t.Run("Slice", func(t *testing.T) {
		type Input struct {
			Tags []string `json:"tags"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))
		props := schema["properties"].(map[string]any)
		tagsProp := props["tags"].(map[string]any)
		if tagsProp["type"] != "array" {
			t.Fatalf("tags type = %v, want array", tagsProp["type"])
		}
		items := tagsProp["items"].(map[string]any)
		if items["type"] != "string" {
			t.Fatalf("items type = %v, want string", items["type"])
		}
	})

	t.Run("Bool", func(t *testing.T) {
		type Input struct {
			Verbose bool `json:"verbose"`
		}

		schema := generateSchema(reflect.TypeOf(Input{}))
		props := schema["properties"].(map[string]any)
		verbProp := props["verbose"].(map[string]any)
		if verbProp["type"] != "boolean" {
			t.Fatalf("verbose type = %v, want boolean", verbProp["type"])
		}
	})
}

// --- Tool execution ---

type EchoInput struct {
	Message string `json:"message" description:"The message to echo"`
}

type EchoOutput struct {
	Echo string `json:"echo"`
}

func TestNewTool(t *testing.T) {
	t.Run("Definition", func(t *testing.T) {
		echoTool := NewTool("echo", "Echoes the message",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: in.Message}, nil
			},
		)

		def := echoTool.Definition()
		if def.Name != "echo" {
			t.Fatalf("name = %q, want echo", def.Name)
		}
		if def.Description != "Echoes the message" {
			t.Fatalf("description = %q", def.Description)
		}

		props := def.Parameters["properties"].(map[string]any)
		msgProp := props["message"].(map[string]any)
		if msgProp["type"] != "string" {
			t.Fatalf("message type = %v", msgProp["type"])
		}
	})

	t.Run("Execute", func(t *testing.T) {
		echoTool := NewTool("echo", "Echoes the message",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: in.Message}, nil
			},
		)

		result, err := echoTool.Execute(context.Background(), map[string]any{"message": "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out EchoOutput
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if out.Echo != "hello" {
			t.Fatalf("echo = %q, want hello", out.Echo)
		}
	})

	t.Run("ExecuteWithError", func(t *testing.T) {
		failTool := NewTool("fail", "Always fails",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{}, provider.ErrDone
			},
		)

		_, err := failTool.Execute(context.Background(), map[string]any{"message": "hello"})
		if !errors.Is(err, provider.ErrDone) {
			t.Fatalf("expected ErrDone, got %v", err)
		}
	})
}

// --- Registry ---

func TestRegistry(t *testing.T) {
	t.Run("ToolsAndExecutor", func(t *testing.T) {
		echoTool := NewTool("echo", "Echoes the message",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: in.Message}, nil
			},
		)

		reg := NewRegistry(echoTool)

		tools := reg.Tools()
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].Name != "echo" {
			t.Fatalf("tool name = %q", tools[0].Name)
		}

		executor := reg.Executor()
		result, err := executor(context.Background(), provider.ToolCall{
			Name: "echo",
			Args: map[string]any{"message": "world"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out EchoOutput
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.Echo != "world" {
			t.Fatalf("echo = %q, want world", out.Echo)
		}
	})

	t.Run("UnknownTool", func(t *testing.T) {
		reg := NewRegistry()
		executor := reg.Executor()

		result, err := executor(context.Background(), provider.ToolCall{Name: "nope", Args: map[string]any{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != `{"error": "unknown tool: nope"}` {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("SubsetHappyPath", func(t *testing.T) {
		alpha := NewTool("alpha", "first tool",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "alpha:" + in.Message}, nil
			},
		)
		beta := NewTool("beta", "second tool",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "beta:" + in.Message}, nil
			},
		)
		gamma := NewTool("gamma", "third tool",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "gamma:" + in.Message}, nil
			},
		)

		reg := NewRegistry(alpha, beta, gamma)
		sub := reg.Subset([]string{"gamma", "alpha"})

		tools := sub.Tools()
		if len(tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(tools))
		}
		// Insertion order from original registry: alpha before gamma
		if tools[0].Name != "alpha" {
			t.Fatalf("tools[0] = %q, want alpha", tools[0].Name)
		}
		if tools[1].Name != "gamma" {
			t.Fatalf("tools[1] = %q, want gamma", tools[1].Name)
		}
	})

	t.Run("SubsetUnknownNames", func(t *testing.T) {
		echo := NewTool("echo", "Echoes",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: in.Message}, nil
			},
		)

		reg := NewRegistry(echo)
		sub := reg.Subset([]string{"echo", "nonexistent", "also_missing"})

		tools := sub.Tools()
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].Name != "echo" {
			t.Fatalf("tool name = %q, want echo", tools[0].Name)
		}
	})

	t.Run("SubsetEmpty", func(t *testing.T) {
		echo := NewTool("echo", "Echoes",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: in.Message}, nil
			},
		)

		reg := NewRegistry(echo)
		sub := reg.Subset([]string{})

		tools := sub.Tools()
		if len(tools) != 0 {
			t.Fatalf("expected 0 tools, got %d", len(tools))
		}

		executor := sub.Executor()
		result, err := executor(context.Background(), provider.ToolCall{Name: "echo", Args: map[string]any{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != `{"error": "unknown tool: echo"}` {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("SubsetFullSet", func(t *testing.T) {
		alpha := NewTool("alpha", "first",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "a"}, nil
			},
		)
		beta := NewTool("beta", "second",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "b"}, nil
			},
		)

		reg := NewRegistry(alpha, beta)
		sub := reg.Subset([]string{"alpha", "beta"})

		tools := sub.Tools()
		if len(tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(tools))
		}
		if tools[0].Name != "alpha" {
			t.Fatalf("tools[0] = %q, want alpha", tools[0].Name)
		}
		if tools[1].Name != "beta" {
			t.Fatalf("tools[1] = %q, want beta", tools[1].Name)
		}
	})

	t.Run("SubsetExecutorDispatch", func(t *testing.T) {
		alpha := NewTool("alpha", "first",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "alpha:" + in.Message}, nil
			},
		)
		beta := NewTool("beta", "second",
			func(ctx context.Context, in EchoInput) (EchoOutput, error) {
				return EchoOutput{Echo: "beta:" + in.Message}, nil
			},
		)

		reg := NewRegistry(alpha, beta)
		sub := reg.Subset([]string{"alpha"})

		executor := sub.Executor()

		// Included tool works
		result, err := executor(context.Background(), provider.ToolCall{
			Name: "alpha",
			Args: map[string]any{"message": "hi"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out EchoOutput
		if unmarshalErr := json.Unmarshal([]byte(result), &out); unmarshalErr != nil {
			t.Fatalf("unmarshal: %v", unmarshalErr)
		}
		if out.Echo != "alpha:hi" {
			t.Fatalf("echo = %q, want alpha:hi", out.Echo)
		}

		// Excluded tool returns error
		result, err = executor(context.Background(), provider.ToolCall{
			Name: "beta",
			Args: map[string]any{"message": "hi"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != `{"error": "unknown tool: beta"}` {
			t.Fatalf("unexpected result: %s", result)
		}
	})
}
