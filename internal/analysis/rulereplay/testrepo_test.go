package rulereplay

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// testRepo is a synthetic git repository built under t.TempDir(), used to
// prove new-vs-inherited classification without touching any real
// checkout. requireGit skips the test outright when no git binary is on
// PATH (should never happen in this project's own gate, but keeps the
// test honest about its one external dependency).
type testRepo struct {
	t    *testing.T
	dir  string
	seq  int
	when time.Time
}

func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	r := &testRepo{t: t, dir: dir, when: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.name", "rulereplay-test")
	r.git("config", "user.email", "rulereplay-test@example.com")
	r.git("config", "commit.gpgsign", "false")
	return r
}

func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(
		os.Environ(),
		"GIT_AUTHOR_NAME=rulereplay-test", "GIT_AUTHOR_EMAIL=rulereplay-test@example.com",
		"GIT_COMMITTER_NAME=rulereplay-test", "GIT_COMMITTER_EMAIL=rulereplay-test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// write creates or overwrites rel (repo-relative) with content.
func (r *testRepo) write(rel, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commit stages everything and commits with msg, at a deterministic,
// strictly increasing author/committer date — listCommits parses %cI, and
// distinct dates make the fixture's timeline legible in a failure message.
func (r *testRepo) commit(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.seq++
	r.when = r.when.Add(time.Hour)
	date := r.when.Format(time.RFC3339)
	cmd := exec.Command("git", "-C", r.dir, "commit", "-q", "-m", msg)
	cmd.Env = append(
		os.Environ(),
		"GIT_AUTHOR_NAME=rulereplay-test", "GIT_AUTHOR_EMAIL=rulereplay-test@example.com",
		"GIT_COMMITTER_NAME=rulereplay-test", "GIT_COMMITTER_EMAIL=rulereplay-test@example.com",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git commit: %v\n%s", err, out)
	}
	return r.git("rev-parse", "HEAD")[:40]
}

// goMod writes a minimal go.mod declaring module.
func (r *testRepo) goMod(module string) {
	r.write("go.mod", "module "+module+"\n\ngo 1.24\n")
}

// guardCue writes .fathom/guard.cue with the given layer_rules body
// (already-formatted CUE list contents).
func (r *testRepo) guardCue(layerRules string) {
	r.write(".fathom/guard.cue", "guards: []\nlayer_rules: "+layerRules+"\n")
}

func (r *testRepo) guardCuePath() string {
	return filepath.Join(r.dir, ".fathom", "guard.cue")
}
