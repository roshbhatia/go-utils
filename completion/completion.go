package completion

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

type Flag struct {
	Name        string
	Short       string
	Description string
	Value       bool
	Values      []string
}

type Command struct {
	Name            string
	Description     string
	Synopsis        string
	LongDescription string
	Flags           []Flag
	Subcommands     []Command
}

type commandTemplateData struct {
	Command
	Candidates  []string
	Children    []Command
	Context     string
	Invocation  string
	Position    int
	Subcommands []string
}

type commandTransition struct {
	From string
	To   string
	Word string
}

type flagValues struct {
	Context string
	Names   []string
	Values  []string
}

type completionTemplateData struct {
	Command
	Commands    []commandTemplateData
	Transitions []commandTransition
	Values      []flagValues
}

type markdownSection struct {
	Flags           []Flag
	Invocation      string
	LongDescription string
	Synopsis        string
}

//go:embed templates/*.tmpl
var completionTemplateFiles embed.FS

var completionTemplates = template.Must(template.New("completion").Funcs(template.FuncMap{
	"escapeFish":       escapeFish,
	"commandSynopsis":  commandSynopsis,
	"join":             strings.Join,
	"markdownFlagName": markdownFlagName,
	"markdownText":     markdownText,
	"zshText":          zshText,
}).ParseFS(completionTemplateFiles, "templates/*.tmpl"))

func Generate(shell string, command Command) (string, error) {
	sort.Slice(command.Flags, func(i, j int) bool {
		return command.Flags[i].Name < command.Flags[j].Name
	})
	switch shell {
	case "bash":
		return bash(command), nil
	case "fish":
		return fish(command), nil
	case "nu":
		return nu(command), nil
	case "zsh":
		return zsh(command), nil
	default:
		return "", fmt.Errorf("unsupported completion shell %q", shell)
	}
}

// Markdown renders command metadata for a README generated section.
func Markdown(command Command) string {
	sections := []markdownSection{{
		Flags:           command.Flags,
		Invocation:      command.Name,
		LongDescription: command.LongDescription,
		Synopsis:        commandSynopsis(command),
	}}
	walkCommands(command.Subcommands, func(path []string, subcommand Command) {
		sections = append(sections, markdownSection{
			Flags:           subcommand.Flags,
			Invocation:      command.Name + " " + strings.Join(path, " "),
			LongDescription: subcommand.LongDescription,
			Synopsis:        commandSynopsis(subcommand),
		})
	})
	return strings.TrimSpace(executeTemplate("markdown", sections)) + "\n"
}

// ReplaceSection updates one generated Markdown section.
func ReplaceSection(document, name, generated string) (string, error) {
	start := "<!-- BEGIN GENERATED:" + name + " -->"
	end := "<!-- END GENERATED:" + name + " -->"
	before, after, found := strings.Cut(document, start)
	if !found {
		return "", fmt.Errorf("missing %s marker", start)
	}
	_, tail, found := strings.Cut(after, end)
	if !found {
		return "", fmt.Errorf("missing %s marker", end)
	}
	if strings.Contains(tail, start) {
		return "", errors.New("generated section markers are duplicated")
	}
	body := strings.TrimSpace(generated)
	return strings.TrimRight(before, "\n") + "\n" + start + "\n\n" + body + "\n\n" + end + tail, nil
}

func markdownFlagName(flag Flag) string {
	name := "`--" + flag.Name + "`"
	if flag.Short != "" {
		name += ", `-" + flag.Short + "`"
	}
	if flag.Value {
		name += " `<value>`"
	}
	return name
}

func markdownText(value string) string { return strings.ReplaceAll(value, "|", "\\|") }

func commandSynopsis(command Command) string {
	if command.Synopsis != "" {
		return command.Synopsis
	}
	return command.Description
}

func walkCommands(commands []Command, visit func([]string, Command)) {
	var walk func([]string, []Command)
	walk = func(parent []string, children []Command) {
		for _, command := range children {
			path := append(append([]string{}, parent...), command.Name)
			visit(path, command)
			walk(path, command.Subcommands)
		}
	}
	walk(nil, commands)
}

