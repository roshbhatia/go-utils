package ui

type Color string

type Palette struct {
	Background Color
	Surface    Color
	Border     Color
	Muted      Color
	Text       Color
	Title      Color
	Accent     Color
	Success    Color
	Warning    Color
	Danger     Color
}

func DefaultPalette() Palette {
	return Palette{
		Background: "#1e2030",
		Surface:    "#24273a",
		Border:     "#5b6078",
		Muted:      "#8087a2",
		Text:       "#cad3f5",
		Title:      "#b7bdf8",
		Accent:     "#8aadf4",
		Success:    "#a6da95",
		Warning:    "#eed49f",
		Danger:     "#ed8796",
	}
}

// TerminalPalette uses ANSI roles and inherits terminal backgrounds.
func TerminalPalette(overrides ...Palette) Palette {
	palette := Palette{Border: "8", Muted: "8", Text: "7", Title: "4", Accent: "6", Success: "2", Warning: "3", Danger: "1"}
	for _, override := range overrides {
		palette = MergePalette(palette, override)
	}
	return palette
}

// MergePalette overlays non-empty semantic roles on base.
func MergePalette(base, override Palette) Palette {
	roles := []struct {
		target *Color
		value  Color
	}{
		{&base.Background, override.Background}, {&base.Surface, override.Surface},
		{&base.Border, override.Border}, {&base.Muted, override.Muted},
		{&base.Text, override.Text}, {&base.Title, override.Title}, {&base.Accent, override.Accent},
		{&base.Success, override.Success}, {&base.Warning, override.Warning},
		{&base.Danger, override.Danger},
	}
	for _, role := range roles {
		if role.value != "" {
			*role.target = role.value
		}
	}
	return base
}
