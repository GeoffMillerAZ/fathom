package iacfacts

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// N10: the order exports are added in cannot change the catalog's bytes, even when the exports disagree on
// format_version.
func TestAddExportOrderCannotChangeTheBytes(t *testing.T) {
	e1 := fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a")}, nil)
	e2 := bytes.Replace(fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a", "b")}, nil), []byte(`"format_version":"1.0"`), []byte(`"format_version":"1.2"`), 1)
	build := func(first, second string) []byte {
		dir := t.TempDir()
		w, err := NewIaCCatalogWriter(IaCWriterOptions{WorkDir: filepath.Join(dir, "work"), Tool: "fixture"})
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()
		exports := map[string][]byte{"1.0.0": e1, "1.1.0": e2}
		for _, v := range []string{first, second} {
			if _, err := w.AddExport(v, bytes.NewReader(exports[v])); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(dir, "c.iaccat.zip")
		if _, err := w.WriteFile(out); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(out)
		return b
	}
	if !bytes.Equal(build("1.0.0", "1.1.0"), build("1.1.0", "1.0.0")) {
		t.Fatal("reversing AddExport order changed the catalog bytes")
	}
}

// N10: a refused export leaves no blob behind, and a blob nothing refers to never reaches the catalog.
func TestRefusedExportLeavesNoOrphanBlob(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	w, err := NewIaCCatalogWriter(IaCWriterOptions{WorkDir: work})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.AddExport("1.0.0", bytes.NewReader(fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a")}, nil))); err != nil {
		t.Fatal(err)
	}
	blobs := len(w.blobs)
	files := func() int { e, _ := os.ReadDir(work); return len(e) }
	before := files()
	// "a_ok" is pruned and written before "z_big" refuses the export.
	bad := fxExport(t, fxSource, fxProvider(), map[string]any{"a_ok": fxResource("only_here"), "z_big": fxBig(t, int(MaxBlobBytes)*13/10)}, nil)
	if _, err := w.AddExport("1.1.0", bytes.NewReader(bad)); err == nil {
		t.Fatal("an oversized schema must refuse the export")
	}
	if len(w.blobs) != blobs || files() != before {
		t.Fatalf("a refused export left blobs behind: %d -> %d tracked, %d -> %d files", blobs, len(w.blobs), before, files())
	}
	// a duplicate type key inside one export orphans its first blob: it is written to the work directory but not to the catalog.
	dup := `{"format_version":"1.0","provider_schemas":{"` + fxSource + `":{"resource_schemas":{"r":{"version":1,"block":{"attributes":{"x":{"type":"string"}}}},"r":{"version":1,"block":{"attributes":{"y":{"type":"string"}}}}}}}}`
	if _, err := w.AddExport("1.2.0", strings.NewReader(dup)); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "c.iaccat.zip")
	if _, err := w.WriteFile(out); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIaCCatalog(out); err != nil {
		t.Fatalf("a catalog with an orphaned blob would be refused at add: %v", err)
	}
}

// N10: the pruner is an allowlist. A prose field it has never heard of does not survive, and every flag does.
func TestPrunerKeepsOnlyTheAllowlist(t *testing.T) {
	raw := `{"version":2,"future_schema_field":"x","block":{"deprecation_message":"drop","notes":"drop","deprecated":true,
	"attributes":{
		"description":{"type":"string","optional":true,"description":"drop","summary":"drop","deprecation_message":"drop"},
		"token":{"type":"string","required":true,"sensitive":true,"deprecated":true,"write_only":true,"help":"drop"},
		"nested":{"nested_type":{"nesting_mode":"list","min_items":1,"max_items":3,"help":"drop","attributes":{"inner":{"type":"number","computed":true,"description":"drop","extra":"drop"}}},"optional":true}},
	"block_types":{"rule":{"nesting_mode":"set","min_items":0,"max_items":2,"extra":"drop","block":{"attributes":{"port":{"type":"number","required":true,"message":"drop"}},"description":"drop"}}}}}`
	out, err := pruneIaCSchema([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, gone := range []string{"drop", "future_schema_field", "summary", "help", "message", "extra", "notes"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survived the pruner: %s", gone, got)
		}
	}
	for _, kept := range []string{`"description":{"optional":true,"type":"string"}`, `"sensitive":true`, `"deprecated":true`, `"write_only":true`, `"nesting_mode":"list"`, `"max_items":3`, `"min_items":1`, `"nesting_mode":"set"`, `"port":{"required":true,"type":"number"}`, `"inner":{"computed":true,"type":"number"}`, `"version":2`} {
		if !strings.Contains(got, kept) {
			t.Errorf("%s was pruned away: %s", kept, got)
		}
	}
}
