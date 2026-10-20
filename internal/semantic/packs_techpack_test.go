package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func appendFile(t *testing.T, path, data string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(data); err != nil {
		t.Fatal(err)
	}
}

// TestLoadLayerInUnionsTechPackLayers is FD-18's anchor: the union reads the
// layers/ of every tech pack the repository's pins put on disk, with
// precedence embedded, then tech pack, then project; a later stratum
// overrides an earlier one by layer id, and a fathom-pack/ without a header
// contributes nothing.
func TestLoadLayerInUnionsTechPackLayers(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	cache := filepath.Join(base, "gomodcache")
	t.Setenv("GOMODCACHE", cache)
	t.Setenv("CUE_CACHE_DIR", filepath.Join(base, "cuecache"))
	t.Setenv("GOPROXY", "off")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gomod := "module example.test/app\n\ngo 1.22\n\nrequire (\n\texample.test/layerkit v1.0.0\n\texample.test/noheader v1.0.0\n)\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	kit := filepath.Join(cache, "example.test", "layerkit@v1.0.0", "fathom-pack")
	if err := os.MkdirAll(kit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kit, "pack.cue"), []byte("pack: {name: \"layer-kit\", version: 1, about: \"layers\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A layer only the kit ships, declaring the tech stratum.
	writeFixturePack(t, filepath.Join(kit, "layers", "kit-only"), "kit-only", false)
	appendFile(t, filepath.Join(kit, "layers", "kit-only", "layer.cue"), "stratum: \"tech\"\n")
	// The kit overrides an embedded layer by id.
	writeFixturePack(t, filepath.Join(kit, "layers", LayerObligations), LayerObligations, false)
	// The kit and the project both ship "shared": the project wins.
	writeFixturePack(t, filepath.Join(kit, "layers", "shared"), "shared", false)
	writeFixturePack(t, filepath.Join(root, ProjectLayersDir, "shared"), "shared", false)
	if err := os.WriteFile(filepath.Join(root, ProjectLayersDir, "shared", "prompt.md"), []byte("project prompt"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No header: not a tech pack, so its layer is never read.
	writeFixturePack(t, filepath.Join(cache, "example.test", "noheader@v1.0.0", "fathom-pack", "layers", "ghost"), "ghost", false)

	def, err := LoadLayerIn(root, "kit-only")
	if err != nil {
		t.Fatalf("LoadLayerIn(kit-only): %v", err)
	}
	if def.Stratum != "tech" {
		t.Errorf("kit-only stratum = %q, want tech", def.Stratum)
	}
	if def, err := LoadLayerIn(root, LayerObligations); err != nil || def.Kind != "custom-kind" {
		t.Errorf("obligations = %q, %v; want the tech pack's custom-kind over the embedded layer", def.Kind, err)
	}
	if def, err := LoadLayerIn(root, "shared"); err != nil || def.PromptText != "project prompt" {
		t.Errorf("shared prompt = %q, %v; want the project's over the tech pack's", def.PromptText, err)
	}
	if _, err := LoadLayerIn(root, "ghost"); err == nil {
		t.Errorf("ghost resolved; a fathom-pack/ without a header is not a tech pack")
	}

	infos, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	type row struct {
		Source    LayerSource
		Overrides bool
		TechPack  string
	}
	got := map[string]row{}
	for _, in := range infos {
		got[in.Name] = row{in.Source, in.Overrides, in.TechPack}
	}
	for name, want := range map[string]row{
		"kit-only":       {SourceTech, false, "layer-kit@1"},
		LayerObligations: {SourceTech, true, "layer-kit@1"},
		"shared":         {SourceProject, true, ""},
	} {
		if got[name] != want {
			t.Errorf("%s = %+v, want %+v", name, got[name], want)
		}
	}

	// The union follows the pack: editing a pack file is a rebuild, not a
	// stale memo.
	appendFile(t, filepath.Join(kit, "layers", "kit-only", "prompt.md"), " edited")
	if def, err := LoadLayerIn(root, "kit-only"); err != nil || def.PromptText == "" || def.PromptText[len(def.PromptText)-6:] != "edited" {
		t.Errorf("after editing the pack, kit-only prompt = %q, %v; want the edited text", def.PromptText, err)
	}
}
