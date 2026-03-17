package orchestrator

import (
	"context"
	"fmt"
	"strings"

	chainforge "github.com/lioarce01/chainforge"
)

// Route is a named agent destination registered in a Router.
type Route struct {
	Name        string             // unique identifier used to select this route
	Description string             // shown to the supervisor LLM to describe when to pick this route
	Agent       *chainforge.Agent
}

// RouteOf creates a Route. Description is shown to the supervisor LLM.
func RouteOf(name, description string, agent *chainforge.Agent) Route {
	return Route{Name: name, Description: description, Agent: agent}
}

// Router dispatches input to one of several named agents.
// Use NewRouter for programmatic routing or NewLLMRouter for AI-driven routing.
type Router struct {
	routes map[string]Route
	pick   func(ctx context.Context, input string) (string, error)
}

// NewRouter creates a Router with a custom picker function.
// pick receives the user input and returns the name of the route to use.
//
//	router := orchestrator.NewRouter(
//	    func(ctx context.Context, input string) (string, error) {
//	        if strings.Contains(input, "code") { return "coder", nil }
//	        return "general", nil
//	    },
//	    orchestrator.RouteOf("coder",   "writes and reviews code", coderAgent),
//	    orchestrator.RouteOf("general", "answers general questions", generalAgent),
//	)
func NewRouter(pick func(ctx context.Context, input string) (string, error), routes ...Route) *Router {
	m := make(map[string]Route, len(routes))
	for _, r := range routes {
		m[r.Name] = r
	}
	return &Router{routes: m, pick: pick}
}

// NewLLMRouter creates a Router that uses a supervisor agent to pick the route.
// The supervisor receives the input and a formatted list of available routes,
// and must respond with the exact route name. A concise system prompt is
// automatically injected into the session so the supervisor stays focused.
//
//	supervisor, _ := chainforge.NewAgent(
//	    chainforge.WithProvider(p),
//	    chainforge.WithModel("claude-haiku-4-5-20251001"),
//	    chainforge.WithSystemPrompt("You are a routing agent. Reply with only the route name."),
//	)
//	router := orchestrator.NewLLMRouter(supervisor,
//	    orchestrator.RouteOf("researcher", "searches and summarises information", researchAgent),
//	    orchestrator.RouteOf("coder",      "writes and debugs code",              coderAgent),
//	)
func NewLLMRouter(supervisor *chainforge.Agent, routes ...Route) *Router {
	m := make(map[string]Route, len(routes))
	for _, r := range routes {
		m[r.Name] = r
	}

	pick := func(ctx context.Context, input string) (string, error) {
		prompt := buildRoutingPrompt(input, routes)
		// Use a fixed session so the supervisor does not accumulate history
		// across unrelated routing decisions.
		resp, err := supervisor.Run(ctx, "router:supervisor", prompt)
		if err != nil {
			return "", fmt.Errorf("router: supervisor error: %w", err)
		}
		name := strings.TrimSpace(strings.ToLower(resp))
		// Strip any surrounding punctuation the LLM may add.
		name = strings.Trim(name, `"'.`)
		return name, nil
	}

	return &Router{routes: m, pick: pick}
}

// Route dispatches input to the selected agent and returns its response.
// sessionID is namespaced per route so each agent maintains its own history.
func (r *Router) Route(ctx context.Context, sessionID, input string) (string, error) {
	name, err := r.pick(ctx, input)
	if err != nil {
		return "", err
	}

	route, ok := r.routes[name]
	if !ok {
		names := make([]string, 0, len(r.routes))
		for k := range r.routes {
			names = append(names, k)
		}
		return "", fmt.Errorf("router: unknown route %q (available: %s)", name, strings.Join(names, ", "))
	}

	routeSessionID := fmt.Sprintf("%s:%s", sessionID, route.Name)
	return route.Agent.Run(ctx, routeSessionID, input)
}

// Routes returns the names of all registered routes.
func (r *Router) Routes() []string {
	names := make([]string, 0, len(r.routes))
	for k := range r.routes {
		names = append(names, k)
	}
	return names
}

// buildRoutingPrompt constructs the message sent to the supervisor LLM.
func buildRoutingPrompt(input string, routes []Route) string {
	var sb strings.Builder
	sb.WriteString("Available agents:\n")
	for _, r := range routes {
		fmt.Fprintf(&sb, "- %s: %s\n", r.Name, r.Description)
	}
	sb.WriteString("\nRespond with ONLY the agent name that should handle the following input.\n")
	sb.WriteString("Do not explain your choice. Reply with the exact name from the list above.\n\n")
	sb.WriteString("Input: ")
	sb.WriteString(input)
	return sb.String()
}
