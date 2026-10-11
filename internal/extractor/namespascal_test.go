package extractor

import "testing"

// TestNamesPascalCase (wave lf7, LB-Q-vue): names.pascal_case camelizes and
// capitalizes a name as Vue's resolveComponent resolves a component tag
// (capitalize(camelize(tag))): a hyphen followed by a word character drops and
// upper-cases it, then the first letter is upper-cased; a name already in
// PascalCase or dotted keeps its spelling.
func TestNamesPascalCase(t *testing.T) {
	f := (&tsDialectRules{pascalCase: true}).nameNormalizer()
	if f == nil {
		t.Fatal("pascal_case must install a normalizer")
	}
	for in, want := range map[string]string{
		"el-input":        "ElInput",
		"keep-alive":      "KeepAlive",
		"transition":      "Transition",
		"ElButton":        "ElButton",
		"Foo.Bar":         "Foo.Bar",
		"router-link":     "RouterLink",
		"my-comp-2":       "MyComp2",
		"x--y":            "X-Y",
		"a-":              "A-",
		"v-p-icon-search": "VPIconSearch",
	} {
		if got := f(in); got != want {
			t.Errorf("pascal(%q) = %q, want %q", in, got, want)
		}
	}
	if (&tsDialectRules{}).nameNormalizer() != nil {
		t.Error("a dialect without names keeps its names as captured")
	}
}
