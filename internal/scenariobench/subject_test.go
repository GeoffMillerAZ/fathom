package scenariobench

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// gitFixtureRepo creates a local git repository with one commit (a file
// tracked at "marker.txt") and returns its path and that commit's sha —
// entirely local, so PrepareSubject's clone/worktree-add path is testable
// with no network.
func gitFixtureRepo(t *testing.T) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
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
	writeTestFile(t, filepath.Join(dir, "marker.txt"), "hello from the fixture subject\n")
	run("add", "marker.txt")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

func TestPrepareSubject_UnwiredArm(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{
		ID: "T1",
		Subject: Subject{
			RepoURL:    repoPath,
			BaseCommit: commit,
			Setup:      []string{"echo subject-setup-ran > subject-setup.marker"},
		},
	}
	arm := Arm{
		Wired:        false,
		Instructions: "none",
		Setup:        []string{"echo arm-setup-ran > arm-setup.marker"},
	}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if _, err := os.Stat(filepath.Join(wt.Path, "marker.txt")); err != nil {
		t.Errorf("expected the subject's file to be checked out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "subject-setup.marker")); err != nil {
		t.Errorf("expected the subject's Setup command to have run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "arm-setup.marker")); err != nil {
		t.Errorf("expected the arm's Setup command to have run: %v", err)
	}
	if wt.DaemonURL != "" {
		t.Errorf("expected no scratch daemon for an unwired arm, got DaemonURL=%q", wt.DaemonURL)
	}
	// RenderArm should have run: settings.json exists with no hooks key.
	if wt.Arm.SettingsPath == "" {
		t.Fatal("expected RenderArm to have run (SettingsPath set)")
	}
	raw, err := os.ReadFile(wt.Arm.SettingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if strings.Contains(string(raw), "hooks") {
		t.Errorf("unwired arm's settings.json should carry no hooks key: %s", raw)
	}

	if err := StopWorktree(&wt); err != nil {
		t.Errorf("StopWorktree on an unwired worktree should be a no-op, got: %v", err)
	}
}

func TestPrepareSubject_ReusesSharedClone(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}

	wt1, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (1st): %v", err)
	}
	defer StopWorktree(&wt1)

	wt2, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (2nd): %v", err)
	}
	defer StopWorktree(&wt2)

	if wt1.SubjectClone != wt2.SubjectClone {
		t.Errorf("expected both cells to share one clone, got %q and %q", wt1.SubjectClone, wt2.SubjectClone)
	}
	// Same (subject, commit, arm): the worktree is SHARED and reset between
	// cells, so a compiled store is paid for once; a different arm gets its
	// own. Whatever the first cell left behind must be gone.
	if wt1.Path != wt2.Path {
		t.Errorf("expected cells on one (subject, commit, arm) to share a worktree, got %q and %q", wt1.Path, wt2.Path)
	}
	if _, err := os.Stat(filepath.Join(wt2.Path, "marker.txt")); err != nil {
		t.Errorf("expected the second worktree to also have the subject's file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt2.Path, "leftover.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt3, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (3rd): %v", err)
	}
	defer StopWorktree(&wt3)
	if _, err := os.Stat(filepath.Join(wt3.Path, "leftover.txt")); !os.IsNotExist(err) {
		t.Errorf("a reused worktree must be cleaned between cells; leftover.txt survived (%v)", err)
	}
	other, err := PrepareSubject(context.Background(), task, Arm{Instructions: "plain", NotesFile: true}, root)
	if err != nil {
		t.Fatalf("PrepareSubject (other arm): %v", err)
	}
	defer StopWorktree(&other)
	if other.Path == wt1.Path {
		t.Error("a different arm must get its own worktree")
	}
}

// TestPrepareSubject_ConcurrentCallsShareOneCloneWithoutRacing is the
// -parallel-arms clone race's own proof: two cells of the same subject
// (as -parallel-arms' batching would produce — distinct arms, one shared
// cloneDir keyed only on repo_url) calling PrepareSubject at the same
// moment must both succeed and end up sharing one valid clone, never one
// racing `git clone` into the other's half-written target directory.
func TestPrepareSubject_ConcurrentCallsShareOneCloneWithoutRacing(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}

	const n = 8
	results := make([]Worktree, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			// Distinct arms (as batchCellPlans would run concurrently) so
			// each gets its OWN worktree — only the shared clone is meant
			// to be contended.
			arm := Arm{Instructions: "none", Setup: []string{fmt.Sprintf("echo arm-%d > arm.marker", i)}}
			results[i], errs[i] = PrepareSubject(context.Background(), task, arm, root)
		}()
	}
	wg.Wait()

	var clone string
	for i, err := range errs {
		if err != nil {
			t.Errorf("PrepareSubject[%d]: %v", i, err)
			continue
		}
		defer StopWorktree(&results[i])
		if clone == "" {
			clone = results[i].SubjectClone
		} else if results[i].SubjectClone != clone {
			t.Errorf("PrepareSubject[%d]: SubjectClone = %q, want %q (shared)", i, results[i].SubjectClone, clone)
		}
	}
	if clone == "" {
		t.Fatal("no successful PrepareSubject call to check the clone against")
	}
	if out, err := exec.Command("git", "-C", clone, "rev-parse", "HEAD").CombinedOutput(); err != nil {
		t.Errorf("shared clone %s is not a valid git repo: %v (%s)", clone, err, out)
	}
}

