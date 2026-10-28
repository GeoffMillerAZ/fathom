package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// BX-88 / NP-B1: `run` stamps started_at from cellClock, `ledger` files
// first_cell_at (the earliest started_at over a prereg row's cells), and
// `outcome` refuses a run whose prereg postdates its first cell, whose
// prereg sha moved, or whose rows lack a stamp. A run that passes reads
// no-read (exit 3) until ClassifyOutcome exists (BX-63, D11).

// stepClock makes cellClock return start, start+step, start+2*step, ...
func stepClock(t *testing.T, start time.Time, step time.Duration) {
	t.Helper()
	var mu sync.Mutex
	next := start
	old := cellClock
	cellClock = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		at := next
		next = next.Add(step)
		return at
	}
	t.Cleanup(func() { cellClock = old })
}

func TestCmdRun_StampsStartedAtFromTheInjectedClock(t *testing.T) {
	// The clock runs in another zone: started_at is written in UTC.
	stepClock(t, time.Date(2026, 10, 2, 14, 0, 0, 0, time.FixedZone("x", 2*3600)), time.Minute)
	f := newPreregRunFixture(t)
	if rc := f.run(t, "", "run-1", "-only", "T1"); rc != 0 {
		t.Fatalf("run exited %d", rc)
	}
	cells := f.storedCells(t)
	if len(cells) != 1 {
		t.Fatalf("want 1 cell, got %d", len(cells))
	}
	for p, c := range cells {
		if want := "2026-10-02T12:00:00Z"; c.StartedAt != want {
			t.Errorf("%s: started_at = %q, want %q (the injected clock's first reading, in UTC)", p, c.StartedAt, want)
		}
	}
}

func TestCmdLedger_StampsFirstCellAt(t *testing.T) {
	top, prereg := preregRepoFixture(t, "2026-10-01T00:00:00Z")
	t.Run("the earliest started_at, borrowed cells included", func(t *testing.T) {
		f := newPreregRunFixture(t)
		stepClock(t, time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC), 5*time.Minute)
		// T1 runs first (10:00); a resume of the same run borrows T1 and
		// runs only T2 (10:05).
		if rc := f.run(t, top, "run-1", "-prereg", prereg, "-only", "T1"); rc != 0 {
			t.Fatalf("first run exited %d", rc)
		}
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("resume exited %d", rc)
		}
		if rc := f.fileLedger(t, "run-1"); rc == 2 {
			t.Fatal("ledger exited 2")
		}
		rows, err := scenariobench.LoadScenarioLedger(f.ledger)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, r := range rows {
			got[r.TaskID] = r.FirstCellAt
		}
		if got["T1"] != "2026-10-02T10:00:00Z" || got["T2"] != "2026-10-02T10:05:00Z" {
			t.Errorf("first_cell_at per row = %v, want T1 10:00 and T2 10:05", got)
		}
	})
	t.Run("a prereg cell without started_at refuses the filing", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, top, "run-1", "-prereg", prereg); rc != 0 {
			t.Fatalf("run exited %d", rc)
		}
		f.editStoredCells(t, func(task string, m map[string]any) {
			if task == "T2" {
				m["started_at"] = ""
			}
		})
		stderr := preregRefused(t, func() int { return f.fileLedger(t, "run-1") })
		if !strings.Contains(stderr, "started_at") || !strings.Contains(stderr, "T2/control/t1") {
			t.Errorf("refusal should name the cell and started_at: %s", stderr)
		}
	})
	t.Run("an unstamped run files no first_cell_at", func(t *testing.T) {
		f := newPreregRunFixture(t)
		if rc := f.run(t, "", "run-1"); rc != 0 {
			t.Fatalf("run exited %d", rc)
		}
		if rc := f.fileLedger(t, "run-1"); rc == 2 {
			t.Fatal("ledger exited 2")
		}
		raw, err := os.ReadFile(f.ledger)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "first_cell_at") {
			t.Errorf("an unstamped run files rows as today:\n%s", raw)
		}
	})
}

