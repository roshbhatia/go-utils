package completion

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// ContextPlaceholder passes the shell's current command line to a dynamic completion command.
const ContextPlaceholder = "{completion.context}"

type Flag struct {
	Name              string
	Short             string
	Description       string
	Value             bool
	Values            []string
	CompletionCommand []string
}

type Command struct {
	Name              string
	Description       string
	Synopsis          string
	LongDescription   string
	Flags             []Flag
	Subcommands       []Command
	CompletionCommand []string
}

type flagTemplateData struct {
	Flag
	ValueSet string
}

type commandTemplateData struct {
	Command
	ArgumentSet string
	Candidates  []string
	Children    []Command
	Context     string
	Flags       []flagTemplateData
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
	Set     string
}

type valueFlag struct {
	Context string
	Names   []string
}

type completionSet struct {
	Command []string
	ID      string
	Values  []string
}

type completionTemplateData struct {
	Command
	Commands    []commandTemplateData
	Flags       []flagTemplateData
	Sets        []completionSet
	Transitions []commandTransition
	Values      []flagValues
	ValueFlags  []valueFlag
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
	"bashCommand":      bashCommand,
	"commandSynopsis":  commandSynopsis,
	"fishCommand":      fishCommand,
	"join":             strings.Join,
	"markdownFlagName": markdownFlagName,
	"markdownText":     markdownText,
	"nuCommand":        nuCommand,
	"quoteShell":       quoteShell,
	"quoteValue":       quoteValue,
	"setFunction":      setFunction,
	"zshText":          zshText,
	"zshCommand":       zshCommand,
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
	return executeTemplate("nu", completionData(command))
}

func executeTemplate(name string, data any) string {
	var output bytes.Buffer
	if err := completionTemplates.ExecuteTemplate(&output, name, data); err != nil {
		panic(fmt.Sprintf("execute completion template %s: %v", name, err))
	}
	return strings.TrimSuffix(output.String(), "\n")
}

func zshText(value string) string { return strings.ReplaceAll(value, "'", "") }

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func quoteValue(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func commandWithContext(command []string, context string) string {
	quoted := make([]string, 0, len(command))
	for _, argument := range command {
		if argument == ContextPlaceholder {
			quoted = append(quoted, context)
			continue
		}
		quoted = append(quoted, quoteShell(argument))
	}
	return strings.Join(quoted, " ")
}

func bashCommand(command []string) string {
	return commandWithContext(command, `"${COMP_LINE:0:COMP_POINT}"`)
}

func fishCommand(command []string) string {
	return commandWithContext(command, `(commandline -cp)`)
}

func zshCommand(command []string) string {
	return commandWithContext(command, `"${BUFFER[1,CURSOR]}"`)
}

func nuCommand(command []string) string {
	quoted := make([]string, 0, len(command)+1)
	quoted = append(quoted, "run-external")
	for _, argument := range command {
		if argument == ContextPlaceholder {
			quoted = append(quoted, `($context | default "")`)
			continue
		}
		quoted = append(quoted, quoteValue(argument))
	}
	return strings.Join(quoted, " ")
}

func setFunction(commandName, setID string) string {
	var name strings.Builder
	name.WriteString("__")
	for _, character := range commandName {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			name.WriteRune(character)
		} else {
			name.WriteByte('_')
		}
	}
	name.WriteString("_completion_")
	name.WriteString(setID)
	return name.String()
}

func completionData(command Command) completionTemplateData {
	data := completionTemplateData{Command: command}
	rootFlags := appendFlagValues(&data, "", command.Flags)
	data.Flags = rootFlags
	rootChildren := append([]Command{{
		Name:        "completion",
		Description: "Generate shell completions",
	}}, command.Subcommands...)
	root := commandTemplateData{
		Command:     command,
		ArgumentSet: appendCompletionSet(&data, positionalValues(rootChildren, command.CompletionCommand), command.CompletionCommand),
		Children:    rootChildren,
		Flags:       rootFlags,
		Position:    1,
		Subcommands: commandNames(rootChildren),
	}
	root.Candidates = commandCandidates(root.Command.Flags, root.Subcommands)
	data.Commands = append(data.Commands, root)

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
			ArgumentSet: appendCompletionSet(&data, positionalValues(subcommand.Subcommands, subcommand.CompletionCommand), subcommand.CompletionCommand),
			Candidates:  commandCandidates(subcommand.Flags, children),
			Children:    subcommand.Subcommands,
			Context:     context,
			Flags:       appendFlagValues(&data, context, subcommand.Flags),
			Invocation:  context,
			Position:    len(path) + 1,
			Subcommands: children,
		})
		data.Transitions = append(data.Transitions, commandTransition{
			From: parent,
			To:   context,
			Word: subcommand.Name,
		})
	})
	return data
}

func positionalValues(subcommands []Command, completionCommand []string) []string {
	if len(completionCommand) == 0 {
		return nil
	}
	return commandNames(subcommands)
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

func appendFlagValues(data *completionTemplateData, context string, flags []Flag) []flagTemplateData {
	templateFlags := make([]flagTemplateData, 0, len(flags))
	for _, flag := range flags {
		set := appendCompletionSet(data, flag.Values, flag.CompletionCommand)
		templateFlags = append(templateFlags, flagTemplateData{Flag: flag, ValueSet: set})
		if flag.Value {
			names := []string{"--" + flag.Name}
			if flag.Short != "" {
				names = append(names, "-"+flag.Short)
			}
			data.ValueFlags = append(data.ValueFlags, valueFlag{Context: context, Names: names})
		}
		if set == "" {
			continue
		}
		names := []string{"--" + flag.Name}
		if flag.Short != "" {
			names = append(names, "-"+flag.Short)
		}
		data.Values = append(data.Values, flagValues{
			Context: context,
			Names:   names,
			Set:     set,
		})
	}
	return templateFlags
}

func appendCompletionSet(data *completionTemplateData, values, command []string) string {
	if len(values) == 0 && len(command) == 0 {
		return ""
	}
	id := fmt.Sprintf("values_%d", len(data.Sets))
	data.Sets = append(data.Sets, completionSet{
		Command: append([]string{}, command...),
		ID:      id,
		Values:  append([]string{}, values...),
	})
	return id
}
