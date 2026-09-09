package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/roshbhatia/go-utils/terminal"
)

// diagnosticColors maps a diagnostic kind to the color of its label. Kinds
// outside the map print without color.
var diagnosticColors = map[string]int{
	"fatal":   ColorRed,
	"error":   ColorRed,
	"warning": ColorYellow,
	"hint":    ColorGray,
	"info":    ColorBlue,
}

// Diagnostic writes one git-shaped diagnostic line, "kind: msg", to w. The
// kind is lowercased and the message is written as given, so callers pass it
// without a trailing period. Only the label carries color, and only when w
// is a color-capable terminal: stderr and stdout follow this package's
// per-stream detection, any other writer is asked directly.
func Diagnostic(w io.Writer, kind, msg string) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	label := kind + ":"
	if code, ok := diagnosticColors[kind]; ok && writerColors(w) {
		label = fmt.Sprintf("\033[38;5;%dm%s\033[0m", code, label)
	}
	fmt.Fprintf(w, "%s %s\n", label, msg)
}

// Fatal writes "fatal: msg" to w, the form git uses before it exits.
func Fatal(w io.Writer, msg string) { Diagnostic(w, "fatal", msg) }

// Hint writes "hint: msg" to w, the form git uses for advice.
func Hint(w io.Writer, msg string) { Diagnostic(w, "hint", msg) }

func writerColors(w io.Writer) bool {
	switch w {
	case os.Stderr:
		return colorsEnabled
	case os.Stdout:
		return stdoutColorsEnabled
	}
	return terminal.ColorsEnabled(w)
}
