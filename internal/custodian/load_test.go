package custodian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Three-state load (the capability loader's O22 discipline): absent and
// all-commented-scaffold are both ZERO concerns with NO error — only a
// present-and-malformed file is loud.
func TestLoadAbsentAndScaffoldAreZeroConcerns(t *testing.T) {
	dir := t.TempDir()

	cs, err := LoadFile(filepath.Join(dir, "concerns.cue"), ScopeUser)
	if err != nil || len(cs) != 0 {
		t.Fatalf("absent file: got %d concerns, err %v; want 0, nil", len(cs), err)
	}

	scaffold := filepath.Join(dir, "scaffold.cue")
	writeTestFile(t, scaffold, "// concerns: {\n//   router_thin: {want: \"thin\"}\n// }\n")
	cs, err = LoadFile(scaffold, ScopeUser)
	if err != nil || len(cs) != 0 {
		t.Fatalf("scaffold: got %d concerns, err %v; want 0, nil", len(cs), err)
	}
}

// An uncheckable concern is a wish, and the loader is where wishes are
// turned away — naming the concern, the bad kind, and the accepted kinds.
func TestLoadRefusesUncheckableKind(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	writeTestFile(t, p, `concerns: {
	vibes: {
		want:  "the docs feel fresh"
		check: {kind: "feels_right"}
	}
}
`)
	_, err := LoadFile(p, ScopeUser)
	if err == nil {
		t.Fatal("uncheckable kind loaded without error")
	}
	for _, want := range []string{"vibes", "feels_right", "forbidden_pattern"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err.Error(), want)
		}
	}
}

// `want?:` in a config instance declares the field MAY exist rather than
// setting it — the phase-O trust-boundary inversion, refused here exactly
// as the capability loader refuses it.
func TestLoadRefusesOptionalFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	writeTestFile(t, p, `concerns: {
	router_thin: {
		want?: "the root CLAUDE.md stays a router"
		check: {kind: "pointers_resolve"}
	}
}
`)
	_, err := LoadFile(p, ScopeUser)
	if err == nil || !strings.Contains(err.Error(), "OPTIONAL") {
		t.Fatalf("optional field not refused: %v", err)
	}
}

func TestLoadRefusesMissingKindFields(t *testing.T) {
	cases := map[string]string{
		"pattern": `concerns: {a: {want: "w", check: {kind: "forbidden_pattern"}}}`,
		"command": `concerns: {a: {want: "w", check: {kind: "command_documented_and_exists"}}}`,
		"lines":   `concerns: {a: {want: "w", check: {kind: "max_lines", file: "CLAUDE.md"}}}`,
	}
	for missing, src := range cases {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, src+"\n")
		_, err := LoadFile(p, ScopeUser)
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("missing %q not refused: %v", missing, err)
		}
	}
}

func TestLoadRefusesBadRegexp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	writeTestFile(t, p, `concerns: {a: {want: "w", check: {kind: "forbidden_pattern", pattern: "("}}}`+"\n")
	_, err := LoadFile(p, ScopeUser)
	if err == nil || !strings.Contains(err.Error(), "pattern") {
		t.Fatalf("bad regexp not refused: %v", err)
	}
}

func TestLoadParsesFullConcernSortedWithDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	writeTestFile(t, p, `concerns: {
	router_thin: {
		want:  "the root CLAUDE.md stays a router"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
	}
	deploy_safety: {
		want:  "no raw-cp deploy instruction"
		why:   "O25"
		check: {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom"}
	}
}
`)
	cs, err := LoadFile(p, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].ID != "deploy_safety" || cs[1].ID != "router_thin" {
		t.Fatalf("want 2 concerns sorted by ID, got %+v", cs)
	}
	if cs[0].Check.Where != "agent-config" {
		t.Fatalf("where should default to agent-config, got %q", cs[0].Check.Where)
	}
	if cs[0].Why != "O25" || cs[0].Scope != ScopeProject {
		t.Fatalf("why/scope not carried: %+v", cs[0])
	}
	if cs[1].Check.Lines != 120 || cs[1].Check.File != "CLAUDE.md" {
		t.Fatalf("max_lines fields not carried: %+v", cs[1])
	}
}

// Law 13: the project scope GOVERNS — a same-ID project entry replaces the
// user entry wholesale, and `off: true` disables an inherited concern
// while keeping it visible (a disabled contract is a fact, not a hole).
func TestProjectScopeGoverns(t *testing.T) {
	user := []Concern{{ID: "router_thin", Want: "user says 120", Scope: ScopeUser,
		Check: Check{Kind: KindMaxLines, File: "CLAUDE.md", Lines: 120}}}
	proj := []Concern{{ID: "router_thin", Want: "repo says 200", Scope: ScopeProject,
		Check: Check{Kind: KindMaxLines, File: "CLAUDE.md", Lines: 200}}}

	got := Merge(user, proj)
	if len(got) != 1 || got[0].Want != "repo says 200" || got[0].Check.Lines != 200 {
		t.Fatalf("project entry must replace user entry wholesale, got %+v", got)
	}

	off := []Concern{{ID: "router_thin", Off: true, Scope: ScopeProject}}
	got = Merge(user, off)
	if len(got) != 1 || !got[0].Off {
		t.Fatalf("off entry must disable but remain visible, got %+v", got)
	}

	// Distinct IDs union, sorted.
	other := []Concern{{ID: "deploy_safety", Want: "w", Scope: ScopeProject,
		Check: Check{Kind: KindPointersResolve}}}
	got = Merge(user, other)
	if len(got) != 2 || got[0].ID != "deploy_safety" || got[1].ID != "router_thin" {
		t.Fatalf("union not sorted by ID: %+v", got)
	}
}
