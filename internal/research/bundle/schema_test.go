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
// field it is testing. Every field here is required at #Bundle's top level
// even after Finding 3's optional pointer-groups (schema v2).
const commonFields = `
	schema_version: "v2"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
	goarch:         "arm64"
`

// TestResearchBundleSchema_Negatives mirrors
// schema/research-bundle/testdata/invalid.cue's seven commented cases —
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
		{"negative total_calls inside call_traffic", `
bad3: #Bundle & {` + commonFields + `
	call_traffic: {
		total_calls:             -1
		staleness_report_calls: 0
	}
}`},
		{"required field goarch missing entirely", `
bad4: #Bundle & {
	schema_version: "v2"
	window_since:   "2026-07-21T00:00:00Z"
	window_until:   "2026-07-28T00:00:00Z"
	generated_at:   "2026-07-28T00:00:00Z"
	fathom_version: "fathom v0.1.0"
	commit_dirty:   false
	goos:           "darwin"
}`},
		{"tool name outside #Label's shape (critic Finding 1: client-controlled string)", `
bad5: #Bundle & {` + commonFields + `
	call_traffic: {
		total_calls: 1
		tool_calls: [{tool: "/Users/someone/repo/secret.go", calls: 1}]
		staleness_report_calls: 0
	}
}`},
		{"surface value outside #Label's shape (Markdown-table-breaking char)", `
bad6: #Bundle & {` + commonFields + `
	call_traffic: {
		total_calls: 1
		surface_traffic: [{surface: "agent | injected", calls: 1}]
		staleness_report_calls: 0
	}
}`},
		{"bench arm label outside #Label's shape", `
bad7: #Bundle & {` + commonFields + `
	bench_arms: [{arm: "fathom\nEVIL", task_count: 1, success_count: 1, success_rate: 1.0, mean_tokens: 0, mean_wall_ms: 0}]
}`},
		{"IR-5: a retro outcome label outside #Label's shape", `
bad8: #Bundle & {` + commonFields + `
	retros: {` + retroCommon + `
		outcomes: [{channel: "guard", outcome: "modified-uncommitted", firings: 1}]
	}
}`},
		{"IR-5: a file or session field smuggled into the retros group", `
bad9: #Bundle & {` + commonFields + `
	retros: {` + retroCommon + `
		file: "internal/secret.go"
	}
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
// (the Go-side happy path over the same schema the CUE negatives exercise),
// including the not-instrumented case (nil groups) and the fully-populated
// case.
func TestValidate_ValidBundle(t *testing.T) {
	p50, p95, rate := 12.5, 88.0, 0.9
	obs := 6
	open := 2
	b := Bundle{
		SchemaVersion:     SchemaVersionCurrent,
		WindowSince:       "2026-07-21T00:00:00Z",
		WindowUntil:       "2026-07-28T00:00:00Z",
		GeneratedAt:       "2026-07-28T00:00:00Z",
		FathomVersion:     "fathom v0.1.0",
		CommitSHA:         "abc1234def5",
		GOOS:              "darwin",
		GOARCH:            "arm64",
		TotalObservations: &obs,
		OpenObservations:  &open,
		CallTraffic: &CallTrafficStats{
			TotalCalls:   42,
			LatencyP50MS: &p50,
			LatencyP95MS: &p95,
		},
		Injection: &InjectionStats{
			Seen:         10,
			Answered:     9,
			CoverageRate: &rate,
		},
		VerifyGate: &VerifyGateStats{},
	}
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(valid bundle) = %v, want nil", err)
	}
}

// TestValidate_NotInstrumentedBundle proves the minimal, "nothing was
// readable" bundle (critic Finding 3: no CallTraffic/Injection/VerifyGate
// groups at all) still validates — absence of a group is a legitimate,
// schema-conforming state, not an error.
func TestValidate_NotInstrumentedBundle(t *testing.T) {
	b := Bundle{
		SchemaVersion: SchemaVersionCurrent,
		WindowSince:   "2026-07-21T00:00:00Z",
		WindowUntil:   "2026-07-28T00:00:00Z",
		GeneratedAt:   "2026-07-28T00:00:00Z",
		FathomVersion: "fathom v0.1.0",
		GOOS:          "darwin",
		GOARCH:        "arm64",
	}
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(not-instrumented bundle) = %v, want nil", err)
	}
}

// retroCommon is every required field of #Retros (IR-5).
const retroCommon = `
		retros: 2
		unreadable: 0
		off_arm_retros: 0
		guard_fires: 1
		obligation_injections: 1
		silent_on_clean: 3
		unjoinable: 0
		custodian_remedied: 0
		custodian_unremedied: 0`

// TestValidate_RetrosGroup proves the opt-in retro group (IR-5) validates
// through the same closed allowlist as every other group.
func TestValidate_RetrosGroup(t *testing.T) {
	b := Bundle{
		SchemaVersion: SchemaVersionCurrent,
		WindowSince:   "2026-07-21T00:00:00Z",
		WindowUntil:   "2026-07-28T00:00:00Z",
		GeneratedAt:   "2026-07-28T00:00:00Z",
		FathomVersion: "fathom v0.1.0",
		GOOS:          "darwin",
		GOARCH:        "arm64",
		Retros: &RetroStats{
			Retros: 2, GuardFires: 1, ObligationInjections: 1,
			Outcomes: []RetroOutcomeCount{{Channel: "guard", Outcome: "committed", Firings: 1}, {Channel: "obligations", Outcome: "modified_uncommitted", Firings: 1}},
		},
	}
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(bundle with retros) = %v, want nil", err)
	}
}
