package extractor

import (
	"reflect"
	"testing"
)

// TestExtractWithFileSpecsOneParse (EW-52, O361): ExtractWithFileSpecs returns
// exactly what Extract and ExtractFileSpecs return for the same bytes, from
// one parse where the two calls cost two.
func TestExtractWithFileSpecsOneParse(t *testing.T) {
	cases := []struct{ dialect, path, src string }{
		{"c", "a/util.c", "#include <stdio.h>\n#include \"util.h\"\nstruct Thing { int x; };\nstatic int helper(struct Thing *t) { return t->x; }\nint run(void) { struct Thing t; return helper(&t); }\n"},
		{"javascript", "app.ts", "import { User, helper } from './models';\nexport * from './a';\nexport function make(local: number): User {\n  const x = new User();\n  return [helper, x, local, Other];\n}\n"},
		{"python", "pkg/m.py", "import os\nfrom .models import User\n\nclass Host(User):\n    def method(self):\n        return os.path.join('a')\n"},
		{"rust", "src/lib.rs", "use std::collections::HashMap;\nfn make(n: u32) -> Option<Helper> { let x = User::new(); None }\n"},
	}
	for _, c := range cases {
		t.Run(c.dialect, func(t *testing.T) {
			two, err := NewTSExtractor(c.dialect)
			if err != nil {
				t.Skipf("no %s tags dialect: %v", c.dialect, err)
			}
			defer two.Close()
			one, err := NewTSExtractor(c.dialect)
			if err != nil {
				t.Fatal(err)
			}
			defer one.Close()
			src := []byte(c.src)

			wantNodes, err := two.Extract("svc", c.path, src)
			if err != nil {
				t.Fatal(err)
			}
			wantSpecs, err := two.ExtractFileSpecs(c.path, src)
			if err != nil {
				t.Fatal(err)
			}
			if got := two.Parses(); got != 2 {
				t.Fatalf("Extract + ExtractFileSpecs ran %d parses, want 2 (the counter seam is wrong)", got)
			}

			gotNodes, gotSpecs, ok, err := one.ExtractWithFileSpecs("svc", c.path, src)
			if err != nil || !ok {
				t.Fatalf("ExtractWithFileSpecs ok=%v err=%v", ok, err)
			}
			if got := one.Parses(); got != 1 {
				t.Errorf("ExtractWithFileSpecs ran %d parses, want 1", got)
			}
			if !reflect.DeepEqual(gotNodes, wantNodes) {
				t.Errorf("nodes differ\n got %+v\nwant %+v", gotNodes, wantNodes)
			}
			if !reflect.DeepEqual(gotSpecs, wantSpecs) {
				t.Errorf("specs differ\n got %+v\nwant %+v", gotSpecs, wantSpecs)
			}
		})
	}
}
