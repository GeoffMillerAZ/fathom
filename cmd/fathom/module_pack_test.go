package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestModulePackClearsMachineLocalFields (IXS-0's product path): `fathom
// module pack` writes a copy that carries none of the store's machine-local
// fields (store.MachineLocalFields), still conforms, and leaves the source
// store's own corpus root in place.
func TestModulePackClearsMachineLocalFields(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("pack-machinelocal", []string{"RULES.md"}, testWatermark(t, root, "RULES.md"))), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	meta := store.ServiceMeta{ServiceID: "pack-machinelocal", Path: root, IndexedCommit: "manifest:abc", IndexedAt: 1}
	if err := st.WriteServiceSummary(ctx, meta, store.ServiceSummary{}, []store.FileStat{{Path: "RULES.md", Size: 12, MtimeNS: 99, Ino: 77, ContentHash: "h"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetIngestState(ctx, store.IngestState{ServiceID: meta.ServiceID, State: store.IngestComplete, Path: root}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "packed")
	if _, stderr, rc := runModule(t, "pack", "--manifest", manifestPath, "--store", storePath, "--out", outDir); rc != 0 {
		t.Fatalf("pack rc=%d stderr=%s", rc, stderr)
	}
	packed := filepath.Join(outDir, "module.sqlite")
	db, err := sql.Open("sqlite", "file:"+packed+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, field := range store.MachineLocalFields() {
		table, column, _ := strings.Cut(field, ".")
		var kept int
		q := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " NOT IN ('', 0)"
		if err := db.QueryRow(q).Scan(&kept); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if kept != 0 {
			t.Errorf("packed copy: %s still carries a machine-local value in %d row(s)", field, kept)
		}
	}
	var hash string
	if err := db.QueryRow("SELECT content_hash FROM file_stats WHERE path = 'RULES.md'").Scan(&hash); err != nil || hash != "h" {
		t.Errorf("packed file_stats content_hash = %q (%v), want the portable column kept", hash, err)
	}

	src, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if got, _, _ := src.GetServiceMeta(ctx, meta.ServiceID); got.Path != root {
		t.Errorf("source store's service_meta.path = %q, want it untouched", got.Path)
	}
}
