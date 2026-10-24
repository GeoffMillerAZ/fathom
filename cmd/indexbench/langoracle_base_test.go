package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeOracle(t *testing.T, dir, name string, tf toolOracleFile) string {
	t.Helper()
	b, err := json.Marshal(tf)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMergeBaseOracles: a composing dialect's references and cross-references
// are scored against its own oracle's pairs and the base dialect's oracle's
// (calls and references into references, cross_references into
// cross_references), each family only in the oracle file that grades it; a
// file carrying neither is passed through untouched, and a pair both name
// counts once.
func TestMergeBaseOracles(t *testing.T) {
	dir := t.TempDir()
	own := writeOracle(t, dir, "own.json", toolOracleFile{Tool: "own", Tier: "C", References: []oraclePair{{Path: "a.cue", From: "#A", To: "#B"}}})
	ownX := writeOracle(t, dir, "ownx.json", toolOracleFile{Tool: "ownx", Tier: "A", CrossReferences: []xrefOracleEntry{{Path: "a.cue", From: "#A", ToPath: "b.cue"}}})
	base := writeOracle(t, dir, "base.json", toolOracleFile{Tool: "cue lsp", Version: "v0", Tier: "A",
		Calls:           []oraclePair{{Path: "a.cue", From: "#A", To: "#B"}, {Path: "a.cue", From: "#A", To: "#C"}},
		References:      []oraclePair{{Path: "b.cue", From: "#B", To: "#C"}},
		CrossReferences: []xrefOracleEntry{{Path: "a.cue", From: "#A", ToPath: "b.cue"}, {Path: "a.cue", From: "#A", ToPath: "c.cue"}}})
	defsOnly := writeOracle(t, dir, "defs.json", toolOracleFile{Tool: "defs", Tier: "C"})

	got, notes, err := mergeBaseOracles([]string{own, ownX, defsOnly}, []string{base}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2] != defsOnly {
		t.Fatalf("paths = %v, want the defs-only file passed through", got)
	}
	var m toolOracleFile
	readJSON(t, got[0], &m)
	wantRefs := []oraclePair{{Path: "a.cue", From: "#A", To: "#B"}, {Path: "a.cue", From: "#A", To: "#C"}, {Path: "b.cue", From: "#B", To: "#C"}}
	if !reflect.DeepEqual(m.References, wantRefs) {
		t.Errorf("references = %v, want %v", m.References, wantRefs)
	}
	if len(m.Calls) != 0 || m.Tool != "own" {
		t.Errorf("merged oracle leaked base calls or lost its identity: %+v", m)
	}
	readJSON(t, got[1], &m)
	if len(m.CrossReferences) != 2 || m.CrossReferences[1].ToPath != "c.cue" {
		t.Errorf("cross_references = %v, want own plus the base's new pair", m.CrossReferences)
	}
	if len(m.References) != 0 {
		t.Errorf("xref file gained references: %v", m.References)
	}
	if len(notes) != 1 {
		t.Errorf("want one disclosure note, got %q", notes)
	}
}

func readJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	*(v.(*toolOracleFile)) = toolOracleFile{}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
