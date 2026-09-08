package provider

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

//go:embed spec/fixtures/manifest
var specFixtures embed.FS

// TestSpecConformance holds this package to provider-spec: every valid fixture
// decodes and passes the published schema, every invalid fixture fails both.
func TestSpecConformance(t *testing.T) {
	schema := compileSpecSchema(t)
	for _, verdict := range []struct {
		directory string
		accept    bool
	}{{"valid", true}, {"invalid", false}} {
		entries, err := fs.ReadDir(specFixtures, path.Join("spec/fixtures/manifest", verdict.directory))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatalf("no %s fixtures", verdict.directory)
		}
		for _, entry := range entries {
			name := path.Join(verdict.directory, entry.Name())
			t.Run(name, func(t *testing.T) {
				source, err := fs.ReadFile(specFixtures, path.Join("spec/fixtures/manifest", name))
				if err != nil {
					t.Fatal(err)
				}
				_, decodeErr := Decode(bytes.NewReader(source), path.Ext(entry.Name()))
				schemaErr := validateAgainstSpec(t, schema, source)
				if verdict.accept {
					if decodeErr != nil {
						t.Errorf("Decode rejected a valid fixture: %v", decodeErr)
					}
					if schemaErr != nil {
						t.Errorf("schema rejected a valid fixture: %v", schemaErr)
					}
					return
				}
				if decodeErr == nil {
					t.Error("Decode accepted an invalid fixture")
				}
				if schemaErr == nil {
					t.Error("schema accepted an invalid fixture")
				}
			})
		}
	}
}

func TestSpecVersionMatchesSchemaCopy(t *testing.T) {
	if SpecVersion == "" || strings.ContainsAny(SpecVersion, " \n") {
		t.Fatalf("SpecVersion = %q", SpecVersion)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(schema, specSchema) {
		t.Fatal("Schema() differs from the embedded spec copy")
	}
	schema[0] = '!'
	if bytes.Equal(schema, specSchema) {
		t.Fatal("Schema() returned the embedded slice, not a copy")
	}
}

func compileSpecSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(specSchema))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("provider.schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("provider.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// validateAgainstSpec checks YAML source against the schema with no knowledge
// of the Go types, so a struct-level rule cannot mask a schema gap.
func validateAgainstSpec(t *testing.T, schema *jsonschema.Schema, source []byte) error {
	t.Helper()
	var document any
	if err := yaml.Unmarshal(source, &document); err != nil {
		return err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return schema.Validate(instance)
}
