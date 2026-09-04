package completion

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
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
			"printf '%s\\n' 'completion' 'inspect' '--color' '-c'",
			"printf '%s\\n' 'raw' '--json'",
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

func TestTextRendersRuntimeHelpFromCommandMetadata(t *testing.T) {
	t.Parallel()

	got := Text(Command{
		Name:            "sample",
		Description:     "Legacy summary.",
		LongDescription: "Inspect current work.",
		Synopsis:        "sample [--color] [name]",
		Flags: []Flag{
			{Name: "color", Short: "c", Description: "Select a color", Value: true},
			{Name: "help", Short: "h", Description: "Print help"},
		},
		Subcommands: []Command{{Name: "inspect", Description: "Inspect one record"}},
	})
	for _, want := range []string{
		"Inspect current work.",
		"Usage:\n  sample [--color] [name]",
		"Commands:\n  inspect  Inspect one record",
		"Options:\n  -c, --color <value>  Select a color",
		"  -h, --help  Print help",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Text() lacks %q:\n%s", want, got)
		}
	}
}

func TestGenerateDoesNotMutateCommandMetadata(t *testing.T) {
	t.Parallel()

	command := nestedCommand()
	want := append([]Flag(nil), command.Flags...)
	if _, err := Generate("bash", command); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(command.Flags, want) {
		t.Fatalf("Generate() mutated flags: got %+v, want %+v", command.Flags, want)
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
		"zsh":  `"${BUFFER[1,CURSOR]}"`,
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
			executable := completionShell(t, shell)
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
				return []string{"-c", `compdef() { :; }; source "$1"; compadd() { print -l -- "${values[@]}"; }; __fixture_completion_values_0`, "completion-test", path}
			},
		},
	}
	for shell, test := range tests {
		t.Run(shell, func(t *testing.T) {
			executable := completionShell(t, shell)
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

func TestBashTreatsDynamicCandidatesAsData(t *testing.T) {
	directory := t.TempDir()
	sideEffect := filepath.Join(directory, "unexpected-side-effect")
	provider := filepath.Join(directory, "provider-values")
	dynamic := []string{
		"dynamic value",
		"dynamic'quote",
		"$HOME",
		"$(touch " + sideEffect + ")",
	}
	providerSource := "#!/bin/sh\nprintf '%s\\n'"
	for _, candidate := range dynamic {
		providerSource += " " + quoteShell(candidate)
	}
	providerSource += "\n"
	if err := os.WriteFile(provider, []byte(providerSource), 0o700); err != nil {
		t.Fatal(err)
	}
	command := Command{
		Name: "fixture",
		Flags: []Flag{{
			Name:              "provider",
			Value:             true,
			Values:            []string{"static value", "static'quote"},
			CompletionCommand: []string{provider},
		}},
	}
	got := runBashCompletion(t, command, "fixture --provider ", "fixture", "--provider", "")
	want := append([]string{"static value", "static'quote"}, dynamic...)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("bash completion changed candidate records:\n got: %#v\nwant: %#v", got, want)
	}
	equals := runBashCompletion(t, command, "fixture --provider=dynamic", "fixture", "--provider=dynamic")
	equalsWant := []string{"--provider=dynamic value", "--provider=dynamic'quote"}
	if strings.Join(equals, "\x00") != strings.Join(equalsWant, "\x00") {
		t.Fatalf("bash --flag=value completion changed candidate records: got %#v, want %#v", equals, equalsWant)
	}
	if _, err := os.Stat(sideEffect); !os.IsNotExist(err) {
		t.Fatalf("bash evaluated candidate shell syntax; side effect stat error: %v", err)
	}
}

func TestBashDeduplicatesCombinedRootCandidates(t *testing.T) {
	directory := t.TempDir()
	provider := filepath.Join(directory, "provider-values")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\nprintf '%s\\n' nest 'root dynamic'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := Command{
		Name:              "fixture",
		CompletionCommand: []string{provider},
		Subcommands:       []Command{{Name: "nest"}},
	}
	got := runBashCompletion(t, command, "fixture ", "fixture", "")
	want := []string{"completion", "nest", "root dynamic"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("combined root candidates are not stable and unique: got %#v, want %#v", got, want)
	}
}

func TestNestedContextSkipsFlagValues(t *testing.T) {
	command := Command{
		Name:        "fixture",
		Flags:       []Flag{{Name: "target", Short: "t", Value: true, Values: []string{"nest"}}},
		Subcommands: []Command{{Name: "nest", Subcommands: []Command{{Name: "leaf"}}}},
	}

	t.Run("bash", func(t *testing.T) {
		root := runBashCompletion(t, command, "fixture --target nest ", "fixture", "--target", "nest", "")
		assertContainsCandidate(t, root, "completion")
		assertMissingCandidate(t, root, "leaf")
		nested := runBashCompletion(t, command, "fixture --target=value nest ", "fixture", "--target=value", "nest", "")
		assertContainsCandidate(t, nested, "leaf")
	})

	t.Run("fish", func(t *testing.T) {
		executable := completionShell(t, "fish")
		path := writeCompletion(t, "fish", command)
		root := runLines(t, executable, []string{"-c", `source $argv[1]; complete -C $argv[2]`, path, "fixture --target nest "})
		assertContainsCandidate(t, root, "completion")
		assertMissingCandidate(t, root, "leaf")
		nested := runLines(t, executable, []string{"-c", `source $argv[1]; complete -C $argv[2]`, path, "fixture --target=value nest "})
		assertContainsCandidate(t, nested, "leaf")
	})

	t.Run("zsh", func(t *testing.T) {
		executable := completionShell(t, "zsh")
		path := writeCompletion(t, "zsh", command)
		script := `compdef() { :; }; source "$1"; _arguments() { print -rC1 -- "$@"; }; words=(fixture --target nest ""); CURRENT=4; _fixture`
		root := runLines(t, executable, []string{"-fc", script, "completion-test", path})
		if !containsText(root, "1:command:(completion nest)") || containsText(root, "2:command:(leaf)") {
			t.Fatalf("zsh selected the wrong command context: %#v", root)
		}
		script = `compdef() { :; }; source "$1"; _arguments() { print -rC1 -- "$@"; }; words=(fixture --target=value nest ""); CURRENT=4; _fixture`
		nested := runLines(t, executable, []string{"-fc", script, "completion-test", path})
		if !containsText(nested, "2:command:(leaf)") {
			t.Fatalf("zsh did not enter the nested context after --flag=value: %#v", nested)
		}
	})
}

func TestZshPassesOnlyTheBufferPrefix(t *testing.T) {
	command := Command{
		Name: "fixture",
		Flags: []Flag{{
			Name:              "context",
			Value:             true,
			CompletionCommand: []string{"printf", "%s\\n", ContextPlaceholder},
		}},
	}
	executable := completionShell(t, "zsh")
	path := writeCompletion(t, "zsh", command)
	script := `compdef() { :; }; source "$1"; compadd() { print -rC1 -- "${values[@]}"; }; BUFFER="fixture --context before AFTER"; CURSOR=24; __fixture_completion_values_0`
	got := runLines(t, executable, []string{"-fc", script, "completion-test", path})
	if strings.Join(got, "\n") != "fixture --context before" {
		t.Fatalf("zsh passed text after the cursor: %#v", got)
	}
}

func TestZshPreservesCandidatesContainingColons(t *testing.T) {
	command := Command{Name: "fixture", CompletionCommand: []string{"printf", "%s\n", "ssh://host:22"}}
	output := runZshInteractiveCompletion(t, command, "fixture s")
	if !strings.Contains(output, "ARGS:ssh://host:22") {
		t.Fatalf("zsh changed a candidate containing a colon:\n%s", output)
	}
}

func TestNushellDeduplicatesCandidates(t *testing.T) {
	command := Command{Name: "fixture", CompletionCommand: []string{"printf", "%s\n", "same", "same"}}
	candidates := runNuCompletion(t, "", command, "fixture ")
	count := 0
	for _, candidate := range candidates {
		if candidate == "same" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Nushell returned %d copies of a candidate: %#v", count, candidates)
	}
}

func TestFishRejectsCandidatesContainingTabs(t *testing.T) {
	command := Command{
		Name: "fixture",
		Flags: []Flag{{
			Name:              "provider",
			Value:             true,
			Values:            []string{"static", "static\ttab"},
			CompletionCommand: []string{"printf", "%s\n", "dynamic", "dynamic\ttab"},
		}},
	}
	executable := completionShell(t, "fish")
	path := writeCompletion(t, "fish", command)
	candidates := runLines(t, executable, []string{"-N", "-c", `source $argv[1]; complete -C "fixture --provider "`, path})
	assertContainsCandidate(t, candidates, "static")
	assertContainsCandidate(t, candidates, "dynamic")
	assertMissingCandidate(t, candidates, "dynamic\ttab")
	assertMissingCandidate(t, candidates, "static\ttab")
	assertMissingCandidate(t, candidates, "dynamic\t")
}

func TestGeneratedCompletionsSuppressUnmodeledFiles(t *testing.T) {
	command := Command{Name: "fixture"}
	directory := t.TempDir()
	fileName := "unmodeled-file"
	if err := os.WriteFile(filepath.Join(directory, fileName), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("bash", func(t *testing.T) {
		candidates := runBashCompletion(t, command, "fixture u", "fixture", "u")
		assertMissingCandidate(t, candidates, fileName)
	})

	t.Run("fish", func(t *testing.T) {
		executable := completionShell(t, "fish")
		path := writeCompletion(t, "fish", command)
		candidates := runLinesIn(t, directory, executable, []string{"-N", "-c", `source $argv[1]; complete -C "fixture u"`, path})
		assertMissingCandidate(t, candidates, fileName)
	})

	t.Run("nu", func(t *testing.T) {
		candidates := runNuCompletion(t, directory, command, "fixture u")
		assertMissingCandidate(t, candidates, fileName)
	})

	t.Run("zsh", func(t *testing.T) {
		output := runZshInteractiveCompletionIn(t, directory, command, "fixture u")
		if !strings.Contains(output, "ARGS:u") || strings.Contains(output, "ARGS:"+fileName) {
			t.Fatalf("zsh leaked an unmodeled filesystem candidate:\n%s", output)
		}
	})
}

func TestFishSuppressesFilesForOwnedCandidates(t *testing.T) {
	command := Command{
		Name:              "fixture",
		CompletionCommand: []string{"printf", "%s\\n", "dynamic value"},
	}
	executable := completionShell(t, "fish")
	path := writeCompletion(t, "fish", command)
	directory := t.TempDir()
	file := filepath.Join(directory, "unrelated-file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := runLinesIn(t, directory, executable, []string{"-c", `source $argv[1]; complete -C "fixture "`, path})
	assertContainsCandidate(t, got, "dynamic value")
	assertMissingCandidate(t, got, "unrelated-file")
}

func completionShell(t *testing.T, shell string) string {
	t.Helper()
	executable, err := exec.LookPath(shell)
	if err == nil {
		return executable
	}
	if os.Getenv("GO_UTILS_REQUIRE_COMPLETION_SHELLS") == "1" {
		t.Fatalf("required completion shell %s is unavailable: %v", shell, err)
	}
	t.Skipf("%s is unavailable", shell)
	return ""
}

func writeCompletion(t *testing.T, shell string, command Command) string {
	t.Helper()
	generated, err := Generate(shell, command)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "completion."+shell)
	if err := os.WriteFile(path, []byte(generated), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runBashCompletion(t *testing.T, command Command, line string, words ...string) []string {
	t.Helper()
	executable := completionShell(t, "bash")
	path := writeCompletion(t, "bash", command)
	arguments := []string{"-c", `complete() { :; }; source "$1"; line="$2"; shift 2; COMP_LINE="$line"; COMP_POINT=${#COMP_LINE}; COMP_WORDS=("$@"); COMP_CWORD=$((${#COMP_WORDS[@]} - 1)); _fixture_complete; printf '%s\0' "${COMPREPLY[@]}"`, "completion-test", path, line}
	arguments = append(arguments, words...)
	output, err := exec.Command(executable, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("run bash completion: %v\n%s", err, output)
	}
	if len(output) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
}

func runNuCompletion(t *testing.T, directory string, command Command, line string) []string {
	t.Helper()
	executable := completionShell(t, "nu")
	completionPath := writeCompletion(t, "nu", command)
	script := "source " + quoteValue(completionPath) + "\n" + line
	scriptPath := filepath.Join(t.TempDir(), "completion-input.nu")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	process := exec.Command(executable, "--no-config-file", "--no-std-lib", "--ide-complete", strconv.Itoa(len(script)), scriptPath)
	process.Dir = directory
	output, err := process.CombinedOutput()
	if err != nil {
		t.Fatalf("run Nushell completion: %v\n%s", err, output)
	}
	var result struct {
		Completions []string `json:"completions"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode Nushell completion output: %v\n%s", err, output)
	}
	return result.Completions
}

func runZshInteractiveCompletion(t *testing.T, command Command, line string) string {
	t.Helper()
	return runZshInteractiveCompletionIn(t, "", command, line)
}

func runZshInteractiveCompletionIn(t *testing.T, directory string, command Command, line string) string {
	t.Helper()
	executable := completionShell(t, "zsh")
	completionPath := writeCompletion(t, "zsh", command)
	dumpPath := filepath.Join(t.TempDir(), "zcompdump")
	script := `
zmodload zsh/zpty
export GO_UTILS_COMPLETION_PATH=$1
export GO_UTILS_ZCOMPDUMP=$2
zpty completion-shell zsh -f
zpty -w completion-shell 'PS1=READY\>; autoload -Uz compinit; compinit -C -d "$GO_UTILS_ZCOMPDUMP"; fixture() { print -r -- "ARGS:${(j:\x1f:)@}"; }; source "$GO_UTILS_COMPLETION_PATH"'
zpty -r completion-shell output '*READY>*'
zpty -w completion-shell "$3"$'\t\n'
zpty -r completion-shell output '*READY>*'
print -r -- "$output"
zpty -d completion-shell
`
	process := exec.Command(executable, "-fc", script, "completion-test", completionPath, dumpPath, line)
	process.Dir = directory
	output, err := process.CombinedOutput()
	if err != nil {
		t.Fatalf("run interactive Zsh completion: %v\n%s", err, output)
	}
	return string(output)
}

func runLines(t *testing.T, executable string, arguments []string) []string {
	t.Helper()
	return runLinesIn(t, "", executable, arguments)
}

func runLinesIn(t *testing.T, directory, executable string, arguments []string) []string {
	t.Helper()
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s: %v\n%s", executable, err, output)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func assertContainsCandidate(t *testing.T, candidates []string, want string) {
	t.Helper()
	if !containsText(candidates, want) {
		t.Fatalf("completion lacks %q: %#v", want, candidates)
	}
}

func assertMissingCandidate(t *testing.T, candidates []string, unwanted string) {
	t.Helper()
	if containsText(candidates, unwanted) {
		t.Fatalf("completion contains %q: %#v", unwanted, candidates)
	}
}

func containsText(lines []string, text string) bool {
	for _, line := range lines {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
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
