package xdg

import (
	"path/filepath"
	"testing"
)

func TestResolveDefaultsAndOverrides(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	roots, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if roots.Config != config || roots.Cache != filepath.Join(home, ".cache") ||
		roots.Data != filepath.Join(home, ".local", "share") ||
		roots.State != filepath.Join(home, ".local", "state") || roots.Runtime != "" {
		t.Fatalf("Resolve() = %+v", roots)
	}
}

func TestResolveIgnoresRelativeRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "relative")
	roots, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if roots.State != filepath.Join(home, ".local", "state") {
		t.Fatalf("state root = %q", roots.State)
	}
}

func TestAccessorIgnoresUnrelatedInvalidRoot(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", "relative")
	got, err := ConfigHome()
	if err != nil || got != config {
		t.Fatalf("ConfigHome() = %q, %v", got, err)
	}
}

func TestRuntimeDirIgnoresRelativeValue(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "relative")
	got, err := RuntimeDir()
	if err != nil || got != "" {
		t.Fatalf("RuntimeDir() = %q, %v", got, err)
	}
}
