package completion

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	command := nestedCommand()
	wants := map[string][]string{
		"bash": {
			"':inspect') context='inspect'",
			"'inspect:raw') context='inspect raw'",
			"local candidates='completion inspect --color -c'",
			"local candidates='raw --json'",
			"__sample_completion_values_0()",
			"'sample' 'values' '--kind' 'color'",
		},
		"fish": {
			`test (__sample_completion_context) = ""' -a inspect -d 'Inspect one record'`,
			`test (__sample_completion_context) = "inspect"' -a raw -d 'Inspect raw data'`,
			"-l color -s c -r -a '(__sample_completion_values_0)'",
			"command 'sample' 'values' '--kind' 'color'",
		},
		"nu": {
			`export extern "sample inspect"`,
			`export extern "sample inspect raw"`,
			`--color(-c): string@"__sample_completion_values_0" # Color's mode`,
			`...args: string@"__sample_completion_values_1"`,
			`run-external "sample" "values" "--kind" "color"`,
		},
		"zsh": {
			"':inspect') context='inspect'",
			"'inspect:raw') context='inspect raw'",
			"'1:command:(completion inspect)'",
			"'2:command:(raw)'",
			":value:__sample_completion_values_0",
			"'*:argument:__sample_completion_values_1'",
		},
	}
	rejects := map[string][]string{
		"bash": {"completion bash zsh fish nu inspect raw"},
		"fish": {`test (__sample_completion_context) = ""' -a raw`},
		"nu":   {`export extern "sample raw"`},
		"zsh":  {"'1:command:(completion inspect raw)'"},
	}
	for shell, expected := range wants {
		shell := shell
		expected := expected
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			out, err := Generate(shell, command)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "sample") || !strings.Contains(out, "color") || !strings.Contains(out, "inspect") || !strings.Contains(out, "raw") {
				t.Fatalf("completion lacks command data: %q", out)
			}
			for _, want := range expected {
				if !strings.Contains(out, want) {
					t.Fatalf("completion lacks %q:\n%s", want, out)
				}
			}
			for _, reject := range rejects[shell] {
				if strings.Contains(out, reject) {
					t.Fatalf("completion flattens nested context with %q:\n%s", reject, out)
				}
			}
		})
	}
}

func TestZshCombinesRootSubcommandsWithPositionalCompletion(t *testing.T) {
	generated, err := Generate("zsh", Command{
		Name:              "sample",
		CompletionCommand: []string{"sample", "values"},
		Subcommands:       []Command{{Name: "inspect"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"values=( 'completion' 'inspect')",
		"'sample' 'values'",
		"'*:argument:__sample_completion_values_0'",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("zsh completion lacks %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "'1:command:(completion inspect)'") {
		t.Fatalf("zsh root positional completion was shadowed by subcommands:\n%s", generated)
	}
}

func TestDynamicCompletionCanReceiveTheCurrentCommandLine(t *testing.T) {
	command := Command{
		Name: "sample",
		Flags: []Flag{{
			Name:              "model",
			Value:             true,
			CompletionCommand: []string{"sample", "models", ContextPlaceholder},
		}},
	}
	wants := map[string]string{
		"bash": `"${COMP_LINE:0:COMP_POINT}"`,
		"fish": `(commandline -cp)`,
		"nu":   `($context | default "")`,
		"zsh":  `"$BUFFER"`,
	}
	for shell, want := range wants {
		generated, err := Generate(shell, command)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(generated, want) || strings.Contains(generated, ContextPlaceholder) {
			t.Fatalf("%s completion did not render its context placeholder:\n%s", shell, generated)
		}
	}
}

func nestedCommand() Command {
	return Command{
		Name: "sample",
		Flags: []Flag{{
			Name:              "color",
			Short:             "c",
			Description:       "Color's mode",
			Value:             true,
			Values:            []string{"auto", "always", "never"},
			CompletionCommand: []string{"sample", "values", "--kind", "color"},
		}},
		Subcommands: []Command{{Name: "inspect", Description: "Inspect one record", Flags: []Flag{{Name: "json"}}, Subcommands: []Command{{
			Name:              "raw",
			Description:       "Inspect raw data",
			CompletionCommand: []string{"sample", "objects"},
		}}}},
	}
}

func TestGeneratedCompletionsParse(t *testing.T) {
	tests := map[string]struct {
		extension string
		arguments func(string) []string
	}{
		"bash": {extension: "bash", arguments: func(path string) []string { return []string{"-n", path} }},
		"fish": {extension: "fish", arguments: func(path string) []string { return []string{"-n", path} }},
		"nu": {
			extension: "nu",
			arguments: func(path string) []string {
				return []string{"--no-config-file", "--no-std-lib", "--commands", "source " + path}
			},
		},
		"zsh": {extension: "zsh", arguments: func(path string) []string { return []string{"-n", path} }},
	}
	for shell, test := range tests {
		shell := shell
		test := test
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s is unavailable", shell)
			}
			generated, err := Generate(shell, nestedCommand())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "completion."+test.extension)
			if err := os.WriteFile(path, []byte(generated), 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(executable, test.arguments(path)...)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("parse generated completion: %v\n%s", err, output)
			}
		})
	}
}

