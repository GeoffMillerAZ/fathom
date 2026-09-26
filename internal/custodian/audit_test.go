package custodian

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestForbiddenPatternViolatesWithEvidence(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
		"# deploy\ncp bin/fathom ~/.local/libexec/fathom/fathom\n")
	c := Concern{
		ID: "deploy_safety", Want: "no raw-cp deploy",
		Check: Check{Kind: KindForbiddenPattern, Pattern: `cp .*libexec/fathom`, Where: "agent-config"},
	}

	fs := Audit(root, []Concern{c})
	if len(fs) != 1 || fs[0].Status != StatusViolated {
		t.Fatalf("want violated, got %+v", fs)
	}
	if len(fs[0].Evidence) != 1 || fs[0].Evidence[0] != "CLAUDE.md:2" {
		t.Fatalf("evidence must be file:line, got %v", fs[0].Evidence)
	}

	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "# deploy\nuse `task deploy`\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds {
		t.Fatalf("clean file must hold, got %+v", fs[0])
	}
	if !strings.Contains(fs[0].Detail, "1 agent-config file") {
		t.Fatalf("holds detail must name what was checked, got %q", fs[0].Detail)
	}
}

func TestMaxLines(t *testing.T) {
	root := t.TempDir()
	c := Concern{
		ID: "router_thin", Want: "router stays thin",
		Check: Check{Kind: KindMaxLines, File: "CLAUDE.md", Lines: 2},
	}

	// Absent: the concern expects the file — a named violation, not a skip.
	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, "absent") {
		t.Fatalf("absent file: %+v", fs[0])
	}

	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "1\n2\n3\n4\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, "4 lines (max 2)") {
		t.Fatalf("over budget: %+v", fs[0])
	}

	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "1\n2\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds {
		t.Fatalf("under budget must hold: %+v", fs[0])
	}
}

func TestCommandDocumentedAndExists(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"),
		"version: '3'\n\ntasks:\n  validate:\n    cmds:\n      - echo ok\n")
	c := Concern{
		ID: "gate_documented", Want: "the gate is named and real",
		Check: Check{Kind: KindCommandDocumentedAndExists, Command: "task validate"},
	}

	// Not documented anywhere.
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "# nothing about gates\n")
	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, "not documented") {
		t.Fatalf("undocumented: %+v", fs[0])
	}

	// Documented and the target exists.
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "Run `task validate` before pushing.\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds {
		t.Fatalf("documented+exists must hold: %+v", fs[0])
	}

	// Documented but the target is gone — the O25 shape: instruction
	// outlives reality.
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"),
		"version: '3'\n\ntasks:\n  build:\n    cmds:\n      - echo ok\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, "target") {
		t.Fatalf("documented-but-missing must violate naming the target: %+v", fs[0])
	}
}

func TestPointersResolve(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "real.md"), "here\n")
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
		"See [real](docs/real.md), [ext](https://example.com), and [dead](docs/gone.md).\n")
	c := Concern{
		ID: "pointers_hold", Want: "every relative pointer resolves",
		Check: Check{Kind: KindPointersResolve},
	}

	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated {
		t.Fatalf("dead pointer must violate: %+v", fs[0])
	}
	if len(fs[0].Evidence) != 1 || fs[0].Evidence[0] != "CLAUDE.md:1 → docs/gone.md" {
		t.Fatalf("evidence must pin file:line → target, got %v", fs[0].Evidence)
	}

	writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
		"See [real](docs/real.md) and [ext](https://example.com).\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds {
		t.Fatalf("resolving pointers must hold: %+v", fs[0])
	}
}

