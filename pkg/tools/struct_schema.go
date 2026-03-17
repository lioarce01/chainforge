package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SchemaFromStruct generates a JSON schema object from a Go struct using field tags.
//
// Supported tags:
//   - json:"fieldName"          → property name (falls back to field name; skipped if "-")
//   - cf:"required"             → marks the field as required
//   - cf:"description=My desc"  → sets the property description
//   - cf:"enum=a|b|c"           → restricts the value to listed items
//
// Multiple cf directives can be combined with commas: cf:"required,description=The query".
//
// Go type → JSON schema type mapping:
//   - string              → "string"
//   - int*, uint*         → "integer"
//   - float32, float64    → "number"
//   - bool                → "boolean"
//   - slice, array        → "array"
//   - everything else     → "object"
//
// Pointer types are dereferenced one level. Unexported fields are skipped.
// Returns an error if T is not a struct.
func SchemaFromStruct[T any]() (json.RawMessage, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("tools: SchemaFromStruct: T must be a struct, got %s", t.Kind())
	}

	type property struct {
		Type        string   `json:"type"`
		Description string   `json:"description,omitempty"`
		Enum        []string `json:"enum,omitempty"`
	}
	type schema struct {
		Type       string              `json:"type"`
		Properties map[string]property `json:"properties"`
		Required   []string            `json:"required,omitempty"`
	}

	sc := schema{
		Type:       "object",
		Properties: make(map[string]property),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		// Determine property name from json tag.
		name := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.SplitN(jsonTag, ",", 2)
			if parts[0] != "" && parts[0] != "-" {
				name = parts[0]
			}
		}

		// Parse cf tag directives.
		var (
			required    bool
			description string
			enumValues  []string
		)
		if cfTag := field.Tag.Get("cf"); cfTag != "" {
			for _, directive := range strings.Split(cfTag, ",") {
				directive = strings.TrimSpace(directive)
				switch {
				case directive == "required":
					required = true
				case strings.HasPrefix(directive, "description="):
					description = strings.TrimPrefix(directive, "description=")
				case strings.HasPrefix(directive, "enum="):
					enumStr := strings.TrimPrefix(directive, "enum=")
					enumValues = strings.Split(enumStr, "|")
				}
			}
		}

		// Resolve JSON schema type.
		ft := field.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}

		prop := property{
			Type:        goKindToJSONType(ft.Kind()),
			Description: description,
			Enum:        enumValues,
		}
		sc.Properties[name] = prop
		if required {
			sc.Required = append(sc.Required, name)
		}
	}

	b, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("tools: SchemaFromStruct: marshal: %w", err)
	}
	return b, nil
}

// MustSchemaFromStruct is like SchemaFromStruct but panics on error.
// Use only in init() or test setup where a non-struct T is a programmer error.
func MustSchemaFromStruct[T any]() json.RawMessage {
	b, err := SchemaFromStruct[T]()
	if err != nil {
		panic(err)
	}
	return b
}

func goKindToJSONType(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "object"
	}
}
