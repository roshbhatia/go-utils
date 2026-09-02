package completion

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	command := Command{
		Name:        "sample",
		Flags:       []Flag{{Name: "color", Value: true, Values: []string{"auto", "always", "never"}}},
		Subcommands: []Command{{Name: "inspect", Description: "Inspect one record", Flags: []Flag{{Name: "json"}}, Subcommands: []Command{{Name: "raw", Description: "Inspect raw data"}}}},
	}
	for _, shell := range []string{"bash", "zsh", "fish", "nu"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			out, err := Generate(shell, command)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "sample") || !strings.Contains(out, "color") || !strings.Contains(out, "inspect") || !strings.Contains(out, "raw") {
				t.Fatalf("completion lacks command data: %q", out)
			}
		})
	}
}

func TestMarkdownAndReplaceSection(t *testing.T) {
	command := Command{
		Name: "tool", Description: "Inspect work.",
		Flags:       []Flag{{Name: "json", Description: "Print JSON"}},
		Subcommands: []Command{{Name: "show", Description: "Show one item.", Subcommands: []Command{{Name: "raw", Description: "Show raw data."}}}},
	}
	generated := Markdown(command)
	for _, want := range []string{"### `tool`", "`--json`", "### `tool show`", "### `tool show raw`"} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated Markdown does not contain %q:\n%s", want, generated)
		}
	}
	document := "before\n<!-- BEGIN GENERATED:cli -->\nold\n<!-- END GENERATED:cli -->\nafter\n"
	got, err := ReplaceSection(document, "cli", generated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\nold\n") || !strings.Contains(got, generated) {
		t.Fatalf("replaced document:\n%s", got)
	}
}
