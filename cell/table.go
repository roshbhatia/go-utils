package cell

import (
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/roshbhatia/go-utils/terminal"
)

// colorsEnabled decides whether a writer receives ANSI sequences.
var colorsEnabled = terminal.ColorsEnabled

// Table writes rows to w as columns padded by display width and separated by
// two spaces, with no trailing padding on a row's last cell. ANSI sequences
// inside a cell do not count toward its width. When w is not a color-capable
// terminal, every sequence is stripped first, so a piped table is plain
// bytes. Cells are single lines; rows may have different lengths.
func Table(w io.Writer, rows [][]string) error {
	color := colorsEnabled(w)
	var widths []int
	cells := make([][]string, len(rows))
	for index, row := range rows {
		cells[index] = make([]string, len(row))
		for column, value := range row {
			if !color {
				value = ansi.Strip(value)
			}
			cells[index][column] = value
			for len(widths) <= column {
				widths = append(widths, 0)
			}
			widths[column] = max(widths[column], Width(value))
		}
	}
	var out strings.Builder
	for _, row := range cells {
		for column, value := range row {
			if column > 0 {
				out.WriteString("  ")
			}
			out.WriteString(value)
			if column < len(row)-1 {
				out.WriteString(strings.Repeat(" ", widths[column]-Width(value)))
			}
		}
		out.WriteByte('\n')
	}
	_, err := io.WriteString(w, out.String())
	return err
}
