package main

// support_test.go — TestCmdSupportKindsAndJSON (SM-10, design §4): the
// join from a kind's registry/targets/readings into internal/support's
// Matrix, and the `fathom support` CLI surface over it.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/support"
)

func fixedSupportNow() time.Time {
	return time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
}

// TestCmdSupportKindsAndJSON proves buildSupportMatrix's join: a language
// with a reading below its declared target shows `tolerated` for the
// unmet levels and `proven-*` for the met ones (never a level above the
// reading, I54); a client with no reading yet is `tolerated`; a shared-kind
// target with no registry entry is `declared`; and `--kind` / `--json`
// both project the same underlying matrix (design §4's "one projection").
func TestCmdSupportKindsAndJSON(t *testing.T) {
	src := supportSources{
		Queue: []langbench.QueueEntry{
			{ID: "python", Name: "Python", Kind: "language", TargetLevel: "L4", Wave: 1, Engine: "tags", PlanRow: "LB-Q-python"},
			{ID: "dockerfile", Name: "Dockerfile", Kind: "tooling", TargetLevel: "L3", Wave: 1, Engine: "tags", PlanRow: "LB-Q-dockerfile"},
		},
		Registered: map[string]langbench.Registration{
			"python":     {Extractor: "dialects/python", LangTag: "python"},
			"dockerfile": {Extractor: "dialects/dockerfile", LangTag: "dockerfile"},
		},
		Readings: map[string]langbench.Reading{
			"python": {
				Dialect: "python", Kind: "language", TargetLevel: "L4", LevelMet: "L2",
				Grade: langbench.Grade{L1: langbench.GradeExcellent, L2: langbench.GradeGood},
			},
		},
		Targets: []support.Target{
			{Kind: support.KindClient, ID: "claude-code", Name: "Claude Code", PlanRow: "SM-13", Wave: 1,
				Targets: map[string]support.CapabilityTarget{"mcp-tools": {ProvenOnly: true}}},
			{Kind: support.KindRules, ID: "agents-md", Name: "AGENTS.md", PlanRow: "SM-15", Wave: 1,
				Targets: map[string]support.CapabilityTarget{"read": {ProvenOnly: true}}},
		},
		Clients: []support.RegistryEntry{
			{Kind: support.KindClient, ID: "claude-code", Name: "Claude Code", Capabilities: []string{"mcp-tools", "stanza"}},
		},
		Commit: "abc1234",
	}

	matrix := buildSupportMatrix(src, fixedSupportNow)

	if matrix.Schema != "fathom.support/v1" {
		t.Fatalf("schema = %q, want fathom.support/v1", matrix.Schema)
	}
	if err := support.ValidateMatrixJSON(mustMarshal(t, matrix)); err != nil {
		t.Fatalf("matrix fails #Matrix validation: %v", err)
	}

	python := findSupportEntry(t, matrix, support.KindLanguage, "python")
	l1 := findSupportCell(t, python, "L1")
	if l1.State != support.StateProvenExcellent {
		t.Fatalf("python L1 state = %s, want proven-excellent", l1.State)
	}
	l2 := findSupportCell(t, python, "L2")
	if l2.State != support.StateProvenGood {
		t.Fatalf("python L2 state = %s, want proven-good", l2.State)
	}
	l3 := findSupportCell(t, python, "L3")
	if l3.State != support.StateTolerated {
		t.Fatalf("python L3 state = %s, want tolerated (registered, not yet read at L3 — never a level above the reading)", l3.State)
	}
	if l3.LevelMet != "" {
		t.Fatalf("python L3 carries level_met %q on a tolerated cell — I54 violation", l3.LevelMet)
	}

	claude := findSupportEntry(t, matrix, support.KindClient, "claude-code")
	mcp := findSupportCell(t, claude, "mcp-tools")
	if mcp.State != support.StateTolerated {
		t.Fatalf("claude-code mcp-tools state = %s, want tolerated (registered, no reading yet)", mcp.State)
	}

	agents := findSupportEntry(t, matrix, support.KindRules, "agents-md")
	if agents.Registered {
		t.Fatal("agents-md has no rules registry adapter yet — Registered must be false")
	}
	readCell := findSupportCell(t, agents, "read")
	if readCell.State != support.StateDeclared {
		t.Fatalf("agents-md read state = %s, want declared (target row, no registry entry)", readCell.State)
	}

	// --kind filters the same underlying projection.
	filtered := filterEntriesByKind(matrix.Entries, support.KindClient)
	if len(filtered) != 1 || filtered[0].ID != "claude-code" {
		t.Fatalf("filterEntriesByKind(client) = %+v, want just claude-code", filtered)
	}

	// The text render never claims a level it did not just prove not to
	// carry, and the JSON render round-trips through the same schema.
	var buf strings.Builder
	renderSupportTable(&buf, matrix)
	out := buf.String()
	if !strings.Contains(out, "python") || !strings.Contains(out, "tolerated") {
		t.Fatalf("renderSupportTable output missing expected rows:\n%s", out)
	}

	// cmdSupport itself (SM-10's own anchor, not just the join it wraps):
	// a minimal checkout with an empty queue and an empty target file
	// still loads, joins and emits a valid `fathom.support/v1` document
	// (the client kind alone, from the real harness registry), and an
	// unknown --kind is refused.
	t.Run("cli", cmdSupportCLISmoke)
}

func cmdSupportCLISmoke(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "docs", "plans", "language-breadth.queue.json"), "[]")
	mustWriteFile(t, filepath.Join(dir, "docs", "plans", "support.targets.json"), "[]")
	t.Chdir(dir)

	var stdout, stderr strings.Builder
	if code := cmdSupport([]string{"--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("cmdSupport(--json) = %d, stderr=%s", code, stderr.String())
	}
	var m support.Matrix
	if err := json.Unmarshal([]byte(stdout.String()), &m); err != nil {
		t.Fatalf("cmdSupport --json produced invalid JSON: %v\n%s", err, stdout.String())
	}
	if m.Schema != "fathom.support/v1" {
		t.Fatalf("schema = %q, want fathom.support/v1", m.Schema)
	}
	if err := support.ValidateMatrixJSON([]byte(stdout.String())); err != nil {
		t.Fatalf("cmdSupport --json output fails #Matrix validation: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmdSupport([]string{"--kind", "bogus"}, &stdout, &stderr); code != 1 {
		t.Fatalf("cmdSupport(--kind bogus) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown --kind") {
		t.Fatalf("stderr = %q, want an unknown --kind message", stderr.String())
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func findSupportEntry(t *testing.T, m support.Matrix, kind support.Kind, id string) support.Entry {
	t.Helper()
	for _, e := range m.Entries {
		if e.Kind == kind && e.ID == id {
			return e
		}
	}
	t.Fatalf("no entry %s/%s in matrix (have %d entries)", kind, id, len(m.Entries))
	return support.Entry{}
}

func findSupportCell(t *testing.T, e support.Entry, capability string) support.Cell {
	t.Helper()
	for _, c := range e.Cells {
		if c.Capability == capability {
			return c
		}
	}
	t.Fatalf("entry %s/%s has no cell %q", e.Kind, e.ID, capability)
	return support.Cell{}
}
