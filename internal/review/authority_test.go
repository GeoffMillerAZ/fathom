// authority_test.go proves ADR-040's boundary structurally, not just by
// assertion (CX-8): this package cannot post, approve or block anything
// because it imports nothing capable of reaching a network or a forge API.
package review

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// deniedImportSubstrings names import-path fragments a leaf evidence
// package must never carry — any of these would mean this package COULD
// reach outside its own process (a forge client, raw HTTP, a git remote,
// or an os/exec shell-out), which is exactly the authority ADR-040 reserves
// to CI, never to fathom itself.
var deniedImportSubstrings = []string{
	"net/http",
	"net/rpc",
	"os/exec",
	"go-github",
	"google/go-github",
	"github.com/google/go-github",
	"/forge",
	"/github",
	"/gitlab",
}

func TestReceiptNeverPostsOrBlocks(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				p = imp.Path.Value
			}
			for _, denied := range deniedImportSubstrings {
				if strings.Contains(p, denied) {
					t.Errorf("%s imports %q — internal/review must never be able to reach a network or a forge API (ADR-040)", name, p)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test .go files found to check — the import scan ran over nothing")
	}

	// Every RiskReceipt carries the fixed boundary statement.
	r := BuildRiskReceipt(Input{}, DefaultRiskTable())
	if r.Authority != receiptAuthority() {
		t.Errorf("Authority = %q, want the fixed receiptAuthority() statement", r.Authority)
	}
	for _, want := range []string{"never posts", "approves", "blocks", "CI decides"} {
		if !strings.Contains(r.Authority, want) {
			t.Errorf("Authority %q does not mention %q", r.Authority, want)
		}
	}
}

// TestReceiptAuthorityNeverActionable is a belt-and-braces structural
// check: RiskReceipt and RiskFactor are plain data (no exported method
// whose name could plausibly perform a side-effecting action). A receipt
// is evidence handed to something that decides — it has no verbs of its
// own.
func TestReceiptAuthorityNeverActionable(t *testing.T) {
	deniedVerbs := []string{"Post", "Approve", "Block", "Comment", "Merge", "Close", "Reject"}
	for _, name := range deniedVerbs {
		// A compile-time-shaped check: if RiskReceipt ever grows a method
		// with one of these names, this file (which declares no such
		// method) would need to add one to keep building — the AST scan
		// below is the actual enforcement, this loop only documents the
		// denylist the AST scan uses.
		_ = name
	}

	dir, _ := os.Getwd()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil {
					continue
				}
				recvType := receiverTypeName(fn.Recv)
				if recvType != "RiskReceipt" && recvType != "RiskFactor" {
					continue
				}
				for _, verb := range deniedVerbs {
					if fn.Name.Name == verb {
						t.Errorf("%s.%s: a data type must not carry an action-shaped method (ADR-040)", recvType, fn.Name.Name)
					}
				}
			}
		}
	}
}

func receiverTypeName(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	expr := fl.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
