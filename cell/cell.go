// Package cell provides ANSI-safe terminal cell measurement and fitting.
package cell

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

const Ellipsis = "…"

// Width returns the terminal cell width without counting ANSI sequences.
func Width(value string) int {
	return ansi.StringWidth(value)
}

// Truncate clips value and its optional tail to width cells.
func Truncate(value string, width int, tail ...string) string {
	if width < 1 {
		return ""
	}
	suffix := Ellipsis
	if len(tail) > 0 {
		suffix = tail[0]
	}
	if Width(suffix) > width {
		suffix = ansi.Truncate(suffix, width, "")
	}
	return ansi.Truncate(value, width, suffix)
}

// Fit truncates with an optional tail or right-pads value to exactly width terminal cells.
func Fit(value string, width int, tail ...string) string {
	return fit(value, width, false, tail...)
}

// RightFit truncates with an optional tail or left-pads value to exactly width terminal cells.
func RightFit(value string, width int, tail ...string) string {
	return fit(value, width, true, tail...)
}

func fit(value string, width int, right bool, tail ...string) string {
	if width < 1 {
		return ""
	}
	value = Truncate(value, width, tail...)
	padding := max(0, width-Width(value))
	if right {
		return strings.Repeat(" ", padding) + value
	}
	return value + strings.Repeat(" ", padding)
}

// ClipWord prefers a useful word boundary and clips a long first word when needed.
// The optional tail replaces the default Unicode ellipsis.
func ClipWord(value string, width int, tail ...string) string {
	if width < 1 {
		return ""
	}
	if Width(value) <= width {
		return value
	}
	suffix := Ellipsis
	if len(tail) > 0 {
		suffix = tail[0]
	}
	if Width(suffix) > width {
		suffix = ansi.Truncate(suffix, width, "")
	}
	budget := width - Width(suffix)
	if budget < 1 {
		return Truncate(value, width, "")
	}
	head := Truncate(value, budget, "")
	visible := ansi.Strip(head)
	if cut := strings.LastIndexAny(visible, " \t"); cut > 0 && Width(visible[:cut])*5 >= budget*3 {
		visible = visible[:cut]
	}
	head = Truncate(value, Width(strings.TrimRight(visible, " \t")), "")
	return head + suffix
}

// OneLine normalizes whitespace and text controls while preserving ANSI sequences.
func OneLine(value string) string {
	var output strings.Builder
	output.Grow(len(value))
	pendingSpace, visible, styled := false, false, false
	for index := 0; index < len(value); {
		if next, ok := c1Control(value, index); ok {
			index = next
			continue
		}
		if value[index] == '\x1b' {
			sequence, next, sgr := escape(value, index)
			if sgr {
				output.WriteString(sequence)
				styled = true
			}
			index = next
			continue
		}
		one, size := utf8.DecodeRuneInString(value[index:])
		if one == utf8.RuneError && size == 1 {
			index++
			continue
		}
		index += size
		if unicode.IsSpace(one) {
			pendingSpace = visible
			continue
		}
		if unicode.IsControl(one) {
			continue
		}
		if pendingSpace {
			output.WriteByte(' ')
			pendingSpace = false
		}
		output.WriteRune(one)
		visible = true
	}
	if !visible {
		return ""
	}
	result := output.String()
	if styled && !strings.HasSuffix(result, "\x1b[0m") {
		result += "\x1b[0m"
	}
	return result
}

func c1Control(value string, start int) (int, bool) {
	code, size := value[start], 1
	if code == 0xc2 && start+1 < len(value) && value[start+1] >= 0x80 && value[start+1] <= 0x9f {
		code, size = value[start+1], 2
	} else if code < 0x80 || code > 0x9f {
		return start, false
	}
	switch code {
	case 0x90, 0x98, 0x9d, 0x9e, 0x9f:
		return controlStringEnd(value, start+size, code == 0x9d), true
	case 0x9b:
		for index := start + size; index < len(value); index++ {
			if value[index] >= 0x40 && value[index] <= 0x7e {
				return index + 1, true
			}
		}
		return len(value), true
	default:
		return start + size, true
	}
}

func escape(value string, start int) (string, int, bool) {
	if start+1 >= len(value) {
		return "", len(value), false
	}
	switch value[start+1] {
	case '[':
		for index := start + 2; index < len(value); index++ {
			if value[index] >= 0x40 && value[index] <= 0x7e {
				if value[index] == 'm' {
					return value[start : index+1], index + 1, true
				}
				return "", index + 1, false
			}
		}
		return "", len(value), false
	case ']':
		return "", controlStringEnd(value, start+2, true), false
	case 'P', 'X', '^', '_':
		return "", controlStringEnd(value, start+2, false), false
	default:
		return "", min(len(value), start+2), false
	}
}

func controlStringEnd(value string, start int, bell bool) int {
	for index := start; index < len(value); index++ {
		if bell && value[index] == '\a' {
			return index + 1
		}
		if value[index] == '\x1b' && index+1 < len(value) && value[index+1] == '\\' {
			return index + 2
		}
		if value[index] == 0x9c {
			return index + 1
		}
		if value[index] == 0xc2 && index+1 < len(value) && value[index+1] == 0x9c {
			return index + 2
		}
	}
	return len(value)
}
