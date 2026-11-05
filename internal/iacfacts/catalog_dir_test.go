package iacfacts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddIaCCatalogTakesALocalPathOnly(t *testing.T) {
	dir := t.TempDir()
	for _, src := range []string{"https://example.test/cat.zip", "http://example.test/cat.zip", "file:///tmp/cat.zip", "git::https://example.test/x"} {
		_, err := AddIaCCatalog(dir, src)
		if err == nil || !strings.Contains(err.Error(), "local file path") {
			t.Errorf("%s: %v", src, err)
		}
	}
	if _, err := AddIaCCatalog(dir, t.TempDir()); err == nil {
		t.Error("a directory is not a catalog")
	}
	if _, err := AddIaCCatalog(dir, filepath.Join(dir, "missing.zip")); err == nil {
		t.Error("a missing file must fail")
	}
}

func TestAddListRemoveIaCCatalog(t *testing.T) {
	src := twoVersionCatalog(t)
	dir := filepath.Join(t.TempDir(), "iac")
	info, err := AddIaCCatalog(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := fileSHA256(src)
	if info.SHA256 != want || info.Name != "registry.opentofu.org_example_widget.v1.iaccat.zip" || info.Manifest.Line != "1" {
		t.Fatalf("%+v want digest %s", info, want)
	}
	// adding again replaces atomically and leaves no temp file behind.
	if _, err := AddIaCCatalog(dir, src); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("%d files after a replace, want 1", len(ents))
	}
	list, bad := ListIaCCatalogs(dir, true)
	if len(bad) != 0 || len(list) != 1 || list[0].SHA256 != want || list[0].Manifest.Source != fxSource {
		t.Fatalf("%+v %v", list, bad)
	}
	// a file that is not a catalog is reported, never dropped.
	if err := os.WriteFile(filepath.Join(dir, "broken"+catalogSuffix), []byte("nope, but long enough to stat as a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, bad = ListIaCCatalogs(dir, false)
	if len(list) != 1 || len(bad) != 1 {
		t.Fatalf("%+v %v", list, bad)
	}
	for _, name := range []string{"../x", "a/b", "", ".hidden"} {
		if _, err := RemoveIaCCatalog(dir, name); err == nil {
			t.Errorf("%q must not remove", name)
		}
	}
	if _, err := RemoveIaCCatalog(dir, "absent"); err == nil {
		t.Error("an absent catalog must say so")
	}
	got, err := RemoveIaCCatalog(dir, "registry.opentofu.org_example_widget.v1")
	if err != nil || got != info.Name {
		t.Fatalf("%q %v", got, err)
	}
	if list, _ = ListIaCCatalogs(dir, false); len(list) != 0 {
		t.Fatalf("%+v", list)
	}
}

func TestAddIaCCatalogRefusesABadCatalogAndInstallsNothing(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(t.TempDir(), "bad.iaccat.zip")
	if err := os.WriteFile(bad, []byte("this is certainly not a zip file at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := AddIaCCatalog(dir, bad)
	var ref *IaCRefusal
	if !errors.As(err, &ref) || ref.Scope != ScopeCatalog {
		t.Fatalf("%v", err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Fatalf("a refused catalog left files: %v", ents)
	}
}

func TestImportIaCExportsBuildsAndInstalls(t *testing.T) {
	tmp := t.TempDir()
	e1, e2 := filepath.Join(tmp, "a.json"), filepath.Join(tmp, "b.json")
	os.WriteFile(e1, fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a")}, nil), 0o644)
	os.WriteFile(e2, fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a", "b")}, nil), 0o644)
	dir := filepath.Join(tmp, "iac")
	info, stats, err := ImportIaCExports(dir, []IaCImportInput{{"1.2.0", e2}, {"1.1.0", e1}}, IaCWriterOptions{Tool: "operator", Licence: "MPL-2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 || stats[0].Resources != 1 || stats[0].InputSHA256 == "" {
		t.Fatalf("%+v", stats)
	}
	m := info.Manifest
	if strings.Join(m.Versions, ",") != "1.1.0,1.2.0" || m.Replacement != ReplacementAbsent || m.Licence != "MPL-2.0" || m.Tool != "operator" {
		t.Fatalf("%+v", m)
	}
	f, err := ReadIaCCatalog(info.Path, IaCCatalogRequest{Version: "1.2.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "b", "type"}})
	if err != nil || string(f.Value) != `"string"` {
		t.Fatalf("%s %v", f.Value, err)
	}
	if _, _, err := ImportIaCExports(dir, nil, IaCWriterOptions{}); err == nil {
		t.Error("no inputs must fail")
	}
	// a bad export installs nothing.
	bad := filepath.Join(tmp, "bad.json")
	os.WriteFile(bad, []byte(`{"format_version":"1.0","provider_schemas":{}}`), 0o644)
	if _, _, err := ImportIaCExports(filepath.Join(tmp, "iac2"), []IaCImportInput{{"1.0.0", bad}}, IaCWriterOptions{}); err == nil {
		t.Error("an export without a provider must fail")
	}
}

func TestImportRefusesAnOversizedSchemaWholesale(t *testing.T) {
	lim := DefaultIaCLimits()
	w, err := NewIaCCatalogWriter(IaCWriterOptions{WorkDir: t.TempDir(), Limits: lim})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	big := fxExport(t, fxSource, fxProvider(), map[string]any{"widget_ok": fxResource("a"), "widget_big": fxBig(t, int(MaxBlobBytes)*13/10)}, nil)
	_, err = w.AddExport("1.0.0", strings.NewReader(string(big)))
	var ref *IaCRefusal
	if !errors.As(err, &ref) || ref.Code != RefuseBlobBytes || !strings.Contains(ref.Reason, "widget_big") {
		t.Fatalf("an over-bound schema must refuse the whole export, naming it: %v", err)
	}
}
