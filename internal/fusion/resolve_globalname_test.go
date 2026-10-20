package fusion

// resolve_globalname_test.go — the global-name strategy (LB-F580, design §6):
// a schema that is the union of its files (GraphQL's, Prisma's) binds a type
// reference to the one file of the scope defining the name.

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// globalNameDef is one definition of the test universe: its file, name,
// source kind, and the type names it references (its calls).
type globalNameDef struct {
	file, name, kind string
	refs             []string
}

func globalNameUniverse(defs []globalNameDef) ([]string, []core.SymbolNode, map[string]extractor.FileSpecs) {
	var paths []string
	var syms []core.SymbolNode
	specs := map[string]extractor.FileSpecs{}
	for i, d := range defs {
		if !slices.Contains(paths, d.file) {
			paths = append(paths, d.file)
		}
		syms = append(syms, core.SymbolNode{Service: "svc", Package: relDir(d.file), Name: d.name, Kind: core.SourceKind(d.kind), FilePath: d.file, Line: i + 1, EndLine: i + 1, Calls: d.refs})
		fs := specs[d.file]
		for _, r := range d.refs {
			fs.Imports = append(fs.Imports, extractor.ImportSpec{LocalName: r, ExportedName: r, Module: r, Kind: extractor.ImportNamed, Line: i + 1})
		}
		specs[d.file] = fs
	}
	sort.Strings(paths)
	return paths, syms, specs
}

