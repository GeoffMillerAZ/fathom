package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

var homeCorpusExcludePaths = []string{
	"testdata/contextbench/**",
	"testdata/scenariobench/**",
	"docs/bench/**",
}

// TestAnswerKeyLeakScan_DetectsExcludedPathInGrepResult is O53 defect 3's
// leak-scan proof: a Grep tool_result whose text names an excluded path
// must be caught even though prepare-time removal was supposed to have
// kept it out of the worktree entirely — the scan is the backstop for
// whatever removal missed (a symlink, a subject Setup that re-creates the
// file, a corpus author who forgot a glob).
func TestAnswerKeyLeakScan_DetectsExcludedPathInGrepResult(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "leak.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:3:  \"ground_truth\": \"fusion.New\""},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak to be detected")
	}
}

// TestAnswerKeyLeakScan_NoLeakReturnsFalse covers the converse: ordinary
// tool results naming none of the excluded paths must not false-positive.
func TestAnswerKeyLeakScan_NoLeakReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "clean.jsonl", []overlapRead{
		{Name: "Read", File: "internal/core/model.go", Result: "package core\n\ntype Edge struct{}\n"},
	})
	session := SessionResult{RawPath: path}
	if AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected no leak from an unrelated tool result")
	}
}

// TestAnswerKeyLeakScan_EmptyInputsAreFalse: no raw transcript, or no
// exclude_paths declared, is "not measurable", never a false positive —
// AND never a false "leak detected" that would wrongly void a cell whose
// task carries no exclusions at all (a task not derived from the home
// corpus).
func TestAnswerKeyLeakScan_EmptyInputsAreFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "x.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:1: leak"},
	})
	if AnswerKeyLeakScan(SessionResult{RawPath: ""}, homeCorpusExcludePaths) {
		t.Error("expected false with no RawPath")
	}
	if AnswerKeyLeakScan(SessionResult{RawPath: path}, nil) {
		t.Error("expected false with no excludePaths")
	}
}

// TestAnswerKeyLeakScan_BashFindOutputIsCaught: the same defect the
// vs-pilot-3 evidence names — a Bash tool result (a `find`, a directory
// listing) can carry the excluded path just as readily as Grep/Read. O65
// (wave N12) narrowed the RESULT-side check to lines ROOTED at the
// excluded path (resultTextNamesExcludedPath): a `find` listing is exactly
// that shape (optionally "./"-prefixed), unlike the unrealistic "$ cat
// <path>" echo this test used to carry — a real Bash tool_result never
// echoes the command it ran, only its output, so a `cat` of an excluded
// path is caught via its tool_use INPUT instead (see
// TestAnswerKeyLeakScan_BashCommandNamesExcludedPath below).
func TestAnswerKeyLeakScan_BashFindOutputIsCaught(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "bash-leak.jsonl", []overlapRead{
		{Name: "Bash", File: "n/a", Result: "./testdata/contextbench/tasks-home.json\n"},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak to be detected in a Bash find-style tool result rooted at the excluded path")
	}
}

// TestAnswerKeyLeakScan_ProseMentionInUnrelatedFileIsNotALeak is O65 (wave
// N12) item 1's own regression: vs-pilot-5's H1-T05/gopls/t1 read an
// ordinary worktree source file (not itself excluded) whose own comment
// happened to name an excluded path as documentation — a bare mention
// inside prose, not the answer key itself reaching the agent. The old
// substring-anywhere scan caught this; the line-rooted rule must not.
func TestAnswerKeyLeakScan_ProseMentionInUnrelatedFileIsNotALeak(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "prose-mention.jsonl", []goldStep{
		{
			Name:   "Read",
			Input:  `{"file_path":"/repo/internal/mcp/import.go"}`,
			Result: "package mcp\n\n// registerReadSource wires the read_source tool (import.go; docs/bench/public-loc.md\n// narrates its own budget rules).\nfunc registerReadSource() {}\n",
		},
	})
	session := SessionResult{RawPath: path}
	if AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected no leak: docs/bench/ is merely named in a comment of a file that is not itself excluded")
	}
}

