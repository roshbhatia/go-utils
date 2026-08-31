package ui

import "testing"

func TestResolveLayout(t *testing.T) {
	t.Parallel()

	layout := ResolveLayout(160, 48, true)
	if layout.TreeWidth != 32 || layout.MainWidth != 128 {
		t.Fatalf("unexpected columns: %+v", layout)
	}
	if layout.DetailRows != 14 || layout.MainRows != 34 {
		t.Fatalf("unexpected rows: %+v", layout)
	}
}

func TestResolveLayoutHidesTreeInNarrowFrame(t *testing.T) {
	t.Parallel()

	layout := ResolveLayout(60, 20, true)
	if layout.TreeWidth != 0 || layout.MainWidth != 60 {
		t.Fatalf("unexpected narrow layout: %+v", layout)
	}
}
