// Package modspec is the Go-side companion to schema/module/schema.cue —
// the expert-module manifest shape (docs/spec/expert-module-v0.md §3). No
// runtime loader lives here yet (the honest gap documented in
// docs/spec/expert-module-v0.md §7: unlike internal/semantic's #Layer,
// #Module is validated but not yet decoded/executed by any Go code in this
// repo). This file exists solely to assert the CUE negatives documented,
// commented out, in schema/module/testdata/invalid.cue — the repo
// convention (CLAUDE.md "New CUE schemas": "negatives are asserted by Go
// tests, not live-vetted").
package modspec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "module", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func validSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "module", "testdata", "valid.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid.cue: %v", err)
	}
	// Strip the leading "package modspec" clause: CompileString compiles
	// schemaSource + validSource as ONE file, and a second package clause
	// mid-file is a syntax error. Both files declare the same package, so
	// dropping the second declaration is lossless.
	return strings.Replace(string(data), "package modspec\n", "", 1)
}

// TestModuleSchema_ValidInstancesConcrete proves schema/module/testdata/
// valid.cue's two worked examples (an eval-gated recompilable module, and a
// closed non-recompilable snapshot over a non-git corpus) are both
// structurally valid AND fully concrete — the same strength of check
// TestModuleSchema_Negatives applies to the invalid cases, so a "valid"
// fixture that quietly relies on an unset required field can't pass by
// accident.
func TestModuleSchema_ValidInstancesConcrete(t *testing.T) {
	cctx := cuecontext.New()
	v := cctx.CompileString(schemaSource(t) + "\n" + validSource(t))
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(cue.Concrete(true)); err != nil {
		t.Fatalf("valid.cue instances should validate concretely: %v", err)
	}
}

// TestModuleSchema_Negatives mirrors schema/module/testdata/invalid.cue's
// six commented cases — asserted here in Go so `cue vet ./...` stays green
// on the package itself (repo convention: negatives are Go-asserted, never
// live-vetted). cue.Concrete(true) throughout: case 2 (missing watermark)
// is only catchable at concreteness — a merely-abstract required field is
// structurally valid but incomplete (internal/config/schema_test.go's own
// precedent for the same distinction) — and Concrete(true) is a strict
// superset of the plain Validate() the other cases would also fail under,
// so using it uniformly keeps every case exercised the same way.
func TestModuleSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"empty corpus include", `
bad1: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: []
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`},
		{"missing watermark (the freshness doctrine: a module without a watermark is invalid)", `
bad2: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`},
		{"malformed watermark (neither git SHA nor manifest: content hash)", `
bad3: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "not-a-real-watermark"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`},
		{"evaled:true with no eval_evidence.path (D3 trust seam)", `
bad4: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: true
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`},
		{"recompile.allowed:true with no prompt_identity", `
bad5: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: true
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`},
		{"malformed provenance.compiled_at (not RFC3339-shaped)", `
bad6: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "not-a-date"}
}`},
	}
	cctx := cuecontext.New()

	// Sanity: the schema itself validates.
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
