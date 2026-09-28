package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestCanonicalKindClosedList (LB-F24, §2.3): the closed list is the CUE
// schema's #Canonical exactly, every manifest kind maps into it, and
// CanonicalKind answers every declared SourceKind from the registry —
// one answer per SourceKind, even when two dialects share it.
func TestCanonicalKindClosedList(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "dialect", "dialect.cue"))
	if err != nil {
		t.Fatal(err)
	}
	var fromCUE []string
	for _, def := range []string{"#LanguageCanonical", "#ToolCanonical"} {
		i := strings.Index(string(raw), def+":")
		if i < 0 {
			t.Fatalf("%s not in dialect.cue", def)
		}
		rest := string(raw)[i+len(def)+1:]
		end := strings.Index(rest, "\n\n")
		if end < 0 {
			end = len(rest)
		}
		for _, m := range regexp.MustCompile(`"([a-z_]+)"`).FindAllStringSubmatch(rest[:end], -1) {
			fromCUE = append(fromCUE, m[1])
		}
	}
	sort.Strings(fromCUE)
	if got := CanonicalKinds(); strings.Join(got, ",") != strings.Join(fromCUE, ",") {
		t.Fatalf("CanonicalKinds() = %v\nschema #Canonical = %v", got, fromCUE)
	}

	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, d := range ds {
		for key, k := range d.Kinds {
			if !isCanonicalKind(k.Canonical) {
				t.Errorf("%s.%s: canonical %q not in the closed list", d.ID, key, k.Canonical)
			}
			if prev, ok := seen[k.SourceKind]; ok && prev != k.Canonical {
				t.Errorf("SourceKind %s maps to both %s and %s", k.SourceKind, prev, k.Canonical)
			}
			seen[k.SourceKind] = k.Canonical
			got, ok := CanonicalKind(k.SourceKind)
			if !ok || got != k.Canonical {
				t.Errorf("CanonicalKind(%s) = %q,%v; want %q", k.SourceKind, got, ok, k.Canonical)
			}
		}
	}
	for sk, want := range map[string]string{"JSClass": "class", "TSType": "type_alias", "PythonFunc": "function", "RubyModule": "module", "KotlinImport": "import"} {
		if got, ok := CanonicalKind(sk); !ok || got != want {
			t.Errorf("CanonicalKind(%s) = %q,%v; want %q", sk, got, ok, want)
		}
	}
	if got, ok := CanonicalKind("NoSuchKind"); ok {
		t.Errorf("CanonicalKind(NoSuchKind) = %q, want not ok", got)
	}
}
