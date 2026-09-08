package keymap

import (
	"errors"
	"testing"
)

func exampleCatalog(t *testing.T) Catalog {
	t.Helper()
	catalog, err := New(
		Binding{ID: "focus", Keys: []string{"ctrl+h", "ctrl+l"}, Display: "ctrl+h/l", Short: "pane", Description: "move pane focus"},
		Binding{ID: "tab", Keys: []string{"tab", "shift+tab"}, Short: "tab", Description: "move through local tabs", Contexts: []string{"review"}},
		Binding{ID: "quit", Keys: []string{"q"}, Short: "quit", Description: "leave the application", Hidden: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestCatalogMatchHintsAndHelp(t *testing.T) {
	t.Parallel()
	catalog := exampleCatalog(t)

	if binding, ok, err := catalog.Match(" CTRL+h ", Context{"review"}); err != nil || !ok || binding.ID != "focus" {
		t.Fatalf("Match() = %+v, %t, %v", binding, ok, err)
	}
	if _, ok, err := catalog.Match("tab", Context{"other"}); err != nil || ok {
		t.Fatal("group-specific binding matched the wrong group")
	}
	line, err := catalog.HintLine("   ", "focus", "tab")
	if err != nil || line != "ctrl+h/l pane   tab / shift+tab tab" {
		t.Fatalf("HintLine() = %q, %v", line, err)
	}
	rows, err := catalog.HelpRows(Context{"review"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Keys != "ctrl+h/l" || rows[1].ID != "tab" {
		t.Fatalf("HelpRows() = %+v", rows)
	}
}

func TestCatalogCopiesInputAndOutput(t *testing.T) {
	t.Parallel()
	binding := Binding{ID: "open", Keys: []string{"enter"}, Description: "open item"}
	catalog, err := New(binding)
	if err != nil {
		t.Fatal(err)
	}
	binding.Keys[0] = "q"
	returned := catalog.Bindings()
	returned[0].Keys[0] = "x"
	matched, ok, err := catalog.Match("enter", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("catalog shares caller-owned slices")
	}
	matched.Keys[0] = "x"
	if _, ok, err := catalog.Match("enter", nil); err != nil || !ok {
		t.Fatal("matched binding shares catalog slices")
	}
}

func TestCatalogValidation(t *testing.T) {
	t.Parallel()

	for _, bindings := range [][]Binding{
		{{Keys: []string{"q"}, Description: "quit"}},
		{{ID: "quit", Description: "quit"}},
		{{ID: "quit", Keys: []string{"q"}}},
		{{ID: "quit", Keys: []string{"q", "q"}, Description: "quit"}},
		{{ID: "quit", Keys: []string{"q"}, Description: "quit"}, {ID: "quit", Keys: []string{"x"}, Description: "quit"}},
		{{ID: "context", Keys: []string{"q"}, Description: "context", Contexts: []string{"review", "REVIEW"}}},
	} {
		if _, err := New(bindings...); err == nil {
			t.Fatalf("New(%+v) accepted invalid bindings", bindings)
		}
	}
}

func TestCatalogAllowsSameKeyInDisjointGroups(t *testing.T) {
	t.Parallel()
	catalog, err := New(
		Binding{ID: "files", Keys: []string{"enter"}, Description: "open file", Contexts: []string{"files"}},
		Binding{ID: "history", Keys: []string{"enter"}, Description: "open commit", Contexts: []string{"history"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if binding, ok, err := catalog.Match("enter", Context{"history"}); err != nil || !ok || binding.ID != "history" {
		t.Fatalf("Match() = %+v, %t, %v", binding, ok, err)
	}
	if _, ok, err := catalog.Match("enter", nil); err != nil || ok {
		t.Fatal("group-specific binding matched without an active group")
	}
}

func TestMatchPreservesKeyCase(t *testing.T) {
	t.Parallel()
	catalog, err := New(
		Binding{ID: "start", Keys: []string{"g"}, Description: "go to start"},
		Binding{ID: "end", Keys: []string{"G"}, Description: "go to end"},
		Binding{ID: "ctrl-start", Keys: []string{"ctrl+g"}, Description: "go to start with control"},
		Binding{ID: "ctrl-end", Keys: []string{"ctrl+G"}, Description: "go to end with control"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"g": "start", "G": "end", "CONTROL+g": "ctrl-start", "CTRL+G": "ctrl-end"} {
		binding, ok, matchErr := catalog.Match(key, nil)
		if matchErr != nil || !ok || binding.ID != want {
			t.Errorf("Match(%q) = %+v, %t, %v", key, binding, ok, matchErr)
		}
	}
}

func TestCompoundContextIsConjunctiveAndAmbiguityIsExplicit(t *testing.T) {
	t.Parallel()
	catalog, err := New(
		Binding{ID: "file", Keys: []string{"enter"}, Description: "open file", Contexts: []string{"review", "files"}},
		Binding{ID: "history", Keys: []string{"enter"}, Description: "open commit", Contexts: []string{"review", "history"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, matchErr := catalog.Match("enter", Context{"review"}); matchErr != nil || ok {
		t.Fatalf("partial context matched: %t, %v", ok, matchErr)
	}
	if binding, ok, matchErr := catalog.Match("enter", Context{"review", "files"}); matchErr != nil || !ok || binding.ID != "file" {
		t.Fatalf("compound Match() = %+v, %t, %v", binding, ok, matchErr)
	}
	context := Context{"review", "files", "history"}
	if _, _, matchErr := catalog.Match("enter", context); !errors.Is(matchErr, ErrAmbiguous) {
		t.Fatalf("ambiguous Match() error = %v", matchErr)
	}
	if _, helpErr := catalog.HelpRows(context); !errors.Is(helpErr, ErrAmbiguous) {
		t.Fatalf("ambiguous HelpRows() error = %v", helpErr)
	}
}

func TestUnknownHintFails(t *testing.T) {
	t.Parallel()
	if _, err := exampleCatalog(t).Hint("missing"); err == nil {
		t.Fatal("unknown hint did not fail")
	}
}
