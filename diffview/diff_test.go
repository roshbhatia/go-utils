package diffview

import "testing"

func TestParseDoesNotCarryMetadataIntoPreviousFile(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/one.go b/one.go\n--- a/one.go\n+++ b/one.go\n@@ -1 +1 @@\n-old\n+new\ndiff --git a/two.go b/two.go\nindex 1111111..2222222 100644\n--- a/two.go\n+++ b/two.go\n@@ -1 +1 @@\n-before\n+after\n"

	files := Parse(patch)
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if got := files[0].Hunks[0].Lines; len(got) != 2 {
		t.Fatalf("first hunk line count = %d, want 2: %#v", len(got), got)
	}
}
