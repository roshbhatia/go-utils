package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// An inherited GIT_DIR silently retargets a command that names its own
// repository, so every helper here scrubs the three that do it.
var inherited = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_EXTERNAL_DIFF",
	"GIT_DIFF_OPTS",
	"GIT_DIFFTOOL_EXTCMD",
}

func CleanEnv() []string {
	env := os.Environ()
	kept := make([]string, 0, len(env))
	for _, entry := range env {
		drop := false
		for _, name := range inherited {
			if strings.HasPrefix(entry, name+"=") {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, entry)
		}
	}
	return kept
}

func ShadowEnv(gitDir, workTree string) []string {
	return append(CleanEnv(),
		"GIT_DIR="+gitDir,
		"GIT_WORK_TREE="+workTree,
		"GIT_TERMINAL_PROMPT=0",
	)
}

func command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = CleanEnv()
	return cmd
}

// Output runs git in dir and returns its trimmed stdout. The error carries
// stderr, which is the only place git explains itself.
func Output(dir string, args ...string) (string, error) {
	stdout, stderr, err := output(dir, args...)
	if err != nil {
		return "", failure(dir, args, stderr, err)
	}
	return stdout, nil
}

func failure(dir string, args []string, stderr string, err error) error {
	verb := ""
	if len(args) > 0 {
		verb = args[0]
	}
	return fmt.Errorf("git %s failed in %s: %s: %w", verb, dir, stderr, err)
}

// output runs git in dir and returns its trimmed stdout and stderr with the
// raw process error, for callers that read the exit status or stderr itself.
func output(dir string, args ...string) (string, string, error) {
	cmd := command(dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// ExitStatus returns the exit status of the git process behind err, or -1
// when err did not come from a git exit. git reports its own failures with
// 128; a usage error is 129.
func ExitStatus(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

func Run(dir string, args ...string) error {
	_, err := Output(dir, args...)
	return err
}

func Succeeds(dir string, args ...string) bool {
	_, err := Output(dir, args...)
	return err == nil
}

func IsRepo(dir string) bool { return Succeeds(dir, "rev-parse", "--git-dir") }

func Root(dir string) (string, error) {
	root, err := Output(dir, "rev-parse", "--show-toplevel")
	if err != nil || root == "" {
		return "", errors.New("not inside a git repository")
	}
	return root, nil
}

func Head(dir string) (string, error) { return Output(dir, "rev-parse", "HEAD") }

func Branch(dir string) (string, error) {
	return Output(dir, "rev-parse", "--abbrev-ref", "HEAD")
}
