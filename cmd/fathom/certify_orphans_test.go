package main

// certify_orphans_test.go — TC-22's own test: TestCmdCertifyOrphans_ThreeConditions.
// Fixture: a real git repo (dated commits, so the 180-day window is
// exercised against ACTUAL git log --since, not a fake) with four Test*
// functions, each violating exactly one of the three orphan conditions
// except one:
//
//   - TestCertifies  -> OldFunc; carries "// certifies:" -> excluded (condition 1)
//   - TestOrphan     -> OldFunc; no marker, no failed run -> THE ORPHAN
//   - TestRecentReach -> NewFunc (committed inside the 180-day window)   -> excluded (condition 2)
//   - TestFailed     -> OldFunc; an ingested run with status "fail"      -> excluded (condition 3)
//
// old.go/orph_test.go/tests/orph.cue are committed together, OLD (200
// days before the injected "now") — the test FILE's own commit date must
// be old too, or every test would trivially "reach" a recently changed
// symbol by reaching itself (Reachable's own documented "a node trivially
// reaches itself"). new.go is committed separately, RECENT (10 days
// before "now"), so only NewFunc is a "changed in the last 180 days"
// symbol.
import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const certifyOrphansManifest = `
component: "orph"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

const certifyOrphansSrc = `package orph

// certifies: invariant I1 (asserts)
func TestCertifies(t *T) { OldFunc() }

func TestOrphan(t *T) { OldFunc() }

func TestRecentReach(t *T) { NewFunc() }

func TestFailed(t *T) { OldFunc() }

type T struct{}
`

const certifyOrphansOldSrc = "package orph\n\nfunc OldFunc() {}\n"
const certifyOrphansNewSrc = "package orph\n\nfunc NewFunc() {}\n"

func certifyOrphansEnvelope() string {
	return `{
  "suite": {"component": "orph", "name": "unit", "class": "unit"},
  "key": "k1",
  "verdict": "fail",
  "cases": [
    {"id": "1", "name": "TestCertifies", "status": "pass", "duration": "1ms", "attempts": 1},
    {"id": "2", "name": "TestFailed", "status": "fail", "duration": "1ms", "attempts": 1}
  ],
  "totals": {"run": 2, "passed": 1, "failed": 1, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "2026-09-23T00:00:00Z", "wallClock": "2ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "amd64", "os": "darwin"},
  "artifacts": []
}`
}

// gitAt runs git in dir with author/committer identity and date pinned to
// when — GIT_AUTHOR_DATE/GIT_COMMITTER_DATE, the standard override, so
// "git log --since" is exercised against a controlled, deterministic
// history instead of the real wall clock.
func gitAt(t *testing.T, dir string, when time.Time, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	ts := when.Format(time.RFC3339)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts,
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\nstdout: %s\nstderr: %s", args, err, out.String(), errBuf.String())
	}
}

// gitRunnerIn returns a gitRunner (runCertifyOrphans' own injected shape)
// that runs real git rooted at dir, mirroring defaultGitRunner but pinned
// to the fixture directory instead of the process cwd.
func gitRunnerIn(dir string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		return cmd.Output()
	}
}

func setUpCertifyOrphansFixture(t *testing.T, now time.Time) (root, storePath, service string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	gitAt(t, root, now, "init", "-q")
	gitAt(t, root, now, "config", "commit.gpgsign", "false")

	oldDate := now.AddDate(0, 0, -200)
	write("old.go", certifyOrphansOldSrc)
	write("orph_test.go", certifyOrphansSrc)
	write("tests/orph.cue", certifyOrphansManifest)
	gitAt(t, root, oldDate, "add", "old.go", "orph_test.go", "tests/orph.cue")
	gitAt(t, root, oldDate, "commit", "-q", "-m", "old + tests")

	recentDate := now.AddDate(0, 0, -10)
	write("new.go", certifyOrphansNewSrc)
	gitAt(t, root, recentDate, "add", "new.go")
	gitAt(t, root, recentDate, "commit", "-q", "-m", "recent")

	service = "orph-svc"
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
	}

	resultsPath := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(resultsPath, []byte(certifyOrphansEnvelope()), 0o644); err != nil {
		t.Fatal(err)
	}
	var tiOut, tiErr bytes.Buffer
	if code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root, "--results", resultsPath}, &tiOut, &tiErr); code != 0 {
		t.Fatalf("tests ingest exit = %d, stderr: %s", code, tiErr.String())
	}
	return root, storePath, service
}

func TestCmdCertifyOrphans_ThreeConditions(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	root, storePath, service := setUpCertifyOrphansFixture(t, now)

	var out, errBuf bytes.Buffer
	code := runCertifyOrphans(
		[]string{"--service", service, "--store", storePath, "--json"},
		gitRunnerIn(root), func() time.Time { return now }, &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("certify orphans exit = %d, stderr: %s", code, errBuf.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	orphans, _ := payload["orphans"].([]any)
	if len(orphans) != 1 {
		t.Fatalf("orphans = %+v, want exactly 1 (TestOrphan) — TestCertifies has a marker, TestRecentReach reaches recently-changed code, TestFailed has a failed run", payload["orphans"])
	}
	got := orphans[0].(map[string]any)
	if got["test"] != "TestOrphan" {
		t.Errorf("orphan[0].test = %v, want TestOrphan", got["test"])
	}
	if got["file"] != "orph_test.go" {
		t.Errorf("orphan[0].file = %v, want orph_test.go", got["file"])
	}
}

func TestCmdCertifyOrphans_NoneFound(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	root := t.TempDir()
	gitAt(t, root, now, "init", "-q")
	gitAt(t, root, now, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "empty.go"), []byte("package empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, now, "add", "empty.go")
	gitAt(t, root, now, "commit", "-q", "-m", "init")

	service := "empty-svc"
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	code := runCertifyOrphans([]string{"--service", service, "--store", storePath}, gitRunnerIn(root), func() time.Time { return now }, &out, &errBuf)
	if code != 0 {
		t.Fatalf("certify orphans exit = %d, stderr: %s", code, errBuf.String())
	}
	if got := out.String(); got != "certify orphans: none found\n" {
		t.Errorf("output = %q, want the none-found line", got)
	}
}