func TestPrepareSubject_UnknownCommitIsAnError(t *testing.T) {
	repoPath, _ := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: strings.Repeat("0", 40)}}
	if _, err := PrepareSubject(context.Background(), task, Arm{Instructions: "none"}, root); err == nil {
		t.Fatal("expected an error for a base_commit that does not exist")
	}
}

// TestRequiresCompiledLayer covers the O45 item 1 default: a task's own
// RequiresLayer wins when set; otherwise it defaults true for the
// lookup-corpus categories whose prompts assume a compiled layer, false
// otherwise.
func TestRequiresCompiledLayer(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name string
		task Task
		want bool
	}{
		{"obligations category defaults true", Task{Category: "obligations"}, true},
		{"staleness category defaults true", Task{Category: "staleness"}, true},
		{"claim_provenance category defaults true", Task{Category: "claim_provenance"}, true},
		{"verify category defaults true", Task{Category: "verify"}, true},
		{"blast_radius category defaults false", Task{Category: "blast_radius"}, false},
		{"unrelated category defaults false", Task{Category: "cross_package_feature"}, false},
		{"explicit false overrides an obligations default", Task{Category: "obligations", RequiresLayer: &no}, false},
		{"explicit true overrides a non-default category", Task{Category: "blast_radius", RequiresLayer: &yes}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requiresCompiledLayer(c.task); got != c.want {
				t.Errorf("requiresCompiledLayer = %v, want %v", got, c.want)
			}
		})
	}
}

// fakeObligationsDaemon returns an httptest.Server standing in for the
// scratch daemon's /rpc/agent, replying to a tools/call "obligations"
// request with rules (a JSON array of arbitrary objects, one per rule).
func fakeObligationsDaemon(t *testing.T, rules []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rulesJSON := "[" + strings.Join(rules, ",") + "]"
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"service":"fathom","file":"internal/core/model.go","rules":%s}}`, rulesJSON)
	}))
}

func TestProbeCompiledLayerServesRules_RulesPresent(t *testing.T) {
	srv := fakeObligationsDaemon(t, []string{`{"id":"R1","text":"some rule"}`})
	defer srv.Close()

	ok, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go")
	if err != nil {
		t.Fatalf("probeCompiledLayerServesRules: %v", err)
	}
	if !ok {
		t.Error("expected ok=true when the fake daemon returns a rule")
	}
}

func TestProbeCompiledLayerServesRules_ZeroRules(t *testing.T) {
	srv := fakeObligationsDaemon(t, nil)
	defer srv.Close()

	ok, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go")
	if err != nil {
		t.Fatalf("probeCompiledLayerServesRules: %v", err)
	}
	if ok {
		t.Error("expected ok=false when the fake daemon returns zero rules")
	}
}

func TestProbeCompiledLayerServesRules_DaemonError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"service not indexed"}}`)
	}))
	defer srv.Close()

	if _, err := probeCompiledLayerServesRules(context.Background(), srv.URL, "fathom", "internal/core/model.go"); err == nil {
		t.Fatal("expected an error when the daemon's own response carries an error")
	}
}

// TestApplyPostWireSetup_RunsSetupThenIngest proves the order O45 item 4
// requires: PostWireSetup's own commands run BEFORE runIngest is called —
// a fake "daemon" here is just runIngest recording what it saw, so no
// real binary or daemon is needed.
func TestApplyPostWireSetup_RunsSetupThenIngest(t *testing.T) {
	dir := t.TempDir()
	orderFile := filepath.Join(dir, "order.log")
	if err := os.WriteFile(orderFile, []byte("ingest-would-have-run-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := Worktree{Path: dir, CellDir: dir}
	task := Task{Subject: Subject{PostWireSetup: []string{"echo drift >> order.log"}}}

	var reingested bool
	runIngest := func(ctx context.Context) error {
		reingested = true
		raw, err := os.ReadFile(orderFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "ingest-would-have-run-here\ndrift") {
			t.Errorf("expected post_wire_setup's drift line AFTER the earlier ingest marker, got %q", raw)
		}
		return nil
	}
	if err := applyPostWireSetup(context.Background(), wt, task, runIngest); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
	if !reingested {
		t.Error("expected runIngest to be called for a wired cell")
	}
}

func TestApplyPostWireSetup_UnwiredCellSkipsReingest(t *testing.T) {
	dir := t.TempDir()
	wt := Worktree{Path: dir, CellDir: dir}
	task := Task{Subject: Subject{PostWireSetup: []string{"true"}}}
	if err := applyPostWireSetup(context.Background(), wt, task, nil); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
}

func TestApplyPostWireSetup_EmptyIsNoop(t *testing.T) {
	called := false
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	if err := applyPostWireSetup(context.Background(), wt, Task{}, func(context.Context) error { called = true; return nil }); err != nil {
		t.Fatalf("applyPostWireSetup: %v", err)
	}
	if called {
		t.Error("expected no re-ingest when PostWireSetup is empty")
	}
}

func TestStopWorktree_NilAndZeroValueAreNoOps(t *testing.T) {
	if err := StopWorktree(nil); err != nil {
		t.Errorf("StopWorktree(nil): %v", err)
	}
	wt := Worktree{}
	if err := StopWorktree(&wt); err != nil {
		t.Errorf("StopWorktree on a zero-value Worktree: %v", err)
	}
}