func TestGeneratedDynamicCompletionHelpers(t *testing.T) {
	directory := t.TempDir()
	provider := filepath.Join(directory, "provider-values")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\nprintf 'alpha\\nbeta\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := Command{
		Name: "fixture",
		Flags: []Flag{{
			Name:              "provider",
			Value:             true,
			Values:            []string{"static"},
			CompletionCommand: []string{provider},
		}},
	}
	tests := map[string]struct {
		extension string
		arguments func(string) []string
	}{
		"bash": {
			extension: "bash",
			arguments: func(path string) []string {
				return []string{"-c", `source "$1"; __fixture_completion_values_0`, "completion-test", path}
			},
		},
		"fish": {
			extension: "fish",
			arguments: func(path string) []string {
				return []string{"-c", `source $argv[1]; __fixture_completion_values_0`, path}
			},
		},
		"nu": {
			extension: "nu",
			arguments: func(path string) []string {
				return []string{"--no-config-file", "--no-std-lib", "--commands", "source " + quoteValue(path) + "; __fixture_completion_values_0 | to json --raw"}
			},
		},
		"zsh": {
			extension: "zsh",
			arguments: func(path string) []string {
				return []string{"-c", `compdef() { :; }; source "$1"; _describe() { print -l -- "${values[@]}"; }; __fixture_completion_values_0`, "completion-test", path}
			},
		},
	}
	for shell, test := range tests {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s is unavailable", shell)
			}
			generated, err := Generate(shell, command)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "completion."+test.extension)
			if err := os.WriteFile(path, []byte(generated), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(executable, test.arguments(path)...).CombinedOutput()
			if err != nil {
				t.Fatalf("run generated helper: %v\n%s", err, output)
			}
			for _, want := range []string{"static", "alpha", "beta"} {
				if !strings.Contains(string(output), want) {
					t.Fatalf("dynamic output lacks %q: %s", want, output)
				}
			}
		})
	}
}

func TestMarkdownAndReplaceSection(t *testing.T) {
	command := Command{
		Name: "tool", Description: "Legacy summary.", Synopsis: "Inspect work.", LongDescription: "Inspect current and historical work in one view.",
		Flags:       []Flag{{Name: "json", Short: "j", Description: "Print JSON | YAML", Value: true}},
		Subcommands: []Command{{Name: "show", Description: "Show one item.", Subcommands: []Command{{Name: "raw", Description: "Show raw data."}}}},
	}
	generated := Markdown(command)
	for _, want := range []string{"### `tool`", "Inspect work.", "Inspect current and historical work in one view.", "`--json`, `-j` `<value>`", "Print JSON \\| YAML", "### `tool show`", "### `tool show raw`"} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated Markdown does not contain %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "Legacy summary.") {
		t.Fatalf("generated Markdown ignored the synopsis override:\n%s", generated)
	}
	if !strings.Contains(generated, "Show one item.\n\n### `tool show raw`") {
		t.Fatalf("generated Markdown does not separate command sections:\n%s", generated)
	}
	if strings.Contains(generated, "\n\n\n") {
		t.Fatalf("generated Markdown contains excess section spacing:\n%s", generated)
	}
	document := `before
<!-- BEGIN GENERATED:cli -->
old
<!-- END GENERATED:cli -->
after
`
	got, err := ReplaceSection(document, "cli", generated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\nold\n") || !strings.Contains(got, generated) {
		t.Fatalf("replaced document:\n%s", got)
	}
}
