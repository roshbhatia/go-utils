// Package provider discovers, validates, renders, and invokes external providers.
//
// Providers are ordinary executables. The package never inserts a shell between
// a caller and a provider command.
package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

const Version = "provider/v1"

var (
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	envPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Duration is a human-readable duration in a provider manifest.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *Duration) UnmarshalText(value []byte) error {
	parsed, err := time.ParseDuration(string(value))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (Duration) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Pattern:     `^[0-9]+(ns|us|µs|ms|s|m|h)$`,
		Description: "Go duration such as 500ms, 10s, or 2m",
	}
}

// Manifest describes one external provider and the actions it implements.
type Manifest struct {
	Version     string            `json:"version" yaml:"version" jsonschema:"enum=provider/v1"`
	Name        string            `json:"name" yaml:"name" jsonschema:"pattern=^[a-z][a-z0-9._-]*$"`
	Description string            `json:"description" yaml:"description"`
	Command     []string          `json:"command" yaml:"command" jsonschema:"minItems=1"`
	Actions     map[string]Action `json:"actions" yaml:"actions"`
	Requires    Requirements      `json:"requires,omitempty" yaml:"requires,omitempty"`
	Defaults    Defaults          `json:"defaults,omitempty" yaml:"defaults,omitempty"`
}

// Action describes one capability and its action-specific arguments and environment.
type Action struct {
	Description string            `json:"description" yaml:"description"`
	Argv        []string          `json:"argv,omitempty" yaml:"argv,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// Requirements declares dependencies which must exist before invocation.
type Requirements struct {
	Commands    []string `json:"commands,omitempty" yaml:"commands,omitempty"`
	Environment []string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Paths       []string `json:"paths,omitempty" yaml:"paths,omitempty"`
}

// Defaults contains provider-wide execution policy.
type Defaults struct {
	Timeout  Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Priority int      `json:"priority,omitempty" yaml:"priority,omitempty"`
}

// Validate checks the manifest without inspecting the host environment.
func (m Manifest) Validate() error {
	var problems []error
	if m.Version != Version {
		problems = append(problems, fmt.Errorf("version must be %q", Version))
	}
	if !namePattern.MatchString(m.Name) {
		problems = append(problems, errors.New("name must match ^[a-z][a-z0-9._-]*$"))
	}
	if strings.TrimSpace(m.Description) == "" {
		problems = append(problems, errors.New("description is required"))
	}
	if len(m.Command) == 0 {
		problems = append(problems, errors.New("command must contain an executable"))
	}
	for index, argument := range m.Command {
		if argument == "" {
			problems = append(problems, fmt.Errorf("command[%d] must not be empty", index))
		}
	}
	if len(m.Actions) == 0 {
		problems = append(problems, errors.New("actions must not be empty"))
	}
	for name, action := range m.Actions {
		if !namePattern.MatchString(name) {
			problems = append(problems, fmt.Errorf("action %q has an invalid name", name))
		}
		if strings.TrimSpace(action.Description) == "" {
			problems = append(problems, fmt.Errorf("action %q description is required", name))
		}
		for key := range action.Env {
			if !envPattern.MatchString(key) {
				problems = append(problems, fmt.Errorf("action %q environment key %q is invalid", name, key))
			}
		}
	}
	if m.Defaults.Timeout.Duration() < 0 {
		problems = append(problems, errors.New("defaults.timeout must not be negative"))
	}
	for _, name := range m.Requires.Commands {
		if strings.TrimSpace(name) == "" {
			problems = append(problems, errors.New("requires.commands must not contain an empty value"))
		}
	}
	for _, name := range m.Requires.Environment {
		if !envPattern.MatchString(name) {
			problems = append(problems, fmt.Errorf("required environment name %q is invalid", name))
		}
	}
	for _, path := range m.Requires.Paths {
		if strings.TrimSpace(path) == "" {
			problems = append(problems, errors.New("requires.paths must not contain an empty value"))
		}
	}
	return errors.Join(problems...)
}

// Schema returns the provider manifest JSON Schema.
func Schema() ([]byte, error) {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
	schema := reflector.Reflect(new(Manifest))
	schema.Title = "External provider manifest"
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode provider schema: %w", err)
	}
	return append(data, '\n'), nil
}
