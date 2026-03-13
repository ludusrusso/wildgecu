package tool

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"gonesis/provider"
)

// --- Schema generation ---

func TestGenerateSchema_SimpleStruct(t *testing.T) {
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
}

func TestGenerateSchema_Omitempty(t *testing.T) {
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
}

func TestGenerateSchema_SkipDash(t *testing.T) {
	type Input struct {
		Name    string `json:"name"`
		Skipped string `json:"-"`
	}

	schema := generateSchema(reflect.TypeOf(Input{}))
	props := schema["properties"].(map[string]any)
	if len(props) != 1 {
		t.Fatalf("expected 1 property, got %d", len(props))
	}
}

func TestGenerateSchema_NestedStruct(t *testing.T) {
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
}

func TestGenerateSchema_Slice(t *testing.T) {
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
}

func TestGenerateSchema_Bool(t *testing.T) {
	type Input struct {
		Verbose bool `json:"verbose"`
	}

	schema := generateSchema(reflect.TypeOf(Input{}))
	props := schema["properties"].(map[string]any)
	verbProp := props["verbose"].(map[string]any)
	if verbProp["type"] != "boolean" {
		t.Fatalf("verbose type = %v, want boolean", verbProp["type"])
	}
}

// --- Tool execution ---

type EchoInput struct {
	Message string `json:"message" description:"The message to echo"`
}

type EchoOutput struct {
	Echo string `json:"echo"`
}

func TestNewTool_Definition(t *testing.T) {
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
}

func TestNewTool_Execute(t *testing.T) {
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
}

func TestNewTool_ExecuteWithError(t *testing.T) {
	failTool := NewTool("fail", "Always fails",
		func(ctx context.Context, in EchoInput) (EchoOutput, error) {
			return EchoOutput{}, provider.ErrDone
		},
	)

	_, err := failTool.Execute(context.Background(), map[string]any{"message": "hello"})
	if !errors.Is(err, provider.ErrDone) {
		t.Fatalf("expected ErrDone, got %v", err)
	}
}

// --- Registry ---

func TestRegistry_ToolsAndExecutor(t *testing.T) {
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
	result, err := executor(provider.ToolCall{
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
}

func TestRegistry_UnknownTool(t *testing.T) {
	reg := NewRegistry()
	executor := reg.Executor()

	result, err := executor(provider.ToolCall{Name: "nope", Args: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"error": "unknown tool: nope"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}
