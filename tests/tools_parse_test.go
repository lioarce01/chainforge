package tests

import (
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/tools"
)

type parseTarget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestParseInput_HappyPath(t *testing.T) {
	got, err := tools.ParseInput[parseTarget](`{"name":"hello","count":3}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "hello" || got.Count != 3 {
		t.Errorf("got %+v, want {Name:hello Count:3}", got)
	}
}

func TestParseInput_EmptyInput(t *testing.T) {
	_, err := tools.ParseInput[parseTarget](``)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, err := tools.ParseInput[parseTarget](`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "tools: invalid input") {
		t.Errorf("error = %q, want 'tools: invalid input' prefix", err.Error())
	}
}

func TestParseInput_MissingOptionalField(t *testing.T) {
	// count is not required; should default to zero value
	got, err := tools.ParseInput[parseTarget](`{"name":"only"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Count != 0 {
		t.Errorf("Count = %d, want 0", got.Count)
	}
}

func TestParseInput_ExtraFields(t *testing.T) {
	// extra fields are ignored by json.Unmarshal
	got, err := tools.ParseInput[parseTarget](`{"name":"x","count":1,"unknown":"y"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "x" {
		t.Errorf("Name = %q, want %q", got.Name, "x")
	}
}

func TestParseInput_WrongType(t *testing.T) {
	// count field expects int but receives a string
	_, err := tools.ParseInput[parseTarget](`{"name":"x","count":"notanint"}`)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}
