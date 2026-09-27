package client_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestObligationsHotPathOpensFilesOnlyThroughCappedReader is the static
// half of the "reads only the guides and the edited file" proofs (RT-81,
// RT-143, RT-26 review). The dynamic proofs watch readFileCapped
// (ObserveReadsUnder) and the connector's Fetch, so a regression to a
// plain os.ReadFile would pass them. This walks pkg/client's own call
// graph from the obligations hot-path entry points and fails when any
// reached function other than readFileCapped opens a file directly, or
// when a function outside the bounded ones fetches through the connector.
// Calls into other packages are not followed (internal/semantic's own
// reads are proven by its tests, e.g. TestCitedTextResolverStatsBeforeReading).
func TestObligationsHotPathOpensFilesOnlyThroughCappedReader(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	funcs := map[string][]*ast.FuncDecl{}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					funcs[fd.Name.Name] = append(funcs[fd.Name.Name], fd)
				}
			}
		}
	}
	entries := []string{"ObligationsLayerCoverage", "RawFallbackObligations", "ObligationsIndexCheck", "ObligationsIndexStale"}
	for _, e := range entries {
		if len(funcs[e]) == 0 {
			t.Fatalf("entry point %s not found: the walk would prove nothing", e)
		}
	}
	openers := map[string]map[string]bool{
		"os":     {"ReadFile": true, "Open": true, "OpenFile": true},
		"ioutil": {"ReadFile": true},
		"fs":     {"ReadFile": true},
	}
	allowedOpen := map[string]bool{"readFileCapped": true}
	// Connector fetches allowed only where they are bounded: one selected
	// guide at a time (layerCorpus) or the one edited file (editedFileTokens),
	// each capped after the fetch, for a service with no on-disk root.
	allowedFetch := map[string]bool{"layerCorpus": true, "editedFileTokens": true}

	seen := map[string]bool{}
	queue := append([]string(nil), entries...)
	var violations []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, fd := range funcs[name] {
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					if _, ok := funcs[fn.Name]; ok {
						queue = append(queue, fn.Name)
					}
				case *ast.SelectorExpr:
					if x, ok := fn.X.(*ast.Ident); ok && openers[x.Name][fn.Sel.Name] && !allowedOpen[name] {
						violations = append(violations, name+" calls "+x.Name+"."+fn.Sel.Name+" at "+fset.Position(call.Pos()).String())
					}
					if fn.Sel.Name == "Fetch" {
						if inner, ok := fn.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "conn" && !allowedFetch[name] {
							violations = append(violations, name+" fetches through the connector at "+fset.Position(call.Pos()).String())
						}
					}
					if _, ok := funcs[fn.Sel.Name]; ok {
						queue = append(queue, fn.Sel.Name)
					}
				}
				return true
			})
		}
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("obligations hot path opens a file outside the capped reader: %s", v)
	}
	if !seen["readFileCapped"] || !seen["layerCorpus"] || !seen["editedFileTokens"] {
		t.Errorf("the walk did not reach the hot path's readers (reached %d functions); the proof is vacuous", len(seen))
	}
}
