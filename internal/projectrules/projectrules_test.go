package projectrules_test

// C4-2 (docs/plans/review-verb.md): unit tests for the .fathom/rules/
// discovery + CUE-validation loader. Covers the three states arch_conformance
// (C4-3) needs to distinguish: a valid project rule pack, no file at all
// (attributed, never silently empty), and a malformed file (loud refusal,
// never silently empty).

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

func writeRuleFile(t *testing.T, corpusRoot, content string) string {
	t.Helper()
	dir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "archconformance.cue")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	return path
}

// TestLoad_MissingFile_NoError proves an absent rule file is not an error:
// Result.Found is false and Source is explicitly "none" — never confused
// with a validated-but-empty ruleset.
func TestLoad_MissingFile_NoError(t *testing.T) {
	corpusRoot := t.TempDir()

	result, err := projectrules.Load(corpusRoot)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if result.Found {
		t.Errorf("expected Found=false for a corpus root with no rule file, got true")
	}
	if result.Source != projectrules.SourceNone {
		t.Errorf("expected Source=%q, got %q", projectrules.SourceNone, result.Source)
	}
	if len(result.Rules) != 0 {
		t.Errorf("expected no rules, got %v", result.Rules)
	}
	wantPath := filepath.Join(corpusRoot, projectrules.RelPath)
	if result.Path != wantPath {
		t.Errorf("Path = %q, want %q", result.Path, wantPath)
	}
}

// TestLoad_ValidFile proves a well-formed rule pack decodes into typed
// rules, attributed as "project".
func TestLoad_ValidFile(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `
rules: [
	{from_pkg: "connector", to_pkg: "query", allow: false},
	{from_pkg: "extractor", to_pkg: "mcp", allow: false},
]
deny_by_default: true
`)

	result, err := projectrules.Load(corpusRoot)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if !result.Found {
		t.Fatalf("expected Found=true")
	}
	if result.Source != projectrules.SourceProject {
		t.Errorf("expected Source=%q, got %q", projectrules.SourceProject, result.Source)
	}
	if !result.DenyByDefault {
		t.Errorf("expected DenyByDefault=true")
	}
	if len(result.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d: %+v", len(result.Rules), result.Rules)
	}
	if result.Rules[0].FromPkg != "connector" || result.Rules[0].ToPkg != "query" || result.Rules[0].Allow {
		t.Errorf("unexpected rule[0]: %+v", result.Rules[0])
	}
	if result.Rules[1].FromPkg != "extractor" || result.Rules[1].ToPkg != "mcp" || result.Rules[1].Allow {
		t.Errorf("unexpected rule[1]: %+v", result.Rules[1])
	}
}

// TestLoad_ValidFile_DenyByDefaultDefaultsFalse proves the CUE schema's
// deny_by_default: bool | *false default carries through decode when a rule
// pack omits the field.
func TestLoad_ValidFile_DenyByDefaultDefaultsFalse(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `
rules: [
	{from_pkg: "a", to_pkg: "b", allow: false},
]
`)

	result, err := projectrules.Load(corpusRoot)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if result.DenyByDefault {
		t.Errorf("expected DenyByDefault to default to false")
	}
}

// TestLoad_EmptyRuleList_StillAttributedProject proves a file that exists
// and validates but declares zero rules is still Source=project — the
// caller (arch_conformance, C4-3) is the one that decides whether "project
// rules exist but are empty" should read as checked:false; the loader's job
// is honest attribution of what it found, not policy.
func TestLoad_EmptyRuleList_StillAttributedProject(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `
rules: []
`)

	result, err := projectrules.Load(corpusRoot)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if !result.Found {
		t.Errorf("expected Found=true for an existing (if empty) rule file")
	}
	if result.Source != projectrules.SourceProject {
		t.Errorf("expected Source=%q, got %q", projectrules.SourceProject, result.Source)
	}
	if len(result.Rules) != 0 {
		t.Errorf("expected 0 rules, got %v", result.Rules)
	}
}

// TestLoad_RelativeCorpusRoot_Refused proves Load refuses a non-absolute
// corpus root with ErrRelativeCorpusRoot instead of resolving it against
// the ambient process CWD. A relative ServiceMeta path (the pre-O10 meta
// class, docs/dogfood-observations.md) resolved from a daemon's CWD would
// load a FOREIGN repo's .fathom/rules/ pack and answer rule_source:
// "project", checked:true for rules the target service never declared —
// demonstrated live by the review critic. Never resolve, always refuse.
func TestLoad_RelativeCorpusRoot_Refused(t *testing.T) {
	for _, root := range []string{".", "sextant", "some/relative/dir"} {
		result, err := projectrules.Load(root)
		if err == nil {
			t.Fatalf("Load(%q): expected ErrRelativeCorpusRoot, got result %+v", root, result)
		}
		if !errors.Is(err, projectrules.ErrRelativeCorpusRoot) {
			t.Errorf("Load(%q): error should wrap ErrRelativeCorpusRoot, got: %v", root, err)
		}
		if !strings.Contains(err.Error(), root) {
			t.Errorf("Load(%q): error should name the offending root, got: %v", root, err)
		}
	}
}

// TestLoad_MalformedFile_LoudError proves a syntactically broken file is a
// hard error, never a silent empty ruleset.
func TestLoad_MalformedFile_LoudError(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `this is { not valid cue :::`)

	result, err := projectrules.Load(corpusRoot)
	if err == nil {
		t.Fatalf("expected an error for a malformed rule file, got result: %+v", result)
	}
	if !strings.Contains(err.Error(), "archconformance.cue") {
		t.Errorf("error should name the offending file, got: %v", err)
	}
}

