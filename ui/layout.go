package ui

const (
	minimumMainWidth  = 48
	minimumMainRows   = 12
	defaultTreeWidth  = 32
	defaultDetailRows = 14
)

type Layout struct {
	Width      int
	Height     int
	TreeWidth  int
	MainWidth  int
	MainRows   int
	DetailRows int
}

func ResolveLayout(width, height int, treeVisible bool) Layout {
	width = max(width, 0)
	height = max(height, 0)
	treeWidth := 0
	if treeVisible && width >= minimumMainWidth+20 {
		treeWidth = min(defaultTreeWidth, width-minimumMainWidth)
	}
	detailRows := 0
	if height >= minimumMainRows+8 {
		detailRows = min(defaultDetailRows, height-minimumMainRows)
	}
	return Layout{
		Width:      width,
		Height:     height,
		TreeWidth:  treeWidth,
		MainWidth:  width - treeWidth,
		MainRows:   height - detailRows,
		DetailRows: detailRows,
	}
}
