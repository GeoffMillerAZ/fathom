package conserve

// resurrection.go — CV-32 (docs/design/refactor-conservation.md §3.3,
// ADR-044 §3 "absence and anti-Blindness"): the deterministic Blindness
// detector. A function present at base in a changed Go file and gone from
// that file at head, whose body hash — with at least 5 statements,
// clone.FuncBodyHash's own statement count — matches a function present
// in a changed file at head under ANOTHER name, is a finding on the
// "resurrection" plane (ADR-044 §0's plane list; distinct from "absence",
// deprecatedcallers.go, even though both belong to this same stage).
//
// This is exactly the failure vision 34 §2 names as fathom's reason to
// exist here: "SWE Refactor Bench (arXiv 2608.23564): 28 of 520 runs pass
// its three stages; agents copy the old implementation behind a new name
// while the old package import is removed." Tests keep passing because
// the code never actually left — this check is the one place fathom
// looks for exactly that.
//
// Resurrection honesty (amended 2026-09-24, owner decision, design §3.3):
// an ordinary rename or move must never be a loss — most reappeared
// bodies are exactly that, nothing was ever claimed about them, and
// flagging every rename as a "loss" would make the plane noise, not
// signal. So the SAME body-hash match now splits in two:
//
//   - No retirement was CLAIMED for the deleted function (a `retires:`
//     key), its package, or an import it lived behind (a `gone: import
//     <path>` row — in.Retired, either from base or from head: a
//     retirement declared inside the very same range still counts as a
//     claim here, unlike Declarations' ordinary I45 prior/self split) —
//     kind "moved", Declared DeclaredPrior. This is deliberately NOT the
//     ordinary meaning of "prior" (a declaration excusing a loss);
//     nothing was declared at all. It is planeVerdict's own escape hatch
//     for "never a loss on its own," reused here because Finding has no
//     third loss-exempt state and the shape already means exactly that.
//   - A retirement WAS claimed, and the body still turns up anyway — the
//     claim was false. THAT is the actual Blindness case ADR-044 exists
//     to catch (an agent writes a `gone: import legacy` row to satisfy
//     the gate while quietly keeping the code alive elsewhere): kind
//     "body_resurrected", Declared DeclaredNone — a loss, exactly as
//     before this amendment.
import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/clone"
)

func init() { Register(resurrectionPlane{}) }

// resurrectionPlane is the "resurrection" plane's Resurrected check.
type resurrectionPlane struct{}

func (resurrectionPlane) Name() string { return "resurrection" }

func (resurrectionPlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	return Resurrected(in)
}

// funcSite is one Go function's identity for Resurrected's purposes: its
// name (funcDeclName — bare, or "Receiver.Method"), its file's own
// package clause (pkg — resurrection honesty's package-retirement check),
// defining file:line, and clone.FuncBodyHash's own hash/statement count
// for its source text.
type funcSite struct {
	name  string
	pkg   string
	file  string
	line  int
	hash  string
	stmts int
}

// Resurrected is resurrectionPlane's own analysis (CV-32), callable
// directly (a test, or a caller that wants just this check). Only
// in.ChangedFiles' base/head content is read — cost scales with the
// change (design §0) on both sides: the "gone" set (base functions absent
// from their file at head) and the "reappeared" set (every function in a
// changed file at head) are both drawn from the same changed-file walk.
func Resurrected(in Input) ([]Finding, []string) {
	var missing []string
	var gone []funcSite // base functions absent from their OWN file at head
	var head []funcSite // every function present in a changed file at head

	for _, path := range in.ChangedFiles {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		headBody, headOK, err := in.HeadFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading head content: %v", path, err))
			continue
		}
		headFuncs := map[string]bool{}
		if headOK {
			for _, f := range funcSitesIn(path, headBody) {
				head = append(head, f)
				headFuncs[f.name] = true
			}
		}

		baseBody, baseOK, err := in.BaseFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading base content: %v", path, err))
			continue
		}
		if !baseOK {
			continue // the file itself is new — nothing at base to be gone
		}
		for _, f := range funcSitesIn(path, baseBody) {
			if !headFuncs[f.name] {
				gone = append(gone, f)
			}
		}
	}

	var findings []Finding
	for _, g := range gone {
		if g.stmts < 5 || g.hash == "" {
			continue // design §3.3's floor: too small to mean anything
		}
		for _, h := range head {
			if h.name == g.name || h.hash != g.hash {
				continue
			}
			base := fmt.Sprintf("%s:%d", g.file, g.line)
			headCite := fmt.Sprintf("%s:%d", h.file, h.line)
			if !in.Retired.ClaimsKey(fmt.Sprintf("api:%s.%s", path.Dir(g.file), g.name)) &&
				!in.Retired.ClaimsImport(g.pkg) {
				// No retirement was claimed for this function, its
				// package, or an import it lived behind — an ordinary,
				// undeclared rename. Informational, never a loss (design
				// §3.3, amended 2026-09-24).
				findings = append(findings, Finding{
					Plane: "resurrection",
					Kind:  "moved",
					Key:   g.name,
					Detail: fmt.Sprintf(
						"%s (moved from %s) reappears as %s at %s — identical body, %d statement(s); no retirement was declared, treated as an ordinary move",
						g.name, g.file, h.name, h.file, g.stmts),
					Base:     base,
					Head:     headCite,
					Declared: DeclaredPrior,
				})
				continue
			}
			findings = append(findings, Finding{
				Plane: "resurrection",
				Kind:  "body_resurrected",
				Key:   g.name,
				Detail: fmt.Sprintf(
					"%s (deleted from %s) reappears as %s at %s — identical body, %d statement(s), despite a retirement claim for it",
					g.name, g.file, h.name, h.file, g.stmts),
				Base:     base,
				Head:     headCite,
				Declared: DeclaredNone,
				Remedy: fmt.Sprintf(
					"%s looks copied to %s rather than moved — delete the copy, or rename %s in place so history stays honest",
					g.name, h.name, g.name),
			})
		}
	}
	return findings, missing
}

// funcSitesIn parses src (one Go file's content at some point in the
// range) and returns every top-level function/method declaration's
// funcSite, in file declaration order (deterministic: go/parser preserves
// source order in af.Decls). A function with no body (an external/
// assembly declaration) is skipped — clone.FuncBodyHash has nothing to
// hash for it, and it can never be "the same body" as anything.
func funcSitesIn(filePath string, src []byte) []funcSite {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil // heuristic scan over already-read content: never fatal
	}
	pkgName := ""
	if af.Name != nil {
		pkgName = af.Name.Name
	}
	var out []funcSite
	for _, decl := range af.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		if start < 0 || end > len(src) || start >= end {
			continue
		}
		hash, stmts, ok := clone.FuncBodyHash(string(src[start:end]))
		if !ok {
			continue
		}
		out = append(out, funcSite{
			name:  funcDeclName(fn),
			pkg:   pkgName,
			file:  filePath,
			line:  fset.Position(fn.Pos()).Line,
			hash:  hash,
			stmts: stmts,
		})
	}
	return out
}

// funcDeclName names fn the same way safe_delete_check and the plan
// grammar's own symbol: contract do for a method — "Receiver.Method",
// pointer/generic receivers unwrapped to their base type name — or just
// the bare function name when fn has no receiver.
func funcDeclName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	if recv := receiverTypeName(fn.Recv.List[0].Type); recv != "" {
		return recv + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// receiverTypeName unwraps a method receiver's type expression to its
// base identifier: "*Foo" and "Foo" both name "Foo"; "Foo[T]" (a generic
// receiver, one or several type parameters) also names "Foo".
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	default:
		return ""
	}
}
