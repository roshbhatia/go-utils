// Package panes provides renderer-neutral pane geometry and navigation.
package panes

import (
	"cmp"
	"slices"
)

// Rect is a half-open terminal-cell rectangle.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (rectangle Rect) Right() int  { return rectangle.X + max(0, rectangle.Width) }
func (rectangle Rect) Bottom() int { return rectangle.Y + max(0, rectangle.Height) }

// Contains reports whether the terminal cell is inside the rectangle.
func (rectangle Rect) Contains(x, y int) bool {
	return x >= rectangle.X && x < rectangle.Right() && y >= rectangle.Y && y < rectangle.Bottom()
}

func (rectangle Rect) center() (int, int) {
	return rectangle.X*2 + max(0, rectangle.Width), rectangle.Y*2 + max(0, rectangle.Height)
}

// Region assigns a stable identity to a focusable rectangle.
type Region struct {
	ID       string
	Rect     Rect
	Disabled bool
}

// Hit records the region and local cell coordinates beneath a pointer.
type Hit struct {
	Region Region
	X      int
	Y      int
}

// HitTest treats later overlapping regions as frontmost.
func HitTest(regions []Region, x, y int) (Hit, bool) {
	for index := len(regions) - 1; index >= 0; index-- {
		region := regions[index]
		if !region.Disabled && region.Rect.Contains(x, y) {
			return Hit{Region: region, X: x - region.Rect.X, Y: y - region.Rect.Y}, true
		}
	}
	return Hit{}, false
}

type Direction string

const (
	Up    Direction = "up"
	Down  Direction = "down"
	Left  Direction = "left"
	Right Direction = "right"
)

func (direction Direction) Valid() bool {
	return direction == Up || direction == Down || direction == Left || direction == Right
}

type candidate struct {
	region    Region
	primary   int
	secondary int
	overlap   bool
	order     int
}

// Neighbor prefers overlapping spans, then center distance and input order.
func Neighbor(regions []Region, current string, direction Direction) (Region, bool) {
	if !direction.Valid() {
		return Region{}, false
	}
	var origin Region
	found := false
	for _, region := range regions {
		if region.ID == current && !region.Disabled && validRect(region.Rect) {
			origin, found = region, true
			break
		}
	}
	if !found {
		return Region{}, false
	}
	ox, oy := origin.Rect.center()
	options := make([]candidate, 0, len(regions))
	for order, region := range regions {
		if region.ID == current || region.ID == "" || region.Disabled || !validRect(region.Rect) {
			continue
		}
		x, y := region.Rect.center()
		dx, dy := x-ox, y-oy
		if (direction == Left && dx >= 0) || (direction == Right && dx <= 0) ||
			(direction == Up && dy >= 0) || (direction == Down && dy <= 0) {
			continue
		}
		one := candidate{region: region, order: order}
		if direction == Left || direction == Right {
			one.primary, one.secondary = abs(dx), abs(dy)
			one.overlap = overlaps(origin.Rect.Y, origin.Rect.Bottom(), region.Rect.Y, region.Rect.Bottom())
		} else {
			one.primary, one.secondary = abs(dy), abs(dx)
			one.overlap = overlaps(origin.Rect.X, origin.Rect.Right(), region.Rect.X, region.Rect.Right())
		}
		options = append(options, one)
	}
	if len(options) == 0 {
		return Region{}, false
	}
	slices.SortStableFunc(options, func(left, right candidate) int {
		if left.overlap != right.overlap {
			if left.overlap {
				return -1
			}
			return 1
		}
		if order := cmp.Compare(left.primary, right.primary); order != 0 {
			return order
		}
		if order := cmp.Compare(left.secondary, right.secondary); order != 0 {
			return order
		}
		return cmp.Compare(left.order, right.order)
	})
	return options[0].region, true
}

func validRect(rectangle Rect) bool {
	return rectangle.Width > 0 && rectangle.Height > 0
}

func overlaps(startA, endA, startB, endB int) bool {
	return max(startA, startB) < min(endA, endB)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Cycle moves index by delta around a collection of size items.
func Cycle(index, delta, size int) int {
	if size < 1 {
		return -1
	}
	index = (index + delta) % size
	if index < 0 {
		index += size
	}
	return index
}
