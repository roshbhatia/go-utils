package diffview

import (
	"strings"
	"testing"
)

func TestRenderDoesNotRepeatFileNameInHunk(t *testing.T) {
	t.Parallel()
	files := Parse("diff --git a/default.nix b/default.nix\n--- a/default.nix\n+++ b/default.nix\n@@ -1 +1 @@\n-old\n+new\n")
	out := Render(Options{Files: files, Width: 120})
	if strings.Count(out, "default.nix") != 1 {
		t.Fatalf("file name count = %d\n%s", strings.Count(out, "default.nix"), out)
	}
	if !strings.Contains(out, "line 1") {
		t.Fatalf("hunk line missing\n%s", out)
	}
}

func TestRenderSummaryKeepsSymbolsWithoutHunks(t *testing.T) {
	t.Parallel()
	files := Parse("diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -2 +2 @@ func run() {\n-old()\n+new()\n")
	out := Render(Options{
		Files: files,
		Symbols: map[string][]Symbol{
			"main.go": {{Kind: "function", Name: "run", From: 1, To: 3}},
		},
		Summary: true,
		Width:   100,
	})
	if !strings.Contains(out, "function run") {
		t.Fatalf("symbol missing\n%s", out)
	}
	if strings.Contains(out, "old()") || strings.Contains(out, "new()") {
		t.Fatalf("summary contains hunk body\n%s", out)
	}
}
