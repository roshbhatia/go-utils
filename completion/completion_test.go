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
			"compgen -W 'completion inspect --color -c'",
			"compgen -W 'raw --json'",
		},
		"fish": {
			`test (__sample_completion_context) = ""' -a inspect -d 'Inspect one record'`,
			`test (__sample_completion_context) = "inspect"' -a raw -d 'Inspect raw data'`,
			"-l color -s c -r -a 'auto always never'",
		},
		"nu": {
			`export extern "sample inspect"`,
			`export extern "sample inspect raw"`,
			"--color(-c): string # Color's mode",
		},
		"zsh": {
			"':inspect') context='inspect'",
			"'inspect:raw') context='inspect raw'",
			"'1:command:(completion inspect)'",
			"'2:command:(raw)'",
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

func nestedCommand() Command {
	return Command{
		Name: "sample",
		Flags: []Flag{{
			Name:        "color",
			Short:       "c",
			Description: "Color's mode",
			Value:       true,
			Values:      []string{"auto", "always", "never"},
		}},
		Subcommands: []Command{{Name: "inspect", Description: "Inspect one record", Flags: []Flag{{Name: "json"}}, Subcommands: []Command{{Name: "raw", Description: "Inspect raw data"}}}},
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
