package fusion

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestFileResolve_ObjectiveCImports is LB-Q-objective-c (product-reached): an
// Objective-C .m (the dialect's through the .m sniff) and an Objective-C .h
// (through the header router) are read by the file resolver, so a #import of a
// sibling header and of a nested one draws a file edge, a C header the .m
// includes is still resolved, and a MATLAB .m beside them draws none.
func TestFileResolve_ObjectiveCImports(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"App/Shape.h":      "#import <Foundation/Foundation.h>\n#import \"Util.h\"\n#import \"Types.h\"\n@interface Shape : NSObject\n- (int)sides;\n@end\n",
		"App/Shape.m":      "#import \"Shape.h\"\n@implementation Shape\n- (int)sides { ShapeCount n = util_count(); return n; }\n@end\n",
		"App/Types.h":      "#import <Foundation/Foundation.h>\ntypedef NSInteger ShapeCount;\n",
		"App/Util.h":       "#include <stdio.h>\nint util_count(void);\n",
		"App/Util.c":       "#include \"Util.h\"\nint util_count(void) { return 3; }\n",
		"App/Main.m":       "#import \"Shape.h\"\n#include \"Util.h\"\nint main(void) { @autoreleasepool { Shape *s = [Shape new]; return [s sides]; } }\n",
		"lab/solve.m":      "function y = solve(x)\n  y = x;\nend\n",
		"lab/plain.h":      "int plain(void);\n",
		"lab/use_plain.c":  "#include \"plain.h\"\nint run(void) { return plain(); }\n",
		"lab/plain_impl.c": "int plain(void) { return 1; }\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "objc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edges, _ := st.TypedEdges(ctx, "objc")
	files := []string{"App/Shape.h", "App/Shape.m", "App/Types.h", "App/Util.h", "App/Util.c", "App/Main.m", "lab/solve.m", "lab/plain.h"}
	fileOf := map[string]string{}
	obs, _ := st.SourceObservations(ctx, "objc", files)
	for _, o := range obs {
		s := o.ToSymbolNode()
		fileOf[s.Key()] = s.FilePath
	}
	var got []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+" -> "+fileOf[e.ToSymbol])
		}
	}
	has := func(want string) {
		t.Helper()
		for _, g := range got {
			if g == want {
				return
			}
		}
		t.Errorf("no file edge %q; got %s", want, strings.Join(got, "; "))
	}
	has("App/Main.m -> App/Shape.h")  // a class a .m sends a message to is defined in a header the router sent to Objective-C
	has("App/Shape.m -> App/Types.h") // a type name reaches a header through the Objective-C header the .m imports
	for _, g := range got {
		if strings.HasPrefix(g, "lab/solve.m") {
			t.Errorf("a MATLAB .m drew a file edge: %q", g)
		}
	}
}
