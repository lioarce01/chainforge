package tools

import (
	"encoding/json"
	"fmt"
)

// ParseInput unmarshals a JSON tool input string into a typed value T.
// It is the public equivalent of the private parseToolInput helper used inside the agent.
//
//	type SearchInput struct {
//	    Query string `json:"query"`
//	    Limit int    `json:"limit"`
//	}
//
//	input, err := tools.ParseInput[SearchInput](rawInput)
func ParseInput[T any](input string) (T, error) {
	var v T
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return v, fmt.Errorf("tools: invalid input: %w", err)
	}
	return v, nil
}
