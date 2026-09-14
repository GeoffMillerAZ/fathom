package custodian

// codefacts_test.go — VB-28. TestClassifySentenceLabelledSet is the
// precision/recall gate (M14's shape, run against the frozen fixture
// under testdata/codefacts/ rather than a live, evolving checkout —
// determinism over currency, same call the fixture fleet already makes
// elsewhere in this repo).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
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

	// Sorted, not a map iteration — determinism is a contract even in a
	// log line (internal/CLAUDE.md).
	classNames := make([]string, 0, len(byClass))
	for c := range byClass {
		classNames = append(classNames, string(c))
	}
	sort.Strings(classNames)
	counts := make([]string, 0, len(classNames))
	for _, c := range classNames {
		counts = append(counts, fmt.Sprintf("%s=%d", c, byClass[CodeFactClass(c)]))
	}
	t.Logf("labelled set: %d sentences, classes: [%s]", len(labels), strings.Join(counts, " "))
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

// TestClaudeCodeMemoryDir uses wholly invented, neutral paths (ADR-011) —
// never a real developer's home or repo checkout — with a hardcoded
// expected value, so the test independently verifies the documented
// transform rather than re-deriving it from the function under test.
func TestClaudeCodeMemoryDir(t *testing.T) {
	dir := ClaudeCodeMemoryDir("/srv/acme/repo", "/home/dev")
	want := "/home/dev/.claude/projects/-srv-acme-repo/memory"
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

	reads := readmanifest.NewSet()
	files, err := AutoLoadedFiles(root, home, reads)
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
	wantMemPath := "~/" + strings.TrimPrefix(strings.TrimPrefix(memDir, home), string(filepath.Separator)) + "/MEMORY.md"
	if claudeMemPath != filepath.ToSlash(wantMemPath) {
		t.Errorf("claude-auto-memory display path = %q, want %q", claudeMemPath, filepath.ToSlash(wantMemPath))
	}
	if m := reads.Manifest(); !m.Complete() {
		t.Errorf("manifest reports incomplete for a run with no unreadable directory: %v", m.Unreadable())
	}

	// No home directory: auto-memory is skipped, not an error.
	files2, err := AutoLoadedFiles(root, "", readmanifest.NewSet())
	if err != nil {
		t.Fatalf("AutoLoadedFiles with no home: %v", err)
	}
	for _, f := range files2 {
		if f.Kind == "claude-auto-memory" {
			t.Errorf("AutoLoadedFiles with home=\"\" still found claude-auto-memory: %+v", f)
		}
	}
}

// TestAutoLoadedFilesNamesUnreadableDirectory is BLOCK item 1: an
// unreadable memory-bank/ must show up in the manifest as unreadable,
// never silently read as "no Cline memory bank here".
func TestAutoLoadedFilesNamesUnreadableDirectory(t *testing.T) {
	root := t.TempDir()
	// A real (if empty) git repo so AgentConfigFiles takes the git
	// ls-files path — `git` silently tolerates an unreadable subdirectory
	// (a warning on stderr, exit 0) — rather than the non-git fallback's
	// plain fs.WalkDir, which would itself fail descending into the
	// chmod-000 directory below and mask what this test means to check.
	initGitRepo(t, root)
	blockedDir := filepath.Join(root, "memory-bank")
	mustWriteCF(t, filepath.Join(blockedDir, "notes.md"), "notes\n")
	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blockedDir, 0o755) })

	if _, err := os.ReadDir(blockedDir); err == nil {
		t.Skip("this process can still list a 0000 directory — running as root or on a filesystem that ignores mode; skipping")
	}

	reads := readmanifest.NewSet()
	files, err := AutoLoadedFiles(root, "", reads)
	if err != nil {
		t.Fatalf("AutoLoadedFiles: %v", err)
	}
	for _, f := range files {
		if f.Kind == "memory-bank" {
			t.Errorf("found memory-bank file %+v despite the directory being unreadable", f)
		}
	}

	m := reads.Manifest()
	if m.Complete() {
		t.Fatal("manifest reports complete despite an unreadable memory-bank/ directory")
	}
	found := false
	for _, e := range m.Unreadable() {
		if e.Path == "memory-bank" {
			found = true
		}
	}
	if !found {
		t.Errorf("manifest.Unreadable() = %v, want an entry for %q", m.Unreadable(), "memory-bank")
	}
}

