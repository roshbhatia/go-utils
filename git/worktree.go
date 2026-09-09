package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Worktree is one record from `git worktree list --porcelain`.
type Worktree struct {
	Path        string
	HEAD        string
	Branch      string
	LockReason  string
	PruneReason string
	Bare        bool
	Detached    bool
	Locked      bool
	Prunable    bool
}

// WorktreeAddOptions shapes one `git worktree add` call.
type WorktreeAddOptions struct {
	// Branch names the branch to create with -b, or to check out when Reuse
	// is set and it already exists. Empty lets git pick, as it does on the
	// command line: the last path component becomes the branch name.
	Branch string
	// Start is the commit the new worktree starts from. Empty means HEAD.
	Start string
	// Reuse checks out an existing Branch instead of failing on -b. When it
	// applies, WorktreeAdd reports reused as true.
	Reuse bool
	// Detach checks out the start point without a branch.
	Detach bool
}

// CommonDir returns the absolute path of the directory shared by every
// worktree of the repository at dir, from `git rev-parse --git-common-dir`.
func CommonDir(dir string) (string, error) {
	common, err := Output(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(common) {
		return filepath.Clean(common), nil
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", dir, err)
	}
	return filepath.Join(base, common), nil
}

// MainWorktree returns the main working tree of the repository at dir. A
// linked worktree resolves to the tree that holds the common directory. A
// bare repository has no main working tree and is an error.
func MainWorktree(dir string) (string, error) {
	bare, err := Output(dir, "rev-parse", "--is-bare-repository")
	if err != nil {
		return "", err
	}
	if bare == "true" {
		return "", fmt.Errorf("%s is a bare repository without a main working tree", dir)
	}
	common, err := CommonDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(common), nil
}

// Worktrees lists every worktree of the repository at repo, main tree first.
// It reads `git worktree list --porcelain -z`, whose LIST OUTPUT FORMAT
// git-worktree(1) documents as stable and recommends with -z so that paths
// and lock reasons arrive unquoted.
func Worktrees(repo string) ([]Worktree, error) {
	args := []string{"worktree", "list", "--porcelain", "-z"}
	stdout, stderr, err := output(repo, args...)
	if err != nil {
		return nil, failure(repo, args, stderr, err)
	}
	return parseWorktrees(stdout), nil
}

// parseWorktrees reads records of NUL-terminated attribute lines, each record
// closed by one empty line.
func parseWorktrees(porcelain string) []Worktree {
	var trees []Worktree
	var current *Worktree
	for _, line := range strings.Split(porcelain, "\x00") {
		if line == "" {
			if current != nil {
				trees = append(trees, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &Worktree{}
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Path = value
		case "HEAD":
			current.HEAD = value
		case "branch":
			current.Branch = value
		case "bare":
			current.Bare = true
		case "detached":
			current.Detached = true
		case "locked":
			current.Locked = true
			current.LockReason = value
		case "prunable":
			current.Prunable = true
			current.PruneReason = value
		}
	}
	if current != nil {
		trees = append(trees, *current)
	}
	return trees
}

// WorktreeAdd creates a worktree at path for the repository at repo. It
// mirrors the command line: without a Branch, git names one after path; with
// Reuse, an existing Branch is checked out instead of created and reused is
// true so a caller that wanted a fresh branch can warn. It never passes
// --no-guess-remote, so the user's worktree.guessRemote setting applies.
func WorktreeAdd(repo, path string, o WorktreeAddOptions) (reused bool, err error) {
	args := []string{"worktree", "add"}
	if o.Detach {
		args = append(args, "--detach")
	}
	if o.Branch != "" && o.Reuse && Succeeds(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+o.Branch) {
		reused = true
		args = append(args, path, o.Branch)
	} else {
		if o.Branch != "" {
			args = append(args, "-b", o.Branch)
		}
		args = append(args, path)
		if o.Start != "" {
			args = append(args, o.Start)
		}
	}
	if err := Run(repo, args...); err != nil {
		return false, err
	}
	return reused, nil
}

// WorktreeRemove removes the worktree at path. force passes that many
// --force flags: 0 refuses a dirty tree, 1 discards local changes, 2 also
// removes a locked worktree.
func WorktreeRemove(repo, path string, force int) error {
	args := []string{"worktree", "remove"}
	for range max(0, min(force, 2)) {
		args = append(args, "--force")
	}
	return Run(repo, append(args, path)...)
}

// WorktreePrune drops administrative files for worktrees whose directories
// are gone.
func WorktreePrune(repo string) error {
	return Run(repo, "worktree", "prune")
}

// WorktreeRepair reconnects worktree administrative files after a move. With
// no paths it repairs from the current repository; each path names a moved
// worktree to repair.
func WorktreeRepair(repo string, paths ...string) error {
	return Run(repo, append([]string{"worktree", "repair"}, paths...)...)
}
