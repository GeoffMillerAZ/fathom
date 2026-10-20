package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// The Tailwind reading (calibration/oracles/tailwind/manifest.json) is a layer of
// the css dialect: its files are those the variant's sniff accepts, and the
// harness dispatches by the host.
func TestTailwindManifestMeasuresOnlyTheVariantsFiles(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "calibration", "oracles", "tailwind", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := langbench.ParseManifest(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "tailwind" || m.HostID() != "css" || m.HostVariant != "tailwind" {
		t.Fatalf("manifest = %s host %s variant %s", m.ID, m.HostID(), m.HostVariant)
	}
	root := t.TempDir()
	files := map[string]string{
		"v4.css":    "@theme {\n  --a: 1;\n}\n",
		"plain.css": ".a { color: red; }\n",
		"apply.css": ".a { @apply p-4; }\n",
		"note.txt":  "@theme",
	}
	var refs []core.SourceRef
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, core.SourceRef{Path: name})
	}
	var got []string
	for _, r := range ownedSourceRefs(m, root, refs, nil, nil) {
		got = append(got, r.Path)
	}
	want := map[string]bool{"v4.css": true, "apply.css": true}
	if len(got) != len(want) {
		t.Fatalf("owned = %v, want %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("owned %s", g)
		}
	}
	// A manifest of the host itself measures every .css file.
	host := m
	host.HostVariant = ""
	if n := len(ownedSourceRefs(host, root, refs, nil, nil)); n != 3 {
		t.Errorf("without a host variant %d files are owned, want 3", n)
	}
}

// A custom utility is a name a pair is graded on: W2's closure holds @utility
// names beside custom properties and @keyframes.
func TestCSSClosureNamesHoldsUtilities(t *testing.T) {
	m := langbench.Manifest{ID: "tailwind", LangTag: "css", NameMatch: langbench.NameMatchIdentity,
		OracleKinds: map[string]map[string]string{"tool": {"utility": "mixin", "theme": "variable", "selector": "selector"}}}
	tf := &toolOracleFile{}
	for _, d := range []struct{ path, name, kind string }{
		{"a.css", "card", "utility"}, {"a.css", "--c", "theme"}, {"b.css", ".sel", "selector"},
	} {
		tf.Definitions = append(tf.Definitions, struct {
			Path string `json:"path"`
			Name string `json:"name"`
			Line int    `json:"line"`
			End  int    `json:"end"`
			Kind string `json:"kind"`
		}{Path: d.path, Name: d.name, Kind: d.kind})
	}
	tf.Calls = []oraclePair{{Path: "a.css", From: ".x", To: "card"}}
	got := cssClosureNames(m, tf)["a.css"]
	if !got["card"] || !got["--c"] || got[".sel"] {
		t.Errorf("closure names = %v, want card and --c only", got)
	}
}
