package provider

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func validManifest() Manifest {
	return Manifest{
		Version:     Version,
		Name:        "example",
		Description: "Example provider",
		Command:     []string{"example-provider"},
		Actions: map[string]Action{
			"diff.inspect": {
				Description: "Inspect a diff",
				Argv:        []string{"--repo", "{{ .Input.repository }}"},
				Env:         map[string]string{"REQUEST_ID": "{{ .RequestID }}"},
			},
		},
		Defaults: Defaults{Timeout: Duration(5 * time.Second), Priority: 10},
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	for _, test := range []struct {
		name      string
		extension string
		source    string
	}{
		{"yaml", ".yaml", `version: provider/v1
name: example
description: Example
command: [echo]
actions:
  inspect:
    description: Inspect
    typo: true
`},
		{"json", ".json", `{"version":"provider/v1","name":"example","description":"Example","command":["echo"],"actions":{"inspect":{"description":"Inspect","typo":true}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(test.source), test.extension); err == nil {
				t.Fatal("unknown field was accepted")
			}
		})
	}
}

func TestDiscoverSortsAndRejectsDuplicateNames(t *testing.T) {
	directory := t.TempDir()
	writeManifest := func(name, providerName string) {
		t.Helper()
		manifest := validManifest()
		manifest.Name = providerName
		source, err := yaml.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, name), source, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest("b.yaml", "beta")
	writeManifest("a.yaml", "alpha")
	loaded, err := Discover(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded[0].Manifest.Name + "," + loaded[1].Manifest.Name; got != "alpha,beta" {
		t.Fatalf("discovery order = %q", got)
	}
	writeManifest("c.yaml", "alpha")
	if _, err := Discover(directory); err == nil || !strings.Contains(err.Error(), "duplicate provider") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestRenderUsesStrictTemplatesAndPreservesArguments(t *testing.T) {
	manifest := validManifest()
	data := map[string]any{
		"RequestID": "request-1",
		"Input": map[string]any{
			"repository": "a path; touch should-not-run",
		},
	}
	plan, err := manifest.Render("diff.inspect", data)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.Argv, "|"); got != "example-provider|--repo|a path; touch should-not-run" {
		t.Fatalf("argv = %q", got)
	}
	if plan.Env["REQUEST_ID"] != "request-1" || plan.Timeout != 5*time.Second || plan.Priority != 10 {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := manifest.Render("diff.inspect", map[string]any{"RequestID": "request-1", "Input": map[string]any{}}); err == nil {
		t.Fatal("missing template key was accepted")
	}
}

func TestSchemaIsDeterministic(t *testing.T) {
	first, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("schema output changed between calls")
	}
	for _, want := range []string{`"provider/v1"`, `"actions"`, `"timeout"`} {
		if !bytes.Contains(first, []byte(want)) {
			t.Fatalf("schema does not contain %s", want)
		}
	}
}

func TestResolveModelUsesDeclaredRoles(t *testing.T) {
	manifest := Manifest{Name: "sample", Defaults: Defaults{Model: "big", Light: "small"}}
	for requested, want := range map[string]string{"": "big", "default": "big", "light": "small", "other-id": "other-id"} {
		got, err := manifest.ResolveModel(requested)
		if err != nil || got != want {
			t.Fatalf("ResolveModel(%q) = %q, %v; want %q", requested, got, err, want)
		}
	}
	bare := Manifest{Name: "bare"}
	if got, err := bare.ResolveModel(""); err != nil || got != "" {
		t.Fatalf("a provider with no default resolved to %q, %v", got, err)
	}
	if _, err := bare.ResolveModel("light"); err == nil {
		t.Fatal("a provider with no light model resolved a light request")
	}
}

func TestValidateRejectsRoleWordsAsModelIDs(t *testing.T) {
	manifest := Manifest{
		Version: Version, Name: "sample", Description: "d", Command: []string{"x"},
		Actions:  map[string]Action{"a": {Description: "d"}},
		Defaults: Defaults{Model: "light", Light: "  "},
	}
	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "role word") || !strings.Contains(err.Error(), "blank") {
		t.Fatalf("Validate = %v", err)
	}
}

func TestDurationTextIsSpecGrammar(t *testing.T) {
	for text, want := range map[string]time.Duration{
		"500ms":       500 * time.Millisecond,
		"1h30m":       90 * time.Minute,
		"1.5s":        1500 * time.Millisecond,
		"1h30m15.25s": 90*time.Minute + 15250*time.Millisecond,
		"0s":          0,
	} {
		var got Duration
		if err := got.UnmarshalText([]byte(text)); err != nil || got.Duration() != want {
			t.Errorf("UnmarshalText(%q) = %v, %v; want %v", text, got.Duration(), err, want)
		}
	}
	// time.ParseDuration accepts every one of these; the spec grammar does not.
	for _, text := range []string{"0", "-1s", "+1s", ".5s", "1.s"} {
		var got Duration
		if err := got.UnmarshalText([]byte(text)); err == nil {
			t.Errorf("UnmarshalText(%q) accepted a duration outside the spec grammar", text)
		}
	}
}
