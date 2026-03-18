// Package structured provides JSON output validation helpers for chainforge agents.
package structured

import (
	"encoding/json"
	"fmt"
)

// ValidateJSON checks that data is valid JSON. If schema is non-nil it also
// checks that the top-level JSON type (object, array, string, number, boolean)
// matches the "type" field declared in schema, if present.
//
// This is a lightweight check — it does not perform full JSON Schema Draft-07
// validation. Use it to catch obviously malformed LLM responses.
func ValidateJSON(data string, schema json.RawMessage) error {
	if !json.Valid([]byte(data)) {
		return fmt.Errorf("output is not valid JSON")
	}
	if len(schema) == 0 {
		return nil
	}

	// Extract the expected "type" from the schema (if present).
	var schemaMeta struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(schema, &schemaMeta); err != nil || schemaMeta.Type == "" {
		// Schema has no parseable "type" field — accept any valid JSON.
		return nil
	}

	// Determine the actual JSON type of data.
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return fmt.Errorf("output is not valid JSON: %w", err)
	}
	actual := jsonType(raw)
	if actual != schemaMeta.Type {
		return fmt.Errorf("output type %q does not match schema type %q", actual, schemaMeta.Type)
	}
	return nil
}

// jsonType returns the JSON Schema type name for a raw JSON value.
func jsonType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	switch raw[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		// numeric
		return "number"
	}
}
