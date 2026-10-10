package extractor

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestModuleSymbolRefusedAtLoad (LB-F346, K18): the manifest's module_symbol
// is checked at load as the schema checks it — it names a kind of the
// manifest, canonical "module", neither a container nor a callable, outside
// oracle_kinds, on a tags engine whose package is the file's own name.
func TestModuleSymbolRefusedAtLoad(t *testing.T) {
	manifest := func(engine, kinds, extra string) string {
		grammar := ""
		if engine == "tags" {
			grammar = `"grammar":{"blob":"x","entry_point":"tree_sitter_x"},"queries":{"tags":"tags.scm"},`
		}
		return `{"id":"a","display":"A","kind":"language","engine":"` + engine + `","lang_tag":"a",
"match":{"extensions":[".a"]},` + grammar + `"kinds":` + kinds + `,
"required_kinds":["f"],"target_level":"L1","queue_id":"a"` + extra + `}`
	}
	const kinds = `{"f":{"source_kind":"AF","canonical":"function","callable":true},"module":{"source_kind":"AModule","canonical":"module"}}`
	load := func(m string) error {
		_, err := loadDialects(fstest.MapFS{
			"dialects/a/dialect.json": &fstest.MapFile{Data: []byte(m)},
			"dialects/a/tags.scm":     &fstest.MapFile{Data: nil},
		})
		return err
	}
	if err := load(manifest("tags", kinds, `,"module_symbol":"module"`)); err != nil {
		t.Fatalf("a tags dialect naming its module kind must load: %v", err)
	}
	cases := map[string]struct{ manifest, want string }{
		"names no kind":       {manifest("tags", kinds, `,"module_symbol":"mod"`), `module_symbol "mod" is not a kind`},
		"not canonical":       {manifest("tags", kinds, `,"module_symbol":"f"`), `canonical "module"`},
		"a container":         {manifest("tags", strings.Replace(kinds, `"canonical":"module"}`, `"canonical":"module","container":true}`, 1), `,"module_symbol":"module"`), "neither a container nor a callable"},
		"a callable":          {manifest("tags", strings.Replace(kinds, `"canonical":"module"}`, `"canonical":"module","callable":true}`, 1), `,"module_symbol":"module"`), "neither a container nor a callable"},
		"native engine":       {manifest("native", kinds, `,"module_symbol":"module"`), "tags engine"},
		"inside oracle_kinds": {manifest("tags", kinds, `,"oracle_kinds":{"ctags":{"m":"module"}},"module_symbol":"module"`), "outside oracle_kinds"},
		"package by capture":  {manifest("tags", kinds, `,"package":{"strategy":"capture"},"module_symbol":"module"`), `package strategy "dir"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := load(c.manifest); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}
