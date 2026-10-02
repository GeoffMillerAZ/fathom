package fusion

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestConanClassification (LB-Q-conan, design §5): the dialect claims the four
// Conan files by exact basename at any depth, and conanfile.py is claimed by it
// ahead of the python dialect's .py; every other .py, .txt and .yml is not
// Conan's.
func TestConanClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"conanfile.py", "conan", connector.ClassifiedByBasename},
		{"recipes/zlib/all/conanfile.py", "conan", connector.ClassifiedByBasename},
		{"recipes/zlib/all/test_package/conanfile.py", "conan", connector.ClassifiedByBasename},
		{"conanfile.txt", "conan", connector.ClassifiedByBasename},
		{"app/conan.lock", "conan", connector.ClassifiedByBasename},
		{"recipes/zlib/all/conandata.yml", "conan", connector.ClassifiedByBasename},
		{"conanfile.pyc", "", connector.ClassifiedNone},
		{"myconanfile.py", "python", connector.ClassifiedByExtension},
		{"setup.py", "python", connector.ClassifiedByExtension},
		{"requirements.txt", "", connector.ClassifiedNone},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

func conanCorpus() map[string]string {
	return map[string]string{
		"recipes/zlib/all/conanfile.py": `from conan import ConanFile


class ZlibConan(ConanFile):
    name = "zlib"
    requires = "base/1.0"

    def requirements(self):
        self.requires("fmt/10.2.1")
`,
		"recipes/zlib/all/test_package/conanfile.py": `from conan import ConanFile


class TestPackageConan(ConanFile):
    def requirements(self):
        self.requires(self.tested_reference_str)
`,
		"recipes/zlib/all/conandata.yml": "sources:\n  \"1.3.1\":\n    url: u\n",
		"recipes/zlib/all/helper.py":     "def helper():\n    return 1\n",
		"app/conanfile.txt":              "[requires]\nzlib/1.3.1\n[generators]\nCMakeDeps\n",
		"app/conan.lock":                 "{\"requires\": [\"zlib/1.3.1#r%1\"]}\n",
		"app/requirements.txt":           "[requires]\nnot-conan\n",
	}
}

// TestIngest_Conan: an ingest reads every Conan file through the conan dialect
// (conanfile.py as a recipe on the python grammar, the other three as
// documents) and tags it "conan", leaves every other .py with the python
// dialect, and reads no unrelated .txt.
func TestIngest_Conan(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, conanCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cn", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cn", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
	}
	want := map[string]string{
		"recipes/zlib/all/conanfile.py|zlib":                   "ConanRecipe",
		"recipes/zlib/all/conandata.yml|sources.1.3.1":         "ConanSource",
		"app/conanfile.txt|conanfile.txt":                      "ConanConsumer",
		"app/conanfile.txt|CMakeDeps":                          "ConanGenerator",
		"app/conan.lock|conan.lock":                            "ConanLock",
		"recipes/zlib/all/helper.py|helper":                    "PythonFunc",
		"recipes/zlib/all/test_package/conanfile.py|(nothing)": "",
	}
	delete(want, "recipes/zlib/all/test_package/conanfile.py|(nothing)")
	for k, kind := range want {
		if got[k] != kind {
			t.Errorf("%s: kind %q, want %q (all: %v)", k, got[k], kind, got)
		}
	}
	for k := range got {
		if filepath.Base(filepath.Dir(k)) == "test_package" || len(k) >= len("app/requirements.txt") && k[:len("app/requirements.txt")] == "app/requirements.txt" {
			t.Errorf("%s: an observation where the dialect has no entity", k)
		}
	}
	for _, o := range obs {
		if o.SourceRef == "recipes/zlib/all/conanfile.py" && o.Symbol == "zlib" {
			calls := append([]string(nil), o.Calls...)
			sort.Strings(calls)
			if len(calls) != 2 || calls[0] != "base" || calls[1] != "fmt" {
				t.Errorf("zlib calls %v, want [base fmt]", calls)
			}
		}
	}
}
