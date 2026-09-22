package config

// schema_test.go — the manifest.go precedent (internal/semantic): proves
// the embedded config_schema.cue is byte-identical to its source
// (schema/config/schema.cue) and asserts the negatives documented, commented
// out, in schema/config/testdata/invalid.cue actually fail validation.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestConfigSchemaMatchesSource is the drift gate CF-1 requires: the
// embedded config_schema.cue (go:embed'd into the binary) must be
// byte-identical to schema/config/schema.cue, which stays the single
// authoritative source.
func TestConfigSchemaMatchesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "schema", "config", "schema.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	if string(want) != string(configSchema) {
		t.Fatalf(
			"internal/config/config_schema.cue has drifted from %s — "+
				"copy the source file verbatim (schema/config/schema.cue is authoritative)",
			src,
		)
	}
}

func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "config", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

// TestConfigSchema_Negatives mirrors schema/config/testdata/invalid.cue's
// four commented cases — asserted here in Go so `cue vet ./...` stays green
// (repo convention: negatives are Go-asserted, never live-vetted).
func TestConfigSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"literal api_key field (closed #Profile has none)", `
bad1: #Config & {
	profiles: leaked: {
		provider: "anthropic"
		api_key:  "sk-ant-not-representable"
	}
}`},
		{"unknown provider value", `
bad2: #Config & {
	profiles: typo: {
		provider: "anthropic-cloud"
	}
}`},
		{"empty api_key_cmd", `
bad3: #Config & {
	profiles: broken: {
		provider:     "openai"
		api_key_cmd: []
	}
}`},
		{"profile missing required provider", `
bad4: #Config & {
	profiles: noprovider: {
		model: "gpt-4o"
	}
}`},
		{"unrecognised daemon.state_dir value", `
bad5: #Config & {
	daemon: state_dir: "/abs/path"
}`},
		{"unknown [decide] provider value", `
bad6: #Config & {
	decide: provider: "jev"
}`},
		{"[decide] threshold outside [0,1]", `
bad7: #Config & {
	decide: compile_threshold: 1.5
}`},
	}

	cctx := cuecontext.New()

	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			if v.Err() != nil {
				return // compile-time rejection is a pass
			}
			// Concrete(true) matches config.go's own validateAgainstSchema —
			// a required field left entirely absent (case 4) is structurally
			// valid but INCOMPLETE, which only a concreteness check catches;
			// the plain Validate() the semantic package's negatives use is
			// too lenient for that case.
			if err := v.Validate(cue.Concrete(true)); err == nil {
				t.Errorf("expected validation failure, got none")
			}
		})
	}
}
