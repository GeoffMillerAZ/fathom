package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// labsFixtureProjects writes one transcript into a project directory
// named the way Claude Code encodes this repository's path, and points
// readoutProjectsDir at its root.
func labsFixtureProjects(t *testing.T, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	repo := labsFixtureRepo(t)
	encoded := labsEncodeProject(repo)
	dir := filepath.Join(root, encoded)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "session-one.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := readoutProjectsDir
	readoutProjectsDir = func() string { return root }
	t.Cleanup(func() { readoutProjectsDir = previous })
	return root
}

// labsFixtureRepo is a git repository the corpus can own, entered for the
// test's duration so the verb reads it as the subject.
func labsFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	labsGit(t, dir, "init")
	t.Chdir(dir)
	top := gitToplevel(dir)
	if top == "" {
		t.Fatalf("fixture repo %s has no toplevel", dir)
	}
	return top
}

func labsGit(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git: %s %v", b, err)
	}
}

const labsAssistantLine = `{"cwd":"fixture-repo","gitBranch":"fixture-branch","timestamp":"2026-10-01T12:00:00Z","type":"assistant","message":{"content":[{"type":"tool_use","id":"r1","name":"Read","input":{"file_path":"a.go","offset":1,"limit":20}},{"type":"tool_use","id":"r2","name":"mcp__fathom__read_source","input":{"service":"fathom","file":"a.go","start_line":1,"end_line":20}}]}}`

const labsResultLine = `{"cwd":"fixture-repo","gitBranch":"fixture-branch","timestamp":"2026-10-01T12:00:01Z","type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"r2","content":"hello"}]}}`

