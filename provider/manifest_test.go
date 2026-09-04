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