// TestAnswerKeyLeakScan_GrepListingFileWhoseProseNamesExcludedPathIsNotALeak
// is O65 item 2's own regression: vs-pilot-5's H1-T07/graphify/t2 ran
// `grep -rl "ADR-020"`, whose own output is a bare list of matching FILE
// NAMES — one of them, docs/plans/archive/falsifiability.md, is not
// itself excluded, but that file's own prose (shown here as if a
// follow-up step surfaced it in the same result) mentions
// testdata/contextbench/tasks.json in running text, not as a listed path.
// Neither line is rooted at an excluded directory.
func TestAnswerKeyLeakScan_GrepListingFileWhoseProseNamesExcludedPathIsNotALeak(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "grep-listing.jsonl", []overlapRead{
		{Name: "Grep", File: "ADR-020", Result: "docs/plans/archive/falsifiability.md\n" +
			"the falsifiability check reads its fixtures from testdata/contextbench/tasks.json directly.\n"},
	})
	session := SessionResult{RawPath: path}
	if AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected no leak: the listed file is not excluded, and the excluded path is named only in that file's own prose, not as a rooted line")
	}
}

// TestSandboxEscapeScan_FindOutsideWorktreeIsCaught is O65 item 3's own
// proof: vs-pilot-5's H1-T05/graphify/t1 ran `find /private/tmp/claude-501
// -maxdepth 8 -iname "*fathom*"`, a command with no path argument
// confining it to the worktree, and its result walked OUTSIDE the
// worktree entirely, listing the operator's own push clone. This must
// register as a sandbox escape, distinct from (and not double-counted as)
// an answer-key leak, even though the listed path's tail also matches an
// exclude_paths pattern.
func TestSandboxEscapeScan_FindOutsideWorktreeIsCaught(t *testing.T) {
	dir := t.TempDir()
	escaped := "/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946/scratchpad/pushclone/testdata/scenariobench/guards-fathom.json"
	path := writeOverlapTranscript(t, dir, "sandbox-escape.jsonl", []overlapRead{
		{Name: "Bash", File: "n/a", Result: escaped + "\n"},
	})
	session := SessionResult{RawPath: path}
	if !SandboxEscapeScan(session) {
		t.Error("expected a sandbox escape to be detected from a path outside the worktree naming the operator's own push clone")
	}
	if AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected no answer_key_leak: the excluded sub-path is not rooted at the start of the line, it is buried inside a longer escaped path")
	}
}

// TestSandboxEscapeScan_NoEscapeReturnsFalse covers the converse: an
// ordinary in-worktree tool result never trips the sandbox-escape scan.
func TestSandboxEscapeScan_NoEscapeReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "no-escape.jsonl", []overlapRead{
		{Name: "Read", File: "internal/core/model.go", Result: "package core\n\ntype Edge struct{}\n"},
	})
	session := SessionResult{RawPath: path}
	if SandboxEscapeScan(session) {
		t.Error("expected no sandbox escape from an ordinary in-worktree tool result")
	}
}

// TestResultTextNamesExcludedPath covers the line-rooted matching rule
// directly: a needle must be the LEADING text of its own line (grep -n /
// grep -l / find shapes), never merely present somewhere inside it.
func TestResultTextNamesExcludedPath(t *testing.T) {
	needles := []string{"testdata/contextbench/", "docs/bench/"}
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"grep -n rooted hit", `testdata/contextbench/tasks-home.json:3:  "ground_truth": "fusion.New"`, true},
		{"grep -l bare listing", "testdata/contextbench/tasks-home.json", true},
		{"find ./-prefixed listing", "./testdata/contextbench/tasks-home.json", true},
		{"prose mention mid-line", "see docs/bench/public-loc.md for the budget", false},
		{"unrelated file listed, unrelated prose", "docs/plans/archive/falsifiability.md\nreads testdata/contextbench/tasks.json", false},
	}
	for _, c := range cases {
		if got := resultTextNamesExcludedPath(c.text, needles); got != c.want {
			t.Errorf("%s: resultTextNamesExcludedPath(%q) = %v, want %v", c.name, c.text, got, c.want)
		}
	}
}

// TestAnswerKeyLeakScan_ReadInputNamesExcludedPath is the fix-first
// review's own proof: a Read tool_use whose INPUT names an excluded path
// returns raw file content in its RESULT with no path mentioned anywhere
// — the earlier version scanned tool_result text only, so a plain Read
// of the answer key escaped the scan entirely.
func TestAnswerKeyLeakScan_ReadInputNamesExcludedPath(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "read-leak.jsonl", []goldStep{
		{
			Name:   "Read",
			Input:  `{"file_path":"/repo/testdata/contextbench/tasks-home.json"}`,
			Result: "     1\t{\n     2\t  \"H1-T01\": {\"ground_truth\": \"fusion.New\"}\n     3\t}\n",
		},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak from the Read tool_use's own input path, even though the result text names no path at all")
	}
}

