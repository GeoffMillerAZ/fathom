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
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
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

// gitFixtureRepoWithFiles is gitFixtureRepo plus extra tracked files
// (relative path -> content) committed in the same initial commit — for
// tests that need an answer-key-shaped file already present in the
// subject before PrepareSubject ever runs.
func gitFixtureRepoWithFiles(t *testing.T, extra map[string]string) (repoPath, commit string) {
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
	for rel, content := range extra {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		writeTestFile(t, full, content)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// TestPrepareSubject_RemovesExcludedPathsBeforeSetup is O53 defect 3's
// prepare-time-removal proof: a Subject.ExcludePaths glob must be gone
// from the worktree BEFORE the subject's own Setup command runs (so a
// script cannot depend on the excluded file's presence), for an unwired
// arm exactly like a wired one — nothing about this removal is
// wired-arm-specific. A file NOT matching any exclude pattern must
// survive untouched.
func TestPrepareSubject_RemovesExcludedPathsBeforeSetup(t *testing.T) {
	repoPath, commit := gitFixtureRepoWithFiles(t, map[string]string{
		"testdata/contextbench/tasks-home.json": `{"H1-T01":{"ground_truth":"the answer key"}}`,
		"docs/bench/scenarios.md":               "# bench scenarios\n",
		"keep/me.txt":                           "not excluded\n",
	})
	root := t.TempDir()
	task := Task{
		ID: "T1",
		Subject: Subject{
			RepoURL:    repoPath,
			BaseCommit: commit,
			ExcludePaths: []string{
				"testdata/contextbench/**",
				"testdata/scenariobench/**",
				"docs/bench/**",
			},
			// If exclusion ran AFTER Setup instead of before, this command
			// (which fails when the excluded file is still present) would
			// catch it too — belt and suspenders on top of the direct
			// os.Stat assertions below.
			Setup: []string{"test ! -e testdata/contextbench/tasks-home.json"},
		},
	}
	arm := Arm{Instructions: "none"}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	if _, err := os.Stat(filepath.Join(wt.Path, "testdata", "contextbench", "tasks-home.json")); !os.IsNotExist(err) {
		t.Errorf("expected the excluded answer-key file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "docs", "bench")); !os.IsNotExist(err) {
		t.Errorf("expected docs/bench to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "keep", "me.txt")); err != nil {
		t.Errorf("expected a non-excluded file to survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "marker.txt")); err != nil {
		t.Errorf("expected the subject's own file to survive: %v", err)
	}
}

// gitFixtureRepoWithFuture is gitFixtureRepo plus a SECOND commit after the
// base commit — the shape a repo with a merged fix has: the fix (or, as in
// O55, a control planner's discovery of it) lives strictly after base in
// `git log`. Returns the repo path, base's sha, and the future commit's own
// sha.
func gitFixtureRepoWithFuture(t *testing.T) (repoPath, base, future string) {
	t.Helper()
	dir, baseSHA := gitFixtureRepo(t)
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
	writeTestFile(t, filepath.Join(dir, "future.txt"), "PR #10124's merged fix\n")
	run("add", "future.txt")
	run("commit", "-q", "-m", "merge: PR #10124 the fix the task is measuring")
	futureSHA := run("rev-parse", "HEAD")
	return dir, baseSHA, futureSHA
}

// TestPrepareSubject_WorktreeHasNoFuture pins O55 item 1: a cell worktree
// built off a full clone must never let `git log --all` (or any other
// means) resolve a commit that only exists after base_commit — a control
// planner found the task's own merged fix that way. After PrepareSubject,
// the worktree's own history is exactly one commit and the future commit's
// sha is unresolvable inside it.
func TestPrepareSubject_WorktreeHasNoFuture(t *testing.T) {
	repoPath, base, future := gitFixtureRepoWithFuture(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: base}}
	arm := Arm{Instructions: "none"}

	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	out, err := exec.Command("git", "-C", wt.Path, "log", "--all", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log --all: %v (%s)", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Errorf("expected exactly one commit in the cell worktree's history, got %d: %q", len(lines), out)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "future.txt")); err == nil {
		t.Error("the future commit's own file must not be present in the cell worktree's tree")
	}
	resolve, err := exec.Command("git", "-C", wt.Path, "cat-file", "-e", future+"^{commit}").CombinedOutput()
	if err == nil {
		t.Errorf("expected the future commit's sha to be unresolvable in the cell worktree, but it resolved: %s", resolve)
	}

	// Reuse (the same subject+base_commit+arm cell prepared again) must
	// still land on a single-commit history, not accumulate one per call.
	wt2, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (2nd): %v", err)
	}
	defer StopWorktree(&wt2)
	out2, err := exec.Command("git", "-C", wt2.Path, "log", "--all", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log --all (2nd): %v (%s)", err, out2)
	}
	lines2 := strings.Split(strings.TrimSpace(string(out2)), "\n")
	if len(lines2) != 1 || lines2[0] == "" {
		t.Errorf("expected exactly one commit after reuse, got %d: %q", len(lines2), out2)
	}
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

// TestInstrumentOwnedPaths_ReadsTheLedgerPlusStatics pins O55 item 4:
// every file internal/wiring's own ledger (.fathom/owned.json) records a
// KEY for (KindKey — a structured file like .claude/settings.json's own
// hooks block, or another lefthook.yml-shaped entry) is instrument-owned,
// plus the two paths that carry no ledger entry of their own —
// .fathomignore and the ledger file itself. CLAUDE.md/AGENTS.md's own
// fathom stanza is deliberately NOT exercised here — it is a KindREGION,
// which apply.go never records in this ledger at all (fix-first review,
// 2026-09-14); TestInstrumentRegionRanges_* below covers that path.
func TestInstrumentOwnedPaths_ReadsTheLedgerPlusStatics(t *testing.T) {
	dir := t.TempDir()
	ledger := wiring.NewLedger()
	ledger.Record(".claude/settings.json", wiring.OwnedKey{ID: "hooks", Digest: "sha256:y", Wrote: "2026-09-14T00:00:00Z"})
	ledger.Record(".claude/skills/foo/SKILL.md", wiring.OwnedKey{ID: "file", Digest: "sha256:z", Wrote: "2026-09-14T00:00:00Z"})
	if err := wiring.SaveLedger(dir, ledger); err != nil {
		t.Fatal(err)
	}

	owned, err := InstrumentOwnedPaths(Worktree{Path: dir})
	if err != nil {
		t.Fatalf("InstrumentOwnedPaths: %v", err)
	}
	for _, want := range []string{".claude/settings.json", ".claude/skills/foo/SKILL.md", ".fathomignore", ".fathom/owned.json"} {
		if !owned[want] {
			t.Errorf("expected %q to be instrument-owned, got %v", want, owned)
		}
	}
	if owned["real_agent_file.go"] {
		t.Error("a file the ledger never mentions must not be reported owned")
	}
	if owned["CLAUDE.md"] {
		t.Error("CLAUDE.md must never be reported owned via the ledger — its stanza is a KindRegion, never ledger-tracked")
	}
}

// TestInstrumentRegionRanges_RealRegionFromWiring pins O55 item 4
// (fix-first review): InstrumentRegionRanges must find a region internal
// wiring's OWN production code actually wrote — wiring.ApplyWiring, never
// a hand-crafted marker string standing in for it — and report exactly the
// lines its own markers (wiring.HTMLMarkers) delimit.
func TestInstrumentRegionRanges_RealRegionFromWiring(t *testing.T) {
	dir := t.TempDir()
	fixedNow := func() time.Time { return time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC) }
	spec := wiring.Spec{Regions: []wiring.Region{{
		File: "CLAUDE.md", Syntax: wiring.HTMLMarkers, Version: "v3",
		Body: "line one of the real stanza\nline two of the real stanza",
	}}}
	if _, err := wiring.ApplyWiring(dir, spec, wiring.ApplyOptions{Now: fixedNow}); err != nil {
		t.Fatalf("wiring.ApplyWiring: %v", err)
	}

	ranges := InstrumentRegionRanges(context.Background(), Worktree{Path: dir})
	rng, ok := ranges["CLAUDE.md"]
	if !ok {
		t.Fatalf("expected CLAUDE.md to carry a detected region, got %v", ranges)
	}
	if rng.oldStart != 0 || rng.oldEnd != 0 {
		t.Errorf("expected no base-side region (no git repo/base commit here), got old=[%d,%d]", rng.oldStart, rng.oldEnd)
	}
	if rng.newStart == 0 || rng.newEnd < rng.newStart {
		t.Fatalf("expected a real new-side region range, got %+v", rng)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	if !strings.HasPrefix(lines[rng.newStart-1], wiring.HTMLMarkers.BeginPrefix) {
		t.Errorf("line %d = %q, want the begin marker", rng.newStart, lines[rng.newStart-1])
	}
	if !strings.HasPrefix(lines[rng.newEnd-1], wiring.HTMLMarkers.End) {
		t.Errorf("line %d = %q, want the end marker", rng.newEnd, lines[rng.newEnd-1])
	}
}

// TestInstrumentOwnedPaths_NoLedgerIsJustTheStatics covers the ordinary
// unwired-arm case: no .fathom/owned.json at all is not an error, and the
// two static entries are still reported.
func TestInstrumentOwnedPaths_NoLedgerIsJustTheStatics(t *testing.T) {
	owned, err := InstrumentOwnedPaths(Worktree{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("InstrumentOwnedPaths: %v", err)
	}
	if len(owned) != 2 || !owned[".fathomignore"] || !owned[".fathom/owned.json"] {
		t.Errorf("expected just the two static entries, got %v", owned)
	}
}

// TestObligationsProbeServesRules_RealEnvelope pins O51: a real tools/call
// response carries the tool's JSON inside result.content[].text; the first
// probe read only result.rules and so voided every layer-required wired
// cell against a real daemon. Both shapes must be read.
func TestObligationsProbeServesRules_RealEnvelope(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"real envelope with rules", `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"file\":\"internal/core/model.go\",\"rules\":[{\"rule\":\"Sort outputs.\",\"src\":\"internal/CLAUDE.md#L3\"}]}"}]}}`, true},
		{"real envelope with zero rules", `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"file\":\"x.go\",\"rules\":[]}"}]}}`, false},
		{"legacy flat result", `{"jsonrpc":"2.0","id":1,"result":{"rules":[{"rule":"x"}]}}`, true},
		{"empty result", `{"jsonrpc":"2.0","id":1,"result":{}}`, false},
	}
	for _, c := range cases {
		got, err := obligationsProbeServesRules([]byte(c.raw))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: served=%v, want %v", c.name, got, c.want)
		}
	}
	if _, err := obligationsProbeServesRules([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"service not indexed"}}`)); err == nil {
		t.Error("an error envelope must surface as an error, never as zero rules")
	}
}
