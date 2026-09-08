package panes

import "testing"

func TestRectAndHitTest(t *testing.T) {
	t.Parallel()

	regions := []Region{
		{ID: "back", Rect: Rect{X: 1, Y: 2, Width: 4, Height: 3}},
		{ID: "front", Rect: Rect{X: 2, Y: 3, Width: 2, Height: 2}},
	}
	if !regions[0].Rect.Contains(1, 2) || regions[0].Rect.Contains(5, 5) {
		t.Fatal("Rect.Contains uses the wrong boundary")
	}
	hit, ok := HitTest(regions, 3, 4)
	if !ok || hit.Region.ID != "front" || hit.X != 1 || hit.Y != 1 {
		t.Fatalf("HitTest() = %+v, %t", hit, ok)
	}
}

func TestNeighborUsesDirectionOverlapAndDistance(t *testing.T) {
	t.Parallel()

	regions := []Region{
		{ID: "main", Rect: Rect{X: 20, Y: 10, Width: 20, Height: 10}},
		{ID: "left-near", Rect: Rect{X: 0, Y: 12, Width: 10, Height: 4}},
		{ID: "left-diagonal", Rect: Rect{X: 14, Y: 40, Width: 4, Height: 4}},
		{ID: "right", Rect: Rect{X: 50, Y: 10, Width: 10, Height: 10}},
		{ID: "down", Rect: Rect{X: 20, Y: 30, Width: 20, Height: 5}},
	}
	for _, test := range []struct {
		direction Direction
		want      string
	}{
		{direction: Left, want: "left-near"},
		{direction: Right, want: "right"},
		{direction: Down, want: "down"},
	} {
		got, ok := Neighbor(regions, "main", test.direction)
		if !ok || got.ID != test.want {
			t.Errorf("Neighbor(%s) = %q, %t; want %q", test.direction, got.ID, ok, test.want)
		}
	}
	if _, ok := Neighbor(regions, "missing", Left); ok {
		t.Fatal("missing origin has a neighbor")
	}
}

func TestNeighborExcludesZeroAreaRegions(t *testing.T) {
	t.Parallel()
	regions := []Region{
		{ID: "main", Rect: Rect{Width: 10, Height: 10}},
		{ID: "empty", Rect: Rect{X: 11, Width: 0, Height: 10}},
	}
	if _, ok := Neighbor(regions, "main", Right); ok {
		t.Fatal("zero-width target was focusable")
	}
	regions[0].Rect.Width = 0
	regions[1].Rect.Width = 10
	if _, ok := Neighbor(regions, "main", Right); ok {
		t.Fatal("zero-width origin was focusable")
	}
}

func TestCycle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ index, delta, size, want int }{
		{index: 0, delta: 1, size: 3, want: 1},
		{index: 2, delta: 1, size: 3, want: 0},
		{index: 0, delta: -1, size: 3, want: 2},
		{index: 0, delta: 1, size: 0, want: -1},
	} {
		if got := Cycle(test.index, test.delta, test.size); got != test.want {
			t.Errorf("Cycle(%d, %d, %d) = %d, want %d", test.index, test.delta, test.size, got, test.want)
		}
	}
}
