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
