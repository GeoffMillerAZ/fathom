package clone

// funcbodyhash.go — CV-31 (ADR-044 §3.3, design doc "Stage 3 — absence and
// anti-Blindness"): FuncBodyHash exposes, per function, the SAME
// identifier-insensitive normalisation Detect already uses internally
// (NormalizedHash) — the deterministic Blindness detector conserve.
// Resurrected (CV-32) is built on needs to agree with clone.Detect about
// when two function bodies are "the same shape," never a second, drifting
// hashing scheme.

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/geoffmilleraz/fathom/internal/cluster"
)

// FuncBodyHash hashes ONE Go function's source text identifier-
// insensitively: renaming every parameter, receiver, local or return value
// leaves the hash unchanged (cluster.NormalizedHash's own contract —
// renamed identifiers collapse to the same placeholder, numeric literals
// and everything else are preserved), while a structural edit (a different
// literal, a different call, an added or removed statement) changes it.
//
// body is expected to be exactly one function declaration's own source
// text — normally one splitGoFuncs chunk, "func …" through its closing
// brace — never a whole file and never more than one declaration; ok is
// false when body does not parse as exactly one Go function declaration
// (a non-Go fragment, a malformed chunk, a var/type/const declaration).
//
// statementCount is the number of TOP-LEVEL statements in the function's
// body block (go/ast's own *ast.BlockStmt.List length — a nested if/for's
// own statements are not counted again), the floor conserve.Resurrected
// (design §3.3: "with at least 5 statements") uses to keep a coincidental
// match on a trivial one-or-two-line body from reading as a Blindness
// finding. A function with no body at all (an external/assembly
// declaration, `func F()` with no `{ }`) parses fine and reports 0.
func FuncBodyHash(body string) (hash string, statementCount int, ok bool) {
	fset := token.NewFileSet()
	// Wrapping in a minimal package clause is the standard trick for
	// parsing a single top-level declaration in isolation; go/parser
	// requires a complete source file, not a bare declaration.
	af, err := parser.ParseFile(fset, "", "package p\n\n"+body, parser.ParseComments)
	if err != nil {
		return "", 0, false
	}
	var fn *ast.FuncDecl
	n := 0
	for _, decl := range af.Decls {
		if fd, isFn := decl.(*ast.FuncDecl); isFn {
			fn = fd
			n++
		}
	}
	if n != 1 {
		// Zero function declarations (a var/type/const, or nothing at
		// all), or more than one (body held several — the caller's
		// contract is violated, refuse rather than silently hash the
		// concatenation of two unrelated functions).
		return "", 0, false
	}
	if fn.Body != nil {
		statementCount = len(fn.Body.List)
	}
	return cluster.NormalizedHash(body), statementCount, true
}