// TestAnswerKeyLeakScan_BashCommandNamesExcludedPath covers the review's
// second required case: a Bash tool_use whose "command" INPUT (not its
// result) names an excluded path via `cat`.
func TestAnswerKeyLeakScan_BashCommandNamesExcludedPath(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "bash-cmd-leak.jsonl", []goldStep{
		{Name: "Bash", Input: `{"command":"cat testdata/contextbench/tasks-home.json"}`, Result: "(command not yet run)"},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak from the Bash tool_use's own command text, even though the result carries nothing")
	}
}

// TestContainsPathNeedle covers the word-boundary claim directly: a
// needle must not match as a mere substring of a differently-named
// directory that happens to end the same way.
func TestContainsPathNeedle(t *testing.T) {
	needle := "testdata/contextbench/"
	if !containsPathNeedle("cat testdata/contextbench/tasks-home.json", needle) {
		t.Error("expected a match on an ordinary path reference")
	}
	if containsPathNeedle("cat mytestdata/contextbench/tasks-home.json", needle) {
		t.Error("expected no match: a differently-named directory merely ending the same way")
	}
	if !containsPathNeedle("testdata/contextbench/tasks-home.json", needle) {
		t.Error("expected a match at the very start of text")
	}
}

// TestExcludeNeedle covers the glob-to-literal conversion directly.
func TestExcludeNeedle(t *testing.T) {
	cases := []struct{ pattern, want string }{
		{"testdata/contextbench/**", "testdata/contextbench/"},
		{"docs/bench/**", "docs/bench/"},
		{"exact/file.json", "exact/file.json"},
	}
	for _, c := range cases {
		if got := excludeNeedle(c.pattern); got != c.want {
			t.Errorf("excludeNeedle(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

// TestDerivesFromHomeCorpus / TestExcludePathsCoverHomeCorpus cover the
// corpus-validation helpers directly (corpus_test.go covers them again
// end-to-end through LoadScenarioCorpus).
func TestDerivesFromHomeCorpus(t *testing.T) {
	yes := Task{Derivation: "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01."}
	no := Task{Derivation: "New task, not from tasks-home.json."}
	if !derivesFromHomeCorpus(yes) {
		t.Error("expected derivesFromHomeCorpus(yes) = true")
	}
	if derivesFromHomeCorpus(no) {
		t.Error("expected derivesFromHomeCorpus(no) = false")
	}
}

func TestExcludePathsCoverHomeCorpus(t *testing.T) {
	if missing := excludePathsCoverHomeCorpus(homeCorpusExcludePaths); len(missing) != 0 {
		t.Errorf("expected full coverage to report nothing missing, got %v", missing)
	}
	if missing := excludePathsCoverHomeCorpus([]string{"testdata/contextbench/**"}); len(missing) != 2 {
		t.Errorf("expected 2 missing globs, got %v", missing)
	}
	if missing := excludePathsCoverHomeCorpus(nil); len(missing) != 3 {
		t.Errorf("expected all 3 missing, got %v", missing)
	}
}

// TestSubjectIsFathomSelfHost / TestExcludePathsCoverSelfHost cover wave
// N14 item 7's own validation helpers directly (corpus_test.go covers
// them again end-to-end through LoadScenarioCorpus) — both repo_url
// spellings already committed in this repo's own corpora (SSH in
// guards-*.json, HTTPS in lookups-vs.json), case-insensitively.
func TestSubjectIsFathomSelfHost(t *testing.T) {
	cases := []struct {
		repoURL string
		want    bool
	}{
		{"git@github.com:GeoffMillerAZ/fathom.git", true},
		{"https://github.com/geoffmilleraz/fathom", true},
		{"https://github.com/cli/cli", false},
		{"", false},
	}
	for _, c := range cases {
		got := subjectIsFathomSelfHost(Task{Subject: Subject{RepoURL: c.repoURL}})
		if got != c.want {
			t.Errorf("subjectIsFathomSelfHost(%q) = %v, want %v", c.repoURL, got, c.want)
		}
	}
}

func TestExcludePathsCoverSelfHost(t *testing.T) {
	if missing := excludePathsCoverSelfHost([]string{"testdata/scenariobench/**", "docs/bench/**"}); len(missing) != 0 {
		t.Errorf("expected full coverage to report nothing missing, got %v", missing)
	}
	if missing := excludePathsCoverSelfHost([]string{"testdata/scenariobench/**"}); len(missing) != 1 {
		t.Errorf("expected 1 missing glob, got %v", missing)
	}
	if missing := excludePathsCoverSelfHost(nil); len(missing) != 2 {
		t.Errorf("expected both missing, got %v", missing)
	}
}

// TestRunSessions_AnswerKeyLeakLosesTheCell is the end-to-end proof: a
// session whose transcript leaks an excluded path gets AnswerKeyLeak set,
// and CellLost reports the cell lost because of it — exactly the "marks
// the cell answer_key_leak: true and LOST" the defect asks for.
func TestRunSessions_AnswerKeyLeakLosesTheCell(t *testing.T) {
	streamsTmp := t.TempDir()
	writeOverlapTranscript(t, streamsTmp, "leaky.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:3: ground truth here"},
	})

	wtDir := t.TempDir()
	task := Task{
		ID:       "T1",
		Prompt:   "do the thing",
		Subject:  Subject{ExcludePaths: homeCorpusExcludePaths},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
	}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsTmp,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "leaky.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if !results[0].AnswerKeyLeak {
		t.Error("expected AnswerKeyLeak = true")
	}
	reason := CellLost(results)
	if reason == "" {
		t.Fatal("expected CellLost to report this cell as lost")
	}
	if got := reason; got != "answer_key_leak: a tool result named a path subject.exclude_paths should have removed from the worktree" {
		t.Errorf("CellLost reason = %q", got)
	}
}

// TestRunSessions_NoLeakArmNotAffected is the converse of the end-to-end
// test: a clean transcript against the same excludePaths does not flag
// the session or lose the cell.
func TestRunSessions_NoLeakArmNotAffected(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{
		ID:       "T1",
		Prompt:   "do the thing",
		Subject:  Subject{ExcludePaths: homeCorpusExcludePaths},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
	}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "single-session.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if results[0].AnswerKeyLeak {
		t.Error("expected AnswerKeyLeak = false for a clean transcript")
	}
	if reason := CellLost(results); reason != "" {
		t.Errorf("expected the cell not to be lost, got %q", reason)
	}
}

