package custodian

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestForbiddenPatternViolatesWithEvidence(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"),
		"# deploy\ncp bin/fathom ~/.local/libexec/fathom/fathom\n")
	c := Concern{ID: "deploy_safety", Want: "no raw-cp deploy",
		Check: Check{Kind: KindForbiddenPattern, Pattern: `cp .*libexec/fathom`, Where: "agent-config"}}

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
	c := Concern{ID: "router_thin", Want: "router stays thin",
		Check: Check{Kind: KindMaxLines, File: "CLAUDE.md", Lines: 2}}

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
	c := Concern{ID: "gate_documented", Want: "the gate is named and real",
		Check: Check{Kind: KindCommandDocumentedAndExists, Command: "task validate"}}

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
	c := Concern{ID: "pointers_hold", Want: "every relative pointer resolves",
		Check: Check{Kind: KindPointersResolve}}

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

	files, err := AgentConfigFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"CLAUDE.md", "sub/AGENTS.md"}
	if len(files) != len(want) || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("want %v, got %v", want, files)
	}
}
