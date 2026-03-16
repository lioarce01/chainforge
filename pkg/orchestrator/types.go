package orchestrator

import chainforge "github.com/lioarce01/chainforge"

// Step represents a single agent step in a sequential pipeline.
type Step struct {
	Name      string
	Agent     *chainforge.Agent
	InputTmpl string // text/template string; {{.input}} = first message, {{.previous}} = prior step output
}

// StepOf creates a new Step.
func StepOf(name string, agent *chainforge.Agent, inputTmpl string) Step {
	return Step{Name: name, Agent: agent, InputTmpl: inputTmpl}
}

// Fan represents one agent branch in a parallel execution.
type Fan struct {
	Name    string
	Agent   *chainforge.Agent
	Message string
}

// FanOf creates a new Fan branch.
func FanOf(name string, agent *chainforge.Agent, message string) Fan {
	return Fan{Name: name, Agent: agent, Message: message}
}

// ParallelResult holds the outcome of one Fan branch.
type ParallelResult struct {
	Name   string
	Output string
	Error  error
}

// StepData is the template context for step input rendering.
type StepData struct {
	Input    string // original input to Sequential
	Previous string // output of the previous step
}
