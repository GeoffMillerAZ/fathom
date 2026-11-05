package iacfacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExport(t *testing.T, dir, name string, resources map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, fxExport(t, fxSource, fxProvider(), resources, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// M14: importing a release into a line that is installed keeps every release, deterministically, and says what it did.
func TestImportMergesIntoTheInstalledLine(t *testing.T) {
	tmp := t.TempDir()
	e100 := writeExport(t, tmp, "a.json", map[string]any{"widget_thing": fxResource("a")})
	e110 := writeExport(t, tmp, "b.json", map[string]any{"widget_thing": fxResource("a", "b"), "widget_new": fxResource("n")})
	e120 := writeExport(t, tmp, "c.json", map[string]any{"widget_thing": fxResource("a", "b", "c")})
	opts := IaCWriterOptions{Tool: "operator", Licence: "MPL-2.0"}

	dir := filepath.Join(tmp, "iac")
	if _, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.0.0", e100}, {"1.1.0", e110}}, opts, false); err != nil {
		t.Fatal(err)
	}
	res, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.2.0", e120}}, opts, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged || strings.Join(res.Added, ",") != "1.2.0" || strings.Join(res.Kept, ",") != "1.0.0,1.1.0" || len(res.Unchanged) != 0 || len(res.Replaced) != 0 {
		t.Fatalf("%+v", res)
	}
	if got := strings.Join(res.Info.Manifest.Versions, ","); got != "1.0.0,1.1.0,1.2.0" {
		t.Fatalf("an import must keep every release it did not name, got %s", got)
	}
	for v, attr := range map[string]string{"1.0.0": "a", "1.1.0": "b", "1.2.0": "c"} {
		f, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: v, Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", attr, "type"}})
		if err != nil || string(f.Value) != `"string"` {
			t.Fatalf("%s lost its content: %s %v", v, f.Value, err)
		}
	}
	if _, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: "1.1.0", Kind: SectionResource, Type: "widget_new"}); err != nil {
		t.Fatalf("a type only the kept release holds must survive: %v", err)
	}

	// deterministic: the same three releases imported at once give the same bytes.
	all, err := ImportIaCExportsReport(filepath.Join(tmp, "iac2"), []IaCImportInput{{"1.2.0", e120}, {"1.0.0", e100}, {"1.1.0", e110}}, opts, false)
	if err != nil {
		t.Fatal(err)
	}
	if all.Merged || res.Info.SHA256 != all.Info.SHA256 {
		t.Fatalf("a merged catalog must equal one built from all its exports: %s vs %s", res.Info.SHA256, all.Info.SHA256)
	}

	// naming a release with the content it already has changes nothing.
	same, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.1.0", e110}}, opts, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(same.Unchanged, ",") != "1.1.0" || strings.Join(same.Kept, ",") != "1.0.0,1.2.0" || len(same.Added) != 0 || same.Info.SHA256 != res.Info.SHA256 {
		t.Fatalf("%+v", same)
	}
}

func TestImportRefusesToReplaceARelease(t *testing.T) {
	tmp := t.TempDir()
	e100 := writeExport(t, tmp, "a.json", map[string]any{"widget_thing": fxResource("a")})
	e110 := writeExport(t, tmp, "b.json", map[string]any{"widget_thing": fxResource("a", "b")})
	other := writeExport(t, tmp, "other.json", map[string]any{"widget_thing": fxResource("changed")})
	dir := filepath.Join(tmp, "iac")
	first, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.0.0", e100}, {"1.1.0", e110}}, IaCWriterOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(first.Info.Path)
	_, err = ImportIaCExportsReport(dir, []IaCImportInput{{"1.1.0", other}}, IaCWriterOptions{}, false)
	if err == nil || !strings.Contains(err.Error(), "--replace") || !strings.Contains(err.Error(), "1.1.0") {
		t.Fatalf("a release with different content must be refused, naming the flag: %v", err)
	}
	if now, _ := os.ReadFile(first.Info.Path); string(now) != string(before) {
		t.Fatal("a refused import changed the installed catalog")
	}
	if _, _, err := ImportIaCExports(dir, []IaCImportInput{{"1.1.0", other}}, IaCWriterOptions{}); err == nil {
		t.Fatal("the plain ImportIaCExports must refuse too")
	}
	res, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.1.0", other}}, IaCWriterOptions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Replaced, ",") != "1.1.0" || strings.Join(res.Kept, ",") != "1.0.0" || strings.Join(res.Info.Manifest.Versions, ",") != "1.0.0,1.1.0" {
		t.Fatalf("%+v", res)
	}
	f, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: "1.1.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "changed", "type"}})
	if err != nil || string(f.Value) != `"string"` {
		t.Fatalf("the replacement did not land: %s %v", f.Value, err)
	}
	if _, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "a", "type"}}); err != nil {
		t.Fatalf("--replace must keep the releases it did not name: %v", err)
	}
}

