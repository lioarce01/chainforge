// Package chainforge re-exports core sentinel errors for convenience.
// Users can check errors without importing pkg/core directly.
//
//	if errors.Is(err, chainforge.ErrMaxIterations) { ... }
package chainforge

import (
	"errors"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Re-exported sentinel errors.
var (
	ErrMaxIterations = core.ErrMaxIterations
	ErrToolNotFound  = core.ErrToolNotFound
	ErrProviderError = core.ErrProviderError
	ErrNoProvider    = core.ErrNoProvider
	ErrNoModel       = core.ErrNoModel

	// ErrInvalidOutput is returned by RunWithUsage (and Run) when WithStructuredOutput
	// is configured and the LLM response does not match the declared JSON schema.
	ErrInvalidOutput = errors.New("agent: output does not match schema")
)
