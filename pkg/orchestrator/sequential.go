package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"text/template"
)

// tmplCache is a package-level cache of compiled templates keyed by the raw
// template string. Goroutine-safe via sync.Map.
var tmplCache sync.Map // map[string]*template.Template

// Sequential runs a pipeline of Steps, passing each step's output as the next step's input.
// Each step runs in a namespaced session: "sessionID:step-N".
// Returns the final step's output.
func Sequential(ctx context.Context, sessionID string, input string, steps ...Step) (string, error) {
	previous := input

	for i, step := range steps {
		stepSessionID := fmt.Sprintf("%s:step-%d", sessionID, i)

		msg, err := renderTemplate(step.InputTmpl, StepData{
			Input:    input,
			Previous: previous,
		})
		if err != nil {
			return "", fmt.Errorf("step %q: render template: %w", step.Name, err)
		}

		output, err := step.Agent.Run(ctx, stepSessionID, msg)
		if err != nil {
			return "", fmt.Errorf("step %q: %w", step.Name, err)
		}

		previous = output
	}

	return previous, nil
}

// renderTemplate executes a text/template string with the given data.
// Templates use lowercase keys: {{.input}} and {{.previous}}.
// Parsed templates are cached by template string to avoid repeated parse overhead.
func renderTemplate(tmpl string, data StepData) (string, error) {
	if tmpl == "" {
		return data.Previous, nil
	}

	var t *template.Template
	if cached, ok := tmplCache.Load(tmpl); ok {
		t = cached.(*template.Template)
	} else {
		parsed, err := template.New("step").Parse(tmpl)
		if err != nil {
			return "", fmt.Errorf("parse template: %w", err)
		}
		// LoadOrStore is atomic: only one template wins the race; both are equivalent.
		actual, _ := tmplCache.LoadOrStore(tmpl, parsed)
		t = actual.(*template.Template)
	}

	// Use a map so templates can reference lowercase .input and .previous
	tmplData := map[string]string{
		"input":    data.Input,
		"previous": data.Previous,
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, tmplData); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
