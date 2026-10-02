package fusion_test

// LB-Q-kotlin (design §13.2 step 13): the kotlin dialect's imports.scm parses
// real import statements and resolves them through its manifest's namespace-dir
// strategy. Unlike Java, Kotlin has top-level classes and functions, so the
// resolver's bare-name matching has targets: a named import of a class file
// (`import com.example.util.Helper` -> com/example/util/Helper.kt) resolves the
// constructor call Helper(), and a wildcard (`import com.example.util.*`) brings
// in every top-level name of the files that declare that package. A named import
// of a top-level function or property (`import com.example.util.helperTop`)
// would need the package/member split namespace-dir does not make; the wildcard
// case is what this fixture proves resolves.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_KotlinImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "ktfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "kotlin", []string{
		filepath.Join("com", "example", "util", "Helper.kt"),
		filepath.Join("com", "example", "util", "Strings.kt"),
		filepath.Join("com", "example", "app", "Main.kt"),
	})
	edges, report := fusion.ResolveFileEdges(context.Background(), "kotlin", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 3 || report.ImportsParsed != 2 {
		t.Errorf("FilesScanned = %d ImportsParsed = %d, want 3 and 2", report.FilesScanned, report.ImportsParsed)
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: kind %q, want file-resolved", e, e.Kind)
		}
	}
	got := fileResolveEdgeSetAll(edges)
	from := "fixture::com/example/app/com.example.app.Main.run"
	want := []string{
		from + " -> fixture::com/example/util/com.example.util.Helper",
		from + " -> fixture::com/example/util/com.example.util.trimAll",
		from + " -> fixture::com/example/util/com.example.util.helperTop",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d", len(got), got, len(want))
	}
}

// TestResolveFileEdges_KotlinTopLevelImport is LB-F78 (the namespace-dir
// package/member split): `import com.acme.util.trimAll` names a top-level
// function, not a file, so its module path (com/acme/util/trimAll.kt) names
// nothing; the resolver binds it in the files of the package its path less
// the last segment names (com.acme.util), to the file that declares it — as
// it does `DEFAULT_NAME` (a top-level property, referenced by value) and
// `Text` (a class declared in Model.kt, a file of another name).
func TestResolveFileEdges_KotlinTopLevelImport(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "ktsplit"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "kotlin", fixtureFiles(t, dir, ".kt"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "kotlin", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := namedEdges(edges, symbols)
	want := []string{
		"src/main/kotlin/com/acme/app/Main.kt|run -> src/main/kotlin/com/acme/util/Model.kt|Text",
		"src/main/kotlin/com/acme/app/Main.kt|run -> src/main/kotlin/com/acme/util/Strings.kt|DEFAULT_NAME",
		"src/main/kotlin/com/acme/app/Main.kt|run -> src/main/kotlin/com/acme/util/Strings.kt|trimAll",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
