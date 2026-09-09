package git

import "fmt"

// ConfigGet returns the effective value of key as `git config --get` resolves
// it across the local, global, and system files. The bool reports whether the
// key is set; an unset key is not an error.
func ConfigGet(dir, key string) (string, bool, error) {
	value, stderr, err := output(dir, "config", "--get", key)
	if err == nil {
		return value, true, nil
	}
	// git exits 1 both for an unset key and for a malformed one, and only
	// the malformed one writes to stderr.
	if ExitStatus(err) == 1 && stderr == "" {
		return "", false, nil
	}
	return "", false, fmt.Errorf("git config --get %s in %s: %s: %w", key, dir, stderr, err)
}

// ConfigSet writes key into the repository-local configuration of dir.
func ConfigSet(dir, key, value string) error {
	return Run(dir, "config", "--local", key, value)
}