func TestCmdOutcome_ExitCodes(t *testing.T) {
	top, prereg := preregRepoFixture(t, "2026-10-01T12:00:00Z")
	sha := preregSHA(t, prereg)
	row := func(task, stamp, first string) scenariobench.ScenarioLedgerRow {
		return scenariobench.ScenarioLedgerRow{RunID: "run-1", TaskID: task, Arm: "control", PreregSHA256: stamp, FirstCellAt: first}
	}
	writeLedger := func(t *testing.T, rows ...scenariobench.ScenarioLedgerRow) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "scenarios-ledger.jsonl")
		for _, r := range rows {
			if err := scenariobench.AppendScenarioLedger(path, r); err != nil {
				t.Fatal(err)
			}
		}
		return path
	}
	outcome := func(t *testing.T, args ...string) (int, string, string) {
		t.Helper()
		rc := 0
		var stdout string
		stderr := captureStderr(t, func() {
			stdout = captureStdout(t, func() { rc = run(append([]string{"outcome"}, args...)) })
		})
		return rc, stdout, stderr
	}
	later := "2026-10-01T13:00:00Z"

	for _, tc := range []struct {
		name   string
		rows   []scenariobench.ScenarioLedgerRow
		runID  string
		want   int
		output []string
	}{
		{"provenance passes: no-read until BX-63", []scenariobench.ScenarioLedgerRow{row("T1", sha, later), row("T2", sha, "2026-10-01T12:00:00Z")}, "run-1", 3,
			[]string{"outcome: no-read: ClassifyOutcome is not built (BX-63)", "prereg: localization-v1", "sha256: " + sha, "commit_time: 2026-10-01T12:00:00Z", "first_cell_at: 2026-10-01T12:00:00Z", "rows: 2"}},
		{"a hash mismatch", []scenariobench.ScenarioLedgerRow{row("T1", strings.Repeat("0", 64), later)}, "run-1", 2, []string{"refusal: prereg-sha256-mismatch", "outcome: refused"}},
		{"a newer commit", []scenariobench.ScenarioLedgerRow{row("T1", sha, "2026-10-01T11:00:00Z")}, "run-1", 2, []string{"refusal: committed-after-first-cell"}},
		{"a missing sha", []scenariobench.ScenarioLedgerRow{row("T1", "", later)}, "run-1", 2, []string{"refusal: missing-prereg-sha256"}},
		{"a missing first_cell_at", []scenariobench.ScenarioLedgerRow{row("T1", sha, "")}, "run-1", 2, []string{"refusal: missing-first-cell-at"}},
		{"no rows", []scenariobench.ScenarioLedgerRow{row("T1", sha, later)}, "run-9", 2, []string{"refusal: no-rows", "rows: 0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := writeLedger(t, tc.rows...)
			rc, stdout, stderr := outcome(t, "-prereg", prereg, "-ledger", ledger, "-run-id", tc.runID)
			if rc != tc.want {
				t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", rc, tc.want, stdout, stderr)
			}
			for _, want := range tc.output {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout should contain %q:\n%s", want, stdout)
				}
			}
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			for i := 1; i < len(lines); i++ {
				if lines[i-1] > lines[i] {
					t.Errorf("output lines must be sorted: %q before %q", lines[i-1], lines[i])
				}
			}
		})
	}

	t.Run("usage", func(t *testing.T) {
		if rc, _, stderr := outcome(t, "-prereg", prereg); rc != 2 || !strings.Contains(stderr, "-prereg, -ledger and -run-id are required") {
			t.Errorf("missing -run-id: exit %d, want 2 naming the required flags: %s", rc, stderr)
		}
		if rc, _, stderr := outcome(t, "-no-such-flag"); rc != 2 || strings.Contains(stderr, "unknown subcommand") {
			t.Errorf("an unknown flag: exit %d, want 2 from outcome's own flag set: %s", rc, stderr)
		}
		if rc, _, stderr := outcome(t, "-prereg", prereg, "-ledger", filepath.Join(t.TempDir(), "absent.jsonl"), "-run-id", "run-1"); rc != 2 || !strings.Contains(stderr, "absent.jsonl") {
			t.Errorf("a missing ledger is named and exits 2, got %d: %s", rc, stderr)
		}
	})
	t.Run("an unknown commit time", func(t *testing.T) {
		untracked := filepath.Join(top, "docs", "bench", "prereg", "untracked-v1.json")
		preregWrite(t, untracked, preregWithID(t, "untracked-v1"))
		t.Cleanup(func() { os.Remove(untracked) })
		ledger := writeLedger(t, row("T1", preregSHA(t, untracked), later))
		rc, stdout, _ := outcome(t, "-prereg", untracked, "-ledger", ledger, "-run-id", "run-1")
		if rc != 2 || !strings.Contains(stdout, "refusal: commit-time-unknown") || !strings.Contains(stdout, "commit_time: unknown") {
			t.Errorf("a prereg with no commit: exit %d, want 2 naming commit-time-unknown:\n%s", rc, stdout)
		}
	})
	t.Run("a shallow repository", func(t *testing.T) {
		shallow := filepath.Join(t.TempDir(), "shallow")
		preregGit(t, top, "2026-10-01T12:30:00Z", "commit", "-q", "--allow-empty", "-m", "a second commit")
		preregGit(t, filepath.Dir(shallow), "", "clone", "-q", "--depth", "1", "file://"+top, shallow)
		ledger := writeLedger(t, row("T1", sha, later))
		rc, _, stderr := outcome(t, "-prereg", filepath.Join(shallow, "docs", "bench", "prereg", preregTestID+".json"), "-ledger", ledger, "-run-id", "run-1")
		if rc != 2 || !strings.Contains(stderr, "shallow") {
			t.Errorf("a shallow repository is refused by name: exit %d: %s", rc, stderr)
		}
	})
}

