// schema_test.go — asserts the schema/module CUE negatives documented,
// commented out, in schema/module/testdata/invalid.cue (repo convention,
// schema/CLAUDE.md: "negatives are asserted by Go tests, not
// live-vetted"). See doc.go for the package-level doc comment.
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
//
// Each case also asserts wantErrSubstr appears in the actual CUE error text
// (compile-time or validate-time, whichever fires) — not just that SOME
// error occurred. A bare "expected an error, got none" check can't tell a
// case failing for the right reason (the field this case is actually
// probing) from one failing for the wrong one (e.g. a typo elsewhere in the
// snippet that happens to also be invalid); asserting the offending field's
// path in the message closes that gap.
func TestModuleSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name          string
		bad           string
		wantErrSubstr string
	}{
		{"empty corpus include", `
bad1: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: []
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`, "include"},
		{"missing watermark (the freshness doctrine: a module without a watermark is invalid)", `
bad2: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`, "watermark"},
		{"malformed watermark (neither git SHA nor manifest: content hash)", `
bad3: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "not-a-real-watermark"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`, "watermark"},
		{"evaled:true with no eval_evidence.path (D3 trust seam)", `
bad4: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: true
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`, "eval_evidence.path"},
		{"recompile.allowed:true with no prompt_identity", `
bad5: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: true
	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
}`, "prompt_identity"},
		{"malformed provenance.compiled_at (not RFC3339-shaped)", `
bad6: #Module & {
	name: "x", version: "v1", description: "x"
	corpus: include: ["docs/**"]
	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	eval_evidence: evaled: false
	recompile: allowed: false
	provenance: {compiled_by: "deterministic", compiled_at: "not-a-date"}
}`, "compiled_at"},
	}
	cctx := cuecontext.New()

	// Sanity: the schema itself validates.
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			var errText string
			if v.Err() != nil {
				errText = v.Err().Error() // compile-time rejection
			} else if err := v.Validate(cue.Concrete(true)); err != nil {
				errText = err.Error() // validate-time rejection
			} else {
				t.Fatal("expected a validation failure, got none")
			}
			if !strings.Contains(errText, tc.wantErrSubstr) {
				t.Errorf("error %q does not mention %q — this case may be failing for the wrong reason", errText, tc.wantErrSubstr)
			}
		})
	}
}
