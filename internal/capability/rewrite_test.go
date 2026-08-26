package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpliceSettingsBlock pins the round-trip contract behind
// `fathom capabilities set`: only the `capabilities` declaration's own
// bytes change; everything around it — the `budgets` block (BA-9), comments
// before, inside-the-file and trailing, a scaffold — survives verbatim. The
// shapes the splice cannot round-trip are refused, and the message names
// the manual edit rather than approximating.
func TestSpliceSettingsBlock(t *testing.T) {
	const block = "capabilities: {\n\tguards: on: true \n}"

	t.Run("replaces the block in place, every other byte verbatim", func(t *testing.T) {
		before := "// header kept as-is\n\n" +
			"capabilities: {\n\tnarrate: on: false  // old\n} // trailing comment kept\n\n" +
			"// budgets sized by hand\n" +
			"budgets: {\n\tlist:      3000\n\tcomposite: 8000 // roomy\n}\n"
		got, err := SpliceSettingsBlock("t.cue", []byte(before), []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "// header kept as-is\n\n" +
			block + " // trailing comment kept\n\n" +
			"// budgets sized by hand\n" +
			"budgets: {\n\tlist:      3000\n\tcomposite: 8000 // roomy\n}\n"
		if string(got) != want {
			t.Errorf("splice changed bytes outside the block:\n--- got ---\n%s\n--- want ---\n%s", got, want)
		}
	})

	t.Run("appends after a file with no capabilities block", func(t *testing.T) {
		before := "budgets: {\n\tmap: 1200\n}\n"
		got, err := SpliceSettingsBlock("t.cue", []byte(before), []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := before + "\n" + block + "\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("appends to a scaffold keeping every teaching line", func(t *testing.T) {
		scaffold := ExampleTemplate(Default())
		got, err := SpliceSettingsBlock("t.cue", scaffold, []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(string(got), string(scaffold)) {
			t.Error("the scaffold's bytes must survive in front of the appended block")
		}
		if !strings.HasSuffix(string(got), "\n"+block+"\n") {
			t.Errorf("the block must be appended after a blank line:\n%s", got)
		}
	})

	t.Run("a missing final newline is repaired before appending", func(t *testing.T) {
		got, err := SpliceSettingsBlock("t.cue", []byte("budgets: list: 900"), []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "budgets: list: 900\n\n" + block + "\n"; string(got) != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("an empty file gets just the block", func(t *testing.T) {
		got, err := SpliceSettingsBlock("t.cue", nil, []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != block+"\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("a file that does not parse is refused, unchanged", func(t *testing.T) {
		_, err := SpliceSettingsBlock("t.cue", []byte("capabilities: {\n"), []byte(block))
		if err == nil || !strings.Contains(err.Error(), "does not parse") || !strings.Contains(err.Error(), "by hand") {
			t.Fatalf("want a parse refusal naming the manual edit, got %v", err)
		}
	})

	t.Run("two top-level capabilities declarations are refused", func(t *testing.T) {
		before := "capabilities: narrate: on: true\nbudgets: list: 900\ncapabilities: guards: on: false\n"
		_, err := SpliceSettingsBlock("t.cue", []byte(before), []byte(block))
		if err == nil || !strings.Contains(err.Error(), "more than one") || !strings.Contains(err.Error(), "merge them") {
			t.Fatalf("want a duplicate-block refusal naming the merge, got %v", err)
		}
	})

	t.Run("capabilities reached through an embedding is refused", func(t *testing.T) {
		before := "_caps: capabilities: narrate: on: true\n_caps\n"
		_, err := SpliceSettingsBlock("t.cue", []byte(before), []byte(block))
		if err == nil || !strings.Contains(err.Error(), "plain top-level") {
			t.Fatalf("want a refusal naming the plain-block edit, got %v", err)
		}
	})

	t.Run("what the splice writes loads back through both loaders", func(t *testing.T) {
		before := "capabilities: {\n\tnarrate: on: false\n}\n\nbudgets: {\n\tlist: 3000\n}\n"
		got, err := SpliceSettingsBlock("t.cue", []byte(before), []byte(block))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p := filepath.Join(t.TempDir(), "capabilities.cue")
		if err := os.WriteFile(p, got, 0o600); err != nil {
			t.Fatal(err)
		}
		s, lerr := Default().LoadSettings(p, ScopeProject)
		if lerr != nil {
			t.Fatalf("settings must load: %v", lerr)
		}
		if len(s) != 1 || !s["guards"] {
			t.Errorf("settings = %v, want {guards:true}", s)
		}
		b, berr := LoadBudgets(p, ScopeProject)
		if berr != nil {
			t.Fatalf("budgets must load: %v", berr)
		}
		if b["list"] != 3000 {
			t.Errorf("budgets = %v, want list 3000 untouched", b)
		}
	})
}
