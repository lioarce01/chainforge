package tests

import (
	"encoding/json"
	"testing"

	"github.com/lioarce01/chainforge/pkg/tools"
)

func schemaToMap(t *testing.T, s *tools.Schema) map[string]interface{} {
	t.Helper()
	raw := s.MustBuild()
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return m
}

func propType(t *testing.T, m map[string]interface{}, name string) string {
	t.Helper()
	props := m["properties"].(map[string]interface{})
	prop := props[name].(map[string]interface{})
	return prop["type"].(string)
}

func TestAddString_CorrectType(t *testing.T) {
	s := tools.NewSchema().AddString("q", "a query", false)
	m := schemaToMap(t, s)
	if got := propType(t, m, "q"); got != "string" {
		t.Errorf("type = %q, want string", got)
	}
}

func TestAddInt_CorrectType(t *testing.T) {
	s := tools.NewSchema().AddInt("n", "a number", false)
	m := schemaToMap(t, s)
	if got := propType(t, m, "n"); got != "integer" {
		t.Errorf("type = %q, want integer", got)
	}
}

func TestAddNumber_CorrectType(t *testing.T) {
	s := tools.NewSchema().AddNumber("f", "a float", false)
	m := schemaToMap(t, s)
	if got := propType(t, m, "f"); got != "number" {
		t.Errorf("type = %q, want number", got)
	}
}

func TestAddBool_CorrectType(t *testing.T) {
	s := tools.NewSchema().AddBool("ok", "a flag", false)
	m := schemaToMap(t, s)
	if got := propType(t, m, "ok"); got != "boolean" {
		t.Errorf("type = %q, want boolean", got)
	}
}

func TestAddString_RequiredPropagates(t *testing.T) {
	s := tools.NewSchema().AddString("q", "required query", true)
	m := schemaToMap(t, s)
	req, ok := m["required"].([]interface{})
	if !ok || len(req) == 0 {
		t.Fatal("expected required array to contain 'q'")
	}
	if req[0].(string) != "q" {
		t.Errorf("required[0] = %q, want %q", req[0], "q")
	}
}

func TestAddBool_NotRequired(t *testing.T) {
	s := tools.NewSchema().AddBool("verbose", "verbosity flag", false)
	m := schemaToMap(t, s)
	if _, ok := m["required"]; ok {
		t.Error("expected no required field when required=false")
	}
}

func TestAllShorthandsMixed(t *testing.T) {
	s := tools.NewSchema().
		AddString("query", "search query", true).
		AddInt("limit", "max results", false).
		AddNumber("threshold", "min score", false).
		AddBool("verbose", "verbose mode", false)

	m := schemaToMap(t, s)
	props := m["properties"].(map[string]interface{})
	if len(props) != 4 {
		t.Errorf("expected 4 properties, got %d", len(props))
	}
	if propType(t, m, "query") != "string" {
		t.Error("query should be string")
	}
	if propType(t, m, "limit") != "integer" {
		t.Error("limit should be integer")
	}
	if propType(t, m, "threshold") != "number" {
		t.Error("threshold should be number")
	}
	if propType(t, m, "verbose") != "boolean" {
		t.Error("verbose should be boolean")
	}
}
