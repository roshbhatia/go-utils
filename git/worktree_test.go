package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo returns a repository with one commit and no user or system
// configuration in scope.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	// git reports real paths, and macOS hides /var behind a symlink.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "main")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if err := Run(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func sibling(repo, name string) string { return filepath.Join(filepath.Dir(repo), name) }

func TestCommonDirIsAbsoluteEverywhere(t *testing.T) {
	repo := newRepo(t)
	want := filepath.Join(repo, ".git")
	if _, err := WorktreeAdd(repo, sibling(repo, "linked"), WorktreeAddOptions{Branch: "feat"}); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{repo, sub, sibling(repo, "linked")} {
		got, err := CommonDir(dir)
		if err != nil || got != want {
			t.Errorf("CommonDir(%s) = %q, %v, want %q", dir, got, err, want)
		}
	}
}

func TestMainWorktreeMatchesFirstListedTree(t *testing.T) {
	repo := newRepo(t)
	linked := sibling(repo, "linked")
	if _, err := WorktreeAdd(repo, linked, WorktreeAddOptions{Branch: "feat"}); err != nil {
		t.Fatal(err)
	}
	listed, err := Output(linked, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	first := strings.TrimPrefix(strings.SplitN(listed, "\n", 2)[0], "worktree ")
	for _, dir := range []string{repo, linked} {
		got, err := MainWorktree(dir)
		if err != nil || got != first {
			t.Errorf("MainWorktree(%s) = %q, %v, want %q", dir, got, err, first)
		}
	}
	bare := sibling(repo, "bare.git")
	if err := Run(filepath.Dir(repo), "init", "-q", "--bare", bare); err != nil {
		t.Fatal(err)
	}
	if _, err := MainWorktree(bare); err == nil {
		t.Fatal("bare repository has a main worktree")
	}
}

func TestWorktreesParsesEveryPorcelainAttribute(t *testing.T) {
	repo := newRepo(t)
	head, err := Head(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		path string
		o    WorktreeAddOptions
	}{
		{sibling(repo, "linked"), WorktreeAddOptions{Branch: "feat"}},
		{sibling(repo, "detached"), WorktreeAddOptions{Detach: true}},
		{sibling(repo, "locked"), WorktreeAddOptions{Branch: "locked"}},
		{sibling(repo, "gone"), WorktreeAddOptions{Branch: "gone"}},
	} {
		if _, err := WorktreeAdd(repo, step.path, step.o); err != nil {
			t.Fatalf("WorktreeAdd(%s): %v", step.path, err)
		}
	}
	if err := Run(repo, "worktree", "lock", "--reason", "busy work", sibling(repo, "locked")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(sibling(repo, "gone")); err != nil {
		t.Fatal(err)
	}

	trees, err := Worktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Worktree{}
	for _, tree := range trees {
		byPath[filepath.Base(tree.Path)] = tree
		if tree.HEAD != head {
			t.Errorf("%s HEAD = %q, want %q", tree.Path, tree.HEAD, head)
		}
	}
	if len(trees) != 5 || trees[0].Path != repo {
		t.Fatalf("Worktrees() = %+v", trees)
	}
	want := map[string]Worktree{
		"main":     {Branch: "refs/heads/main"},
		"linked":   {Branch: "refs/heads/feat"},
		"detached": {Detached: true},
		"locked":   {Branch: "refs/heads/locked", Locked: true, LockReason: "busy work"},
		"gone":     {Branch: "refs/heads/gone", Prunable: true, PruneReason: "gitdir file points to non-existent location"},
	}
	for name, expect := range want {
		got := byPath[name]
		got.Path, got.HEAD = "", ""
		if got != expect {
			t.Errorf("%s = %+v, want %+v", name, got, expect)
		}
	}

	bare := sibling(repo, "bare.git")
	if err := Run(filepath.Dir(repo), "init", "-q", "--bare", bare); err != nil {
		t.Fatal(err)
	}
	trees, err = Worktrees(bare)
	if err != nil || len(trees) != 1 || !trees[0].Bare || trees[0].Path != bare {
		t.Fatalf("bare Worktrees() = %+v, %v", trees, err)
	}
}

func TestParseWorktreesKeepsSpacesInPathsAndReasons(t *testing.T) {
	t.Parallel()

	porcelain := "worktree /tmp/with space\x00HEAD abc\x00branch refs/heads/x\x00locked reason with spaces\x00\x00" +
		"worktree /tmp/bare\x00bare\x00\x00"
	trees := parseWorktrees(porcelain)
	if len(trees) != 2 || trees[0].Path != "/tmp/with space" || trees[0].LockReason != "reason with spaces" || !trees[1].Bare {
		t.Fatalf("parseWorktrees() = %+v", trees)
	}
	if trees := parseWorktrees(""); len(trees) != 0 {
		t.Fatalf("empty porcelain parsed as %+v", trees)
	}
}

func TestWorktreeAddReuseAndStart(t *testing.T) {
	repo := newRepo(t)
	if err := Run(repo, "branch", "existing"); err != nil {
		t.Fatal(err)
	}
	reused, err := WorktreeAdd(repo, sibling(repo, "fresh"), WorktreeAddOptions{Branch: "existing"})
	if err == nil || reused {
		t.Fatalf("-b on an existing branch = %t, %v", reused, err)
	}
	reused, err = WorktreeAdd(repo, sibling(repo, "reused"), WorktreeAddOptions{Branch: "existing", Reuse: true})
	if err != nil || !reused {
		t.Fatalf("reuse of an existing branch = %t, %v", reused, err)
	}
	if branch, err := Branch(sibling(repo, "reused")); err != nil || branch != "existing" {
		t.Fatalf("reused branch = %q, %v", branch, err)
	}
	reused, err = WorktreeAdd(repo, sibling(repo, "new"), WorktreeAddOptions{Branch: "new", Reuse: true, Start: "main"})
	if err != nil || reused {
		t.Fatalf("reuse of a missing branch = %t, %v", reused, err)
	}
	if branch, err := Branch(sibling(repo, "new")); err != nil || branch != "new" {
		t.Fatalf("new branch = %q, %v", branch, err)
	}
	if _, err := WorktreeAdd(repo, sibling(repo, "named"), WorktreeAddOptions{}); err != nil {
		t.Fatal(err)
	}
	if branch, err := Branch(sibling(repo, "named")); err != nil || branch != "named" {
		t.Fatalf("git-named branch = %q, %v", branch, err)
	}
}

func TestWorktreeRemovePruneRepair(t *testing.T) {
	repo := newRepo(t)
	dirty := sibling(repo, "dirty")
	if _, err := WorktreeAdd(repo, dirty, WorktreeAddOptions{Branch: "dirty"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, "note"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WorktreeRemove(repo, dirty, 0); err == nil {
		t.Fatal("dirty worktree removed without force")
	}
	if err := Run(repo, "worktree", "lock", dirty); err != nil {
		t.Fatal(err)
	}
	if err := WorktreeRemove(repo, dirty, 1); err == nil {
		t.Fatal("locked worktree removed with one force")
	}
	if err := WorktreeRemove(repo, dirty, 2); err != nil {
		t.Fatal(err)
	}

	gone := sibling(repo, "gone")
	if _, err := WorktreeAdd(repo, gone, WorktreeAddOptions{Branch: "gone"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	if err := WorktreePrune(repo); err != nil {
		t.Fatal(err)
	}
	trees, err := Worktrees(repo)
	if err != nil || len(trees) != 1 {
		t.Fatalf("after prune Worktrees() = %+v, %v", trees, err)
	}

	moved, target := sibling(repo, "moved"), sibling(repo, "elsewhere")
	if _, err := WorktreeAdd(repo, moved, WorktreeAddOptions{Branch: "moved"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, target); err != nil {
		t.Fatal(err)
	}
	if err := WorktreeRepair(repo, target); err != nil {
		t.Fatal(err)
	}
	trees, err = Worktrees(repo)
	if err != nil || len(trees) != 2 || trees[1].Path != target || trees[1].Prunable {
		t.Fatalf("after repair Worktrees() = %+v, %v", trees, err)
	}
}
