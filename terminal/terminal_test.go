package terminal

import (
	"bytes"
	"os"
	"testing"
)

func TestColorsEnabledRequiresTTYAndPermitsNoColor(t *testing.T) {
	var output bytes.Buffer
	if ColorsEnabled(&output) {
		t.Fatal("buffer unexpectedly supports color")
	}
	t.Setenv("NO_COLOR", "1")
	if ColorsEnabled(os.Stderr) {
		t.Fatal("NO_COLOR was ignored")
	}
}

func TestEmptyNoColorDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if colorEnvironmentAllows() {
		t.Fatal("empty NO_COLOR was ignored")
	}
}

func TestControlReplies(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"11;rgb:2424/2727/3a3a",
		"]10;rgb:ffff/ffff/ffff",
		"\x1b]11;rgb:0000/0000/0000\x1b\\",
		">|WezTerm 0-unstable;OK",
		"\x1bP>|WezTerm 0-unstable;OK\x1b\\",
		"alt+]",
		`alt+\`,
	} {
		if !IsControlReply(value) {
			t.Errorf("IsControlReply(%q) = false", value)
		}
	}
	if IsControlReply("normal output") {
		t.Fatal("normal output recognized as a reply")
	}
}

func TestDropControlReplyLinesPreservesOtherANSI(t *testing.T) {
	t.Parallel()

	got := DropControlReplyLines("\x1b[31mkeep\x1b[0m\n11;rgb:0000/0000/0000\nmore\nmessage 11;rgb:0000/0000/0000")
	want := "\x1b[31mkeep\x1b[0m\nmore\nmessage 11;rgb:0000/0000/0000"
	if got != want {
		t.Fatalf("DropControlReplyLines() = %q, want %q", got, want)
	}
}
