package orchestrator

import (
	"context"
	"sync"
)

// Parallel runs all Fan branches concurrently.
// It always returns all results — it never cancels siblings on failure.
// The top-level error is always nil; check result.Error or results.FirstError() per branch.
func Parallel(ctx context.Context, sessionID string, fans ...Fan) (ParallelResults, error) {
	results := make(ParallelResults, len(fans))
	var wg sync.WaitGroup

	for i, fan := range fans {
		wg.Add(1)
		go func(idx int, f Fan) {
			defer wg.Done()
			fanSessionID := sessionID + ":" + f.Name
			output, err := f.Agent.Run(ctx, fanSessionID, f.Message)
			results[idx] = ParallelResult{
				Name:   f.Name,
				Output: output,
				Error:  err,
			}
		}(i, fan)
	}

	wg.Wait()
	return results, nil
}
