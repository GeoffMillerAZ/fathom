package archconformance_test

// schema_test.go — C4-1 (docs/plans/review-verb.md): asserts
// schema/rules/archconformance/schema.cue's #RuleSet definition validates
// its own testdata/valid.cue positive fixture, rejects every negative
// documented (commented) in testdata/invalid.cue (repo convention:
// negatives are Go-asserted, never live-vetted — see
// internal/config/schema_test.go for the precedent this mirrors), and that
// a #RuleSet decoded from CUE actually drives Check the same way a
// hand-built []Rule does — proving the schema and the Go type it feeds stay
// in lockstep.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// schemaSource reads schema/rules/archconformance/schema.cue relative to
// this test file, mirroring internal/config/schema_test.go's schemaSource
// helper.
func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "rules", "archconformance", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

// testdataSource reads a testdata file and strips its leading "package
// archconformance" clause — schemaSource's schema.cue already declares that
// package, and CompileString accepts at most one package clause per
// compiled string, so schema+testdata are concatenated as one anonymous
// file sharing the schema's package.
func testdataSource(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "rules", "archconformance", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return strings.Replace(string(data), "package archconformance\n", "", 1)
}

// TestSchema_ValidatesOwnValidFixture proves schema.cue's #RuleSet accepts
// testdata/valid.cue as-is.
func TestSchema_ValidatesOwnValidFixture(t *testing.T) {
	cctx := cuecontext.New()
	schema := schemaSource(t)
	valid := testdataSource(t, "valid.cue")

	v := cctx.CompileString(schema + "\n" + valid)
	if err := v.Err(); err != nil {
		t.Fatalf("compile schema+valid.cue: %v", err)
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue does not validate against #RuleSet: %v", err)
	}
}

// TestSchema_Negatives mirrors testdata/invalid.cue's five commented cases
// — asserted here in Go so `cue vet ./...` stays green (repo convention).
func TestSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"rule missing required to_pkg", `
bad1: #RuleSet & {
	rules: [
		{from_pkg: "connector", allow: false},
	]
}`},
		{"allow is not a bool", `
bad2: #RuleSet & {
	rules: [
		{from_pkg: "a", to_pkg: "b", allow: "yes"},
	]
}`},
		{"deny_by_default is not a bool", `
bad3: #RuleSet & {
	rules: []
	deny_by_default: "true"
}`},
		{"unknown field on a closed #Rule", `
bad4: #RuleSet & {
	rules: [
		{from_pkg: "a", to_pkg: "b", allow: true, extra: "nope"},
	]
}`},
		{"rules is not a list", `
bad5: #RuleSet & {
	rules: "not-a-list"
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
			// Concrete(true) matches internal/projectrules.Load's own
			// validation call — a required field left entirely absent
			// (case 1) is structurally valid but INCOMPLETE, which only a
			// concreteness check catches.
			if err := v.Validate(cue.Concrete(true)); err == nil {
				t.Errorf("expected validation failure, got none")
			}
		})
	}
}

// cueRule/cueRuleSet mirror internal/projectrules' own decode target
// (json struct tags, the internal/semantic/manifest.go convention) — kept
// local to this test so it stays independent of internal/projectrules'
// package (C4-1 lands before C4-2; this test's job is to prove the SCHEMA,
// not the loader).
type cueRule struct {
	FromPkg string `json:"from_pkg"`
	ToPkg   string `json:"to_pkg"`
	Allow   bool   `json:"allow"`
}

type cueRuleSet struct {
	Rules         []cueRule `json:"rules"`
	DenyByDefault bool      `json:"deny_by_default"`
}

// TestSchema_DecodedRulesDriveCheck proves a #RuleSet decoded straight out
// of CUE (via valid.cue's own fixture) drives archconformance.Check
// identically to a hand-built []Rule — the schema and the Check contract
// stay in lockstep. Grounds C4-1's evidence against Check itself, not just
// the schema in isolation.
func TestSchema_DecodedRulesDriveCheck(t *testing.T) {
	cctx := cuecontext.New()
	schema := schemaSource(t)
	valid := testdataSource(t, "valid.cue")

	v := cctx.CompileString(schema + "\n" + valid)
	if err := v.Err(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	example := v.LookupPath(cue.ParsePath("example"))
	if err := example.Err(); err != nil {
		t.Fatalf("lookup example: %v", err)
	}

	var rs cueRuleSet
	if err := example.Decode(&rs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rs.Rules) != 3 {
		t.Fatalf("expected 3 decoded rules, got %d: %+v", len(rs.Rules), rs.Rules)
	}

	rules := make([]archconformance.Rule, len(rs.Rules))
	for i, r := range rs.Rules {
		rules[i] = archconformance.Rule{FromPkg: r.FromPkg, ToPkg: r.ToPkg, Allow: r.Allow}
	}

	edges := []core.Edge{
		// connector -> query: denied by the decoded rule pack.
		{FromSymbol: "connector.Fixture", ToSymbol: "query.Fixture", FromService: "svc", FromFile: "connector/fixture.go"},
		// extractor -> mcp: denied by the decoded rule pack.
		{FromSymbol: "extractor.Fixture", ToSymbol: "mcp.Fixture", FromService: "svc", FromFile: "extractor/fixture.go"},
		// anything -> store: explicitly allowed by the wildcard allow rule.
		{FromSymbol: "query.Fixture", ToSymbol: "store.Fixture", FromService: "svc", FromFile: "query/fixture.go"},
	}

	violations := archconformance.Check(edges, rules)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations (connector->query, extractor->mcp), got %d: %+v", len(violations), violations)
	}
	if violations[0].From != "connector.Fixture" || violations[0].To != "query.Fixture" {
		t.Errorf("unexpected violation[0]: %+v", violations[0])
	}
	if violations[1].From != "extractor.Fixture" || violations[1].To != "mcp.Fixture" {
		t.Errorf("unexpected violation[1]: %+v", violations[1])
	}
}