// TestAuditCodeFactsNamesUnreadableFiles is ADR-027's lesson applied to
// this row: a file the audit cannot read must be named, never silently
// dropped from the count — via the shared read-manifest mechanism, not a
// bespoke unreadable list.
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

	_, coverage, err := AuditCodeFacts(root, "")
	if err != nil {
		t.Fatalf("AuditCodeFacts: %v", err)
	}
	if coverage.Complete() {
		t.Fatal("coverage reports complete despite an unreadable AGENTS.md")
	}
	bad := coverage.Unreadable()
	if len(bad) != 1 || bad[0].Path != "AGENTS.md" {
		t.Errorf("coverage.Unreadable() = %v, want [AGENTS.md] — an unreadable auto-loaded file must be named, not dropped (ADR-027)", bad)
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

	findings, coverage, err := AuditCodeFacts(root, "")
	if err != nil {
		t.Fatalf("AuditCodeFacts: %v", err)
	}
	if !coverage.Complete() {
		t.Fatalf("coverage = %v, want complete", coverage.Unreadable())
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

// TestUnreadableGoFileYieldsUnanchorableNotDrifted is BLOCK item 2: an
// unreadable .go file inside checkSymbolAnchor's walk must never
// manufacture a false anchored-drifted — the honest verdict is that the
// search could not complete.
func TestUnreadableGoFileYieldsUnanchorableNotDrifted(t *testing.T) {
	root := t.TempDir()
	mustWriteCF(t, filepath.Join(root, "internal", "widget", "widget.go"),
		"package widget\n\nfunc Other() {}\n")
	blocked := filepath.Join(root, "internal", "widget", "blocked.go")
	mustWriteCF(t, blocked, "package widget\n\nfunc Compute(x int) int { return x }\n")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })

	if data, err := os.ReadFile(blocked); err == nil {
		t.Skipf("this process can still read a 0000 file (%d bytes) — running as root or on a filesystem that ignores mode; skipping", len(data))
	}

	class, receipt := classifySentence(root, "", "`Compute` lives in `internal/widget`.")
	if class != ClassUnanchorable {
		t.Fatalf("class = %s (receipt %q), want %s — an unreadable .go file must never manufacture a false drift", class, receipt, ClassUnanchorable)
	}
	if !strings.Contains(receipt, "blocked.go") {
		t.Errorf("receipt %q does not name the unreadable file", receipt)
	}
}

// TestBareFilenamePrefersSourceFileDirectory is FIX-BEFORE-MERGE item 5:
// a bare filename (no slash) must resolve relative to its OWN citing
// file before falling back to the repo root's same-named file.
func TestBareFilenamePrefersSourceFileDirectory(t *testing.T) {
	root := t.TempDir()
	mustWriteCF(t, filepath.Join(root, "CLAUDE.md"), "root one\nroot two\n") // 2 lines
	mustWriteCF(t, filepath.Join(root, "cmd", "widget", "CLAUDE.md"),
		"a\nb\nc\nd\ne\nf\ng\nh\ni\n") // 9 lines

	class, receipt := classifySentence(root, "cmd/widget/CLAUDE.md", "`CLAUDE.md` is 9 lines.")
	if class != ClassAnchoredFresh {
		t.Fatalf("class = %s (receipt %q), want anchored-fresh — a nested CLAUDE.md claiming its own line count must resolve to itself, not the root file of the same name", class, receipt)
	}
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable, skipping: %v: %s", err, out)
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
