package provider

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"
	"time"
)

// Plan is a rendered command ready for direct process execution.
type Plan struct {
	Argv     []string
	Env      map[string]string
	Timeout  time.Duration
	Priority int
}

// Render returns an action plan. Each argument and value is one independent Go
// template; no result is reparsed by a shell.
func (m Manifest) Render(actionName string, data any) (Plan, error) {
	if err := m.Validate(); err != nil {
		return Plan{}, err
	}
	action, ok := m.Actions[actionName]
	if !ok {
		return Plan{}, fmt.Errorf("provider %q does not implement action %q", m.Name, actionName)
	}
	plan := Plan{
		Argv:     append([]string(nil), m.Command...),
		Env:      make(map[string]string, len(action.Env)),
		Timeout:  m.Defaults.Timeout.Duration(),
		Priority: m.Defaults.Priority,
	}
	for index, source := range action.Argv {
		value, err := renderTemplate(fmt.Sprintf("%s.argv[%d]", actionName, index), source, data)
		if err != nil {
			return Plan{}, err
		}
		plan.Argv = append(plan.Argv, value)
	}
	keys := make([]string, 0, len(action.Env))
	for key := range action.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := renderTemplate(actionName+".env."+key, action.Env[key], data)
		if err != nil {
			return Plan{}, err
		}
		plan.Env[key] = value
	}
	return plan, nil
}

func renderTemplate(name, source string, data any) (string, error) {
	parsed, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}
	return output.String(), nil
}
