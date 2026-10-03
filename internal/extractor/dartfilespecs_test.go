package extractor

import (
	"strings"
	"testing"
)

// TestDartFileSpecsParts (LB-F246): a library's `part 'x.dart';` makes x.dart's
// declarations the library's own, so the library re-exports every name of its
// part, and a part's `part of 'lib.dart';` brings the whole library into scope
// in the part (a part file has no imports of its own). The library itself reads
// its parts' names unqualified, so a part is also a wildcard import of the
// library that names it. A conditional part uri
// reads its default, the second uri the grammar writes after an ERROR node.
func TestDartFileSpecsParts(t *testing.T) {
	cases := []struct {
		name, path, src string
		exports         []string
		imports         []string
		refs            []string
	}{
		{name: "library with parts", path: "lib/src/bloc.dart", src: `import 'package:meta/meta.dart';

part 'bloc_base.dart';
part 'emitter.dart';

class Bloc extends BlocBase {}
`, exports: []string{"bloc_base.dart@3", "emitter.dart@4"}, imports: []string{"package:meta/meta.dart", "bloc_base.dart", "emitter.dart"}, refs: []string{"BlocBase@6"}},
		{name: "part of a library", path: "lib/src/emitter.dart", src: `part of 'bloc.dart';

class Emitter {
  Bloc? owner;
}
`, imports: []string{"bloc.dart"}, refs: []string{"Bloc@4"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := NewTSExtractor("dart")
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			fs, err := e.ExtractFileSpecs(c.path, []byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := strings.Join(exportStrings(fs.Exports), " "), strings.Join(c.exports, " "); got != want {
				t.Errorf("exports\n got %s\nwant %s", got, want)
			}
			var imps []string
			for _, im := range fs.Imports {
				imps = append(imps, strings.Trim(im.Module, `'"`))
			}
			if got, want := strings.Join(imps, " "), strings.Join(c.imports, " "); got != want {
				t.Errorf("imports\n got %s\nwant %s", got, want)
			}
			if got, want := strings.Join(refStrings(fs.Refs), " "), strings.Join(c.refs, " "); got != want {
				t.Errorf("refs\n got %s\nwant %s", got, want)
			}
		})
	}
}
