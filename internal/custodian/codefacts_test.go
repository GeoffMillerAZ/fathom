package custodian

// codefacts_test.go — VB-28. TestClassifySentenceLabelledSet is the
// precision/recall gate (M14's shape, run against the frozen fixture
// under testdata/codefacts/ rather than a live, evolving checkout —
// determinism over currency, same call the fixture fleet already makes
// elsewhere in this repo).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type labelledSentence struct {
	Sentence string        `json:"sentence"`
	Class    CodeFactClass `json:"class"`
}

func loadLabels(t *testing.T) []labelledSentence {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "codefacts", "labels.json"))
	if err != nil {
		t.Fatalf("reading labels: %v", err)
	}
	var out []labelledSentence
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parsing labels: %v", err)
	}
	return out
}

// TestClassifySentenceLabelledSet is the precision gate a plan row
// unblocks on (VB-28: "behind a flag until precision ≥ 0.8 on twenty
// real files"). Positive = "code-fact-shaped" (any class other than
// preference) — the binary decision the design demands be precision-
// first ("in doubt classify preference"). Recall is recorded, not
// floored, per the brief.
func TestClassifySentenceLabelledSet(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "codefacts", "fixture"))
	if err != nil {
		t.Fatalf("resolving fixture root: %v", err)
	}
	labels := loadLabels(t)
	if len(labels) < 30 {
		t.Fatalf("labelled set has %d sentences, want >= 30", len(labels))
	}

	var truePos, falsePos, falseNeg, trueNeg int
	var exactMatches int
	byClass := map[CodeFactClass]int{}
	for _, lab := range labels {
		byClass[lab.Class]++
		gotClass, _ := classifySentence(root, "", lab.Sentence)
		if gotClass == lab.Class {
			exactMatches++
		}

		wantPositive := lab.Class != ClassPreference
		gotPositive := gotClass != ClassPreference
		switch {
		case wantPositive && gotPositive:
			truePos++
		case !wantPositive && gotPositive:
			falsePos++
			t.Logf("false positive (called %s, labelled preference): %q", gotClass, lab.Sentence)
		case wantPositive && !gotPositive:
			falseNeg++
			t.Logf("false negative (called preference, labelled %s): %q", lab.Class, lab.Sentence)
		default:
			trueNeg++
		}
		if gotClass != lab.Class {
			t.Logf("class mismatch: got %s want %s: %q", gotClass, lab.Class, lab.Sentence)
		}
	}

	precision := 1.0
	if truePos+falsePos > 0 {
		precision = float64(truePos) / float64(truePos+falsePos)
	}
	recall := 1.0
	if truePos+falseNeg > 0 {
		recall = float64(truePos) / float64(truePos+falseNeg)
	}
	exactAccuracy := float64(exactMatches) / float64(len(labels))

	t.Logf("labelled set: %d sentences, classes: %v", len(labels), byClass)
	t.Logf("binary (code-fact-shaped vs preference): precision=%.3f recall=%.3f (recall recorded, not a floor)", precision, recall)
	t.Logf("exact 4-way class accuracy (golden, not asserted): %.3f", exactAccuracy)

	if precision < 0.8 {
		t.Fatalf("precision %.3f < 0.8 — the detector is not conservative enough to ship behind --code-facts", precision)
	}
}

func TestSplitSentences(t *testing.T) {
	text := "# Heading\n\n" +
		"- First point. Second point in the same bullet.\n" +
		"- e.g. this uses an abbreviation. It should not fragment there.\n" +
		"\n" +
		"```go\n" +
		"func Ignored() {} // inside a fence, never a sentence\n" +
		"```\n" +
		"| a | b |\n" +
		"Trailing line with no terminal punctuation\n"

	got := splitSentences(text)
	var texts []string
	for _, s := range got {
		texts = append(texts, s.Text)
	}

	want := []string{
		"Heading",
		"First point.",
		"Second point in the same bullet.",
		"e.g. this uses an abbreviation.",
		"It should not fragment there.",
		"a | b |",
		"Trailing line with no terminal punctuation",
	}
	if len(texts) != len(want) {
		t.Fatalf("splitSentences: got %d sentences %v, want %d %v", len(texts), texts, len(want), want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("sentence %d: got %q want %q", i, texts[i], want[i])
		}
	}

	// Fenced code must never surface as a sentence.
	for _, s := range got {
		if s.Text == "func Ignored() {} // inside a fence, never a sentence" {
			t.Errorf("a fenced line leaked through as a sentence: %q", s.Text)
		}
	}

	// Line numbers point back at the real 1-indexed source line.
	if got[1].Line != 3 {
		t.Errorf("\"First point.\" line = %d, want 3 (the bullet line)", got[1].Line)
	}
}

