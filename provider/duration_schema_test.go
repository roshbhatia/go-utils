package provider

import (
	"testing"

	"github.com/invopop/jsonschema"
)

// A consumer that reflects a config struct holding Duration must get a string
// with the spec grammar, not the underlying integer.
func TestDurationReflectsAsSpecString(t *testing.T) {
	type cfg struct {
		Timeout Duration `json:"timeout"`
	}
	schema := (&jsonschema.Reflector{}).Reflect(&cfg{})
	def, ok := schema.Definitions["Duration"]
	if !ok {
		t.Fatalf("no Duration definition; definitions: %v", keys(schema.Definitions))
	}
	if def.Type != "string" {
		t.Fatalf("Duration type = %q, want string", def.Type)
	}
	if def.Pattern != durationPattern.String() {
		t.Fatalf("Duration pattern = %q, want %q", def.Pattern, durationPattern.String())
	}
}

func keys(m jsonschema.Definitions) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
