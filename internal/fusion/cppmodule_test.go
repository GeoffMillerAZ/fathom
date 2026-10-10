package fusion_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestResolveFileEdges_CppModuleSymbol (LB-F434, O805): a C++ file is a module
// (K18's module symbol, the cpp manifest's module_symbol), so an #include of
// a file of the subject is an edge to the included file's module whether or
// not a name of it is referenced (config.hpp's macro is read by no code here),
// and a reference written at file level, outside every definition, is made
// from the file's module: a namespace macro alone on its line
// (`LIB_NAMESPACE_BEGIN`, which the cpp-macros pre-pass blanks to parse the
// file, nlohmann/json's NLOHMANN_JSON_NAMESPACE_BEGIN) binds the header that
// defines it. Before, an include named nothing and the macro's reference had
// no definition to draw from, so value.hpp reached neither file.
func TestResolveFileEdges_CppModuleSymbol(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cppmodule"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "cpp", fixtureFiles(t, dir, ".hpp"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "cpp", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := strings.Join(namedEdges(edges, symbols), "\n")
	for _, want := range []string{
		"include/lib/value.hpp|<module> -> include/lib/abi.hpp|<module>",
		"include/lib/value.hpp|<module> -> include/lib/abi.hpp|LIB_NAMESPACE_BEGIN",
		"include/lib/value.hpp|<module> -> include/lib/abi.hpp|LIB_NAMESPACE_END",
		"include/lib/value.hpp|<module> -> include/lib/config.hpp|<module>",
	} {
		if !strings.Contains(got+"\n", want+"\n") {
			t.Errorf("missing edge %s; edges:\n%s", want, got)
		}
	}
}
