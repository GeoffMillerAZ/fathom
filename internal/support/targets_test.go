package support

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSupportTargetsVet proves SM-3: docs/plans/support.targets.json
// exists, loads and vets against #Targets; a fixture naming a language kind
// (which stays in the language queue, LB-F22) is refused by the same path.
func TestSupportTargetsVet(t *testing.T) {
	path := filepath.Join(repoRoot(t), "docs", "plans", "support.targets.json")
	targets, err := LoadTargets(path)
	if err != nil {
		t.Fatalf("LoadTargets(%s): %v", path, err)
	}
	for _, tg := range targets {
		if tg.Kind == KindLanguage || tg.Kind == KindTool {
			t.Errorf("target %s/%s: languages and tool formats stay in the language queue, never support.targets.json", tg.Kind, tg.ID)
		}
	}

	dir := t.TempDir()

	good := filepath.Join(dir, "good.json")
	goodBody := `[{"kind":"client","id":"cursor","name":"Cursor","target":{"mcp-tools":true,"obligations-post-edit":true},"plan_row":"SM-13","wave":1}]`
	if err := os.WriteFile(good, []byte(goodBody), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadTargets(good)
	if err != nil {
		t.Fatalf("LoadTargets(good): %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "cursor" {
		t.Fatalf("LoadTargets(good) = %+v, want one cursor row", loaded)
	}
	if ct, ok := loaded[0].Targets["mcp-tools"]; !ok || !ct.ProvenOnly {
		t.Errorf("cursor target[mcp-tools] = %+v, want ProvenOnly", ct)
	}

	bad := filepath.Join(dir, "bad.json")
	badBody := `[{"kind":"language","id":"go","name":"Go","target":{"L1":"L1"},"plan_row":"SM-1","wave":1}]`
	if err := os.WriteFile(bad, []byte(badBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTargets(bad); err == nil {
		t.Fatal("expected LoadTargets to refuse a language-kind row (languages stay in the queue)")
	}
}
