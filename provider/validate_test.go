package provider

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
)

func TestValidatorReturnsSortedStructuredChecks(t *testing.T) {
	manifest := validManifest()
	manifest.Command = []string{"provider-bin"}
	manifest.Requires = Requirements{
		Commands:    []string{"zeta", "alpha", "alpha"},
		Environment: []string{"TOKEN_Z", "TOKEN_A"},
		Paths:       []string{"zeta", "alpha"},
	}
	validator := Validator{
		LookPath: func(name string) (string, error) {
			if name == "zeta" {
				return "", errors.New("not found")
			}
			return "/bin/" + name, nil
		},
		LookupEnv: func(name string) (string, bool) { return name, name == "TOKEN_A" },
		Stat: func(path string) (fs.FileInfo, error) {
			return nil, errors.New("not found")
		},
	}
	report := validator.Validate(manifest, "/workspace")
	if report.OK() {
		t.Fatal("failed dependencies produced a successful report")
	}
	var targets []string
	for _, check := range report.Checks {
		targets = append(targets, check.Kind+":"+check.Target)
	}
	want := []string{
		"manifest:example",
		"command:alpha", "command:provider-bin", "command:zeta",
		"environment:TOKEN_A", "environment:TOKEN_Z",
		"path:alpha", "path:zeta",
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("checks = %#v, want %#v", targets, want)
	}
}
