package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/lioarce01/chainforge/pkg/orchestrator"
)

// unknownPicker always returns a route name that is not registered.
func unknownPicker(_ context.Context, _ string) (string, error) {
	return "nonexistent", nil
}

func TestRouterWithDefault_UsedOnUnknownRoute(t *testing.T) {
	defaultAgent := agentWith(t, "default response")
	otherAgent := agentWith(t, "other response")

	router := orchestrator.NewRouter(
		unknownPicker,
		orchestrator.RouteOf("other", "another agent", otherAgent),
		orchestrator.RouteOf("fallback", "fallback agent", defaultAgent),
	).WithDefault("fallback")

	result, err := router.Route(context.Background(), "sess", "anything")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if result != "default response" {
		t.Errorf("expected default response, got %q", result)
	}
}

func TestRouterWithDefault_NotUsedOnKnownRoute(t *testing.T) {
	knownAgent := agentWith(t, "known response")
	fallbackAgent := agentWith(t, "should not appear")

	router := orchestrator.NewRouter(
		func(_ context.Context, _ string) (string, error) { return "known", nil },
		orchestrator.RouteOf("known", "the real agent", knownAgent),
		orchestrator.RouteOf("fallback", "fallback", fallbackAgent),
	).WithDefault("fallback")

	result, err := router.Route(context.Background(), "sess", "input")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if result != "known response" {
		t.Errorf("expected known response, got %q", result)
	}
}

func TestRouterWithDefault_DefaultNameAlsoUnknown_ReturnsError(t *testing.T) {
	agent := agentWith(t, "x")
	router := orchestrator.NewRouter(
		unknownPicker,
		orchestrator.RouteOf("real", "real agent", agent),
	).WithDefault("also_nonexistent")

	_, err := router.Route(context.Background(), "sess", "input")
	if err == nil {
		t.Fatal("expected error when default route name is also unknown")
	}
}

func TestRouterWithDefault_NoDefault_OriginalError(t *testing.T) {
	agent := agentWith(t, "x")
	router := orchestrator.NewRouter(
		unknownPicker,
		orchestrator.RouteOf("real", "real agent", agent),
	)

	_, err := router.Route(context.Background(), "sess", "input")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown route") {
		t.Errorf("error = %q, want 'unknown route'", err.Error())
	}
}

func TestRouterWithDefault_ChainingReturnsRouter(t *testing.T) {
	agent := agentWith(t, "x")
	router := orchestrator.NewRouter(
		unknownPicker,
		orchestrator.RouteOf("r", "d", agent),
	)
	returned := router.WithDefault("r")
	if returned != router {
		t.Error("WithDefault should return the same *Router for chaining")
	}
}
