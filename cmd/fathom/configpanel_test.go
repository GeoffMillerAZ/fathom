package main

import (
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// TestConfigPanelReadsModulesCue: the Config panel reads platform-module
// bindings from modules.cue, the file every writer and reader of bindings
// uses, through projectdir. It once checked bindings.cue, a name nothing
// writes, so a repository with bindings was always shown none.
func TestConfigPanelReadsModulesCue(t *testing.T) {
	t.Run("modules.cue with bindings is counted", func(t *testing.T) {
		root := t.TempDir()
		writeTestBindings(t, root, `bindings: [{module: "platform-standards"}, {module: "ui-kit"}]`)
		p := buildConfigPanel(root, 0)
		if !strings.Contains(p.BindingNote, "2 binding(s)") || !strings.Contains(p.BindingNote, "modules.cue") {
			t.Errorf("BindingNote = %q, want the 2 bindings modules.cue declares", p.BindingNote)
		}
	})
	t.Run("an all-commented scaffold declares none", func(t *testing.T) {
		root := t.TempDir()
		writeTestBindings(t, root, "// bindings: [{module: \"m\"}]\n")
		p := buildConfigPanel(root, 0)
		if !strings.Contains(p.BindingNote, "no bindings declared") {
			t.Errorf("BindingNote = %q, want the scaffold named as declaring none", p.BindingNote)
		}
	})
	t.Run("an invalid modules.cue is named, not shown as none", func(t *testing.T) {
		root := t.TempDir()
		writeTestBindings(t, root, `bindings: [{module: "Not_Valid"}]`)
		p := buildConfigPanel(root, 0)
		if !strings.Contains(p.BindingNote, "invalid") {
			t.Errorf("BindingNote = %q, want the invalid file named", p.BindingNote)
		}
	})
	t.Run("a stray bindings.cue is not a binding file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(projectdir.Under(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(projectdir.Under(root)+string(os.PathSeparator)+"bindings.cue", []byte(`bindings: [{module: "m"}]`), 0o644); err != nil {
			t.Fatal(err)
		}
		p := buildConfigPanel(root, 0)
		if !strings.Contains(p.BindingNote, "no bindings file") {
			t.Errorf("BindingNote = %q, want no binding file (modules.cue is absent)", p.BindingNote)
		}
	})
}