func TestImportIntoAnUnreadableInstalledCatalogIsAnErrorNotADrop(t *testing.T) {
	tmp := t.TempDir()
	e100 := writeExport(t, tmp, "a.json", map[string]any{"widget_thing": fxResource("a")})
	e110 := writeExport(t, tmp, "b.json", map[string]any{"widget_thing": fxResource("b")})
	dir := filepath.Join(tmp, "iac")
	first, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.0.0", e100}}, IaCWriterOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(first.Info.Path)
	raw = raw[:len(raw)-9] // the zip end record is gone
	os.WriteFile(first.Info.Path, raw, 0o644)
	if _, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.1.0", e110}}, IaCWriterOptions{}, true); err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("a corrupt installed catalog must stop the import: %v", err)
	}
	if now, _ := os.ReadFile(first.Info.Path); string(now) != string(raw) {
		t.Fatal("the corrupt catalog was replaced")
	}
}

// N19: a merged import carries the installed catalog's replacement claim forward, and does not write absent over it.
func TestImportKeepsTheInstalledCatalogsReplacementClaim(t *testing.T) {
	tmp := t.TempDir()
	e100 := writeExport(t, tmp, "a.json", map[string]any{"widget_thing": fxResource("a")})
	e110 := writeExport(t, tmp, "b.json", map[string]any{"widget_thing": fxResource("a", "b")})
	dir := filepath.Join(tmp, "iac")
	// A published catalog that carries replacement facts: the writer stands in for the later source scan.
	w, err := NewIaCCatalogWriter(IaCWriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	f, _ := os.Open(e100)
	if _, err := w.AddExport("1.0.0", f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	w.replacement = "present"
	pub := filepath.Join(tmp, "published.iaccat.zip")
	if _, err := w.WriteFile(pub); err != nil {
		t.Fatal(err)
	}
	if _, err := AddIaCCatalog(dir, pub); err != nil {
		t.Fatal(err)
	}
	res, err := ImportIaCExportsReport(dir, []IaCImportInput{{"1.1.0", e110}}, IaCWriterOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged || res.Info.Manifest.Replacement != "present" {
		t.Fatalf("the merge must keep the published claim, got %q: %+v", res.Info.Manifest.Replacement, res)
	}
	// An import into a line that holds no claim still says absent.
	fresh, err := ImportIaCExportsReport(filepath.Join(tmp, "iac2"), []IaCImportInput{{"1.0.0", e100}}, IaCWriterOptions{}, false)
	if err != nil || fresh.Info.Manifest.Replacement != ReplacementAbsent {
		t.Fatalf("%q %v", fresh.Info.Manifest.Replacement, err)
	}
}

// N12: an add replaces its line, and says which releases that dropped.
func TestAddReportsWhatItReplacedAndDropped(t *testing.T) {
	tmp := t.TempDir()
	e100 := writeExport(t, tmp, "a.json", map[string]any{"widget_thing": fxResource("a")})
	e110 := writeExport(t, tmp, "b.json", map[string]any{"widget_thing": fxResource("a", "b")})
	e120 := writeExport(t, tmp, "c.json", map[string]any{"widget_thing": fxResource("a", "b", "c")})
	build := func(name string, ins ...IaCImportInput) string {
		res, err := ImportIaCExportsReport(filepath.Join(tmp, name), ins, IaCWriterOptions{}, false)
		if err != nil {
			t.Fatal(err)
		}
		return res.Info.Path
	}
	wide := build("wide", IaCImportInput{"1.0.0", e100}, IaCImportInput{"1.1.0", e110}, IaCImportInput{"1.2.0", e120})
	narrow := build("narrow", IaCImportInput{"1.2.0", e120})
	dir := filepath.Join(tmp, "iac")
	first, err := AddIaCCatalogReport(dir, wide)
	if err != nil || first.Replaced || len(first.Dropped) != 0 {
		t.Fatalf("the first add displaces nothing: %+v %v", first, err)
	}
	second, err := AddIaCCatalogReport(dir, narrow)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replaced || strings.Join(second.Dropped, ",") != "1.0.0,1.1.0" || strings.Join(second.Info.Manifest.Versions, ",") != "1.2.0" {
		t.Fatalf("an add over a line must name what it dropped: %+v", second)
	}
	third, err := AddIaCCatalogReport(dir, wide)
	if err != nil || !third.Replaced || len(third.Dropped) != 0 {
		t.Fatalf("a wider add drops nothing: %+v %v", third, err)
	}
}
