// Package chainforge re-exports core sentinel errors for convenience.
// Users can check errors without importing pkg/core directly.
//
//	if errors.Is(err, chainforge.ErrMaxIterations) { ... }
package chainforge

import "github.com/lioarce01/chainforge/pkg/core"

// Re-exported sentinel errors.
var (
	ErrMaxIterations = core.ErrMaxIterations
	ErrToolNotFound  = core.ErrToolNotFound
	ErrProviderError = core.ErrProviderError
	ErrNoProvider    = core.ErrNoProvider
	ErrNoModel       = core.ErrNoModel
)
