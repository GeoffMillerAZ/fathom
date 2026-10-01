package fusion_test

// LB-F74 (F-D reference edges, docs/design/language-breadth.md §6): a name a
// file references outside a call's own text — a type annotation or hint,
// `new X`, extends/implements, a bare `X` or a qualified `Y.Z` — becomes a
// file-resolved edge from the definition enclosing it to the definition the
// name binds, through a named, whole or wildcard import (and a re-export it
// reaches, LB-F75), or through the language's own package scope (Java,
// Kotlin, C#: every enclosing namespace too, PHP). One fixture per language
// under testdata/reffixture; each subtest asserts the exact edge set, so a
// local, a parameter, a same-file name or an unbound name drawing an edge
// fails as surely as a missing one.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// fixtureFiles is every file under dir with one of exts, repo-relative,
// sorted.
func fixtureFiles(t *testing.T, dir string, exts ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		for _, e := range exts {
			if strings.HasSuffix(p, e) {
				rel, _ := filepath.Rel(dir, p)
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// namedEdges renders edges as "file|name -> file|name".
func namedEdges(edges []core.Edge, symbols []core.SymbolNode) []string {
	at := map[string]string{}
	for _, s := range symbols {
		at[s.Key()] = s.FilePath + "|" + s.Name
	}
	var out []string
	for _, e := range edges {
		out = append(out, at[e.FromSymbol]+" -> "+at[e.ToSymbol])
	}
	sort.Strings(out)
	return out
}

func TestResolveFileEdges_ImportBoundReferences(t *testing.T) {
	cases := []struct {
		name, lang string
		exts       []string
		want       []string
	}{
		{"python", "python", []string{".py"}, []string{
			// a hint through a package's re-export (pkg/__init__.py's
			// `from .models import User`, LB-F75) and through a named import
			"app.py|check -> pkg/models.py|User",
			"app.py|load -> pkg/models.py|User",
			"app.py|load -> pkg/types.py|Config",
			// pkg.models.Admin through `import pkg.models`
			"app.py|admins -> pkg/models.py|Admin",
		}},
		{"typescript", "javascript", []string{".ts"}, []string{
			// User and Config through index.ts's `export * from './models'`
			"app.ts|make -> models.ts|Config",
			"app.ts|make -> models.ts|User",
			// `m.Id` through `import * as m`; a field's type belongs to its class
			"app.ts|Service -> models.ts|Id",
			// a value reference through `export { helper as assist }`
			"app.ts|Service.run -> util.ts|helper",
		}},
		{"javascript", "javascript", []string{".js"}, []string{
			// extends through lib/index.js's `export { Base } from`
			"app.js|Widget -> lib/base.js|Base",
			// util.format through a CommonJS require
			"app.js|Widget.render -> util.js|format",
		}},
		{"java", "java", []string{".java"}, []string{
			// a field's type through `import com.acme.model.User;` (the
			// java dialect names a field by its own name)
			"com/acme/app/Service.java|owner -> com/acme/model/User.java|User",
			// `Helper.make()`: Helper is in Service's own package
			"com/acme/app/Service.java|Service.helper -> com/acme/app/Helper.java|Helper.make",
			// a call through `import static com.acme.model.Role.of;` (LB-F78)
			"com/acme/app/Service.java|Service.role -> com/acme/model/Role.java|Role.of",
		}},
		{"csharp", "csharp", []string{".cs"}, []string{
			// Base is declared in Acme, a namespace enclosing Acme.App
			"App/Service.cs|Service -> Core/Base.cs|Base",
			// a field type through `using Acme.Models;`
			"App/Service.cs|Service -> Models/User.cs|User",
			// a return type in Service's own namespace
			"App/Service.cs|Service.Make -> App/Helper.cs|Helper",
		}},
		{"kotlin", "kotlin", []string{".kt"}, []string{
			"com/acme/app/Service.kt|Service.helper -> com/acme/app/Helper.kt|Helper",
			"com/acme/app/Service.kt|Service.owner -> com/acme/model/User.kt|User",
		}},
		{"dart", "dart", []string{".dart"}, []string{
			// return types and a type argument through an unprefixed import
			"lib/app.dart|Repo.all -> lib/src/user.dart|User",
			"lib/app.dart|Repo.find -> lib/src/user.dart|User",
		}},
		{"php", "php", []string{".php"}, []string{
			// `Helper::assist()` with no use: the class of Service's own namespace
			"src/App/Service.php|Service.run -> src/App/Helper.php|Helper.assist",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", "reffixture", map[string]string{
				"python": "py", "typescript": "ts", "javascript": "js", "java": "java", "csharp": "cs",
				"kotlin": "kt", "dart": "dart", "php": "php",
			}[c.name]))
			if err != nil {
				t.Fatal(err)
			}
			files, symbols := loadFileResolveFixture(t, dir, c.lang, fixtureFiles(t, dir, c.exts...))
			edges, report := fusion.ResolveFileEdges(context.Background(), c.lang, dir, files, symbols)
			if report.Skipped {
				t.Fatalf("skipped: %s", report.SkipReason)
			}
			for _, e := range edges {
				if e.Kind != core.EdgeFileResolved || core.EdgeConfidenceOf(e) != core.EdgeConfidenceHeuristic {
					t.Errorf("edge %+v: kind %q confidence %q, want file-resolved and heuristic", e, e.Kind, core.EdgeConfidenceOf(e))
				}
			}
			got := namedEdges(edges, symbols)
			want := append([]string(nil), c.want...)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
			}
		})
	}
}

// TestFileResolvedReferenceEdgeIsAReach is LB-F74's edge-kind decision: a
// reference edge keeps core.EdgeFileResolved (heuristic, never typed) and
// the call graph walks it like a call edge, so callers, blast_radius and
// verify's test_reaches see the definition that only references a type:
// a test that calls make reaches User through make's annotation.
func TestFileResolvedReferenceEdgeIsAReach(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "reffixture", "ts"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "javascript", fixtureFiles(t, dir, ".ts"))
	edges, _ := fusion.ResolveFileEdges(context.Background(), "javascript", dir, files, symbols)
	var make_, user string
	for _, s := range symbols {
		switch {
		case s.FilePath == "app.ts" && s.Name == "make":
			make_ = s.Key()
		case s.FilePath == "models.ts" && s.Name == "User":
			user = s.Key()
		}
	}
	test := core.SymbolNode{Service: "fixture", Package: "app_test", Name: "TestMake", Kind: core.SourceKind("JSFunc"), FilePath: "app_test.ts", Line: 1, EndLine: 3, Calls: []string{"make"}}
	g := callgraph.Build(append(append([]core.SymbolNode(nil), symbols...), test))
	callgraph.AddTypedEdges(g, edges)
	var found bool
	for _, c := range callgraph.CallersDetail(g, user) {
		if c.Key == make_ {
			found = true
			if c.Kind != core.EdgeFileResolved {
				t.Errorf("make -> User kind %q, want file-resolved", c.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("User's callers %v lack make: a reference edge is not walked", callgraph.CallersDetail(g, user))
	}
	if !callgraph.Reachable(g, test.Key(), user) {
		t.Error("TestMake does not reach User through make's reference")
	}
	if got := callgraph.TestImpact(g, user); len(got) != 1 || got[0] != test.Key() {
		t.Errorf("TestImpact(User) = %v, want [%s]", got, test.Key())
	}
}
