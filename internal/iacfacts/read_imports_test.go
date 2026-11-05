package iacfacts

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// The package joins the public door's closure (ADR-092), so it imports no product
// package that pulls the store, fusion, connector, MCP, the network or CUE.
// standAloneLeaves are the only product packages the reads may import: leaves that import the standard library and
// third-party parsers and no product package, so the reads still stand alone. TestStandAloneLeavesImportNoProductPackages
// holds each one to that.
var standAloneLeaves = map[string]bool{
	"github.com/geoffmilleraz/fathom/internal/readlimit": true,
	"github.com/geoffmilleraz/fathom/internal/hclbound":  true,
}

// A leaf the reads import must itself import no product package, or the reads would reach one through it.
func TestStandAloneLeavesImportNoProductPackages(t *testing.T) {
	fset := token.NewFileSet()
	for leaf := range standAloneLeaves {
		dir := filepath.Join("..", filepath.Base(leaf))
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("leaf %s: no source in %s (%v)", leaf, dir, err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range af.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(p, "github.com/geoffmilleraz/fathom/") {
					t.Errorf("%s imports product package %s: a leaf the reads use stands alone", f, p)
				}
			}
		}
	}
}

func TestReadsImportNoProductPackages(t *testing.T) {
	forbidden := []string{
		"/internal/store", "/internal/fusion", "/internal/connector", "/internal/mcp",
		"net/http", "cuelang.org", "/schema",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
			if strings.HasPrefix(p, "github.com/geoffmilleraz/fathom/") && !standAloneLeaves[p] {
				t.Errorf("%s imports product package %s: the reads stand alone", f, p)
			}
		}
	}
}

// The file cap is the product's.
func TestFileCapIsTheProducts(t *testing.T) {
	if MaxFileBytes != langroute.MaxExtractSize {
		t.Fatalf("MaxFileBytes %d, product %d", MaxFileBytes, langroute.MaxExtractSize)
	}
}
