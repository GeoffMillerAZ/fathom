package conserve

// resurrection.go — CV-32 (docs/design/refactor-conservation.md §3.3,
// ADR-044 §3 "absence and anti-Blindness"): the deterministic Blindness
// detector. A function present at base in a changed Go file and gone from
// that file at head, whose body hash — with at least 5 statements,
// clone.FuncBodyHash's own statement count — matches a function present
// in a changed file at head under ANOTHER name that is ITSELF new or
// changed in the range (CV-M5, below), is a finding on the "resurrection"
// plane (ADR-044 §0's plane list; distinct from "absence",
// deprecatedcallers.go, even though both belong to this same stage).
//
// CV-M5 (docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md): the
// match against head used to consider ANY function in a changed file,
// merely because its FILE was in the diff, without asking whether that
// specific head function was itself touched by the change. The real
// ADR-025 DuckDB removal produced 24 false body_resurrected findings this
// way: DuckStore and SQLiteStore had implemented the same Store interface
// with naturally convergent CRUD bodies for years, and sqlite.go was a
// changed file for unrelated reasons, so deleting DuckStore alone made
// every one of its untouched twin's methods look "resurrected." A head
// function is now a candidate ONLY when it is itself new in the range (no
// function of that name existed at base, in the same file) or its body
// hash changed in the range (a function of that name existed at base but
// with different content) — a function whose identical body already
// existed at base under its own name was never touched by this change and
// can never be "the reappearance" of something else.
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
	"crypto/sha256"
	"encoding/hex"
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
//
// hash is IDENTIFIER-INSENSITIVE (cluster.NormalizedHash, via
// clone.FuncBodyHash) — Resurrected's own "same shape under a new name"
// question (CV-31/32) needs exactly that insensitivity. exactHash is the
// UNNORMALIZED source text's own sha256 — residue.go's ChangedFunctions
// uses this one: design §3.4/CV-60 define "modified" as "the exact body
// text differs," a different, stricter question a rename or a literal
// swap must still answer (a changed identifier, keyword, string literal
// or type is an ordinary behaviour change, not "the same function").
// Never conflate the two: hash a rename left alone, exactHash a rename
// changed on its own is Blindness evidence (CV-32); the reverse
// (exactHash unchanged, hash changed — impossible, since exact identity
// implies normalized identity too) never arises.
type funcSite struct {
	name      string
	pkg       string
	file      string
	line      int
	endLine   int
	hash      string
	exactHash string
	stmts     int
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
	var head []funcSite // functions present in a changed file at head that are THEMSELVES new or changed in the range (CV-M5)

	for _, path := range in.ChangedFiles {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		headBody, headOK, err := in.HeadFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading head content: %v", path, err))
			continue
		}

		baseBody, baseOK, err := in.BaseFile(path)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading base content: %v", path, err))
			continue
		}

		// baseByName indexes this SAME file's base functions by name — the
		// only base content Resurrected ever reads (design §0: cost scales
		// with the change, never a full base re-ingest) — so a head
		// function's own history can only be checked within its own file.
		var baseFuncs []funcSite
		baseByName := map[string]funcSite{}
		if baseOK {
			baseFuncs, _ = funcSitesIn(path, baseBody)
			for _, f := range baseFuncs {
				baseByName[f.name] = f
			}
		}

		headFuncs := map[string]bool{}
		if headOK {
			headSites, _ := funcSitesIn(path, headBody)
			for _, f := range headSites {
				headFuncs[f.name] = true
				// CV-M5 (docs/bench/readings/cv-m5-blindness-replay-
				// 2026-09-24.md): a head function can only be a
				// resurrection TARGET when it is itself new in the range
				// (no function of this name existed at base, in this same
				// file) or its body hash changed in the range (a function
				// of this name existed but with different content). A
				// function whose identical body already existed at base
				// under this exact name was never touched by this change
				// — two long-lived, independently-written implementations
				// (e.g. DuckStore and SQLiteStore, ADR-025) can converge
				// on the same body for years before one twin is deleted,
				// and the survivor's untouched twin must never be treated
				// as "the deleted one reappeared here."
				if bf, wasAtBase := baseByName[f.name]; !wasAtBase || bf.hash != f.hash {
					head = append(head, f)
				}
			}
		}

		if !baseOK {
			continue // the file itself is new — nothing at base to be gone
		}
		for _, f := range baseFuncs {
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
						g.name, g.file, h.name, h.file, g.stmts,
					),
					Base:     base,
					Head:     headCite,
					Declared: DeclaredPrior,
					// File/Line: the reappearing function's own head
					// location — the finding's subject is the body that
					// showed back up, not the one that left.
					File: h.file,
					Line: h.line,
				})
				continue
			}
			findings = append(findings, Finding{
				Plane: "resurrection",
				Kind:  "body_resurrected",
				Key:   g.name,
				Detail: fmt.Sprintf(
					"%s (deleted from %s) reappears as %s at %s — identical body, %d statement(s), despite a retirement claim for it",
					g.name, g.file, h.name, h.file, g.stmts,
				),
				Base:     base,
				Head:     headCite,
				Declared: DeclaredNone,
				File:     h.file,
				Line:     h.line,
				Remedy: fmt.Sprintf(
					"%s looks copied to %s rather than moved — delete the copy, or rename %s in place so history stays honest",
					g.name, h.name, g.name,
				),
			})
		}
	}
	return findings, missing
}

// funcSitesIn parses src (one Go file's content at some point in the
// range) and returns every top-level function/method declaration's
// funcSite, in file declaration order (deterministic: go/parser preserves
// source order in af.Decls), plus ok reporting whether src parsed as Go
// at all. A function with no body (an external/assembly declaration) is
// skipped — clone.FuncBodyHash has nothing to hash for it, and it can
// never be "the same body" as anything.
//
// ok is false ONLY for an actual parse failure — src is not valid Go at
// all (residue.go's ChangedFunctions reports this honestly as
// "unanalyzed", never as a silent zero, the same distinction missing
// carries for a content READ failure). A src that parses but declares no
// functions returns ok=true with a nil slice — an ordinary, honest empty
// result, not a parse failure.
func funcSitesIn(filePath string, src []byte) (sites []funcSite, ok bool) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, false
	}
	pkgName := ""
	if af.Name != nil {
		pkgName = af.Name.Name
	}
	var out []funcSite
	for _, decl := range af.Decls {
		fn, declOK := decl.(*ast.FuncDecl)
		if !declOK || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		if start < 0 || end > len(src) || start >= end {
			continue
		}
		body := src[start:end]
		hash, stmts, hashOK := clone.FuncBodyHash(string(body))
		if !hashOK {
			continue
		}
		exactSum := sha256.Sum256(body)
		out = append(out, funcSite{
			name:      funcDeclName(fn),
			pkg:       pkgName,
			file:      filePath,
			line:      fset.Position(fn.Pos()).Line,
			endLine:   fset.Position(fn.End()).Line,
			hash:      hash,
			exactHash: hex.EncodeToString(exactSum[:]),
			stmts:     stmts,
		})
	}
	return out, true
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
