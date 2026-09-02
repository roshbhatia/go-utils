package completion

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Flag struct {
	Name        string
	Short       string
	Description string
	Value       bool
	Values      []string
}

type Command struct {
	Name        string
	Description string
	Flags       []Flag
	Subcommands []Command
}

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
	var out strings.Builder
	out.WriteString("### `" + command.Name + "`\n\n")
	out.WriteString(command.Description + "\n")
	writeFlags(&out, command.Flags)
	for _, subcommand := range command.Subcommands {
		out.WriteString("\n### `" + command.Name + " " + subcommand.Name + "`\n\n")
		out.WriteString(subcommand.Description + "\n")
		writeFlags(&out, subcommand.Flags)
	}
	return strings.TrimSpace(out.String()) + "\n"
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

func writeFlags(out *strings.Builder, flags []Flag) {
	if len(flags) == 0 {
		return
	}
	out.WriteString("\n| Option | Description |\n| --- | --- |\n")
	for _, flag := range flags {
		name := "`--" + flag.Name + "`"
		if flag.Short != "" {
			name += ", `-" + flag.Short + "`"
		}
		if flag.Value {
			name += " `<value>`"
		}
		out.WriteString("| " + name + " | " + strings.ReplaceAll(flag.Description, "|", "\\|") + " |\n")
	}
}

func names(command Command) string {
	out := []string{"completion", "bash", "zsh", "fish", "nu"}
	for _, subcommand := range command.Subcommands {
		out = append(out, subcommand.Name)
	}
	for _, flag := range command.Flags {
		out = append(out, "--"+flag.Name)
		if flag.Short != "" {
			out = append(out, "-"+flag.Short)
		}
		out = append(out, flag.Values...)
	}
	return strings.Join(out, " ")
}

func bash(command Command) string {
	return fmt.Sprintf(`_%[1]s_complete() {
  local current="${COMP_WORDS[COMP_CWORD]}"
  COMPREPLY=($(compgen -W '%[2]s' -- "$current"))
}
complete -F _%[1]s_complete %[1]s`, command.Name, names(command))
}

func fish(command Command) string {
	lines := []string{"complete -c " + command.Name + " -e"}
	for _, flag := range command.Flags {
		line := "complete -c " + command.Name + " -l " + flag.Name
		if flag.Short != "" {
			line += " -s " + flag.Short
		}
		if flag.Value {
			line += " -r"
		}
		if len(flag.Values) > 0 {
			line += " -a '" + strings.Join(flag.Values, " ") + "'"
		}
		line += " -d '" + strings.ReplaceAll(flag.Description, "'", "\\'") + "'"
		lines = append(lines, line)
	}
	lines = append(lines,
		"complete -c "+command.Name+" -n '__fish_use_subcommand' -a completion -d 'Generate shell completions'",
		"complete -c "+command.Name+" -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish nu'",
	)
	for _, subcommand := range command.Subcommands {
		lines = append(lines, "complete -c "+command.Name+" -n '__fish_use_subcommand' -a "+subcommand.Name+" -d '"+escapeFish(subcommand.Description)+"'")
		for _, flag := range subcommand.Flags {
			line := "complete -c " + command.Name + " -n '__fish_seen_subcommand_from " + subcommand.Name + "' -l " + flag.Name
			if flag.Short != "" {
				line += " -s " + flag.Short
			}
			if flag.Value {
				line += " -r"
			}
			if len(flag.Values) > 0 {
				line += " -a '" + strings.Join(flag.Values, " ") + "'"
			}
			line += " -d '" + escapeFish(flag.Description) + "'"
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func escapeFish(value string) string { return strings.ReplaceAll(value, "'", "\\'") }

func zsh(command Command) string {
	lines := []string{"#compdef " + command.Name, "_" + command.Name + "() {", "  _arguments \\"}
	for _, flag := range command.Flags {
		short := ""
		if flag.Short != "" {
			short = "(-" + flag.Short + ")"
		}
		value := ""
		if flag.Value {
			value = ":value:"
			if len(flag.Values) > 0 {
				value += "(" + strings.Join(flag.Values, " ") + ")"
			}
		}
		lines = append(lines, "    '"+short+"--"+flag.Name+"["+strings.ReplaceAll(flag.Description, "'", "")+"]"+value+"' \\")
	}
	subcommands := []string{"completion"}
	for _, subcommand := range command.Subcommands {
		subcommands = append(subcommands, subcommand.Name)
	}
	lines = append(lines, "    '1:command:("+strings.Join(subcommands, " ")+")' \\", "    '2:shell:(bash zsh fish nu)'", "}", "compdef _"+command.Name+" "+command.Name)
	return strings.Join(lines, "\n")
}

func nu(command Command) string {
	lines := []string{"export extern \"" + command.Name + "\" ["}
	for _, flag := range command.Flags {
		name := "  --" + flag.Name
		if flag.Short != "" {
			name += "(-" + flag.Short + ")"
		}
		if flag.Value {
			name += ": string"
		}
		lines = append(lines, name+" # "+flag.Description)
	}
	lines = append(lines,
		"  ...args: string",
		"]",
		"",
		"export extern \""+command.Name+" completion\" [",
		"  shell: string@\"nu-complete "+command.Name+" shell\"",
		"]",
		"",
		"def \"nu-complete "+command.Name+" shell\" [] { [bash zsh fish nu] }",
	)
	for _, subcommand := range command.Subcommands {
		lines = append(lines, "", "export extern \""+command.Name+" "+subcommand.Name+"\" [")
		for _, flag := range subcommand.Flags {
			name := "  --" + flag.Name
			if flag.Short != "" {
				name += "(-" + flag.Short + ")"
			}
			if flag.Value {
				name += ": string"
			}
			lines = append(lines, name+" # "+flag.Description)
		}
		lines = append(lines, "  ...args: string", "]")
	}
	return strings.Join(lines, "\n")
}
