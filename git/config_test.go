package git

import (
	"errors"
	"testing"
)

func TestConfigGetSet(t *testing.T) {
	repo := newRepo(t)
	if value, ok, err := ConfigGet(repo, "goutils.missing"); err != nil || ok || value != "" {
		t.Fatalf("unset key = %q, %t, %v", value, ok, err)
	}
	if err := ConfigSet(repo, "goutils.answer", "forty two"); err != nil {
		t.Fatal(err)
	}
	if value, ok, err := ConfigGet(repo, "goutils.answer"); err != nil || !ok || value != "forty two" {
		t.Fatalf("set key = %q, %t, %v", value, ok, err)
	}
	if _, _, err := ConfigGet(repo, "nosection"); err == nil {
		t.Fatal("malformed key was reported as unset")
	}
}

func TestExitStatus(t *testing.T) {
	t.Parallel()

	if got := ExitStatus(errors.New("plain")); got != -1 {
		t.Fatalf("ExitStatus(plain) = %d", got)
	}
	if got := ExitStatus(nil); got != -1 {
		t.Fatalf("ExitStatus(nil) = %d", got)
	}
	_, err := Output(t.TempDir(), "rev-parse", "HEAD")
	if got := ExitStatus(err); got != 128 {
		t.Fatalf("ExitStatus(outside repo) = %d: %v", got, err)
	}
}
