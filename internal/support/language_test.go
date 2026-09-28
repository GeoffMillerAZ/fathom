package support

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLanguageRegistryFromManifestsAndQueue proves SM-4: the language/tool
// registry joins the dialect manifests (what fathom loads) with the
// language queue (LB-F22's declared target list) by id, without this
// package ever decoding internal/extractor's own Dialect type.
func TestLanguageRegistryFromManifestsAndQueue(t *testing.T) {
	t.Run("real repo dialects join the real queue", func(t *testing.T) {
		root := repoRoot(t)
		dialectsDir := filepath.Join(root, "internal", "extractor", "dialects")
		queuePath := filepath.Join(root, "docs", "plans", "language-breadth.queue.json")

		dirEntries, err := os.ReadDir(dialectsDir)
		if err != nil {
			t.Fatalf("read %s: %v", dialectsDir, err)
		}
		wantManifests := 0
		for _, de := range dirEntries {
			if de.IsDir() {
				if _, err := os.Stat(filepath.Join(dialectsDir, de.Name(), "dialect.json")); err == nil {
					wantManifests++
				}
			}
		}
		if wantManifests == 0 {
			t.Fatal("no dialect manifests found under internal/extractor/dialects — fixture drifted")
		}

		entries, err := languageRegistry(dialectsDir, queuePath)
		if err != nil {
			t.Fatalf("languageRegistry: %v", err)
		}
		if len(entries) != wantManifests {
			t.Fatalf("languageRegistry returned %d entries, want %d (one per manifest)", len(entries), wantManifests)
		}
		for _, e := range entries {
			if e.ID == "" || e.Kind == "" {
				t.Errorf("entry %+v missing id/kind", e)
			}
			if e.Name == e.ID {
				// Every shipped dialect has a matching queue row today —
				// a join miss would silently fall back to the bare id.
				t.Errorf("entry %s: name fell back to id — no matching queue row joined", e.ID)
			}
			wantCaps := languageCapabilities(e.Kind)
			if len(e.Capabilities) != len(wantCaps) {
				t.Errorf("entry %s: capabilities = %v, want %v", e.ID, e.Capabilities, wantCaps)
			}
		}
	})

	t.Run("synthetic manifests join a synthetic queue by id", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteDialect(t, dir, "widget", `{"id":"widget","kind":"language","engine":"tags"}`)
		mustWriteDialect(t, dir, "gizmo", `{"id":"gizmo","kind":"tool","engine":"native"}`)

		queuePath := filepath.Join(dir, "queue.json")
		queueBody := `[
			{"id":"widget","name":"Widget Lang","kind":"language","wave":1,"engine":"tags"},
			{"id":"gizmo","name":"Gizmo Format","kind":"tooling","wave":2,"engine":"native"},
			{"id":"unbuilt","name":"Not Yet Built","kind":"language","wave":3,"engine":"tags"}
		]`
		if err := os.WriteFile(queuePath, []byte(queueBody), 0o644); err != nil {
			t.Fatal(err)
		}

		entries, err := languageRegistry(dir, queuePath)
		if err != nil {
			t.Fatalf("languageRegistry: %v", err)
		}
		// "unbuilt" has no manifest: it is a declared target, not a
		// registered entry — the registry lists exactly the two built.
		if len(entries) != 2 {
			t.Fatalf("entries = %+v, want 2 (unbuilt has no manifest)", entries)
		}

		byID := map[string]RegistryEntry{}
		for _, e := range entries {
			byID[e.ID] = e
		}
		widget, ok := byID["widget"]
		if !ok {
			t.Fatal("missing widget entry")
		}
		if widget.Kind != KindLanguage || widget.Name != "Widget Lang" || widget.Wave != 1 {
			t.Errorf("widget = %+v, want kind=language name=%q wave=1", widget, "Widget Lang")
		}
		if want := []string{"L1", "L2", "L3", "L4"}; !equalStrings(widget.Capabilities, want) {
			t.Errorf("widget.Capabilities = %v, want %v", widget.Capabilities, want)
		}

		gizmo, ok := byID["gizmo"]
		if !ok {
			t.Fatal("missing gizmo entry")
		}
		if gizmo.Kind != KindTool || gizmo.Name != "Gizmo Format" || gizmo.Wave != 2 {
			t.Errorf("gizmo = %+v, want kind=tool name=%q wave=2", gizmo, "Gizmo Format")
		}
		if want := []string{"L1", "L2", "L3"}; !equalStrings(gizmo.Capabilities, want) {
			t.Errorf("gizmo.Capabilities (tool, L4 n/a) = %v, want %v", gizmo.Capabilities, want)
		}
	})
}

func mustWriteDialect(t *testing.T, dialectsDir, id, body string) {
	t.Helper()
	dir := filepath.Join(dialectsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dialect.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