// TestResolveGlobalNameBindsUniqueDefinitionsOnly: a referenced name binds the
// one file of its scope that defines it with a defining kind; a name two
// files define binds nothing and is counted ambiguous; a built-in, an
// undefined name and a name the importer defines bind nothing and are no
// miss; an extension is no definition; the folder scope stops at the
// importer's directory; the edges come out sorted.
func TestResolveGlobalNameBindsUniqueDefinitionsOnly(t *testing.T) {
	gql := map[string]bool{"GraphQLType": true}
	paths, syms, specs := globalNameUniverse([]globalNameDef{
		{file: "schema/user.graphql", name: "User", kind: "GraphQLType"},
		{file: "schema/user.graphql", name: "User.posts", kind: "GraphQLField", refs: []string{"Post", "String"}},
		{file: "schema/post.graphql", name: "Post", kind: "GraphQLType"},
		{file: "schema/post.graphql", name: "Post.author", kind: "GraphQLField", refs: []string{"User"}},
		{file: "schema/post.graphql", name: "Post.next", kind: "GraphQLField", refs: []string{"Post"}},
		{file: "schema/query.graphql", name: "Query", kind: "GraphQLType"},
		{file: "schema/query.graphql", name: "Query.me", kind: "GraphQLField", refs: []string{"User"}},
		{file: "schema/ext.graphql", name: "Query", kind: "GraphQLExtension"},
		{file: "schema/ext.graphql", name: "Query.shared", kind: "GraphQLField", refs: []string{"Shared"}},
		{file: "a/one.graphql", name: "Shared", kind: "GraphQLType"},
		{file: "b/two.graphql", name: "Shared", kind: "GraphQLType"},
		{file: "ops/q.graphql", name: "GetMe", kind: "GraphQLOperation", refs: []string{"Query", "Missing", "Shared", "User"}},
	})
	env := newResolveEnv("", paths, nil)
	env.indexGlobalNames(syms, gql)
	svc := extractor.ImportParams{Scope: "service"}
	for _, c := range []struct {
		importer, name string
		want           []string
	}{
		{"schema/user.graphql", "Post", []string{"schema/post.graphql"}},
		{"schema/post.graphql", "User", []string{"schema/user.graphql"}},
		{"ops/q.graphql", "Query", []string{"schema/query.graphql"}}, // the extension is no second definition
		{"schema/post.graphql", "Post", nil},                         // the importer's own name: no self edge
		{"ops/q.graphql", "Shared", nil},                             // two definitions: ambiguous
		{"ops/q.graphql", "String", nil},                             // a built-in scalar: no definition
		{"ops/q.graphql", "Missing", nil},
	} {
		if got := resolveByStrategy("global-name", svc, env, c.importer, c.name); !reflect.DeepEqual(got, c.want) {
			t.Errorf("service scope: %s from %s = %q, want %q", c.name, c.importer, got, c.want)
		}
	}

	r := &importResolver{strategy: "global-name", params: svc, defines: gql, env: newResolveEnv("", paths, nil)}
	edges, rep := resolveImports(r, paths, specs, syms)
	var got []string
	for _, e := range edges {
		got = append(got, e.FromFile+" "+e.FromSymbol+" -> "+e.ToSymbol)
	}
	want := []string{
		"ops/q.graphql svc::ops/ops.GetMe -> svc::schema/schema.Query",
		"ops/q.graphql svc::ops/ops.GetMe -> svc::schema/schema.User",
		"schema/post.graphql svc::schema/schema.Post.author -> svc::schema/schema.User",
		"schema/post.graphql svc::schema/schema.Post.next -> svc::schema/schema.Post",
		"schema/query.graphql svc::schema/schema.Query.me -> svc::schema/schema.User",
		"schema/user.graphql svc::schema/schema.User.posts -> svc::schema/schema.Post",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edges:\n%s\nwant (sorted):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !sort.SliceIsSorted(edges, func(i, j int) bool { return edgeLess(edges[i], edges[j]) }) {
		t.Errorf("edges are not sorted")
	}
	// Bound: (post, User), (q, Query), (q, User), (query, User), (user, Post);
	// ambiguous: (ext, Shared), (q, Shared). Built-ins, the missing name and
	// Post.next's own Post are neither.
	if rep.GlobalNameBound != 5 || rep.GlobalNameAmbiguous != 2 {
		t.Errorf("report bound=%d ambiguous=%d, want 5 and 2", rep.GlobalNameBound, rep.GlobalNameAmbiguous)
	}

	// Folder scope (a Prisma schema folder): a name binds within the
	// importer's directory only, so a second schema elsewhere is neither a
	// target nor an ambiguity, and a name only another folder defines binds
	// nothing.
	prisma := map[string]bool{"PrismaModel": true, "PrismaEnum": true}
	paths, syms, _ = globalNameUniverse([]globalNameDef{
		{file: "prisma/schema/user.prisma", name: "User", kind: "PrismaModel"},
		{file: "prisma/schema/user.prisma", name: "Role", kind: "PrismaEnum"},
		{file: "prisma/schema/post.prisma", name: "Post", kind: "PrismaModel"},
		{file: "prisma/schema/post.prisma", name: "Post.author", kind: "PrismaField", refs: []string{"User"}},
		{file: "prisma/schema/sub/tag.prisma", name: "Tag", kind: "PrismaModel"},
		{file: "other/schema.prisma", name: "User", kind: "PrismaModel"},
		{file: "other/schema.prisma", name: "Account", kind: "PrismaModel"},
	})
	env = newResolveEnv("", paths, nil)
	env.indexGlobalNames(syms, prisma)
	folder := extractor.ImportParams{Scope: "folder"}
	for _, c := range []struct {
		importer, name string
		params         extractor.ImportParams
		want           []string
	}{
		{"prisma/schema/post.prisma", "User", folder, []string{"prisma/schema/user.prisma"}},
		{"prisma/schema/post.prisma", "Role", folder, []string{"prisma/schema/user.prisma"}},
		{"prisma/schema/post.prisma", "Tag", folder, nil},     // a subdirectory is another folder
		{"prisma/schema/post.prisma", "Account", folder, nil}, // another schema's model
		{"other/schema.prisma", "Post", folder, nil},
		{"prisma/schema/post.prisma", "User", svc, nil}, // the service scope sees two definitions
	} {
		if got := resolveByStrategy("global-name", c.params, env, c.importer, c.name); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s scope: %s from %s = %q, want %q", c.params.Scope, c.name, c.importer, got, c.want)
		}
	}
	if b, a := env.globalNameCounts(); b != 2 || a != 1 {
		t.Errorf("folder pass bound=%d ambiguous=%d, want 2 and 1", b, a)
	}
}

// edgeLess is sortEdges' order.
func edgeLess(a, b core.Edge) bool {
	if a.FromSymbol != b.FromSymbol {
		return a.FromSymbol < b.FromSymbol
	}
	return a.ToSymbol < b.ToSymbol
}
