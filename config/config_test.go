package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testConfig struct {
	Color     string        `json:"color" yaml:"color" jsonschema:"enum=auto,enum=always,enum=never"`
	Enabled   bool          `json:"enabled" yaml:"enabled"`
	Interval  time.Duration `json:"interval" yaml:"interval"`
	Providers struct {
		Diff []string `json:"diff" yaml:"diff"`
	} `json:"providers" yaml:"providers"`
}

func TestLoadPrecedence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("color: always\nenabled: true\nproviders:\n  diff: [git]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_TOOL_COLOR", "never")
	t.Setenv("TEST_TOOL_INTERVAL", "15s")
	t.Setenv("TEST_TOOL_PROVIDERS_DIFF", "[delta, difftastic]")

	got, err := Load(testConfig{Color: "auto"}, Options{
		Name: "test-tool", EnvPrefix: "TEST_TOOL", Path: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Color != "never" || !got.Enabled || got.Interval != 15*time.Second {
		t.Fatalf("loaded config = %+v", got)
	}
	if strings.Join(got.Providers.Diff, ",") != "delta,difftastic" {
		t.Fatalf("providers = %v", got.Providers.Diff)
	}
}

func TestLoadRejectsUnknownYAMLFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(testConfig{}, Options{Name: "test", Path: path}); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func TestSchemaDescribesConfig(t *testing.T) {
	data, err := Schema[testConfig]("Test configuration")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"$schema"`, `"color"`, `"providers"`, `"Test configuration"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema does not contain %s:\n%s", want, text)
		}
	}
}