func TestCmdLabsCorpusWritesSessionsAndPrintsAggregates(t *testing.T) {
	labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	out := filepath.Join(t.TempDir(), "sessions")
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"corpus", "--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs corpus = %d: %s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"labs corpus: 1 project(s), 1 transcript(s), 1 session(s), 2 decision point(s)", "bounded_reread: 2", "corpus sha: "} {
		if !strings.Contains(got, want) {
			t.Fatalf("labs corpus output missing %q:\n%s", want, got)
		}
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 1 {
		t.Fatalf("session files: %v %+v", err, entries)
	}
	// A session file is named by its hash and carries no transcript text.
	if strings.Contains(entries[0].Name(), "session-one") {
		t.Fatalf("session id reached the filesystem: %s", entries[0].Name())
	}
	body, err := os.ReadFile(filepath.Join(out, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"a.go", "fixture-branch", "session-one"} {
		if strings.Contains(string(body), secret) {
			t.Fatalf("session file retained %q: %s", secret, body)
		}
	}
}

func TestCmdLabsCorpusJSONIsDeterministic(t *testing.T) {
	labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	out := filepath.Join(t.TempDir(), "sessions")
	read := func() string {
		var stdout, stderr bytes.Buffer
		if code := cmdLabs([]string{"corpus", "--out", out, "--json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("labs corpus --json = %d: %s", code, stderr.String())
		}
		return stdout.String()
	}
	first, second := read(), read()
	if first != second {
		t.Fatalf("two readings of one corpus differ:\n%s\n%s", first, second)
	}
	var reading struct {
		Files    int `json:"files"`
		Sessions int `json:"sessions"`
		Tally    struct {
			Points          int `json:"points"`
			ShouldFire      int `json:"should_fire"`
			TreeUnrecovered int `json:"tree_unrecovered"`
		} `json:"tally"`
		CorpusSHA string `json:"corpus_sha"`
	}
	if err := json.Unmarshal([]byte(first), &reading); err != nil {
		t.Fatal(err)
	}
	if reading.Files != 1 || reading.Sessions != 1 || reading.Tally.Points != 2 || reading.Tally.ShouldFire != 2 {
		t.Fatalf("reading: %+v", reading)
	}
	if reading.Tally.TreeUnrecovered != 2 || len(reading.CorpusSHA) != 64 {
		t.Fatalf("provenance: %+v", reading)
	}
}

func TestCmdLabsFireReadsTheFunnelAndWritesTheLedger(t *testing.T) {
	labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	cards := filepath.Join(t.TempDir(), "labs.d")
	if err := os.MkdirAll(cards, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cards, "lab-read-source.yaml"), []byte(labsReadSourceCard), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"fire", "--cards", cards}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs fire = %d: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "lab-read-source mcp:read_source: fired at 1 of 2 eligible point(s)") || !strings.Contains(got, "Read served 1 missed point(s)") {
		t.Fatalf("labs fire output:\n%s", got)
	}

	// --json encodes: a rate with no data is NaN and must not reach the encoder.
	stdout.Reset()
	stderr.Reset()
	if code := cmdLabs([]string{"fire", "--cards", cards, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs fire --json = %d: %s", code, stderr.String())
	}
	var fired struct {
		Readings []struct {
			Surface string `json:"surface"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &fired); err != nil || len(fired.Readings) != 1 || fired.Readings[0].Surface != "mcp:read_source" {
		t.Fatalf("labs fire --json: %v %s", err, stdout.String())
	}

	// --write needs the preregistration keys the row is stamped with.
	stdout.Reset()
	stderr.Reset()
	ledger := filepath.Join(t.TempDir(), "docs", "bench", "labs-ledger.jsonl")
	if code := cmdLabs([]string{"fire", "--cards", cards, "--ledger", ledger, "--write"}, &stdout, &stderr); code != 1 {
		t.Fatalf("labs fire --write without a run id = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--run-id") {
		t.Fatalf("missing flag not named: %s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	sha := strings.Repeat("a", 64)
	args := []string{"fire", "--cards", cards, "--ledger", ledger, "--write", "--run-id", "labs-replay-1", "--criterion", "firing", "--prereg-sha", sha}
	previous := labsCommit
	labsCommit = func() string { return "0123456789ab" }
	t.Cleanup(func() { labsCommit = previous })
	code := cmdLabs(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("labs fire --write = %d: %s", code, stderr.String())
	}
	body, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), `{"run_id":"labs-replay-1","task_id":"mcp:read_source","arm":"replay"`) {
		t.Fatalf("ledger row: %s", body)
	}
	if strings.Contains(string(body), `"pins"`) {
		t.Fatalf("a replay row pinned what it cannot know: %s", body)
	}

	// One row per (run, surface): a second write under the key is refused.
	stdout.Reset()
	stderr.Reset()
	if code := cmdLabs(args, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "duplicate lab result") {
		t.Fatalf("second write = %d: %s", code, stderr.String())
	}
}

const labsReadSourceCard = `id: "lab-read-source"
features: ["F-08"]
surface: ["mcp:read_source"]
firing:
  should: "re-reading a cited line range"
  should_not: "open discovery"
  grader: "tool_used"
helped:
  metric: "bytes read per correct answer"
  arm: "plugin-minus-read-source"
  direction: "decrease"
  minimum_effect_of_interest: 1
quality: []
fixtures: ["internal/connector/testdata/fauxtown"]
harness: ["replay"]
kill: [{"criterion": "firing improves but helped is harmed", "reading": "docs/bench/readings/labs/lab-read-source.md"}]
knobs: []
owners: []
`

// M1 (I38): a project subdirectory the walk cannot open is counted, and
// the totals are printed as lower bounds.
func TestCmdLabsCorpusCountsAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 directory")
	}
	root := labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("fixture: %v %+v", err, entries)
	}
	locked := filepath.Join(root, entries[0].Name(), "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"corpus", "--out", filepath.Join(t.TempDir(), "s")}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs corpus = %d: %s", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "1 unreadable file(s)") || !strings.Contains(got, "\u2265 2 decision point(s)") {
		t.Fatalf("an unreadable directory was not counted or bounded:\n%s", got)
	}
	if strings.Contains(got, locked) {
		t.Fatalf("a path reached the output:\n%s", got)
	}
}

// M1: a transcript whose bytes cannot be digested is counted, not dropped.
func TestCmdLabsCorpusCountsAnUnreadableTranscript(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 file")
	}
	root := labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	entries, _ := os.ReadDir(root)
	file := filepath.Join(root, entries[0].Name(), "session-one.jsonl")
	if err := os.Chmod(file, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file, 0o600) })
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"corpus", "--out", filepath.Join(t.TempDir(), "s")}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs corpus = %d: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "1 unreadable file(s)") || !strings.Contains(got, "\u2265") {
		t.Fatalf("an unreadable transcript was not bounded:\n%s", got)
	}
}

// M2: the default glob is the encoded repository path, exactly or as a
// Claude Code worktree of it, never a sibling that shares the prefix.
func TestLabsProjectDirEncodingAndMatch(t *testing.T) {
	enc := labsEncodeProject("/a/b.c/my_repo")
	if enc != "-a-b-c-my-repo" {
		t.Fatalf("encoding = %q", enc)
	}
	for name, want := range map[string]bool{
		enc:                                  true,
		enc + "--claude-worktrees-feature-x": true,
		enc + "-sibling":                     false,
		enc + "2":                            false,
		"x" + enc:                            false,
	} {
		if got := labsProjectDirMatches(enc, name); got != want {
			t.Errorf("match(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCmdLabsCorpusSkipsSiblingProjects(t *testing.T) {
	root := labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	entries, _ := os.ReadDir(root)
	sibling := filepath.Join(root, entries[0].Name()+"-sibling")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "other.jsonl"), []byte(labsAssistantLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"corpus", "--out", filepath.Join(t.TempDir(), "s")}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs corpus = %d: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "1 project(s), 1 transcript(s)") {
		t.Fatalf("a sibling repository was read:\n%s", got)
	}
}

// M2: a linked worktree reads the main worktree's transcripts.
func TestLabsMainRootFromALinkedWorktree(t *testing.T) {
	main := labsFixtureRepo(t)
	labsGit(t, main, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "init")
	linked := filepath.Join(t.TempDir(), "linked")
	labsGit(t, main, "worktree", "add", "-b", "wt", linked)
	t.Chdir(linked)
	top := gitToplevel(linked)
	if got := labsMainRoot(top); got != main {
		t.Fatalf("main root = %q, want %q", got, main)
	}
	if got := labsMainRoot(main); got != main {
		t.Fatalf("main root of the main worktree = %q, want %q", got, main)
	}
}

// m3: a relative --cards and --ledger resolve against the repository, not
// the working directory, and a --write that cannot append every row
// appends none.
func TestCmdLabsFireResolvesRelativePathsAndWritesAllOrNothing(t *testing.T) {
	labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	top := gitToplevel(".")
	cards := filepath.Join(top, "cards.d")
	if err := os.MkdirAll(cards, 0o755); err != nil {
		t.Fatal(err)
	}
	second := strings.NewReplacer("lab-read-source", "lab-read-source-two", "plugin-minus-read-source", "plugin-minus-read-source-two", "mcp:read_source", "mcp:search").Replace(labsReadSourceCard)
	for name, body := range map[string]string{"lab-read-source.yaml": labsReadSourceCard, "lab-read-source-two.yaml": second} {
		if err := os.WriteFile(filepath.Join(cards, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(top, "deep", "er")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	previous := labsCommit
	labsCommit = func() string { return "0123456789ab" }
	t.Cleanup(func() { labsCommit = previous })

	// The second surface's key is already in the ledger: nothing may be appended.
	ledger := filepath.Join(top, "ledger.jsonl")
	args := []string{"fire", "--cards", "cards.d", "--ledger", "ledger.jsonl", "--write", "--run-id", "run-1", "--criterion", "firing", "--prereg-sha", strings.Repeat("a", 64)}
	var stdout, stderr bytes.Buffer
	if code := cmdLabs(args, &stdout, &stderr); code != 0 {
		t.Fatalf("labs fire from a subdirectory = %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(sub, "ledger.jsonl")); err == nil {
		t.Fatal("the ledger was written under the working directory")
	}
	before, err := os.ReadFile(ledger)
	if err != nil || strings.Count(string(before), "\n") != 2 {
		t.Fatalf("ledger: %v %q", err, before)
	}
	// Keep only the second row, then write again: the first row would
	// append before the second is refused as a duplicate, so an
	// all-or-nothing write must append nothing.
	first := strings.SplitAfter(string(before), "\n")[1]
	if err := os.WriteFile(ledger, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := cmdLabs(args, &stdout, &stderr); code == 0 {
		t.Fatalf("a write over a duplicate key succeeded: %s", stdout.String())
	}
	after, _ := os.ReadFile(ledger)
	if string(after) != first {
		t.Fatalf("a refused write still appended:\n%s", after)
	}
}

// The decoder read the file but the digest failed: the file is counted
// unreadable and the corpus sha is bounded.
func TestCmdLabsCorpusCountsADigestOnlyFailure(t *testing.T) {
	labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	old := labsFileDigest
	labsFileDigest = func(string) (string, error) { return "", errors.New("digest failed") }
	t.Cleanup(func() { labsFileDigest = old })
	var stdout, stderr bytes.Buffer
	if code := cmdLabs([]string{"corpus", "--out", filepath.Join(t.TempDir(), "s")}, &stdout, &stderr); code != 0 {
		t.Fatalf("labs corpus = %d: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "1 unreadable file(s)") || !strings.Contains(got, "\u2265") {
		t.Fatalf("a digest-only failure was not counted:\n%s", got)
	}
}
