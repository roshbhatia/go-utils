package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CheckStatus is the outcome of one provider validation check.
type CheckStatus string

const (
	CheckOK      CheckStatus = "ok"
	CheckWarning CheckStatus = "warning"
	CheckFailed  CheckStatus = "failed"
	CheckSkipped CheckStatus = "skipped"
)

// Check is one deterministic validation result.
type Check struct {
	Kind    string      `json:"kind" yaml:"kind"`
	Target  string      `json:"target" yaml:"target"`
	Status  CheckStatus `json:"status" yaml:"status"`
	Message string      `json:"message,omitempty" yaml:"message,omitempty"`
}

// ValidationReport contains manifest and host dependency checks.
type ValidationReport struct {
	Provider string  `json:"provider" yaml:"provider"`
	Checks   []Check `json:"checks" yaml:"checks"`
}

// OK reports whether every check passed.
func (report ValidationReport) OK() bool {
	return len(report.Checks) > 0 && !hasFailedCheck(report.Checks)
}

// Error converts failed checks into one diagnostic.
func (report ValidationReport) Error() error {
	var failures []error
	for _, check := range report.Checks {
		if check.Status == CheckFailed {
			failures = append(failures, fmt.Errorf("%s %q: %s", check.Kind, check.Target, check.Message))
		}
	}
	return errors.Join(failures...)
}

func hasFailedCheck(checks []Check) bool {
	for _, check := range checks {
		if check.Status == CheckFailed {
			return true
		}
	}
	return false
}

// Validator supplies dependency lookups. Its zero value uses the host.
type Validator struct {
	LookPath  func(string) (string, error)
	LookupEnv func(string) (string, bool)
	Stat      func(string) (fs.FileInfo, error)
}

// Validate returns stable, machine-readable checks for a provider and host.
func (validator Validator) Validate(manifest Manifest, workingDirectory string) ValidationReport {
	lookPath := validator.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	lookupEnv := validator.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	stat := validator.Stat
	if stat == nil {
		stat = os.Stat
	}
	report := ValidationReport{Provider: manifest.Name}
	if err := manifest.Validate(); err != nil {
		report.Checks = append(report.Checks, Check{
			Kind: "manifest", Target: manifest.Name, Status: CheckFailed, Message: err.Error(),
		})
		return report
	}
	report.Checks = append(report.Checks, Check{
		Kind: "manifest", Target: manifest.Name, Status: CheckOK,
	})

	commands := append([]string{manifest.Command[0]}, manifest.Requires.Commands...)
	commands = uniqueSorted(commands)
	for _, command := range commands {
		path, err := lookupCommand(command, workingDirectory, lookPath, stat)
		check := Check{Kind: "command", Target: command, Status: CheckOK, Message: path}
		if err != nil {
			check.Status = CheckFailed
			check.Message = err.Error()
		}
		report.Checks = append(report.Checks, check)
	}
	for _, name := range uniqueSorted(manifest.Requires.Environment) {
		value, exists := lookupEnv(name)
		check := Check{Kind: "environment", Target: name, Status: CheckOK}
		if !exists || value == "" {
			check.Status = CheckFailed
			check.Message = "not set"
		}
		report.Checks = append(report.Checks, check)
	}
	for _, required := range uniqueSorted(manifest.Requires.Paths) {
		path := required
		if !filepath.IsAbs(path) && workingDirectory != "" {
			path = filepath.Join(workingDirectory, path)
		}
		path = filepath.Clean(path)
		check := Check{Kind: "path", Target: required, Status: CheckOK, Message: path}
		if _, err := stat(path); err != nil {
			check.Status = CheckFailed
			check.Message = err.Error()
		}
		report.Checks = append(report.Checks, check)
	}
	return report
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func lookupCommand(
	command, workingDirectory string,
	lookPath func(string) (string, error),
	stat func(string) (fs.FileInfo, error),
) (string, error) {
	if strings.ContainsRune(command, filepath.Separator) {
		path := command
		if !filepath.IsAbs(path) && workingDirectory != "" {
			path = filepath.Join(workingDirectory, path)
		}
		path = filepath.Clean(path)
		info, err := stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return "", fmt.Errorf("%s is not executable", path)
		}
		return path, nil
	}
	return lookPath(command)
}
