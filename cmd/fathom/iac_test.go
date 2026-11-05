package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// iacTestExport is a generated `tofu providers schema -json` export for a made-up provider.
func iacTestExport(t *testing.T, path string, attrs ...string) {
	t.Helper()
	as := map[string]any{}
	for _, a := range attrs {
		as[a] = map[string]any{"type": "string", "optional": true, "description": "x"}
	}
	schema := map[string]any{"version": 0, "block": map[string]any{"attributes": as}}
	b, err := json.Marshal(map[string]any{"format_version": "1.0", "provider_schemas": map[string]any{
		"registry.opentofu.org/example/widget": map[string]any{
			"provider":         schema,
			"resource_schemas": map[string]any{"widget_thing": schema},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runIaC(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := runWithStdin(append([]string{"iac"}, args...), &recordingReader{}, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestIaCVerbImportListAddRemove(t *testing.T) {
	home := helpSandbox(t)
	exp := filepath.Join(home, "export.json")
	iacTestExport(t, exp, "name", "tags")

	rc, out, errs := runIaC("list")
	if rc != 0 || !strings.Contains(out, "no IaC catalogs") {
		t.Fatalf("empty list: rc %d %q %q", rc, out, errs)
	}

	// flags may follow the input, and a single export needs --version.
	if rc, _, errs = runIaC("import", exp); rc == 0 || !strings.Contains(errs, "no version") {
		t.Fatalf("import without a version: rc %d %q", rc, errs)
	}
	rc, out, errs = runIaC("import", exp, "--version", "1.2.0", "--tool", "tofu test", "--licence", "MPL-2.0")
	if rc != 0 || !strings.Contains(out, "imported registry.opentofu.org_example_widget.v1.iaccat.zip") || !strings.Contains(out, "sha256") || errs != "" {
		t.Fatalf("import: rc %d %q %q", rc, out, errs)
	}
	catalog := filepath.Join(home, "iac", "registry.opentofu.org_example_widget.v1.iaccat.zip")
	f, err := iacfacts.ReadIaCCatalog(catalog, iacfacts.IaCCatalogRequest{Version: "1.2.0", Kind: iacfacts.SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "tags", "type"}})
	if err != nil || string(f.Value) != `"string"` {
		t.Fatalf("the imported catalog does not answer: %s %v", f.Value, err)
	}

	rc, out, _ = runIaC("list", "--json", "--digest")
	var listed struct {
		Catalogs []iacCatalogJSON `json:"catalogs"`
	}
	if rc != 0 || json.Unmarshal([]byte(out), &listed) != nil || len(listed.Catalogs) != 1 || listed.Catalogs[0].Line != "1" || listed.Catalogs[0].SHA256 == "" || listed.Catalogs[0].Replacement != "absent" {
		t.Fatalf("list --json: rc %d %q", rc, out)
	}
	rc, out, _ = runIaC("list")
	if rc != 0 || !strings.Contains(out, "line 1: 1 releases 1.2.0 to 1.2.0") {
		t.Fatalf("list: %q", out)
	}

	// add takes a local path only, prints the digest, and refuses a URL.
	other := filepath.Join(home, "elsewhere", "copy.iaccat.zip")
	os.MkdirAll(filepath.Dir(other), 0o755)
	b, _ := os.ReadFile(catalog)
	os.WriteFile(other, b, 0o644)
	if rc, _, errs = runIaC("add", "https://example.test/catalog.zip"); rc == 0 || !strings.Contains(errs, "local file path") {
		t.Fatalf("add of a URL: rc %d %q", rc, errs)
	}
	rc, out, errs = runIaC("add", other)
	if rc != 0 || !strings.Contains(out, listed.Catalogs[0].SHA256) || errs != "" {
		t.Fatalf("add: rc %d %q %q", rc, out, errs)
	}
	if rc, _, _ = runIaC("add"); rc == 0 {
		t.Fatal("add without a path must fail")
	}

	if rc, _, errs = runIaC("remove", "../etc"); rc == 0 {
		t.Fatalf("remove must not leave the directory: %q", errs)
	}
	rc, out, _ = runIaC("remove", "registry.opentofu.org_example_widget.v1")
	if rc != 0 || !strings.Contains(out, "removed") {
		t.Fatalf("remove: rc %d %q", rc, out)
	}
	if _, err := os.Stat(catalog); !os.IsNotExist(err) {
		t.Fatalf("catalog still installed: %v", err)
	}
	if rc, _, _ = runIaC("bogus"); rc == 0 {
		t.Fatal("an unknown iac verb must fail")
	}
}

func TestIaCVerbImportsSeveralVersionsAsOneLine(t *testing.T) {
	home := helpSandbox(t)
	a, b := filepath.Join(home, "a.json"), filepath.Join(home, "b.json")
	iacTestExport(t, a, "name")
	iacTestExport(t, b, "name", "extra")
	rc, out, errs := runIaC("import", "1.0.0="+a, "1.1.0="+b)
	if rc != 0 || !strings.Contains(out, "2 releases, 1.0.0 to 1.1.0") {
		t.Fatalf("rc %d %q %q", rc, out, errs)
	}
	if rc, _, errs = runIaC("import", "2.0.0="+a); rc != 0 {
		t.Fatalf("a different line installs beside it: %q", errs)
	}
	rc, out, _ = runIaC("list")
	if !strings.Contains(out, "widget.v1.iaccat.zip") || !strings.Contains(out, "widget.v2.iaccat.zip") {
		t.Fatalf("two lines are two catalogs: %q", out)
	}
}

func TestIaCVerbImportMergesAndSaysWhatItDid(t *testing.T) {
	home := helpSandbox(t)
	a, b, c := filepath.Join(home, "a.json"), filepath.Join(home, "b.json"), filepath.Join(home, "c.json")
	iacTestExport(t, a, "name")
	iacTestExport(t, b, "name", "extra")
	iacTestExport(t, c, "other")
	if rc, _, errs := runIaC("import", "1.0.0="+a, "1.1.0="+b); rc != 0 {
		t.Fatalf("%q", errs)
	}
	rc, out, errs := runIaC("import", "1.2.0="+c)
	if rc != 0 || !strings.Contains(out, "merged into the catalog already installed") || !strings.Contains(out, "added        1.2.0") || !strings.Contains(out, "kept         1.0.0, 1.1.0") || !strings.Contains(out, "3 releases, 1.0.0 to 1.2.0") {
		t.Fatalf("rc %d %q %q", rc, out, errs)
	}
	// a different export for a release that is installed is refused, then replaced with --replace.
	rc, _, errs = runIaC("import", "1.1.0="+c)
	if rc == 0 || !strings.Contains(errs, "--replace") {
		t.Fatalf("rc %d %q", rc, errs)
	}
	rc, out, errs = runIaC("import", "1.1.0="+c, "--replace", "--json")
	var got struct {
		Merged   bool     `json:"merged"`
		Replaced []string `json:"replaced"`
		Kept     []string `json:"kept"`
		Versions []string `json:"versions"`
	}
	if rc != 0 || json.Unmarshal([]byte(out), &got) != nil || !got.Merged || strings.Join(got.Replaced, ",") != "1.1.0" || strings.Join(got.Kept, ",") != "1.0.0,1.2.0" || strings.Join(got.Versions, ",") != "1.0.0,1.1.0,1.2.0" {
		t.Fatalf("rc %d %q %q", rc, out, errs)
	}
}

// N11: the help says an import takes one provider's export, how to make one, and that --version is not checked.
func TestIaCImportUsageExplainsTheExport(t *testing.T) {
	helpSandbox(t)
	rc, _, errs := runIaC("import")
	if rc == 0 {
		t.Fatal("import with no input must fail")
	}
	for _, want := range []string{"ONE provider", "tofu providers schema -json", "scratch directory", "tofu init", "not checked against the export", "--replace", "merged"} {
		if !strings.Contains(errs, want) {
			t.Errorf("the import help does not say %q:\n%s", want, errs)
		}
	}
}

// N12: `iac import --help` carries the scratch-directory text, not the generic flag list alone.
func TestIaCImportHelpExplainsTheExport(t *testing.T) {
	helpSandbox(t)
	rc, out, errs := runIaC("import", "--help")
	if rc != 0 {
		t.Fatalf("rc %d %q %q", rc, out, errs)
	}
	for _, want := range []string{"ONE provider", "scratch directory", "tofu init", "not checked against the export", "--replace"} {
		if !strings.Contains(out+errs, want) {
			t.Errorf("iac import --help does not say %q:\n%s%s", want, out, errs)
		}
	}
}

// N12: an add over an installed line says it replaced it and which releases it dropped.
func TestIaCAddSaysWhatItDropped(t *testing.T) {
	home := helpSandbox(t)
	a, b := filepath.Join(home, "a.json"), filepath.Join(home, "b.json")
	iacTestExport(t, a, "name")
	iacTestExport(t, b, "name", "extra")
	installed := filepath.Join(home, "iac", "registry.opentofu.org_example_widget.v1.iaccat.zip")
	keep := func(from string, args ...string) string {
		t.Helper()
		if rc, _, errs := runIaC(args...); rc != 0 {
			t.Fatalf("%v: %q", args, errs)
		}
		dst := filepath.Join(home, from)
		if err := os.WriteFile(dst, iacReadFile(t, installed), 0o644); err != nil {
			t.Fatal(err)
		}
		if rc, _, errs := runIaC("remove", "registry.opentofu.org_example_widget.v1"); rc != 0 {
			t.Fatalf("%q", errs)
		}
		return dst
	}
	wide := keep("wide.iaccat.zip", "import", "1.0.0="+a, "1.1.0="+b)
	narrow := keep("narrow.iaccat.zip", "import", "1.1.0="+b)
	if rc, out, errs := runIaC("add", wide); rc != 0 || strings.Contains(out, "replaced") || strings.Contains(out, "dropped") {
		t.Fatalf("the first add displaces nothing: rc %d %q %q", rc, out, errs)
	}
	rc, out, errs := runIaC("add", narrow)
	if rc != 0 || !strings.Contains(out, "replaced") || !strings.Contains(out, "dropped      1.0.0") {
		t.Fatalf("an add that drops a release must say so: rc %d %q %q", rc, out, errs)
	}
	if rc, out, _ = runIaC("add", wide, "--json"); rc != 0 || !strings.Contains(out, `"replaced": true`) {
		t.Fatalf("json: %q", out)
	}
}

func iacReadFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
