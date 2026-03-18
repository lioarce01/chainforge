package orchestrator

import (
	"context"
	"fmt"

	chainforge "github.com/lioarce01/chainforge"
)

// Conditional runs ifAgent when predicate(input) returns true, otherwise runs
// elseAgent. If elseAgent is nil and the predicate is false, input is returned
// unchanged. Sessions are namespaced as "sessionID:if" and "sessionID:else".
func Conditional(
	ctx context.Context,
	sessionID string,
	input string,
	predicate func(output string) bool,
	ifAgent *chainforge.Agent,
	elseAgent *chainforge.Agent,
) (string, error) {
	if predicate(input) {
		return ifAgent.Run(ctx, sessionID+":if", input)
	}
	if elseAgent == nil {
		return input, nil
	}
	return elseAgent.Run(ctx, sessionID+":else", input)
}

// Loop repeatedly runs agent while condition(iter, output) returns true, up to
// maxIter iterations. Each iteration receives the previous output as its input;
// the first iteration receives the original input. Sessions are namespaced as
// "sessionID:loop-N" per iteration.
//
// maxIter must be > 0. Returns an error if maxIter is exhausted before
// condition returns false.
func Loop(
	ctx context.Context,
	sessionID string,
	input string,
	agent *chainforge.Agent,
	condition func(iter int, output string) bool,
	maxIter int,
) (string, error) {
	if maxIter <= 0 {
		return "", fmt.Errorf("orchestrator: Loop maxIter must be > 0, got %d", maxIter)
	}

	current := input
	for i := 0; i < maxIter; i++ {
		if !condition(i, current) {
			return current, nil
		}

		stepSessionID := fmt.Sprintf("%s:loop-%d", sessionID, i)
		output, err := agent.Run(ctx, stepSessionID, current)
		if err != nil {
			return "", fmt.Errorf("loop iteration %d: %w", i, err)
		}
		current = output
	}

	return current, nil
}
