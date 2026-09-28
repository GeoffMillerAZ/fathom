package semantic

// rulessource_test.go — GL-1 (docs/plans/governance-learning.md, design
// doc §3 "Rules readers"): a registered RulesSource feeds the SAME
// compile path CLAUDE.md/AGENTS.md already go through, gated by its
// configured [rules.sources] level (off/warn/error).

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// fakeRulesSource is a minimal RulesSource stand-in for a real client
// reader (GL-2..GL-6 land those); it never touches the filesystem so the
// test controls Detect/Read directly.
type fakeRulesSource struct {
	id      string
	present bool
	path    string
	content string
}

func (f fakeRulesSource) ID() string              { return f.id }
func (f fakeRulesSource) Detect(root string) bool { return f.present }
func (f fakeRulesSource) Read(root string) ([]core.SourceFile, error) {
	return []core.SourceFile{{Path: f.path, Content: []byte(f.content)}}, nil
}

func TestRulesSourceLevels(t *testing.T) {
	src := fakeRulesSource{id: "test-fake", present: true, path: ".fake/rules.md", content: "# A rule\n\nBody text naming the rule.\n"}
	RegisterRulesSource(src)
	t.Cleanup(func() { delete(rulesSourceRegistry, src.ID()) })

	// off: excluded entirely — never read, never compiled.
	files, applied := RulesSourceCorpus("/tmp/whatever", map[string]RulesSourceLevel{"test-fake": RulesLevelOff})
	if len(files) != 0 || len(applied) != 0 {
		t.Fatalf("off level must exclude the source entirely, got files=%v applied=%v", files, applied)
	}

	// warn (default: no entry in the levels map at all — design doc §3
	// "Default: the client's own files at warn"): included, level recorded.
	files, applied = RulesSourceCorpus("/tmp/whatever", nil)
	if len(files) != 1 || files[0].Path != ".fake/rules.md" {
		t.Fatalf("warn (default) must include the source, got %v", files)
	}
	if applied["test-fake"] != RulesLevelWarn {
		t.Fatalf("applied level = %q, want warn", applied["test-fake"])
	}

	// error: included, level recorded as error (eligible for the
	// PreToolUse deny elsewhere — ADR-038 — never decided here).
	files, applied = RulesSourceCorpus("/tmp/whatever", map[string]RulesSourceLevel{"test-fake": RulesLevelError})
	if len(files) != 1 || applied["test-fake"] != RulesLevelError {
		t.Fatalf("error level must include the source and record it, got files=%v applied=%v", files, applied)
	}

	// I27: the file this port hands back must feed the SAME section
	// splitter CLAUDE.md/AGENTS.md already go through, with a citation
	// that actually sits inside its own section — never a special-cased
	// second path for a non-CLAUDE.md source.
	secs := SplitMarkdownSections(files[0].Path, files[0].Content)
	if len(secs) == 0 {
		t.Fatal("registered source's content did not split into sections")
	}
	sec := secs[len(secs)-1] // the "# A rule" heading section, not the empty preamble
	if sec.Path != files[0].Path {
		t.Fatalf("section path = %q, want %q", sec.Path, files[0].Path)
	}
	lines := strings.Split(string(files[0].Content), "\n")
	if sec.StartLine < 1 || sec.EndLine > len(lines) || sec.StartLine > sec.EndLine {
		t.Fatalf("section line range %d-%d not inside the %d-line file: I27 would refuse this citation", sec.StartLine, sec.EndLine, len(lines))
	}
	if !strings.Contains(sec.Text, "A rule") {
		t.Fatalf("section text %q does not contain the heading it was split from", sec.Text)
	}

	// Detect gating: a source that reports absent contributes nothing,
	// whatever its configured level.
	absent := fakeRulesSource{id: "test-fake-absent", present: false}
	RegisterRulesSource(absent)
	t.Cleanup(func() { delete(rulesSourceRegistry, absent.ID()) })
	files2, applied2 := RulesSourceCorpus("/tmp/whatever", map[string]RulesSourceLevel{"test-fake": RulesLevelOff, "test-fake-absent": RulesLevelError})
	if len(files2) != 0 || len(applied2) != 0 {
		t.Fatalf("an undetected source must contribute nothing even at level error, got files=%v applied=%v", files2, applied2)
	}
}

func TestParseRulesSourceLevel(t *testing.T) {
	cases := []struct {
		in   string
		want RulesSourceLevel
		ok   bool
	}{
		{"off", RulesLevelOff, true},
		{"warn", RulesLevelWarn, true},
		{"error", RulesLevelError, true},
		{"", RulesLevelWarn, true}, // unset key defaults to warn (design doc §3)
		{"bogus", "", false},
	}
	for _, c := range cases {
		got, ok := ParseRulesSourceLevel(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseRulesSourceLevel(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestRegisteredRulesSourcesSorted(t *testing.T) {
	RegisterRulesSource(fakeRulesSource{id: "zzz-test"})
	RegisterRulesSource(fakeRulesSource{id: "aaa-test"})
	t.Cleanup(func() {
		delete(rulesSourceRegistry, "zzz-test")
		delete(rulesSourceRegistry, "aaa-test")
	})
	ids := RegisteredRulesSources()
	ai, zi := -1, -1
	for i, id := range ids {
		if id == "aaa-test" {
			ai = i
		}
		if id == "zzz-test" {
			zi = i
		}
	}
	if ai < 0 || zi < 0 || ai > zi {
		t.Fatalf("RegisteredRulesSources() not sorted: %v", ids)
	}
}
