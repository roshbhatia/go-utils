package paths

import "testing"

// The golden value is the on-disk layout that nvim and the worker CLI address
// directly. A change here is a migration, not a refactor.
func TestKeyedGolden(t *testing.T) {
	const want = "/state/edits/alpha-017094432d2c0fa9"
	if got := Keyed("/state/edits", "/work/alpha"); got != want {
		t.Fatalf("Keyed = %q, want %q", got, want)
	}
}

func TestKeyedSeparatesRootsWithOneBase(t *testing.T) {
	first := Keyed("/state", "/work/alpha")
	second := Keyed("/state", "/other/alpha")
	if first == second {
		t.Fatalf("different roots share %q", first)
	}
	if Keyed("/state", "/work/alpha") != first {
		t.Fatal("same root produced a different path")
	}
}
