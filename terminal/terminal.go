// Package terminal provides dependency-light terminal capability helpers.
package terminal

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// IsTTY reports whether value is attached to a terminal.
func IsTTY(value any) bool {
	file, ok := value.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(file.Fd()))
}

// ColorsEnabled reports whether a stream should receive ANSI color.
func ColorsEnabled(stream io.Writer) bool {
	return colorEnvironmentAllows() && IsTTY(stream)
}

func colorEnvironmentAllows() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return !noColor && os.Getenv("TERM") != "dumb"
}

// IsControlReply recognizes color and device responses that can surface as input.
func IsControlReply(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "\x1b")
	value = strings.TrimPrefix(value, "]")
	value = strings.TrimPrefix(value, "P")
	value = strings.TrimPrefix(value, ">")
	value = strings.TrimSuffix(value, "\x1b\\")
	value = strings.TrimSuffix(value, "\a")

	return strings.HasPrefix(value, "10;rgb:") ||
		strings.HasPrefix(value, "11;rgb:") ||
		(strings.HasPrefix(value, "4;") && strings.Contains(value, ";rgb:")) ||
		strings.HasPrefix(value, "|WezTerm ") ||
		strings.HasPrefix(value, ">|WezTerm ") ||
		value == "alt+]" || value == `alt+\`
}

// DropControlReplyLines removes lines that consist only of one recognized reply.
func DropControlReplyLines(value string) string {
	lines := strings.Split(value, "\n")
	out := lines[:0]
	for _, line := range lines {
		if !IsControlReply(line) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
