package intent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIntentSchemaMatchesSource is the drift gate: the embedded
// intent_schema.cue (`fathom intent audit` vets futures with it) must be
// byte-identical to schema/intent/schema.cue, the authoritative source.
func TestIntentSchemaMatchesSource(t *testing.T) {
	src := filepath.Join("..", "..", "schema", "intent", "schema.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	if string(want) != string(intentSchema) {
		t.Fatalf("internal/intent/intent_schema.cue has drifted from %s — copy the source file verbatim", src)
	}
}