// This house style points with backtick paths, not markdown links — a
// pointer check blind to them holds vacuously (found live on this repo's
// own router the day the audit shipped). Backtick tokens count as
// pointers only under tight guards: ≥2 path segments, path charset, no
// globs, runtime dirs (workspace/tmp/ref/bin) skipped — a false violation
// teaches people to ignore the custodian.
func TestPointersResolveBacktickPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "usage.md"), "here\n")
	c := Concern{
		ID: "pointers_hold", Want: "pointers resolve",
		Check: Check{Kind: KindPointersResolve},
	}

	writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
		"Live in `docs/usage.md`; dead in `docs/gone.md`.\n"+
			"Skipped: `db/` (one segment), `docs/plans/*.md` (glob), `workspace/db/fleet.sqlite` (runtime), `find_symbol` (no slash).\n"+
			"Foreign names, not pointers: `appkit/ui/dashboard`, `modernc.org/sqlite`, `github.com/x/y` (no such first dir here), `../other-repo` (escape).\n")
	fs := Audit(root, []Concern{c})
	if fs[0].Status != StatusViolated {
		t.Fatalf("dead backtick path must violate: %+v", fs[0])
	}
	if len(fs[0].Evidence) != 1 || fs[0].Evidence[0] != "CLAUDE.md:1 → docs/gone.md" {
		t.Fatalf("exactly the dead path, nothing from the guarded skips: %v", fs[0].Evidence)
	}

	// Root-relative resolution: a nested file citing `docs/usage.md` (the
	// house style cites from the repo root, wherever the citing file
	// lives) must resolve.
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "clean\n")
	writeTestFile(t, filepath.Join(root, "internal", "CLAUDE.md"),
		"Rules live in `docs/usage.md`.\n")
	fs = Audit(root, []Concern{c})
	if fs[0].Status != StatusHolds {
		t.Fatalf("root-relative backtick path from a nested file must hold: %+v", fs[0])
	}
}

// A disabled inherited concern is NAMED by the audit, never silently
// absent (law 3 applied to governance).
func TestAuditNamesOffConcerns(t *testing.T) {
	root := t.TempDir()
	c := Concern{ID: "deploy_safety", Off: true, Scope: ScopeProject}
	fs := Audit(root, []Concern{c})
	if len(fs) != 1 || fs[0].Status != StatusOff {
		t.Fatalf("off concern must appear as off, got %+v", fs)
	}
	if !strings.Contains(fs[0].Detail, "off") {
		t.Fatalf("off detail must say so, got %q", fs[0].Detail)
	}
}

// Outside a git work tree the walk is the discovery path, and it must not
// sweep the junk dirs (workspace/, ref/, …) that git's ignore rules would
// have excluded.
func TestAgentConfigDiscoveryWalkSkips(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "root\n")
	writeTestFile(t, filepath.Join(root, "sub", "AGENTS.md"), "sub\n")
	writeTestFile(t, filepath.Join(root, "workspace", "CLAUDE.md"), "junk\n")
	writeTestFile(t, filepath.Join(root, "ref", "clone", "CLAUDE.md"), "junk\n")
	// Vendored trees carry OTHER projects' instruction surfaces.
	writeTestFile(t, filepath.Join(root, "third_party", "kit", "CLAUDE.md"), "junk\n")

	files, err := AgentConfigFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"CLAUDE.md", "sub/AGENTS.md"}
	if len(files) != len(want) || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("want %v, got %v", want, files)
	}
}

// TestCommandDocumentedTaskHalfReadsTheTaskGraph: the "and that target
// exists" half of command_documented_and_exists resolves `task X` through
// the parsed Taskfile graph commands_resolve uses — never the indented-key
// grep, which read a `vars:` entry named `validate` as the target and
// missed a target defined in an include — so one repo cannot get opposite
// verdicts on the same target from one pack. The documented half never
// counts a YAML comment: a commented-out step runs nothing.
func TestCommandDocumentedTaskHalfReadsTheTaskGraph(t *testing.T) {
	gate := Concern{ID: "lefthook_runs_gate", Want: "the hook runs the gate", Check: Check{Kind: KindCommandDocumentedAndExists, Where: CorpusLefthook, Command: "task validate"}}
	chain := Concern{ID: "lefthook_commands_resolve", Want: "hook commands resolve", Check: Check{Kind: KindCommandsResolve, Where: CorpusLefthook}}
	hook := "pre-push:\n  commands:\n    gate:\n      run: task validate && bin/fathom verify --service s\n"

	// A var named like the target is not the target.
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), hook)
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), "version: '3'\nvars:\n  validate: yes\ntasks:\n  test:\n    cmds: [go test ./...]\n")
	fs := Audit(root, []Concern{gate, chain})
	if fs[0].Status != StatusViolated || !strings.Contains(fs[0].Detail, `Taskfile.yml has no "validate" target`) || fs[1].Status != StatusViolated {
		t.Fatalf("a vars entry is not a target, and both entries must say so: %+v / %+v", fs[0], fs[1])
	}

	// A target in a flattened include exists.
	root = t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), hook)
	writeTestFile(t, filepath.Join(root, "Taskfile.yml"), "version: '3'\nincludes:\n  gates:\n    taskfile: ./gates\n    flatten: true\n")
	writeTestFile(t, filepath.Join(root, "gates", "Taskfile.yml"), "version: '3'\ntasks:\n  validate:\n    cmds: [echo ok]\n")
	fs = Audit(root, []Concern{gate, chain})
	if fs[0].Status != StatusHolds || fs[1].Status != StatusHolds {
		t.Fatalf("a flattened include's target exists for both entries: %+v / %+v", fs[0], fs[1])
	}

	// Another Taskfile spelling is the Taskfile (one that differs by more
	// than case: macOS volumes fold it).
	root = t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), hook)
	writeTestFile(t, filepath.Join(root, "Taskfile.dist.yml"), "version: '3'\ntasks:\n  validate:\n    cmds: [echo ok]\n")
	if f := Audit(root, []Concern{gate})[0]; f.Status != StatusHolds || !strings.Contains(f.Detail, "exists in Taskfile.dist.yml") {
		t.Fatalf("Taskfile.dist.yml is a Taskfile: %+v", f)
	}

	// A commented-out step documents nothing.
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n  commands:\n    lint:\n      run: golangci-lint run\n#   gate:\n#     run: task validate\n")
	if f := Audit(root, []Concern{gate})[0]; f.Status != StatusViolated || !strings.Contains(f.Detail, "is not documented") {
		t.Fatalf("a commented-out gate is not run: %+v", f)
	}
}

