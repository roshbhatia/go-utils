package diffview

import "testing"

func TestParseDoesNotCarryMetadataIntoPreviousFile(t *testing.T) {
	t.Parallel()
	patch := `diff --git a/one.go b/one.go
--- a/one.go
+++ b/one.go
@@ -1 +1 @@
-old
+new
diff --git a/two.go b/two.go
index 1111111..2222222 100644
--- a/two.go
+++ b/two.go
@@ -1 +1 @@
-before
+after
`

	files := Parse(patch)
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if got := files[0].Hunks[0].Lines; len(got) != 2 {
		t.Fatalf("first hunk line count = %d, want 2: %#v", len(got), got)
	}
}
