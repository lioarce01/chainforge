package tools

import (
	"encoding/json"
	"fmt"
)

// PropertyType represents a JSON schema property type.
type PropertyType string

const (
	TypeString  PropertyType = "string"
	TypeNumber  PropertyType = "number"
	TypeInteger PropertyType = "integer"
	TypeBoolean PropertyType = "boolean"
	TypeObject  PropertyType = "object"
	TypeArray   PropertyType = "array"
)

// Property describes a single JSON schema property.
type Property struct {
	Type        PropertyType `json:"type"`
	Description string       `json:"description,omitempty"`
	Enum        []string     `json:"enum,omitempty"`
}

// Schema builds a JSON schema object for tool input definitions.
type Schema struct {
	properties map[string]Property
	required   []string
}

// NewSchema creates a new schema builder.
func NewSchema() *Schema {
	return &Schema{properties: make(map[string]Property)}
}

// Add adds a property to the schema.
func (s *Schema) Add(name string, prop Property, required bool) *Schema {
	s.properties[name] = prop
	if required {
		s.required = append(s.required, name)
	}
	return s
}

// Build serialises the schema to json.RawMessage.
func (s *Schema) Build() (json.RawMessage, error) {
	type schema struct {
		Type       string              `json:"type"`
		Properties map[string]Property `json:"properties"`
		Required   []string            `json:"required,omitempty"`
	}
	sc := schema{
		Type:       "object",
		Properties: s.properties,
		Required:   s.required,
	}
	b, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("tools: schema marshal: %w", err)
	}
	return b, nil
}

// MustBuild is like Build but panics on error (use only in init/test).
func (s *Schema) MustBuild() json.RawMessage {
	b, err := s.Build()
	if err != nil {
		panic(err)
	}
	return b
}
