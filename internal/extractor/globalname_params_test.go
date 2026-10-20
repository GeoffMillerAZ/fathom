package extractor

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestDialectGlobalNameParamsStrict (LB-F580): global-name's scope takes
// "service" or "folder", defines names kinds of the manifest, and no other
// strategy takes either, as the CUE schema says; a valid manifest's defines
// reach FileResolveLang as source kinds.
func TestDialectGlobalNameParamsStrict(t *testing.T) {
	base := `{"id":"a","display":"X","kind":"tooling","engine":"tags","lang_tag":"x",
"match":{"extensions":[".a"]},"grammar":{"blob":"x","entry_point":"tree_sitter_x"},
"queries":{"tags":"tags.scm","imports":"imports.scm"},"kinds":{"t":{"source_kind":"AT","canonical":"type_alias"},"e":{"source_kind":"AE","canonical":"type_alias"}},
"required_kinds":["t"],"target_level":"L3","queue_id":"a"}`
	files := func(manifest string) fstest.MapFS {
		return fstest.MapFS{"dialects/a/dialect.json": {Data: []byte(manifest)}, "dialects/a/tags.scm": {Data: nil}, "dialects/a/imports.scm": {Data: nil}}
	}
	with := func(s string) string { return strings.Replace(base, `"target_level"`, s+`,"target_level"`, 1) }
	for name, c := range map[string]struct{ manifest, want string }{
		"scope off list":       {with(`"imports":{"strategy":"global-name","scope":"repo"}`), "imports.scope"},
		"defines no kind":      {with(`"imports":{"strategy":"global-name","defines":["class"]}`), "imports.defines"},
		"scope off global":     {with(`"imports":{"strategy":"file-symbol","scope":"folder"}`), "global-name's parameters"},
		"defines off global":   {with(`"imports":{"strategy":"relative-path","defines":["t"]}`), "global-name's parameters"},
		"strategy misspelt":    {with(`"imports":{"strategy":"global-names"}`), "closed set"},
		"defines not a list":   {with(`"imports":{"strategy":"global-name","defines":"t"}`), "defines"},
		"unknown global param": {with(`"imports":{"strategy":"global-name","scopes":"folder"}`), "scopes"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadDialects(files(c.manifest))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
	ds, err := loadDialects(files(with(`"imports":{"strategy":"global-name","scope":"folder","defines":["t"]}`)))
	if err != nil || len(ds) != 1 || ds[0].Imports.Params.Scope != "folder" || len(ds[0].Imports.Params.Defines) != 1 {
		t.Fatalf("a valid manifest: %v %+v", err, ds)
	}

	// The shipped manifests: GraphQL binds across the service to a type (never
	// an extension), Prisma within a folder to a model, view, type or enum.
	langs, err := FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"graphql": "service GraphQLType", "prisma": "folder PrismaEnum PrismaModel PrismaType PrismaView"}
	for _, l := range langs {
		if w, ok := want[l.Lang]; ok {
			if got := l.Params.Scope + " " + strings.Join(l.Defines, " "); l.Strategy != "global-name" || got != w {
				t.Errorf("%s: %s %q, want global-name %q", l.Lang, l.Strategy, got, w)
			}
			delete(want, l.Lang)
		}
	}
	if len(want) > 0 {
		t.Errorf("no file-resolve language for %v", want)
	}
}
