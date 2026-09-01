package completion

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	command := Command{Name: "sample", Flags: []Flag{{Name: "color", Value: true, Values: []string{"auto", "always", "never"}}}}
	for _, shell := range []string{"bash", "zsh", "fish", "nu"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			out, err := Generate(shell, command)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "sample") || !strings.Contains(out, "color") {
				t.Fatalf("completion lacks command data: %q", out)
			}
		})
	}
}
