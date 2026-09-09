package git

import "fmt"

// RefFormatError reports a branch name that git rejects. Stderr holds git's
// own explanation.
type RefFormatError struct {
	Name   string
	Stderr string
	Err    error
}

func (e *RefFormatError) Error() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	return fmt.Sprintf("invalid branch name %q", e.Name)
}

func (e *RefFormatError) Unwrap() error { return e.Err }

// CheckRefFormat reports whether name is a valid branch name under the rules
// in git check-ref-format(1). It runs `git check-ref-format --branch`, which
// needs no repository, and returns a *RefFormatError when git exits 128.
// Names that git parses as its own options, such as "-x", fail the same way
// because a branch name cannot begin with a dash.
func CheckRefFormat(name string) error {
	_, stderr, err := output("", "check-ref-format", "--branch", name)
	if err == nil {
		return nil
	}
	if ExitStatus(err) < 0 {
		return fmt.Errorf("git check-ref-format: %w", err)
	}
	return &RefFormatError{Name: name, Stderr: stderr, Err: err}
}