func TestClaudeCodeMemoryDir(t *testing.T) {
	dir := ClaudeCodeMemoryDir("/Volumes/m2ext/code/geoffmilleraz/sextant", "/Users/geoff")
	want := "/Users/geoff/.claude/projects/-Volumes-m2ext-code-geoffmilleraz-sextant/memory"
	if dir != want {
		t.Errorf("ClaudeCodeMemoryDir = %q, want %q", dir, want)
	}
}

func TestAutoLoadedFiles(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	mustWriteCF(t, filepath.Join(root, "CLAUDE.md"), "# root\n")
	mustWriteCF(t, filepath.Join(root, "cmd", "widget", "CLAUDE.md"), "# nested\n")
	mustWriteCF(t, filepath.Join(root, ".cursor", "rules", "style.mdc"), "cursor rule\n")
	mustWriteCF(t, filepath.Join(root, ".windsurf", "rules", "style.md"), "windsurf rule\n")
	mustWriteCF(t, filepath.Join(root, "memory-bank", "progress.md"), "cline memory\n")

	memDir := ClaudeCodeMemoryDir(root, home)
	mustWriteCF(t, filepath.Join(memDir, "MEMORY.md"), "# claude auto-memory\n")

	files, err := AutoLoadedFiles(root, home)
	if err != nil {
		t.Fatalf("AutoLoadedFiles: %v", err)
	}

	kinds := map[string]bool{}
	var claudeMemPath string
	for _, f := range files {
		kinds[f.Kind] = true
		if f.Kind == "claude-auto-memory" {
			claudeMemPath = f.Path
		}
	}
	for _, want := range []string{"agent-config", "cursor-rule", "windsurf-rule", "memory-bank", "claude-auto-memory"} {
		if !kinds[want] {
			t.Errorf("AutoLoadedFiles: missing kind %q among %d file(s)", want, len(files))
		}
	}
	if claudeMemPath != "~/.claude/projects/"+filepathEncode(root)+"/memory/MEMORY.md" {
		t.Errorf("claude-auto-memory display path = %q", claudeMemPath)
	}

	// No home directory: auto-memory is skipped, not an error.
	files2, err := AutoLoadedFiles(root, "")
	if err != nil {
		t.Fatalf("AutoLoadedFiles with no home: %v", err)
	}
	for _, f := range files2 {
		if f.Kind == "claude-auto-memory" {
			t.Errorf("AutoLoadedFiles with home=\"\" still found claude-auto-memory: %+v", f)
		}
	}
}

func filepathEncode(root string) string {
	abs, _ := filepath.Abs(root)
	return replaceSlashes(abs)
}

func replaceSlashes(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out[i] = '-'
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}

// TestAuditCodeFactsNamesUnreadableFiles is ADR-027's lesson applied to
// this row: a file the audit cannot read must be named, never silently
// dropped from the count.
func TestAuditCodeFactsNamesUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteCF(t, filepath.Join(root, "CLAUDE.md"), "# readable\n\nKeep this router thin.\n")
	blocked := filepath.Join(root, "AGENTS.md")
	mustWriteCF(t, blocked, "# blocked\n")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })

	if data, err := os.ReadFile(blocked); err == nil {
		t.Skipf("this process can still read a 0000 file (%d bytes) — running as root or on a filesystem that ignores mode; skipping", len(data))
	}

	_, unreadable, err := AuditCodeFacts(root, "")
	if err != nil {
		t.Fatalf("AuditCodeFacts: %v", err)
	}
	if len(unreadable) != 1 || unreadable[0] != "AGENTS.md" {
		t.Errorf("unreadable = %v, want [AGENTS.md] — an unreadable auto-loaded file must be named, not dropped (ADR-027)", unreadable)
	}
}

func TestAuditCodeFactsEndToEnd(t *testing.T) {
	root := t.TempDir()
	mustWriteCF(t, filepath.Join(root, "internal", "widget", "widget.go"),
		"package widget\n\nfunc Compute(x int) int { return x }\n")
	mustWriteCF(t, filepath.Join(root, "CLAUDE.md"),
		"# CLAUDE.md\n\n"+
			"- `Compute` lives in `internal/widget`.\n"+
			"- Always run the failing test first.\n")

	findings, unreadable, err := AuditCodeFacts(root, "")
	if err != nil {
		t.Fatalf("AuditCodeFacts: %v", err)
	}
	if len(unreadable) != 0 {
		t.Fatalf("unreadable = %v, want none", unreadable)
	}

	var classes []CodeFactClass
	for _, f := range findings {
		if f.File != "CLAUDE.md" {
			t.Errorf("finding.File = %q, want CLAUDE.md", f.File)
		}
		classes = append(classes, f.Class)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	wantHas := map[CodeFactClass]bool{ClassAnchoredFresh: false, ClassPreference: false}
	for _, c := range classes {
		if _, ok := wantHas[c]; ok {
			wantHas[c] = true
		}
	}
	for c, ok := range wantHas {
		if !ok {
			t.Errorf("findings %v missing expected class %s", classes, c)
		}
	}
}

func mustWriteCF(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