func bash(command Command) string {
	return executeTemplate("bash", completionData(command))
}

func fish(command Command) string {
	return executeTemplate("fish", completionData(command))
}

func escapeFish(value string) string { return strings.ReplaceAll(value, "'", "\\'") }

func zsh(command Command) string {
	return executeTemplate("zsh", completionData(command))
}

func nu(command Command) string {
	data := completionTemplateData{Command: command}
	walkCommands(command.Subcommands, func(path []string, subcommand Command) {
		data.Commands = append(data.Commands, commandTemplateData{
			Command:    subcommand,
			Invocation: strings.Join(path, " "),
		})
	})
	return executeTemplate("nu", data)
}

func executeTemplate(name string, data any) string {
	var output bytes.Buffer
	if err := completionTemplates.ExecuteTemplate(&output, name, data); err != nil {
		panic(fmt.Sprintf("execute completion template %s: %v", name, err))
	}
	return strings.TrimSuffix(output.String(), "\n")
}

func zshText(value string) string { return strings.ReplaceAll(value, "'", "") }

func completionData(command Command) completionTemplateData {
	data := completionTemplateData{Command: command}
	rootChildren := append([]Command{{
		Name:        "completion",
		Description: "Generate shell completions",
	}}, command.Subcommands...)
	root := commandTemplateData{
		Command:     command,
		Children:    rootChildren,
		Position:    1,
		Subcommands: commandNames(rootChildren),
	}
	root.Candidates = commandCandidates(root.Command.Flags, root.Subcommands)
	data.Commands = append(data.Commands, root)
	appendFlagValues(&data, "", command.Flags)

	completion := commandTemplateData{
		Command: Command{
			Name:        "completion",
			Description: "Generate shell completions",
		},
		Candidates: []string{"bash", "zsh", "fish", "nu"},
		Context:    "completion",
		Invocation: "completion",
		Position:   2,
	}
	data.Commands = append(data.Commands, completion)
	data.Transitions = append(data.Transitions, commandTransition{To: "completion", Word: "completion"})

	walkCommands(command.Subcommands, func(path []string, subcommand Command) {
		context := strings.Join(path, " ")
		parent := strings.Join(path[:len(path)-1], " ")
		children := childNames(subcommand)
		data.Commands = append(data.Commands, commandTemplateData{
			Command:     subcommand,
			Candidates:  commandCandidates(subcommand.Flags, children),
			Children:    subcommand.Subcommands,
			Context:     context,
			Invocation:  context,
			Position:    len(path) + 1,
			Subcommands: children,
		})
		data.Transitions = append(data.Transitions, commandTransition{
			From: parent,
			To:   context,
			Word: subcommand.Name,
		})
		appendFlagValues(&data, context, subcommand.Flags)
	})
	return data
}

func childNames(command Command) []string {
	return commandNames(command.Subcommands)
}

func commandNames(commands []Command) []string {
	names := make([]string, 0, len(commands))
	for _, subcommand := range commands {
		names = append(names, subcommand.Name)
	}
	return names
}

func commandCandidates(flags []Flag, subcommands []string) []string {
	candidates := append([]string{}, subcommands...)
	for _, flag := range flags {
		candidates = append(candidates, "--"+flag.Name)
		if flag.Short != "" {
			candidates = append(candidates, "-"+flag.Short)
		}
	}
	return candidates
}

func appendFlagValues(data *completionTemplateData, context string, flags []Flag) {
	for _, flag := range flags {
		if len(flag.Values) == 0 {
			continue
		}
		names := []string{"--" + flag.Name}
		if flag.Short != "" {
			names = append(names, "-"+flag.Short)
		}
		data.Values = append(data.Values, flagValues{
			Context: context,
			Names:   names,
			Values:  append([]string{}, flag.Values...),
		})
	}
}
