package conserve

// deprecatedcallers.go — CV-30 (docs/design/refactor-conservation.md §3.2,
// ADR-044 §3 "absence and anti-Blindness"): a call site to a symbol
// documented "Deprecated:" that is NEW in the range (present in a changed
// Go file at head, absent from that same file at base) is a finding on
// the "absence" plane (ADR-044 §0's plane list — a deprecation ratchet
// belongs beside the other "stays gone/stays unused" checks this stage
// runs).
//
// The declared-deprecated NAME set is read once from the corpus at HEAD
// (in.Root): a symbol's own "Deprecated:" doc comment is essentially
// never itself part of the change under review, so this is a corpus-wide
// registry lookup the same way the certificates/governance planes' own
// compiled-claim sets are — never a second per-change re-derivation. The
// call-site DIFF proper reads only in.ChangedFiles' base/head content, so
// ITS cost still scales with the change (design §0).
//
// This is a heuristic (bare-name, AST-syntactic) check, not a typed one:
// a call site is matched by its callee's bare identifier/selector name,
// the same granularity the declaration scan itself uses, so the two
// sides can never disagree about what "the name" means — but two
// unrelated symbols sharing a bare name (a "Run" deprecated in one
// package, an unrelated "Run" called elsewhere) can produce a false
// finding. Declared is always DeclaredNone: design §3.2 names no
// declaration mechanism that excuses a fresh call to something already
// marked deprecated — the ratchet is meant to be absolute.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func init() { Register(deprecatedCallersPlane{}) }

// deprecatedCallersPlane is the "absence" plane's DeprecatedCallers check.
// Registered from its own file (this one), separately from
// resurrectionPlane (resurrection.go) even though both belong to ADR-044
// §3's "absence and anti-Blindness" stage: ADR-044 §0 names "absence" and
// "resurrection" as two DISTINCT planes in the wire contract, and each
// plane's own file registering itself is what lets two workers land a
// plane apiece in the same wave without ever touching the same file
// (conserve.go's package doc comment).
type deprecatedCallersPlane struct{}

func (deprecatedCallersPlane) Name() string { return "absence" }

func (deprecatedCallersPlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	return DeprecatedCallers(in)
}

// DeprecatedCallers is deprecatedCallersPlane's own analysis (CV-30),
// callable directly (a test, or a caller that wants just this check).
func DeprecatedCallers(in Input) ([]Finding, []string) {
	if in.Root == "" {
		return nil, []string{"no corpus root: cannot find symbols documented Deprecated:"}
	}
	deprecated, err := deprecatedSymbolNames(in.Root)
	if err != nil {
		return nil, []string{fmt.Sprintf("scanning %s for Deprecated: symbols: %v", in.Root, err)}
	}
	if len(deprecated) == 0 {
		return nil, nil
	}

	var findings []Finding
	var missing []string
	for _, path := range in.ChangedFiles {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		headBody, headOK, err := in.HeadFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading head content: %v", path, err))
			continue
		}
		if !headOK {
			continue // deleted at head — no NEW call site can live there
		}
		headCalls := callSitesOf(path, headBody, deprecated)
		if len(headCalls) == 0 {
			continue
		}

		baseBody, baseOK, err := in.BaseFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading base content: %v", path, err))
			continue
		}
		var baseNames map[string]bool
		if baseOK {
			baseNames = callSiteNames(path, baseBody, deprecated)
		}

		for _, c := range headCalls {
			if baseNames[c.name] {
				continue // already called from this file at base — not new
			}
			findings = append(findings, Finding{
				Plane:    "absence",
				Kind:     "new_deprecated_caller",
				Key:      c.name,
				Detail:   fmt.Sprintf("new call site to deprecated %s", c.name),
				Head:     fmt.Sprintf("%s:%d", path, c.line),
				Declared: DeclaredNone,
				Remedy:   fmt.Sprintf("remove the new call to %s, or use its documented successor", c.name),
				// File/Line: the new call site itself — it exists only at
				// head (that is the whole finding).
				File: path,
				Line: c.line,
			})
		}
	}
	return findings, missing
}

// call is one call site DeprecatedCallers' AST scan found.
type call struct {
	name string
	line int
}

// callSitesOf returns every call site in src (one Go file's content at
// some point in the range) whose callee's bare name is in deprecated,
// sorted by line then name — deterministic regardless of go/ast's own
// (unspecified) declaration-order walk.
func callSitesOf(path string, src []byte, deprecated map[string]bool) []call {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil // heuristic scan over already-read content: never fatal
	}
	var calls []call
	ast.Inspect(af, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name := calleeName(ce.Fun); name != "" && deprecated[name] {
			calls = append(calls, call{name: name, line: fset.Position(ce.Pos()).Line})
		}
		return true
	})
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].line != calls[j].line {
			return calls[i].line < calls[j].line
		}
		return calls[i].name < calls[j].name
	})
	return calls
}

// callSiteNames reduces callSitesOf to the set of distinct deprecated
// names called anywhere in src — the base-side "was this already called
// from this file" check only needs presence, never position.
func callSiteNames(path string, src []byte, deprecated map[string]bool) map[string]bool {
	names := map[string]bool{}
	for _, c := range callSitesOf(path, src, deprecated) {
		names[c.name] = true
	}
	return names
}

// calleeName returns a call expression's callee's bare name: an ordinary
// function call's identifier, or a selector call's method/field name —
// package-qualified calls (pkg.Old()) and method calls (recv.Old()) alike
// reduce to their bare "Old", the same granularity deprecatedSymbolNames'
// declaration scan uses.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	default:
		return ""
	}
}

// deprecatedSymbolNames walks root's Go source (skipping vendor,
// node_modules, testdata, .git and any other dot-directory) and returns
// the bare names of every top-level func/method/type declaration whose
// doc comment carries a paragraph starting "Deprecated:" — the Go
// convention (godoc, staticcheck SA1019) this check piggybacks on rather
// than inventing its own marker. A method's reported name is its bare
// method name, unqualified by receiver type — calleeName's call-site
// matching (above) only ever compares bare selector names, so the two
// sides of the comparison can never disagree over a qualified name a
// single-file AST walk has no way to resolve anyway.
func deprecatedSymbolNames(root string) (map[string]bool, error) {
	names := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil // unreadable file: skip it, never fail the whole scan
		}
		fset := token.NewFileSet()
		af, perr := parser.ParseFile(fset, path, body, parser.ParseComments)
		if perr != nil {
			return nil
		}
		for _, decl := range af.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if hasDeprecatedDoc(d.Doc) {
					names[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					doc := ts.Doc
					if doc == nil && len(d.Specs) == 1 {
						doc = d.Doc
					}
					if hasDeprecatedDoc(doc) {
						names[ts.Name.Name] = true
					}
				}
			}
		}
		return nil
	})
	return names, err
}

// hasDeprecatedDoc reports whether doc carries a line whose trimmed text
// starts with "Deprecated:" — go/doc's own convention for a deprecation
// notice (a paragraph, not necessarily the first line of the comment).
func hasDeprecatedDoc(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, line := range strings.Split(doc.Text(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Deprecated:") {
			return true
		}
	}
	return false
}
