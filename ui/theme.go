package ui

type Color string

type Palette struct {
	Background Color
	Surface    Color
	Border     Color
	Muted      Color
	Text       Color
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
		Accent:     "#8aadf4",
		Success:    "#a6da95",
		Warning:    "#eed49f",
		Danger:     "#ed8796",
	}
}
