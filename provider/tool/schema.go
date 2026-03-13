package tool

import (
	"reflect"
	"strings"
)

// generateSchema builds a JSON Schema (as map[string]any) from the struct
// tags of the given type. It reads `json` tags for field names and
// `description` tags for field descriptions. Fields without `omitempty`
// in their json tag are added to `required`.
func generateSchema(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	properties := map[string]any{}
	var required []any

	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}

		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}

		name, opts := parseJSONTag(tag)
		if name == "" {
			name = f.Name
		}

		prop := schemaForType(f.Type)

		if desc := f.Tag.Get("description"); desc != "" {
			prop["description"] = desc
		}

		properties[name] = prop

		if !strings.Contains(opts, "omitempty") {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func schemaForType(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "number"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Struct:
		return generateSchema(t)
	default:
		return map[string]any{"type": "string"}
	}
}

// parseJSONTag splits a json tag into name and remaining options.
func parseJSONTag(tag string) (string, string) {
	if before, after, found := strings.Cut(tag, ","); found {
		return before, after
	}
	return tag, ""
}
