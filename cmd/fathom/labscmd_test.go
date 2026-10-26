package main

import (
	"bytes"
	"encoding/json"
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
	encoded := strings.ReplaceAll(filepath.ToSlash(repo), "/", "-")
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
	root := labsFixtureProjects(t, labsAssistantLine, labsResultLine)
	_ = root
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
