package projectdir_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// writeProject lays out root/.fathom with files (name → content).
func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := projectdir.Under(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// FD-4 (docs/plans/fathom-dir.md): each file parses on its own and each
// top-level key validates against its #Project sub-schema. A syntax error
// names its file, a failing key names its file and key, and every other key
// still loads (I9, I38); two files setting one key conflict by name; an
// unknown key is reported, never an error; risk.cue and learning.cue map by
// file name.
func TestProjectLoadIsolatesAFailingKey(t *testing.T) {
	root := writeProject(t, map[string]string{
		// One failing key beside one good key in the same file.
		"capabilities.cue": "capabilities: {\"edit-hook\": {on: \"yes\"}}\nbudgets: {list: 900}\n",
		// A syntax error: the file's keys are unknown, every other file loads.
		"modules.cue":  "bindings: [\n",
		"guard.cue":    "guards: [{scopes: [\"internal/**\"], note: \"n\"}]\nlayer_rules: [{id: \"a\", kind: \"no_cycles\", sentence: \"s\"}, {id: \"b\", kind: \"boundary\", sentence: \"t\"}]\n",
		"concerns.cue": "packs: [\"lefthook\"]\n",
		// A second file setting packs: a conflict naming both files.
		"pack.cue": "packs: [\"codex\"]\nmade_up: true\n",
		// Mapped by file name: the whole file is the key's value.
		"risk.cue":     "blast_radius_high_callers: 40\n",
		"learning.cue": "promote_corroborations: 0\n",
	})

	p, err := projectdir.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The syntax error names its file and no key from it loads.
	var fe *projectdir.FileError
	modules := p.File(projectdir.Modules)
	if !errors.As(modules.Err(), &fe) || fe.Path != projectdir.Modules.Path(root) {
		t.Fatalf("modules.cue: want a *FileError naming %s, got %v", projectdir.Modules.Path(root), modules.Err())
	}
	if _, ok := p.Key("bindings"); ok {
		t.Errorf("bindings loaded from a file that does not parse")
	}

	// The failing key names its file and key; its sibling still loads.
	caps, ok := p.Key("capabilities")
	if !ok {
		t.Fatal("capabilities key missing")
	}
	var ke *projectdir.KeyError
	if !errors.As(caps.Err, &ke) || ke.Key != "capabilities" || ke.Path != projectdir.Capabilities.Path(root) {
		t.Fatalf("capabilities: want a *KeyError naming the file and key, got %v", caps.Err)
	}
	if msg := caps.Err.Error(); !strings.Contains(msg, "capabilities.cue") || !strings.Contains(msg, `"capabilities"`) {
		t.Errorf("capabilities error %q does not name its file and key", msg)
	}
	wantOK := map[string]struct {
		file    projectdir.File
		entries int
	}{
		"budgets":     {projectdir.Capabilities, 1},
		"guards":      {projectdir.Guard, 1},
		"layer_rules": {projectdir.Guard, 2},
		"risk":        {projectdir.Risk, 1},
	}
	for name, want := range wantOK {
		k, ok := p.Key(name)
		if !ok {
			t.Errorf("%s: missing", name)
			continue
		}
		if k.Err != nil || !k.Known || k.File != want.file || k.Entries != want.entries {
			t.Errorf("%s: got file %s known %v entries %d err %v, want file %s known entries %d no error", name, k.File, k.Known, k.Entries, k.Err, want.file, want.entries)
		}
	}

	// learning.cue maps by file name and fails its own schema (#Learning).
	if l, ok := p.Key("learning"); !ok || l.File != projectdir.Learning || !errors.As(l.Err, &ke) {
		t.Errorf("learning: want a *KeyError from learning.cue, got %+v", l)
	}

	// Two files setting one key conflict, by name.
	packs, ok := p.Key("packs")
	if !ok {
		t.Fatal("packs key missing")
	}
	var ce *projectdir.ConflictError
	if !errors.As(packs.Err, &ce) || ce.Key != "packs" || len(ce.Paths) != 2 {
		t.Fatalf("packs: want a *ConflictError over two files, got %v", packs.Err)
	}
	if packs.File != projectdir.Concerns {
		t.Errorf("packs: the conflict keeps the key's home file's value, got %s", packs.File)
	}

	// An unknown key is reported, never an error.
	var unknown []projectdir.Key
	for _, k := range p.Keys() {
		if !k.Known {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) != 1 || unknown[0].Name != "made_up" || unknown[0].File != "pack.cue" || unknown[0].Err != nil {
		t.Errorf("unknown keys = %+v, want made_up from pack.cue with no error", unknown)
	}

	// Determinism: keys come back sorted by name.
	var names []string
	for _, k := range p.Keys() {
		names = append(names, k.Name)
	}
	want := "budgets capabilities guards layer_rules learning made_up packs risk"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("Keys() = %q, want %q", got, want)
	}
	wantKnown := "bindings budgets capabilities concerns guards layer_rules learning orientation packs risk"
	if got := strings.Join(p.KnownKeys(), " "); got != wantKnown {
		t.Errorf("KnownKeys() = %q, want %q", got, wantKnown)
	}
}

// An absent .fathom directory is an empty project, and every known file
// reads as absent with its own read error (a reader can say exactly what it
// said before); a directory that exists but cannot be listed is an error,
// never an empty project (I38).
func TestProjectLoadAbsentAndUnreadable(t *testing.T) {
	root := t.TempDir()
	p, err := projectdir.Load(root)
	if err != nil {
		t.Fatalf("Load(no .fathom): %v", err)
	}
	if len(p.Keys()) != 0 {
		t.Errorf("keys from an absent directory: %+v", p.Keys())
	}
	g := p.File(projectdir.Guard)
	if !g.Absent() || !errors.Is(g.ReadErr, os.ErrNotExist) {
		t.Errorf("guard.cue: want absent with ErrNotExist, got %+v", g)
	}

	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	root2 := writeProject(t, map[string]string{"guard.cue": "guards: []\n"})
	dir := projectdir.Under(root2)
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := projectdir.Load(root2); err == nil {
		t.Errorf("Load over an unlistable directory returned no error")
	}
}

// The embedded schemas are verbatim copies of schema/<name>/schema.cue.
func TestEmbeddedSchemasMatchSource(t *testing.T) {
	for _, name := range []string{"project", "guard", "binding", "techpack", "risk", "learning"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "schema", name, "schema.cue"))
		if err != nil {
			t.Fatal(err)
		}
		mirror, err := os.ReadFile(filepath.Join("schemas", name, name+".cue"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(src, mirror) {
			t.Errorf("internal/projectdir/schemas/%s/%s.cue has drifted from schema/%s/schema.cue — copy the source over the mirror", name, name, name)
		}
	}
}

// This repository's own .fathom loads with no file error, no failing key
// and no unknown key (the cue-vet step's #Project check, in Go).
func TestProjectLoadThisRepository(t *testing.T) {
	p, err := projectdir.Load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range p.Files() {
		if !f.Absent() && f.Err() != nil {
			t.Errorf("%s: %v", f.Name, f.Err())
		}
	}
	for _, k := range p.Keys() {
		if k.Err != nil || !k.Known {
			t.Errorf("%s (%s): known %v err %v", k.Name, k.File, k.Known, k.Err)
		}
	}
	if _, ok := p.Key("guards"); !ok {
		t.Errorf("this repository's guards key did not load")
	}
}
