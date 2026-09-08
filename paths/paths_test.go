package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictRootsIgnoreRelativeEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "relative")
	got, err := StateHomeE()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "state")
	if got != want || StateHome() != want {
		t.Fatalf("state homes = %q and %q, want %q", got, StateHome(), want)
	}
}

func TestGetEValidatesManifestAndValues(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "paths.json")
	t.Setenv("SYSINIT_PATHS_MANIFEST", manifest)
	if err := os.WriteFile(manifest, []byte(`{"paths":{"agents":"relative"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := GetE(AgentsKey); err == nil {
		t.Fatal("relative manifest path was accepted")
	}
}

func TestGetEReturnsCleanAbsoluteValue(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "paths.json")
	value := filepath.Join(directory, "agents", "..", "registry.json")
	t.Setenv("SYSINIT_PATHS_MANIFEST", manifest)
	if err := os.WriteFile(manifest, []byte(`{"paths":{"agents":"`+value+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok, err := GetE(AgentsKey)
	if err != nil || !ok || got != filepath.Clean(value) {
		t.Fatalf("GetE() = %q, %t, %v", got, ok, err)
	}
}
