package cell

import (
	"strings"
	"testing"
)

func TestWidthIgnoresANSIAndMeasuresWideRunes(t *testing.T) {
	t.Parallel()

	if got := Width("\x1b[31m界\x1b[0m!"); got != 3 {
		t.Fatalf("Width() = %d, want 3", got)
	}
}

func TestTruncatePreservesANSIReset(t *testing.T) {
	t.Parallel()

	got := Truncate("\x1b[31mabcdef\x1b[0m", 4)
	if Width(got) != 4 || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("Truncate() = %q at width %d", got, Width(got))
	}
	if got := Truncate("abcdef", 1, "long"); Width(got) != 1 {
		t.Fatalf("wide tail result = %q at width %d", got, Width(got))
	}
}

func TestFitProducesExactWidths(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{name: "left pad", got: Fit("ab", 4), want: "ab  "},
		{name: "right pad", got: RightFit("ab", 4), want: "  ab"},
		{name: "truncate", got: Fit("abcdef", 4), want: "abc…"},
		{name: "right truncate", got: RightFit("abcdef", 4), want: "abc…"},
		{name: "ASCII tail", got: Fit("abcdef", 5, "..."), want: "ab..."},
		{name: "right ASCII tail", got: RightFit("abcdef", 5, "..."), want: "ab..."},
		{name: "wide rune ASCII tail", got: Fit("界界界", 5, "..."), want: "界..."},
		{name: "ANSI ASCII tail", got: Fit("\x1b[31mabcdef\x1b[0m", 5, "..."), want: "\x1b[31mab...\x1b[0m"},
		{name: "zero", got: Fit("abcdef", 0), want: ""},
	} {
		if test.got != test.want {
			t.Errorf("%s = %q, want %q", test.name, test.got, test.want)
		}
	}
}

func TestClipWord(t *testing.T) {
	t.Parallel()

	if got := ClipWord("alpha beta gamma", 11); got != "alpha beta…" {
		t.Fatalf("ClipWord() = %q", got)
	}
	if got := ClipWord("supercalifragilistic", 6); got != "super…" {
		t.Fatalf("ClipWord(long word) = %q", got)
	}
	if got := ClipWord("界界界", 1); Width(got) > 1 {
		t.Fatalf("ClipWord(wide) = %q at width %d", got, Width(got))
	}
	if got := ClipWord("\x1b[31malpha beta gamma\x1b[0m", 11); Width(got) != 11 || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("ClipWord(ANSI) = %q at width %d", got, Width(got))
	}
	if got := ClipWord("alpha beta gamma", 13, "..."); got != "alpha beta..." {
		t.Fatalf("ClipWord(ASCII tail) = %q", got)
	}
	if got := ClipWord("\x1b[31malpha beta gamma\x1b[0m", 13, "..."); Width(got) != 13 || !strings.HasSuffix(got, "...") || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("ClipWord(ANSI ASCII tail) = %q at width %d", got, Width(got))
	}
	if got := ClipWord("界界界界", 7, "..."); got != "界界..." || Width(got) != 7 {
		t.Fatalf("ClipWord(wide ASCII tail) = %q at width %d", got, Width(got))
	}
}

func TestOneLinePreservesANSIAndNormalizesText(t *testing.T) {
	t.Parallel()

	got := OneLine("  \x1b[31mhello\n\tworld\x1b[0m\x00  ")
	want := "\x1b[31mhello world\x1b[0m"
	if got != want {
		t.Fatalf("OneLine() = %q, want %q", got, want)
	}
}

func TestOneLineStripsNonSGRControls(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "OSC", value: "before \x1b]11;rgb:0000/0000/0000\aafter", want: "before after"},
		{name: "DCS", value: "before \x1bP>|WezTerm 1;OK\x1b\\after", want: "before after"},
		{name: "unterminated OSC", value: "before \x1b]11;rgb:0000", want: "before"},
		{name: "unterminated DCS", value: "before \x1bP>|WezTerm", want: "before"},
		{name: "CSI movement", value: "before\x1b[3Cafter", want: "beforeafter"},
		{name: "unclosed SGR", value: "\x1b[31mred", want: "\x1b[31mred\x1b[0m"},
		{name: "C1 OSC", value: "before \u009d11;rgb:0000\aafter", want: "before after"},
		{name: "C1 DCS", value: "before \u0090device\u009cafter", want: "before after"},
	} {
		if got := OneLine(test.value); got != test.want {
			t.Errorf("%s: OneLine() = %q, want %q", test.name, got, test.want)
		}
	}
}
