package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// captureStderr points os.Stderr at a file for the test and returns a reader
// for what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stderr")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = file
	t.Cleanup(func() {
		os.Stderr = previous
		file.Close()
	})
	return func() string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func TestDiagnosticBytesWithAndWithoutColor(t *testing.T) {
	previous := colorsEnabled
	t.Cleanup(func() { SetColorsEnabled(previous) })

	SetColorsEnabled(true)
	read := captureStderr(t)
	Fatal(os.Stderr, "no such branch")
	Hint(os.Stderr, "run git fetch first")
	Diagnostic(os.Stderr, "Warning", "tree is dirty")
	Diagnostic(os.Stderr, "note", "unknown kinds stay plain")
	want := "\x1b[38;5;204mfatal:\x1b[0m no such branch\n" +
		"\x1b[38;5;245mhint:\x1b[0m run git fetch first\n" +
		"\x1b[38;5;214mwarning:\x1b[0m tree is dirty\n" +
		"note: unknown kinds stay plain\n"
	if got := read(); got != want {
		t.Fatalf("colored stderr = %q, want %q", got, want)
	}

	var plain bytes.Buffer
	Fatal(&plain, "no such branch")
	Hint(&plain, "run git fetch first")
	if got := plain.String(); got != "fatal: no such branch\nhint: run git fetch first\n" {
		t.Fatalf("buffer output = %q", got)
	}

	SetColorsEnabled(false)
	read = captureStderr(t)
	Fatal(os.Stderr, "no such branch")
	if got := read(); got != "fatal: no such branch\n" {
		t.Fatalf("uncolored stderr = %q", got)
	}
}

func TestDiagnosticFollowsStdoutDetection(t *testing.T) {
	previous := stdoutColorsEnabled
	t.Cleanup(func() { SetStdoutColorsEnabled(previous) })

	path := filepath.Join(t.TempDir(), "stdout")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	previousStdout := os.Stdout
	os.Stdout = file
	t.Cleanup(func() {
		os.Stdout = previousStdout
		file.Close()
	})

	SetStdoutColorsEnabled(true)
	Hint(os.Stdout, "colored")
	SetStdoutColorsEnabled(false)
	Hint(os.Stdout, "plain")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "\x1b[38;5;245mhint:\x1b[0m colored\nhint: plain\n" {
		t.Fatalf("stdout = %q", got)
	}
}
