package main

// stophook_budget_test.go — the Stop hook's work is bounded by counts, not
// by a clock. doctor's hook_budget row read stop-hook p99 8.9 s against its
// 8.0 s deadline (O140): the hook spawned one `git show` per plan file
// (89 in this repository, ~7 ms each idle and several times that under
// load) and asked the daemon who the repository is before it knew any plan
// had gained a claim. These tests pin the two counts that make the cost
// independent of how many plans a repository carries: git spawns for the
// plan scan, and daemon requests when nothing was claimed. A wall-clock
// assertion would flip under the same load that caused the finding.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// gitSpawnLog puts a `git` shim first on PATH that records each
// invocation's arguments (one line per spawn) before running the real git,
// and returns the log's path. The count of lines is the spawn count.
func gitSpawnLog(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	shim := t.TempDir()
	log := filepath.Join(t.TempDir(), "git-spawns.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// gitSpawns returns the recorded invocations (arguments, one per spawn).
func gitSpawns(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// plansRepo commits n plan files (each with one claimed task) plus one
// whose name carries a space, and returns the repository root.
func plansRepo(t *testing.T, n int) (dir string, rels []string) {
	t.Helper()
	dir = t.TempDir()
	plans := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"with space.md"}
	for i := 0; i < n; i++ {
		names = append(names, fmt.Sprintf("p%02d.md", i))
	}
	for _, name := range names {
		body := fmt.Sprintf("- [x] Done {id: %s-1, symbol: Real}\n- [ ] Open {id: %s-2, symbol: Later}\n", strings.TrimSuffix(strings.ReplaceAll(name, " ", "-"), ".md"), strings.TrimSuffix(strings.ReplaceAll(name, " ", "-"), ".md"))
		if err := os.WriteFile(filepath.Join(plans, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, filepath.Join("docs", "plans", name))
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir, rels
}

// TestHeadFileVersionsIsOneSpawn pins the plan scan's git cost: every plan
// file's HEAD version comes from one process, however many plans there
// are, and each answer is the bytes `git show HEAD:<rel>` gives (nil when
// HEAD has no such file). The missing file is asked first, so its bodiless
// "missing" answer must not end the read for the files after it.
func TestHeadFileVersionsIsOneSpawn(t *testing.T) {
	dir, rels := plansRepo(t, 40)
	log := gitSpawnLog(t)

	asked := append([]string{filepath.Join("docs", "plans", "never-committed.md")}, rels...)
	got := headFileVersions(dir, asked)
	if n := len(gitSpawns(t, log)); n != 1 {
		t.Fatalf("headFileVersions over %d files spawned git %d times, want exactly 1", len(asked), n)
	}
	for _, rel := range rels {
		want, err := exec.Command("git", "-C", dir, "show", "HEAD:"+filepath.ToSlash(rel)).Output()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[rel], want) {
			t.Errorf("%s: batch read = %q, git show = %q", rel, got[rel], want)
		}
	}
	if v, ok := got[filepath.Join("docs", "plans", "never-committed.md")]; ok && v != nil {
		t.Errorf("a file HEAD does not have must read as nil, got %q", v)
	}
}

// TestHeadFileVersionsOutsideGitIsNil: not a repository (or git missing) is
// the same "nil" headFileVersion gives, never an error the hook must handle.
func TestHeadFileVersionsOutsideGitIsNil(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	got := headFileVersions(dir, []string{"docs/plans/a.md", "docs/plans/b.md"})
	for rel, v := range got {
		if v != nil {
			t.Errorf("%s outside a repository = %q, want nil", rel, v)
		}
	}
}

// TestPlansWithNewClaimsReadsAnUppercaseClaim: the scan's cheap "[x]" gate
// must not skip a plan whose only claim is written "[X]", which the plan
// grammar reads as claimed too.
func TestPlansWithNewClaimsReadsAnUppercaseClaim(t *testing.T) {
	dir, _ := plansRepo(t, 1)
	p := filepath.Join(dir, "docs", "plans", "upper.md")
	if err := os.WriteFile(p, []byte("- [X] Done {id: U-1, symbol: Real}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "docs", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range plansWithNewClaims(dir, entries) {
		got = append(got, c.rel+":"+strings.Join(c.newly, ","))
	}
	if want := filepath.Join("docs", "plans", "upper.md") + ":U-1"; strings.Join(got, " ") != want {
		t.Errorf("plansWithNewClaims = %v, want [%s]", got, want)
	}
}

// countingStopDaemon is fakeStopDaemon that counts requests, and verify_plan
// requests among them.
func countingStopDaemon(t *testing.T, service string) (*httptest.Server, *atomic.Int64, *atomic.Int64) {
	t.Helper()
	var all, verifies atomic.Int64
	inner := fakeStopDaemon(t, service)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		all.Add(1)
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "verify_plan" {
			verifies.Add(1)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(func() { front.Close(); inner.Close() })
	return front, &all, &verifies
}

// TestStopHookScanCostIsIndependentOfPlanCount pins both counts: a session
// that claimed nothing costs one git spawn and zero daemon requests however
// many plans exist, and one newly claimed plan costs exactly one verify.
func TestStopHookScanCostIsIndependentOfPlanCount(t *testing.T) {
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	t.Setenv("FATHOM_IDENT_NO_CACHE", "1")
	dir, _ := plansRepo(t, 40)
	svc := strings.ToLower(filepath.Base(dir))
	srv, requests, verifies := countingStopDaemon(t, svc)
	hc := &http.Client{Timeout: 2 * time.Second}
	log := gitSpawnLog(t)

	// Nothing claimed since HEAD: silent, no daemon, one batch read.
	var out bytes.Buffer
	code, catches := runStopHook(dir, hc, srv.URL, false, &out)
	if code != 0 || out.Len() != 0 || catches != nil {
		t.Fatalf("nothing claimed: exit %d, output %q, catches %v — want a silent exit 0 with unknown catches", code, out.String(), catches)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("nothing was claimed but the hook made %d daemon request(s); it must not ask the daemon anything until a plan gained a claim", n)
	}
	var reads, shows int
	for _, sp := range gitSpawns(t, log) {
		switch {
		case strings.Contains(sp, "cat-file"):
			reads++
		case strings.HasPrefix(sp, "-C") && strings.Contains(sp, " show "), strings.HasPrefix(sp, "show "):
			shows++
		}
	}
	if reads != 1 || shows != 0 {
		t.Errorf("plan scan git spawns: %d cat-file, %d show — want 1 cat-file (one process for all 41 plans) and no per-plan show", reads, shows)
	}

	// One plan gains a claim: it alone is verified.
	p := filepath.Join(dir, "docs", "plans", "p07.md")
	if err := os.WriteFile(p, []byte("- [x] Done {id: p07-1, symbol: Real}\n- [x] Open {id: T-1, symbol: Ghost}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code, catches = runStopHook(dir, hc, srv.URL, false, &out)
	if code != 0 {
		t.Fatalf("claimed plan: exit %d", code)
	}
	if catches == nil {
		t.Error("a verified plan must report its catch count, not unknown")
	}
	if n := verifies.Load(); n != 1 {
		t.Errorf("verify_plan calls = %d, want 1 (only the plan that gained a claim)", n)
	}
	if !strings.Contains(out.String(), "T-1") {
		t.Errorf("the newly claimed failing task must still be named: %q", out.String())
	}
}

// TestStopHookRetroIsSkippedPastTheDeadline is RT-7's rule applied to the
// last local work the Stop hook does: once the deadline has passed the retro
// (a whole beat-log read) is not started, so the hook exits instead of
// running past its budget. The ledger row is measurement and is written
// regardless (stophook.go).
func TestStopHookRetroIsSkippedPastTheDeadline(t *testing.T) {
	repo := t.TempDir()
	const sessionID = "deadline-session"
	t.Setenv("FATHOM_HOOKS_LOG", writeBeatsFixture(t, []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "answered", At: time.Now().UTC(), Meta: map[string]string{
			"session": sessionID, "project_dir": repo, "emitted": "true", "phase": "post", "file": "main.go",
		}},
	}))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "ledger.jsonl"))
	artifact := filepath.Join(statedir.Resolve(repo), "artifacts", "retros", "deadline.json")

	spent := &hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now().Add(-time.Minute), 10)}
	if !spent.Deadline.Spent() {
		t.Fatal("test setup: the deadline must already be spent")
	}
	restore := activateHookGuard(spent)
	runStopHookRetro(repo, sessionID)
	restore()
	if _, err := os.Stat(artifact); err == nil {
		t.Fatal("the retro ran after the hook's deadline was spent; it must be skipped")
	}

	// Control: with time left (no guard) the same call lands the artifact.
	runStopHookRetro(repo, sessionID)
	if _, err := os.Stat(artifact); err != nil {
		t.Fatalf("control: expected the retro artifact at %s: %v", artifact, err)
	}
}
