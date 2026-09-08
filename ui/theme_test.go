package ui

import "testing"

func TestDefaultPaletteDefinesEveryRole(t *testing.T) {
	t.Parallel()

	palette := DefaultPalette()
	colors := []Color{
		palette.Border,
		palette.Muted,
		palette.Text,
		palette.Title,
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
	if palette.Background != "#1e2030" || palette.Accent != "#8aadf4" {
		t.Fatalf("DefaultPalette compatibility changed: %+v", palette)
	}
}

func TestTerminalPaletteUsesRolesAndAppliesOverrides(t *testing.T) {
	t.Parallel()

	palette := TerminalPalette(Palette{Accent: "5", Background: "#101010"})
	if palette.Accent != "5" || palette.Title != "4" || palette.Background != "#101010" || palette.Success != "2" {
		t.Fatalf("palette = %+v", palette)
	}
}

func TestTerminalPaletteInheritsBackground(t *testing.T) {
	t.Parallel()
	palette := TerminalPalette()
	if palette.Background != "" || palette.Surface != "" || palette.Title != "4" || palette.Accent != "6" {
		t.Fatalf("terminal palette = %+v", palette)
	}
}
