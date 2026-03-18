package orchestrator

import chainforge "github.com/lioarce01/chainforge"

// Step represents a single agent step in a sequential pipeline.
type Step struct {
	Name      string
	Agent     *chainforge.Agent
	InputTmpl string // text/template string; {{.input}} = first message, {{.previous}} = prior step output
}

// StepOf creates a new Step. The optional inputTmpl argument is a text/template
// string that can reference {{.input}} (original input) and {{.previous}} (prior
// step output). When omitted, the previous step's output is passed verbatim.
//
//	orchestrator.StepOf("summarize", agent)                          // pass previous output as-is
//	orchestrator.StepOf("write", agent, "Write a post about: {{.previous}}")
func StepOf(name string, agent *chainforge.Agent, inputTmpl ...string) Step {
	tmpl := ""
	if len(inputTmpl) > 0 {
		tmpl = inputTmpl[0]
	}
	return Step{Name: name, Agent: agent, InputTmpl: tmpl}
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

// ParallelResults is the slice returned by Parallel.
// It provides convenience methods so callers don't need to write lookup loops.
type ParallelResults []ParallelResult

// Get returns the result for the named branch and whether it was found.
func (r ParallelResults) Get(name string) (ParallelResult, bool) {
	for _, pr := range r {
		if pr.Name == name {
			return pr, true
		}
	}
	return ParallelResult{}, false
}

// FirstError returns the first non-nil branch error, or nil if all succeeded.
func (r ParallelResults) FirstError() error {
	for _, pr := range r {
		if pr.Error != nil {
			return pr.Error
		}
	}
	return nil
}

// Outputs returns a map of branch name → output for all successful branches.
func (r ParallelResults) Outputs() map[string]string {
	m := make(map[string]string, len(r))
	for _, pr := range r {
		if pr.Error == nil {
			m[pr.Name] = pr.Output
		}
	}
	return m
}

// StepData is the template context for step input rendering.
type StepData struct {
	Input    string // original input to Sequential
	Previous string // output of the previous step
}
