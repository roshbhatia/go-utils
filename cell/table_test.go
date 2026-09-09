package cell

import (
	"bytes"
	"io"
	"testing"
)

func TestTableAlignsByDisplayWidth(t *testing.T) {
	rows := [][]string{
		{"\x1b[31mmain\x1b[0m", "/repo", "clean"},
		{"feature/long-name", "/repo/.wt/x", "\x1b[33mdirty\x1b[0m"},
		{"界界", "/w"},
	}

	// A piped writer receives no ANSI at all.
	var plain bytes.Buffer
	if err := Table(&plain, rows); err != nil {
		t.Fatal(err)
	}
	want := "main               /repo        clean\n" +
		"feature/long-name  /repo/.wt/x  dirty\n" +
		"界界               /w\n"
	if got := plain.String(); got != want {
		t.Fatalf("plain table = %q, want %q", got, want)
	}

	// A color-capable writer keeps the sequences and the same alignment.
	previous := colorsEnabled
	colorsEnabled = func(io.Writer) bool { return true }
	t.Cleanup(func() { colorsEnabled = previous })
	var colored bytes.Buffer
	if err := Table(&colored, rows); err != nil {
		t.Fatal(err)
	}
	want = "\x1b[31mmain\x1b[0m               /repo        clean\n" +
		"feature/long-name  /repo/.wt/x  \x1b[33mdirty\x1b[0m\n" +
		"界界               /w\n"
	if got := colored.String(); got != want {
		t.Fatalf("colored table = %q, want %q", got, want)
	}
	if got := Table(&bytes.Buffer{}, nil); got != nil {
		t.Fatalf("empty table = %v", got)
	}
}
