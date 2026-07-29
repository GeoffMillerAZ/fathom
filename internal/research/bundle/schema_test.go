package bundle

// schema_test.go mirrors internal/config/schema_test.go's precedent: proves
// the embedded research_bundle_schema.cue is byte-identical to its source
// (schema/research-bundle/schema.cue) and asserts the negatives documented,
// commented out, in schema/research-bundle/testdata/invalid.cue actually
// fail validation.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func schemaSourcePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "research-bundle", "schema.cue")
}

// TestResearchBundleSchemaMatchesSource is the drift gate: the embedded
// research_bundle_schema.cue (go:embed'd into the binary) must be
// byte-identical to schema/research-bundle/schema.cue, which stays the
// single authoritative source.
func TestResearchBundleSchemaMatchesSource(t *testing.T) {
	src := schemaSourcePath(t)
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	if string(want) != string(researchBundleSchema) {
		t.Fatalf(
			"internal/research/bundle/research_bundle_schema.cue has drifted from %s — "+
				"copy the source file verbatim (schema/research-bundle/schema.cue is authoritative)",
			src,
		)
	}
}

func schemaSourceText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(schemaSourcePath(t))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

// commonFields is the minimal set of required-field values every negative
// case below needs, so each test case only has to add or corrupt the one
// field it is testing.
const commonFields = `
	schema_version: "v1"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"
	total_calls:    1
	injection_seen:                   0
	injection_answered:               0
	obligations_served:               0
	verify_claimed:                   0
	verify_verified:                  0
	verify_failing:                   0
	verify_instrumented_calls:        0
	verify_predates_instrumentation:  0
	review_active_findings:           0
	staleness_report_calls:           0
`

// TestResearchBundleSchema_Negatives mirrors
// schema/research-bundle/testdata/invalid.cue's four commented cases —
// asserted here in Go so `cue vet ./...` stays green (repo convention:
// negatives are Go-asserted, never live-vetted).
func TestResearchBundleSchema_Negatives(t *testing.T) {
	schema := schemaSourceText(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"unknown field rejected by the closed struct", `
bad1: #Bundle & {` + commonFields + `
	file_path: "/Users/someone/repo/secret.go"
}`},
		{"service_hash not a 64-hex-char digest", `
bad2: #Bundle & {` + commonFields + `
	service_activity: [{service_hash: "my-secret-service-name", calls: 3}]
}`},
		{"negative total_calls", `
bad3: #Bundle & {
	schema_version: "v1"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"
	total_calls:    -1
	injection_seen:                   0
	injection_answered:               0
	obligations_served:               0
	verify_claimed:                   0
	verify_verified:                  0
	verify_failing:                   0
	verify_instrumented_calls:        0
	verify_predates_instrumentation:  0
	review_active_findings:           0
	staleness_report_calls:           0
}`},
		{"required field total_calls missing entirely", `
bad4: #Bundle & {
	schema_version: "v1"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"
	injection_seen:                   0
	injection_answered:               0
	obligations_served:               0
	verify_claimed:                   0
	verify_verified:                  0
	verify_failing:                   0
	verify_instrumented_calls:        0
	verify_predates_instrumentation:  0
	review_active_findings:           0
	staleness_report_calls:           0
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
			if err := v.Validate(cue.Concrete(true)); err == nil {
				t.Errorf("expected validation failure, got none")
			}
		})
	}
}

// TestValidate_ValidBundle proves Validate accepts a well-formed Bundle
// (the Go-side happy path over the same schema the CUE negatives exercise).
func TestValidate_ValidBundle(t *testing.T) {
	p50, p95, rate := 12.5, 88.0, 0.9
	obs := 6
	open := 2
	b := Bundle{
		SchemaVersion:         SchemaVersionCurrent,
		WindowSince:           "2026-07-21T00:00:00Z",
		WindowUntil:           "2026-07-28T00:00:00Z",
		GeneratedAt:           "2026-07-28T00:00:00Z",
		FathomVersion:         "fathom v0.1.0",
		CommitSHA:             "abc1234def5",
		GOOS:                  "darwin",
		GOARCH:                "arm64",
		TotalCalls:            42,
		LatencyP50MS:          &p50,
		LatencyP95MS:          &p95,
		InjectionCoverageRate: &rate,
		TotalObservations:     &obs,
		OpenObservations:      &open,
	}
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(valid bundle) = %v, want nil", err)
	}
}