func TestPreregStatusEntries_FoldsRenamesInEitherColumn(t *testing.T) {
	got := preregStatusEntries("R  new.txt\x00old.txt\x00 R moved.txt\x00oldname.txt\x00?? u.txt\x00")
	var paths []string
	for _, e := range got {
		paths = append(paths, e.code+" "+e.path)
	}
	if want := "R  new.txt| R moved.txt|?? u.txt"; strings.Join(paths, "|") != want {
		t.Errorf("entries = %q, want %q", strings.Join(paths, "|"), want)
	}
}

func TestCmdOutcome_DatesTheFileASymlinkNames(t *testing.T) {
	// The prereg is committed at 12:00 and rewritten at 13:00; a symlink to
	// it, committed at 11:00, must not lend its own older date to the target.
	top, prereg := preregRepoFixture(t, "2026-10-01T11:00:00Z")
	link := filepath.Join(top, "docs", "bench", "prereg", "link", preregTestID+".json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", preregTestID+".json"), link); err != nil {
		t.Fatal(err)
	}
	preregGit(t, top, "2026-10-01T11:00:00Z", "add", "-A")
	preregGit(t, top, "2026-10-01T11:00:00Z", "commit", "-q", "-m", "a link")
	preregWrite(t, prereg, append(preregFixtureBytes(t), '\n'))
	preregGit(t, top, "2026-10-01T13:00:00Z", "commit", "-q", "-am", "rewrite after the first cell")
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := scenariobench.AppendScenarioLedger(ledger, scenariobench.ScenarioLedgerRow{
		RunID: "run-1", TaskID: "T1", Arm: "control", PreregSHA256: preregSHA(t, prereg), FirstCellAt: "2026-10-01T12:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	rc := 0
	stdout := captureStdout(t, func() { rc = run([]string{"outcome", "-prereg", link, "-ledger", ledger, "-run-id", "run-1"}) })
	if rc != 2 || !strings.Contains(stdout, "refusal: committed-after-first-cell") || !strings.Contains(stdout, "commit_time: 2026-10-01T13:00:00Z") {
		t.Errorf("a symlinked prereg is dated by its target: exit %d\n%s", rc, stdout)
	}
}