// TestRequiredPatternUnderReadsOnlyThatKeysLines: `under` holds a YAML
// corpus's required_pattern to the lines under one top-level key — the
// lefthook pack's pre-push verify — and a comment never satisfies a
// required_pattern over a YAML corpus: a `# … fathom verify` line, or a
// verify that runs in pre-commit only, does not gate a push (I17).
func TestRequiredPatternUnderReadsOnlyThatKeysLines(t *testing.T) {
	c := Concern{ID: "pre_push_verify", Want: "pre-push runs verify", Check: Check{Kind: KindRequiredPattern, Where: CorpusLefthook, Under: "pre-push", Pattern: `(?i)fathom\}?"?'?\s+verify\b`}}
	for _, tc := range []struct {
		name, body string
		status     Status
		detail     string
		evidence   []string
	}{
		{"commented", "pre-push:\n  commands:\n    gate:\n      run: task validate\n#   verify:\n#     run: bin/fathom verify --service x\n", StatusViolated, "no line under `pre-push` matches", nil},
		{"trailing comment", "pre-push:\n  commands:\n    gate:\n      run: task validate # then bin/fathom verify\n", StatusViolated, "no line under `pre-push` matches", nil},
		{"pre-commit only", "pre-commit:\n  commands:\n    v:\n      run: bin/fathom verify --service x\npre-push:\n  commands:\n    gate:\n      run: task validate\n", StatusViolated, "no line under `pre-push` matches", nil},
		{"no pre-push", "pre-commit:\n  commands:\n    v:\n      run: bin/fathom verify --service x\n", StatusViolated, "no `pre-push` key in lefthook.yml", nil},
		{"pre-push", "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\npre-push:\n  commands:\n    v:\n      run: |\n        # the gate\n        \"$FATHOM\" verify --plan p.md\n", StatusHolds, "1 line(s) under `pre-push` match", []string{"lefthook.yml:10"}},
	} {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "lefthook.yml"), tc.body)
		f := Audit(root, []Concern{c})[0]
		if f.Status != tc.status || !strings.Contains(f.Detail, tc.detail) || !reflect.DeepEqual(f.Evidence, tc.evidence) {
			t.Errorf("%s: got %s %q %q, want %s containing %q, evidence %q", tc.name, f.Status, f.Detail, f.Evidence, tc.status, tc.detail, tc.evidence)
		}
	}

	// Without `under`, a comment still never satisfies the pattern.
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "lefthook.yml"), "# bin/fathom verify used to run here\npre-push:\n  commands:\n    gate:\n      run: task validate\n")
	c.Check.Under = ""
	if f := Audit(root, []Concern{c})[0]; f.Status != StatusViolated {
		t.Errorf("a comment is not a command: %+v", f)
	}

	// The loader: `under` is required_pattern's, over a YAML corpus only.
	for _, tc := range []struct{ check, want string }{
		{`{kind: "required_pattern", pattern: "x", under: "pre-push"}`, `check.under reads a top-level key of a YAML corpus (lefthook, taskfile, ci), not "agent-config"`},
		{`{kind: "commands_resolve", where: "lefthook", under: "pre-push"}`, `check.under applies to required_pattern only, not "commands_resolve"`},
	} {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, "concerns: {c: {want: \"w\", check: "+tc.check+"}}\n")
		if _, err := LoadFile(p, ScopeProject); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want refusal containing %q, got %v", tc.check, tc.want, err)
		}
	}
}