// TestRunSessions_SandboxEscapeLosesTheCellDistinctly is O65 (wave N12)
// item 3's end-to-end proof: a session whose transcript names a path
// outside the worktree (the operator's own push clone) sets SandboxEscape,
// never AnswerKeyLeak, and CellLost reports it with its own distinct
// "sandbox_escape:" reason — not "answer_key_leak:", even though the
// escaped path's tail also matches an exclude_paths pattern.
func TestRunSessions_SandboxEscapeLosesTheCellDistinctly(t *testing.T) {
	streamsTmp := t.TempDir()
	escaped := "/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946/scratchpad/pushclone/testdata/scenariobench/guards-fathom.json"
	writeOverlapTranscript(t, streamsTmp, "escape.jsonl", []overlapRead{
		{Name: "Bash", File: "n/a", Result: escaped + "\n"},
	})

	wtDir := t.TempDir()
	task := Task{
		ID:       "T1",
		Prompt:   "do the thing",
		Subject:  Subject{ExcludePaths: homeCorpusExcludePaths},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
	}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsTmp,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "escape.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if !results[0].SandboxEscape {
		t.Error("expected SandboxEscape = true")
	}
	if results[0].AnswerKeyLeak {
		t.Error("expected AnswerKeyLeak = false: the escaped path is not rooted at the excluded directory")
	}
	reason := CellLost(results)
	if got := reason; got != "sandbox_escape: a tool input or result named a path outside the cell's own worktree (the operator's push clone, another wave clone, or benchroot)" {
		t.Errorf("CellLost reason = %q", got)
	}
}

// gitRepoForSparseCheckoutTest builds a small local git repository — files
// (relative path -> content) plus an always-present "marker.txt", all
// committed in one initial commit — for removeExcludedPaths' own tests,
// which apply a real git sparse checkout and so need a real repo, never a
// plain non-git directory (the fix's own requirement: "verify with a real
// git in the test").
func gitRepoForSparseCheckoutTest(t *testing.T, files map[string]string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "marker.txt"), "keep\n")
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, full, content)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	return dir
}

