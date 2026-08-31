package ui

import "testing"

func TestDefaultPaletteDefinesEveryRole(t *testing.T) {
	t.Parallel()

	palette := DefaultPalette()
	colors := []Color{
		palette.Background,
		palette.Surface,
		palette.Border,
		palette.Muted,
		palette.Text,
		palette.Accent,
		palette.Success,
		palette.Warning,
		palette.Danger,
	}
	for index, color := range colors {
		if color == "" {
			t.Fatalf("palette color %d is empty", index)
		}
	}
}
