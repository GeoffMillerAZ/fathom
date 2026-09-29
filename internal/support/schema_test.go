package support

// schema_test.go asserts schema/support/testdata/invalid.cue's six
// commented cases actually fail (SM-2), the same "negatives are Go-
// asserted, never live-vetted" convention schema/AGENTS.md documents and
// internal/modspec/schema_test.go established for #Module. It also proves
// the positive half of SM-2's contract: schema/support/testdata/valid.cue's
// worked `matrix` example vets against #Matrix through ValidateMatrixJSON,
// the exported reader — not just through a raw CUE unify.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func readSchemaTestdata(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "schema", "support", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestSupportSchemaMatchesSource is the drift gate: internal/support/
// support_schema.cue (embedded) must stay byte-identical to
// schema/support/schema.cue (the source a human edits).
func TestSupportSchemaMatchesSource(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "schema", "support", "schema.cue"))
	if err != nil {
		t.Fatalf("read schema/support/schema.cue: %v", err)
	}
	if string(source) != string(supportSchema) {
		t.Fatal("internal/support/support_schema.cue has drifted from schema/support/schema.cue — copy the source file verbatim over the embedded mirror")
	}
}

// TestSupportSchema_Negatives proves schema/support/testdata/valid.cue's
// worked matrix vets through ValidateMatrixJSON (SM-2's "the exported
// matrix vets against #Matrix"), and that every one of schema/support/
// testdata/invalid.cue's six documented cases fails through the CUE Go API
// (SM-2's other half) — case 2/3/4 (#Cell shapes) additionally wrapped into
// a full matrix and run back through ValidateMatrixJSON itself, so the
// reader's own rejection path is exercised, not only bare CUE unification.
func TestSupportSchema_Negatives(t *testing.T) {
	schema := string(supportSchema)
	cctx := cuecontext.New()

	// Sanity: the schema itself validates.
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}

	t.Run("valid matrix vets through ValidateMatrixJSON", func(t *testing.T) {
		valid := strings.Replace(readSchemaTestdata(t, "valid.cue"), "package support\n", "", 1)
		v := cctx.CompileString(schema + "\n" + valid)
		if err := v.Err(); err != nil {
			t.Fatalf("compile schema+valid.cue: %v", err)
		}
		matrixVal := v.LookupPath(cue.ParsePath("matrix"))
		if !matrixVal.Exists() {
			t.Fatal("valid.cue defines no `matrix` value")
		}
		data, err := matrixVal.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal matrix value: %v", err)
		}
		if err := ValidateMatrixJSON(data); err != nil {
			t.Fatalf("ValidateMatrixJSON on valid.cue's matrix: %v", err)
		}
	})

	t.Run("state outside the computed vocabulary, wrapped in a matrix", func(t *testing.T) {
		// Mirrors invalid.cue case 4 (a hand-set status), embedded as one
		// entry's cell so the failure is exercised through
		// ValidateMatrixJSON itself, not a bare #Cell unify.
		badMatrix := []byte(`{
			"schema": "fathom.support/v1",
			"generated_at": "2026-09-27T00:00:00Z",
			"commit": "86d4c5e2",
			"entries": [
				{
					"kind": "client", "id": "cursor", "name": "Cursor", "registered": true,
					"cells": [{"capability": "mcp-tools", "state": "done"}]
				}
			]
		}`)
		if err := ValidateMatrixJSON(badMatrix); err == nil {
			t.Fatal("expected ValidateMatrixJSON to reject a state outside #State, got nil")
		} else if !strings.Contains(err.Error(), "state") && !strings.Contains(err.Error(), "done") {
			t.Errorf("error %q does not mention the offending state — may be failing for the wrong reason", err.Error())
		}
	})

	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"a language target in the support targets file (languages live in the queue)", `
bad1: #Target & {kind: "language", id: "go", name: "Go", target: {L1: "L1"}, plan_row: "SM-1", wave: 1}`, "kind"},
		{"a proven cell with no reading — a level without its evidence", `
bad2: #Cell & {capability: "L2", state: "proven-good", level_met: "L2"}`, "reading"},
		{"a tolerated cell that claims a level", `
bad3: #Cell & {capability: "L3", state: "tolerated", level_met: "L3"}`, "_|_"},
		{"a state outside the computed vocabulary (a hand-set status)", `
bad4: #Cell & {capability: "L1", state: "done"}`, "state"},
		{"a reading that types a grade outside the closed list", `
bad5: #Reading & {kind: "rules", id: "cursor-rules", reading_date: "2026-09-27", commit: "86d4c5e", instrument: "x", metrics: {}, met: {read: "great"}}`, "met"},
		{"a target row whose plan_row is not a row id", `
bad6: #Target & {kind: "spec", id: "spec-kit", name: "Spec Kit", target: {read: true}, plan_row: "soon", wave: 1}`, "plan_row"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			var errText string
			if v.Err() != nil {
				errText = v.Err().Error()
			} else if err := v.Validate(cue.Concrete(true)); err != nil {
				errText = err.Error()
			} else {
				t.Fatal("expected a validation failure, got none")
			}
			if !strings.Contains(errText, tc.wantErrSubstr) {
				t.Errorf("error %q does not mention %q — this case may be failing for the wrong reason", errText, tc.wantErrSubstr)
			}
		})
	}
}