// TestRemoveExcludedPaths_NestedPatternIsFound proves the sparse-checkout
// application reaches a pattern anchored below root's own top-level
// entries — not just entries directly under root, the shape every real
// exclusion glob in this corpus actually is ("testdata/contextbench/**"
// sits two levels below the worktree root).
func TestRemoveExcludedPaths_NestedPatternIsFound(t *testing.T) {
	root := gitRepoForSparseCheckoutTest(t, map[string]string{
		"testdata/contextbench/tasks-home.json": "{}",
		"testdata/keep.txt":                     "keep",
	})
	nested := filepath.Join(root, "testdata", "contextbench", "tasks-home.json")
	kept := filepath.Join(root, "testdata", "keep.txt")

	if err := removeExcludedPaths(context.Background(), root, []string{"testdata/contextbench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths: %v", err)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Errorf("expected %s to be absent from disk, got err=%v", nested, err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("expected %s to survive: %v", kept, err)
	}
}

// TestRemoveExcludedPaths_EmptyPatternsIsNoop: a task with no exclusions
// declared (the ordinary case) must not touch the worktree at all.
func TestRemoveExcludedPaths_EmptyPatternsIsNoop(t *testing.T) {
	root := gitRepoForSparseCheckoutTest(t, nil)
	kept := filepath.Join(root, "marker.txt")
	if err := removeExcludedPaths(context.Background(), root, nil); err != nil {
		t.Fatalf("removeExcludedPaths: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("expected %s to survive a no-op call: %v", kept, err)
	}
}

// TestRemoveExcludedPaths_GitStatusCleanAndHeadUnchanged is O61's own
// proof: applying exclude_paths must leave `git status` with NOTHING to
// say about them (not "deleted: …", the defect that lost two of vs-pilot-4's
// first ten cells to the leak scanner below reading those literal names
// out of an agent's own `git status` tool result) and must never move HEAD
// — no synthetic commit, base_commit/LocalBase stay exactly what
// checkoutCommitIntoWorktree produced.
func TestRemoveExcludedPaths_GitStatusCleanAndHeadUnchanged(t *testing.T) {
	root := gitRepoForSparseCheckoutTest(t, map[string]string{
		"testdata/contextbench/tasks-home.json": `{"H1-T01":"answer"}`,
		"docs/bench/scenarios.md":               "# scenarios\n",
	})
	head := func() string {
		out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v", err)
		}
		return strings.TrimSpace(string(out))
	}
	before := head()

	if err := removeExcludedPaths(context.Background(), root, []string{
		"testdata/contextbench/**", "docs/bench/**",
	}); err != nil {
		t.Fatalf("removeExcludedPaths: %v", err)
	}

	statusOut, err := exec.Command("git", "-C", root, "status", "--short").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if got := string(statusOut); got != "" {
		t.Errorf("expected `git status --short` to be clean, got:\n%s", got)
	}
	if strings.Contains(string(statusOut), "testdata/contextbench") || strings.Contains(string(statusOut), "docs/bench") {
		t.Errorf("git status must name none of the excluded paths, got:\n%s", statusOut)
	}
	if after := head(); after != before {
		t.Errorf("HEAD moved: before=%s after=%s (sparse checkout must never add a commit)", before, after)
	}
}

// TestRemoveExcludedPaths_ReuseWithFewerExcludesRestoresFullTree covers
// PrepareSubject's own worktree reuse (one worktree per (subject,
// base_commit, arm), shared across the tasks and trials that share it,
// subject.go's own comment): a LATER task sharing the same physical
// worktree but declaring FEWER (or no) exclusions must see its own full
// tree, never a stale exclusion an earlier cell in the same worktree left
// active. `git sparse-checkout set` (a full pattern replacement, not an
// additive one) and `git sparse-checkout disable` both prove this.
func TestRemoveExcludedPaths_ReuseWithFewerExcludesRestoresFullTree(t *testing.T) {
	root := gitRepoForSparseCheckoutTest(t, map[string]string{
		"testdata/contextbench/tasks-home.json": "{}",
		"docs/bench/scenarios.md":               "# scenarios\n",
	})
	ctx := context.Background()

	// Cell 1: excludes both.
	if err := removeExcludedPaths(ctx, root, []string{"testdata/contextbench/**", "docs/bench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths (cell 1): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "testdata", "contextbench", "tasks-home.json")); !os.IsNotExist(err) {
		t.Fatalf("expected cell 1's exclusion to apply, got err=%v", err)
	}

	// Cell 2 reuses the SAME worktree (subject.go resets it between cells;
	// this test skips the reset since it does not touch what
	// removeExcludedPaths itself is responsible for) but excludes only one
	// of the two paths — the other must come BACK.
	if err := removeExcludedPaths(ctx, root, []string{"docs/bench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths (cell 2): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "testdata", "contextbench", "tasks-home.json")); err != nil {
		t.Errorf("expected cell 2 (which does not exclude testdata/contextbench) to restore it, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "bench", "scenarios.md")); !os.IsNotExist(err) {
		t.Errorf("expected cell 2's own exclusion (docs/bench) to still apply, got err=%v", err)
	}

	// Cell 3 declares NO exclusions at all — the full tree must return.
	if err := removeExcludedPaths(ctx, root, nil); err != nil {
		t.Fatalf("removeExcludedPaths (cell 3): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "bench", "scenarios.md")); err != nil {
		t.Errorf("expected cell 3 (no exclusions) to restore the full tree, got err=%v", err)
	}
}

// TestRemoveExcludedPaths_TwoWorktreesOfSameCloneDoNotInterfere addresses
// the fix's own worry about a shared clone's linked worktrees stepping on
// each other's sparse-checkout state (which would need
// extensions.worktreeConfig, git-worktree(1)). PrepareSubject's own cell
// worktrees are NOT linked worktrees of the shared clone at all —
// checkoutCommitIntoWorktree makes each one its OWN standalone repository
// via `git init` (subject.go), so this never applies; proved directly here
// by applying DIFFERENT exclusions to two independent
// gitRepoForSparseCheckoutTest repos built from equivalent content and
// checking neither's sparse-checkout state leaked into the other.
func TestRemoveExcludedPaths_TwoWorktreesOfSameCloneDoNotInterfere(t *testing.T) {
	files := map[string]string{
		"testdata/contextbench/tasks-home.json": "{}",
		"docs/bench/scenarios.md":               "# scenarios\n",
	}
	repoA := gitRepoForSparseCheckoutTest(t, files)
	repoB := gitRepoForSparseCheckoutTest(t, files)
	ctx := context.Background()

	if err := removeExcludedPaths(ctx, repoA, []string{"testdata/contextbench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths (repo A): %v", err)
	}
	if err := removeExcludedPaths(ctx, repoB, []string{"docs/bench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths (repo B): %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoA, "testdata", "contextbench", "tasks-home.json")); !os.IsNotExist(err) {
		t.Errorf("repo A: expected its own exclusion to apply, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repoA, "docs", "bench", "scenarios.md")); err != nil {
		t.Errorf("repo A: expected repo B's exclusion NOT to leak in, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repoB, "docs", "bench", "scenarios.md")); !os.IsNotExist(err) {
		t.Errorf("repo B: expected its own exclusion to apply, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repoB, "testdata", "contextbench", "tasks-home.json")); err != nil {
		t.Errorf("repo B: expected repo A's exclusion NOT to leak in, got err=%v", err)
	}
}

// TestRemoveExcludedPaths_BeltAndBracesRefusesLoudly proves the fix's own
// belt-and-braces requirement directly: when an excluded path is still on
// disk after sparse-checkout was applied (the shape a mid-directory
// pattern typo, or any other mismatch between internal/filter's matcher
// and git's own, would produce), removeExcludedPaths must fail the
// subject preparation outright — an instrument error surfaced immediately,
// never a cell that silently runs and is discovered LOST only later.
// findExcludedPathStillPresent (the check itself) is exercised directly,
// since reproducing a genuine git/filter mismatch through the real
// sparse-checkout path is not practical to construct.
func TestRemoveExcludedPaths_BeltAndBracesRefusesLoudly(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "testdata", "contextbench", "tasks-home.json")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := findExcludedPathStillPresent(root, []string{"testdata/contextbench/**"})
	if err != nil {
		t.Fatalf("findExcludedPathStillPresent: %v", err)
	}
	if found == "" {
		t.Fatal("expected a still-present excluded path to be reported, got none")
	}
}

// TestExcludeGlobToSparsePattern covers the glob-to-sparse-pattern
// translation directly: a pattern containing "/" is root-anchored (a
// leading "/" the caller already supplied is never doubled), a bare name
// matches at any depth — the same anchoring rule internal/filter's own
// compile() uses.
func TestExcludeGlobToSparsePattern(t *testing.T) {
	cases := []struct{ pattern, want string }{
		{"testdata/contextbench/**", "!/testdata/contextbench/**"},
		{"docs/bench/**", "!/docs/bench/**"},
		{"exact/file.json", "!/exact/file.json"},
		{"/already/anchored/**", "!/already/anchored/**"},
		{"answerkey.txt", "!answerkey.txt"},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := excludeGlobToSparsePattern(c.pattern); got != c.want {
			t.Errorf("excludeGlobToSparsePattern(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

// TestAnswerKeyLeakScan_GitShowHistoryMiningIsCaught is item 3's own proof:
// sparse checkout hides an excluded path from the WORKING TREE only —
// checkoutCommitIntoWorktree still gives every cell worktree a real commit
// whose tree contains the excluded blob, so `git show HEAD:<excluded>`
// still resolves it. The leak scanner still catches this: the mining
// COMMAND's own text (a Bash tool_use's "command" field) names the
// excluded path even though the command's RESULT (the file's raw content)
// does not — the same toolUseInputText path that already catches a plain
// Read. See answerkey.go's own file header for why closing the vector
// itself (not just catching a mining attempt) is out of this fix's scope.
func TestAnswerKeyLeakScan_GitShowHistoryMiningIsCaught(t *testing.T) {
	dir := t.TempDir()
	path := writeGoldTranscript(t, dir, "git-show-leak.jsonl", []goldStep{
		{
			Name:   "Bash",
			Input:  `{"command":"git show HEAD:testdata/contextbench/tasks-home.json"}`,
			Result: "{\"H1-T01\": {\"ground_truth\": \"fusion.New\"}}\n",
		},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak from the `git show HEAD:<excluded>` mining command's own text, even though its result names no path at all")
	}
}

// writeTestLedger records file as a KindKey ownership at repoRelPath's own
// path (mirroring what `fathom init`'s wiring actually records — subject_test.go's
// TestInstrumentOwnedPaths_ReadsTheLedgerPlusStatics) so HideInstrumentOwnedFilesFromStatus's
// tests never need a real `fathom init` run to exercise InstrumentOwnedPaths.
func writeTestLedger(t *testing.T, root, repoRelPath string) {
	t.Helper()
	ledger := wiring.NewLedger()
	ledger.Record(repoRelPath, wiring.OwnedKey{ID: "hooks", Digest: "sha256:test", Wrote: "2026-09-16T00:00:00Z"})
	if err := wiring.SaveLedger(root, ledger); err != nil {
		t.Fatalf("writing test ledger: %v", err)
	}
}

func gitStatusShort(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--short").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(out)
}

func gitDiffWorkingTree(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "diff").Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	return string(out)
}

// TestHideInstrumentOwnedFilesFromStatus_ScoresIdenticallyToFiltering is
// O61 item 2's own proof: applying `git update-index --assume-unchanged`
// to a tracked, ledger-owned file (.claude/settings.json — the exact case
// vs-pilot-4's `modified: .claude/settings.json` came from) hides any
// further rewrite of it from `git status`/`git diff` for the rest of the
// session, and the resulting (empty) scored contribution is identical to
// what the EXISTING mechanism (FilterInstrumentOwnedDiff, cell.go, O55
// item 4) already produces for the same rewrite when assume-unchanged is
// never applied at all — proving the new hiding step changes nothing about
// what gets scored, only what an agent's own `git status` shows mid-session.
func TestHideInstrumentOwnedFilesFromStatus_ScoresIdenticallyToFiltering(t *testing.T) {
	ctx := context.Background()
	const settingsRel = ".claude/settings.json"

	// repoA: HideInstrumentOwnedFilesFromStatus applied, then the file is
	// rewritten again — standing in for either the instrument's own later
	// write or an agent's own edit to it; assume-unchanged cannot and need
	// not tell the difference (see the function's own doc comment).
	repoA := gitRepoForSparseCheckoutTest(t, map[string]string{settingsRel: `{"hooks":{}}` + "\n"})
	writeTestLedger(t, repoA, settingsRel)
	if err := HideInstrumentOwnedFilesFromStatus(ctx, Worktree{Path: repoA}); err != nil {
		t.Fatalf("HideInstrumentOwnedFilesFromStatus: %v", err)
	}
	writeTestFile(t, filepath.Join(repoA, settingsRel), `{"hooks":{"changed":true}}`+"\n")
	// writeTestLedger's own .fathom/owned.json is untracked (a NEW file, the
	// ordinary case for a fresh cell — checkoutCommitIntoWorktree commits the
	// tree before `fathom init` ever runs) and so is entirely expected to
	// show as "??" here; it is not a "rewrite of a tracked file" (item 2's
	// own scope) and assume-unchanged has nothing to do with it. Only
	// settingsRel itself — the one path actually rewritten above — must be
	// absent from `git status`.
	if got := gitStatusShort(t, repoA); strings.Contains(got, settingsRel) {
		t.Errorf("expected `git status --short` to name nothing of %s after assume-unchanged, got:\n%s", settingsRel, got)
	}
	diffA := gitDiffWorkingTree(t, repoA)
	if diffA != "" {
		t.Errorf("expected `git diff` to show nothing for the assume-unchanged path, got:\n%s", diffA)
	}

	// repoB: the SAME rewrite, WITHOUT assume-unchanged — `git diff` shows
	// it, and the existing FilterInstrumentOwnedDiff (O55 item 4) drops it
	// wholesale. The scored diff this produces must match repoA's exactly.
	repoB := gitRepoForSparseCheckoutTest(t, map[string]string{settingsRel: `{"hooks":{}}` + "\n"})
	writeTestFile(t, filepath.Join(repoB, settingsRel), `{"hooks":{"changed":true}}`+"\n")
	diffB := gitDiffWorkingTree(t, repoB)
	if diffB == "" {
		t.Fatal("expected `git diff` (no assume-unchanged) to show the rewrite, got nothing")
	}
	filtered, excluded := FilterInstrumentOwnedDiff([]byte(diffB), map[string]bool{settingsRel: true}, nil)
	if excluded != 1 {
		t.Fatalf("expected FilterInstrumentOwnedDiff to exclude exactly 1 file section, got %d", excluded)
	}
	if string(filtered) != diffA {
		t.Errorf("scored diff diverges: assume-unchanged path produced %q, filtering path produced %q", diffA, string(filtered))
	}
}

// TestHideInstrumentOwnedFilesFromStatus_RegionFilesAreSkipped is the
// "skip with the reason" half of O61 item 2: CLAUDE.md/AGENTS.md must never
// receive `--assume-unchanged`, because — unlike a whole-file-owned path —
// only part of the file (the marked stanza) is the instrument's; an agent's
// own edit elsewhere in the same file must stay visible and scored. This
// proves the divergence directly: applying assume-unchanged to CLAUDE.md by
// hand (standing in for what HideInstrumentOwnedFilesFromStatus refuses to
// do itself) hides an agent's own out-of-region edit from `git diff`
// entirely, where the real region-aware path (StripInstrumentRegions)
// preserves it — the reason isInstrumentRegionFile exists.
func TestHideInstrumentOwnedFilesFromStatus_RegionFilesAreSkipped(t *testing.T) {
	ctx := context.Background()
	const claudeMD = "CLAUDE.md"
	base := wiring.HTMLMarkers.BeginPrefix + "\nfathom stanza\n" + wiring.HTMLMarkers.End + "\n# Real project notes\n"

	// isInstrumentRegionFile itself refuses CLAUDE.md/AGENTS.md.
	if !isInstrumentRegionFile(claudeMD) || !isInstrumentRegionFile("AGENTS.md") {
		t.Fatal("expected isInstrumentRegionFile to refuse CLAUDE.md and AGENTS.md")
	}

	// Even handed a ledger that (hypothetically) names CLAUDE.md, the real
	// function must still never touch it.
	repo := gitRepoForSparseCheckoutTest(t, map[string]string{claudeMD: base})
	writeTestLedger(t, repo, claudeMD)
	if err := HideInstrumentOwnedFilesFromStatus(ctx, Worktree{Path: repo}); err != nil {
		t.Fatalf("HideInstrumentOwnedFilesFromStatus: %v", err)
	}
	writeTestFile(t, filepath.Join(repo, claudeMD), base+"\nan agent's own new section\n")
	if got := gitStatusShort(t, repo); !strings.Contains(got, claudeMD) {
		t.Errorf("expected CLAUDE.md's own edit to still show in `git status`, got:\n%q", got)
	}
	realDiff := gitDiffWorkingTree(t, repo)
	if !strings.Contains(realDiff, "an agent's own new section") {
		t.Error("expected the agent's own out-of-region edit to survive into `git diff` since CLAUDE.md was never hidden")
	}

	// Contrast: if assume-unchanged WERE applied to the whole file (the
	// thing this fix refuses to do), that same out-of-region edit would
	// vanish from `git diff` too — proving why isInstrumentRegionFile exists
	// rather than merely asserting it.
	hidden := gitRepoForSparseCheckoutTest(t, map[string]string{claudeMD: base})
	if out, err := exec.Command("git", "-C", hidden, "update-index", "--assume-unchanged", "--", claudeMD).CombinedOutput(); err != nil {
		t.Fatalf("git update-index --assume-unchanged: %v (%s)", err, out)
	}
	writeTestFile(t, filepath.Join(hidden, claudeMD), base+"\nan agent's own new section\n")
	if got := gitDiffWorkingTree(t, hidden); got != "" {
		t.Errorf("expected assume-unchanged to hide even an out-of-region edit (demonstrating the risk), got:\n%s", got)
	}
}
