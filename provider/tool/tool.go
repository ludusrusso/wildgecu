package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"wildgecu/provider"
)

// Tool is the interface that all typed tools implement.
type Tool interface {
	Definition() provider.Tool
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// typedTool is the generic implementation of Tool.
type typedTool[In any, Out any] struct {
	name        string
	description string
	handler     func(ctx context.Context, in In) (Out, error)
	schema      map[string]any
}

// NewTool creates a new typed tool. JSON Schema is generated from In's struct tags.
func NewTool[In any, Out any](
	name string,
	description string,
	handler func(ctx context.Context, in In) (Out, error),
) Tool {
	var zero In
	schema := generateSchema(reflect.TypeOf(zero))
	return &typedTool[In, Out]{
		name:        name,
		description: description,
		handler:     handler,
		schema:      schema,
	}
}

func (t *typedTool[In, Out]) Definition() provider.Tool {
	return provider.Tool{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.schema,
	}
}

func (t *typedTool[In, Out]) Execute(ctx context.Context, args map[string]any) (string, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error()), nil
	}

	var in In
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error()), nil
	}

	out, err := t.handler(ctx, in)
	if err != nil {
		// Marshal whatever output we have before returning the error.
		b, _ := json.Marshal(out)
		return string(b), err
	}

	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error()), nil
	}
	return string(b), nil
}