// TestLoad_SchemaViolation_LoudError proves a syntactically valid CUE file
// that does not satisfy #RuleSet (e.g. a rule missing to_pkg) is rejected,
// not silently coerced or dropped.
func TestLoad_SchemaViolation_LoudError(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `
rules: [
	{from_pkg: "connector", allow: false},
]
`)

	_, err := projectrules.Load(corpusRoot)
	if err == nil {
		t.Fatalf("expected a schema-validation error for a rule missing to_pkg")
	}
}

// TestLoad_UnknownField_LoudError proves the closed #Rule struct rejects an
// unrecognized field (e.g. a typo) instead of silently ignoring it.
func TestLoad_UnknownField_LoudError(t *testing.T) {
	corpusRoot := t.TempDir()
	writeRuleFile(t, corpusRoot, `
rules: [
	{from_pkg: "a", to_pkg: "b", allow: true, form_pkg: "typo"},
]
`)

	_, err := projectrules.Load(corpusRoot)
	if err == nil {
		t.Fatalf("expected an error for an unknown field on a closed #Rule")
	}
}

// repoRoot locates this repo's root from this test file's own path, so the
// C4-4 fixture test below loads fathom's REAL .fathom/rules/archconformance.cue
// — proving the loader + Check pipeline actually enforces this repo's own
// dogfooded rule pack — without hardcoding an absolute path.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestLoad_RepoOwnRuleFile_CatchesFixtureViolation is C4-4's required
// fixture test: it proves fathom's own .fathom/rules/archconformance.cue
// (the three-layer boundary from CLAUDE.md/ADR.md — connector must not
// import query, extractor must not import mcp, analysis packages import
// neither mcp nor store) actually catches a violation when Check is run
// against it — using SYNTHETIC fixture edges built from package names, not
// any live edge extracted from fathom's real source (this repo has zero
// such violations today, confirmed separately; a live violation would mean
// the architecture itself is already broken, which is not what this test is
// about).
func TestLoad_RepoOwnRuleFile_CatchesFixtureViolation(t *testing.T) {
	result, err := projectrules.Load(repoRoot(t))
	if err != nil {
		t.Fatalf("Load(repoRoot): unexpected error: %v", err)
	}
	if !result.Found {
		t.Fatalf("expected fathom's own .fathom/rules/archconformance.cue to be found")
	}
	if len(result.Rules) == 0 {
		t.Fatalf("expected fathom's own rule pack to declare at least one rule")
	}

	fixtureEdges := []core.Edge{
		// Fixture: a hypothetical connector.FetchAll calling query.Engine's
		// Run — exactly the "connector must not import query" edge named in
		// CLAUDE.md's architecture diagram, encoded as a synthetic edge
		// (FixtureFunc is not a real fathom symbol).
		{FromSymbol: "connector.FixtureFetchAll", ToSymbol: "query.FixtureRun", FromService: "fixture", FromFile: "fixture/connector.go"},
		// Fixture: a hypothetical analysis package reaching into mcp — the
		// "analysis packages import neither mcp nor store" edge, using a
		// real analysis package name (archconformance itself) against a
		// synthetic callee.
		{FromSymbol: "archconformance.FixtureCheck", ToSymbol: "mcp.FixtureRegisterTool", FromService: "fixture", FromFile: "fixture/archconformance.go"},
		// Non-violation control: an edge between two packages this rule
		// pack says nothing about must not be flagged.
		{FromSymbol: "cmd.FixtureMain", ToSymbol: "core.FixtureModel", FromService: "fixture", FromFile: "fixture/main.go"},
	}

	var opts []archconformance.Option
	if result.DenyByDefault {
		opts = append(opts, archconformance.DenyByDefault())
	}
	violations := archconformance.Check(fixtureEdges, result.Rules, opts...)

	if len(violations) != 2 {
		t.Fatalf("expected exactly 2 violations (connector->query, archconformance->mcp), got %d: %+v", len(violations), violations)
	}
	got := map[string]bool{}
	for _, v := range violations {
		got[v.From+"->"+v.To] = true
	}
	if !got["connector.FixtureFetchAll->query.FixtureRun"] {
		t.Errorf("expected connector->query fixture edge to be flagged, violations: %+v", violations)
	}
	if !got["archconformance.FixtureCheck->mcp.FixtureRegisterTool"] {
		t.Errorf("expected analysis-package->mcp fixture edge to be flagged, violations: %+v", violations)
	}
}

// TestArchconformanceSchemaMatchesSource is the drift gate: the embedded
// archconformance_schema.cue must be byte-identical to
// schema/rules/archconformance/schema.cue, the authoritative source
// (internal/config/schema_test.go's TestConfigSchemaMatchesSource
// precedent).
func TestArchconformanceSchemaMatchesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "schema", "rules", "archconformance", "schema.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	embedded, err := os.ReadFile(filepath.Join(filepath.Dir(file), "archconformance_schema.cue"))
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	if string(want) != string(embedded) {
		t.Fatalf(
			"internal/projectrules/archconformance_schema.cue has drifted from %s — "+
				"copy the source file verbatim (schema/rules/archconformance/schema.cue is authoritative)",
			src,
		)
	}
}
