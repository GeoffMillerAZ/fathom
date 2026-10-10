package extractor

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestSpecExtractorIsCGOFree (LB-F394): the one-parse contract fusion's
// extract stage asks for (SpecExtractor, specmemo.go) lives in a file that
// imports no CGO tree-sitter binding, so it outlives the native-tree helpers
// in nativespecs.go when the last CGO walker is retired (LB-F19).
func TestSpecExtractorIsCGOFree(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "specextractor.go", nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if strings.HasPrefix(p, "github.com/tree-sitter/") || p == "C" {
			t.Errorf("specextractor.go imports %s: the one-parse contract must not need a CGO binding", p)
		}
	}
	var se SpecExtractor = (*TSExtractor)(nil)
	_ = se
}
