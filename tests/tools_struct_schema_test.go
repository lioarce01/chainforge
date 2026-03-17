package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/tools"
)

func parseStructSchema(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func structPropType(t *testing.T, m map[string]interface{}, name string) string {
	t.Helper()
	props := m["properties"].(map[string]interface{})
	prop, ok := props[name].(map[string]interface{})
	if !ok {
		t.Fatalf("property %q not found", name)
	}
	return prop["type"].(string)
}

// --- Test structs ---

type BasicStruct struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Admin bool   `json:"admin"`
}

type RequiredStruct struct {
	Query string `json:"query" cf:"required"`
	Limit int    `json:"limit"`
}

type DescStruct struct {
	Query string `json:"query" cf:"description=The search query"`
}

type EnumStruct struct {
	Color string `json:"color" cf:"enum=red|green|blue"`
}

type TagNameStruct struct {
	MyField string `json:"my_field"`
}

type UnexportedStruct struct {
	Public  string `json:"public"`
	private string //nolint:unused
}

type MultiCFStruct struct {
	Query string `json:"query" cf:"required,description=The query,enum=foo|bar"`
}

type IntTypesStruct struct {
	A int8  `json:"a"`
	B int64 `json:"b"`
	C uint  `json:"c"`
}

type FloatStruct struct {
	Score float64 `json:"score"`
}

type SliceStruct struct {
	Tags []string `json:"tags"`
}

// --- Tests ---

func TestSchemaFromStruct_BasicFields(t *testing.T) {
	raw, err := tools.SchemaFromStruct[BasicStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	if structPropType(t, m, "name") != "string" {
		t.Error("name should be string")
	}
	if structPropType(t, m, "age") != "integer" {
		t.Error("age should be integer")
	}
	if structPropType(t, m, "admin") != "boolean" {
		t.Error("admin should be boolean")
	}
}

func TestSchemaFromStruct_RequiredTag(t *testing.T) {
	raw, err := tools.SchemaFromStruct[RequiredStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	req, ok := m["required"].([]interface{})
	if !ok {
		t.Fatal("expected required array")
	}
	found := false
	for _, v := range req {
		if v.(string) == "query" {
			found = true
		}
	}
	if !found {
		t.Error("query should be in required")
	}
}

func TestSchemaFromStruct_DescriptionTag(t *testing.T) {
	raw, err := tools.SchemaFromStruct[DescStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	props := m["properties"].(map[string]interface{})
	prop := props["query"].(map[string]interface{})
	if prop["description"] != "The search query" {
		t.Errorf("description = %v, want 'The search query'", prop["description"])
	}
}

func TestSchemaFromStruct_EnumTag(t *testing.T) {
	raw, err := tools.SchemaFromStruct[EnumStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	props := m["properties"].(map[string]interface{})
	prop := props["color"].(map[string]interface{})
	enum, ok := prop["enum"].([]interface{})
	if !ok || len(enum) != 3 {
		t.Fatalf("enum = %v, want [red green blue]", prop["enum"])
	}
	if enum[0] != "red" || enum[1] != "green" || enum[2] != "blue" {
		t.Errorf("enum values = %v", enum)
	}
}

func TestSchemaFromStruct_JSONTagName(t *testing.T) {
	raw, err := tools.SchemaFromStruct[TagNameStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	props := m["properties"].(map[string]interface{})
	if _, ok := props["my_field"]; !ok {
		t.Error("expected property named 'my_field'")
	}
	if _, ok := props["MyField"]; ok {
		t.Error("should not have property 'MyField'")
	}
}

func TestSchemaFromStruct_UnexportedFieldsIgnored(t *testing.T) {
	raw, err := tools.SchemaFromStruct[UnexportedStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	props := m["properties"].(map[string]interface{})
	if _, ok := props["private"]; ok {
		t.Error("unexported field 'private' should be skipped")
	}
	if _, ok := props["public"]; !ok {
		t.Error("exported field 'public' should be present")
	}
}

func TestSchemaFromStruct_NonStructError(t *testing.T) {
	_, err := tools.SchemaFromStruct[string]()
	if err == nil {
		t.Fatal("expected error for non-struct type")
	}
	if !strings.Contains(err.Error(), "SchemaFromStruct") {
		t.Errorf("error = %q, want 'SchemaFromStruct' in message", err.Error())
	}
}

func TestSchemaFromStruct_MultipleCFDirectives(t *testing.T) {
	raw, err := tools.SchemaFromStruct[MultiCFStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	props := m["properties"].(map[string]interface{})
	prop := props["query"].(map[string]interface{})
	if prop["description"] != "The query" {
		t.Errorf("description = %v", prop["description"])
	}
	enum := prop["enum"].([]interface{})
	if len(enum) != 2 {
		t.Errorf("enum = %v, want [foo bar]", enum)
	}
	req := m["required"].([]interface{})
	found := false
	for _, v := range req {
		if v.(string) == "query" {
			found = true
		}
	}
	if !found {
		t.Error("query should be required")
	}
}

func TestMustSchemaFromStruct_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-struct type")
		}
	}()
	tools.MustSchemaFromStruct[int]()
}

func TestSchemaFromStruct_IntTypes(t *testing.T) {
	raw, err := tools.SchemaFromStruct[IntTypesStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	for _, name := range []string{"a", "b", "c"} {
		if structPropType(t, m, name) != "integer" {
			t.Errorf("field %q: expected integer", name)
		}
	}
}

func TestSchemaFromStruct_Float64(t *testing.T) {
	raw, err := tools.SchemaFromStruct[FloatStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	if structPropType(t, m, "score") != "number" {
		t.Error("float64 should map to 'number'")
	}
}

func TestSchemaFromStruct_Slice(t *testing.T) {
	raw, err := tools.SchemaFromStruct[SliceStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseStructSchema(t, raw)
	if structPropType(t, m, "tags") != "array" {
		t.Error("[]string should map to 'array'")
	}
}
