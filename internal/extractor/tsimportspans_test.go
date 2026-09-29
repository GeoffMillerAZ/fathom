package extractor

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestExtractImportSpans (LB-F40): each import spec comes with the line span
// of the statement that binds it, so the harness can pair an oracle's import
// references (a SCIP occurrence on a name inside a multi-line import, a
// dump's statement line) with the spec. The specs themselves are exactly
// ExtractImports' — same order, same fields.
func TestExtractImportSpans(t *testing.T) {
	for _, tc := range []struct {
		lang, path, src string
		want            string // "start-end:local<-module" per spec, space-separated
	}{
		{
			lang: "javascript", path: "src/app.ts",
			src: "import {\n" + // 1
				"  a,\n" + // 2
				"  b as c,\n" + // 3
				"} from './x';\n" + // 4
				"import d from './y';\n" + // 5
				"const e = require('./z');\n" + // 6
				"export function f() { return a(); }\n", // 7
			want: "1-4:a<-./x 1-4:c<-./x 5-5:d<-./y 6-6:e<-./z",
		},
		{
			lang: "rust", path: "src/main.rs",
			src: "use crate::a::{\n" + // 1
				"    b,\n" + // 2
				"    c,\n" + // 3
				"};\n" + // 4
				"mod m;\n" + // 5
				"fn main() { b(); }\n", // 6
			want: "1-4:b<-crate::a 1-4:c<-crate::a 5-5:m<-m",
		},
		{
			lang: "python", path: "pkg/mod.py",
			src: "from .sibling import (\n" + // 1
				"    one,\n" + // 2
				"    two,\n" + // 3
				")\n" + // 4
				"import os\n", // 5
			want: "1-4:one<-.sibling 1-4:two<-.sibling 5-5:os<-os",
		},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			e, err := NewTSExtractor(tc.lang)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			spans, err := e.ExtractImportSpans(tc.path, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			specs, err := e.ExtractImports(tc.path, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]ImportSpec, len(spans))
			var parts []string
			for i, s := range spans {
				got[i] = s.ImportSpec
				if s.StartLine > s.Line || s.EndLine < s.Line {
					t.Errorf("span %d-%d does not hold the spec's line %d", s.StartLine, s.EndLine, s.Line)
				}
				parts = append(parts, fmt.Sprintf("%d-%d:%s<-%s", s.StartLine, s.EndLine, s.LocalName, s.Module))
			}
			if !reflect.DeepEqual(got, specs) {
				t.Errorf("specs differ from ExtractImports:\n got %v\nwant %v", got, specs)
			}
			if g := strings.Join(parts, " "); g != tc.want {
				t.Errorf("spans\n got %s\nwant %s", g, tc.want)
			}
		})
	}
}
