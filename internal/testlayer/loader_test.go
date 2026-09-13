package testlayer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLayers_AbsentFileReturnsDefaults(t *testing.T) {
	set, err := LoadLayers(filepath.Join(t.TempDir(), "does-not-exist.cue"))
	if err != nil {
		t.Fatalf("LoadLayers: %v", err)
	}
	want := DefaultLayers()
	if len(set.Layers) != len(want.Layers) || set.Layers[0].Name != want.Layers[0].Name {
		t.Errorf("LoadLayers(absent) = %+v, want DefaultLayers() %+v", set, want)
	}
}

func TestLoadLayers_ExplicitEmptyIsErrorFree(t *testing.T) {
	path := writeLayerFile(t, "layers: []\n")
	set, err := LoadLayers(path)
	if err != nil {
		t.Fatalf("LoadLayers(explicit empty): %v", err)
	}
	if len(set.Layers) != 0 {
		t.Errorf("LoadLayers(explicit empty) = %+v, want zero layers", set)
	}
}

func TestLoadLayers_PresentFileUsedExactly(t *testing.T) {
	path := writeLayerFile(t, `layers: [{name: "contract", globs: ["contract/**"]}]`+"\n")
	set, err := LoadLayers(path)
	if err != nil {
		t.Fatalf("LoadLayers: %v", err)
	}
	if len(set.Layers) != 1 || set.Layers[0].Name != "contract" || len(set.Layers[0].Globs) != 1 {
		t.Fatalf("LoadLayers(present) = %+v", set)
	}
	// No blending with DefaultLayers' integration/e2e entries.
	for _, l := range set.Layers {
		if l.Name == "integration" || l.Name == "e2e" {
			t.Errorf("LoadLayers(present) unexpectedly carried a default layer %q", l.Name)
		}
	}
}

func TestLoadLayers_EmptyNameIsError(t *testing.T) {
	path := writeLayerFile(t, `layers: [{name: ""}]`+"\n")
	if _, err := LoadLayers(path); err == nil {
		t.Fatal("expected an error for an empty layer name")
	}
}

func TestLoadLayers_EmptyGlobsIsError(t *testing.T) {
	path := writeLayerFile(t, `layers: [{name: "x", globs: []}]`+"\n")
	if _, err := LoadLayers(path); err == nil {
		t.Fatal("expected an error for an explicit empty globs list")
	}
}

func TestLoadLayers_MalformedCUEIsError(t *testing.T) {
	path := writeLayerFile(t, "not valid cue {{{\n")
	if _, err := LoadLayers(path); err == nil {
		t.Fatal("expected an error for malformed CUE")
	}
}

func writeLayerFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "testlayer.cue")
	if err := os.WriteFile(path, []byte("package testlayer\n\n"+body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
