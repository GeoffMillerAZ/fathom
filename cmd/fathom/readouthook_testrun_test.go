package main

// readouthook_testrun_test.go — RO-21 (docs/plans/readouts.md): with the
// readout hook and the test-run hook both on, PostToolUse(Bash) and
// PostToolUseFailure(Bash) run one fathom process each, the readout hook
// running the test-run tier in process; init writes one entry per event;
// the old session-hook markers stay accepted.

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

const (
	foldReadoutConfig = "[readouts]\nmode = \"observe\"\n"
	foldTailFail      = "--- FAIL: TestX (0.00s)\n    x_test.go:12: boom\nFAIL\nFAIL\tgithub.com/o/r/x\t0.2s\n"
)

var (
	foldTestRunMarkers = []string{"fathom session-hook --event test-run", "fathom session-hook --event failed-test-run"}
	foldReadoutMarkers = []string{"fathom readout-hook --event post-tool", "fathom readout-hook --event tool-failure"}
)

// foldEnv isolates one fold run: the readout hook's private HOME, per-user
// directory and hook log with userCfg as the user's config, a repository
// whose Taskfile declares `go test` with `[hooks] test_run` on, and a
// daemon that answers 404 named by FATHOM_HTTP.
func foldEnv(t *testing.T, userCfg string) string {
	t.Helper()
	readoutHookEnv(t, userCfg)
	srv, _ := countingDaemon(t)
	t.Setenv("FATHOM_HTTP", srv.URL)
	repo := t.TempDir()
	gitInitTempRepo(t, repo)
	writeRepoFile(t, repo, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
	return repo
}

// setUserConfig replaces the user's config.toml; "" removes it.
func setUserConfig(t *testing.T, cfg string) {
	t.Helper()
	p := filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml")
	if cfg == "" {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(p, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

// freshHookLog points the hook log at a new empty file.
func freshHookLog(t *testing.T) {
	t.Helper()
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
}

// foldRows is the test-run rows of the log, without the invocation stamp
// (a pid and a start time, distinct in every process).
func foldRows(t *testing.T) []map[string]string {
	t.Helper()
	rows := testRunBeats(t)
	for _, r := range rows {
		delete(r, "hook_inv")
	}
	return rows
}

func markersOf(specs []hookSpec) []string {
	var out []string
	for _, s := range specs {
		out = append(out, s.marker)
	}
	return out
}

func TestReadoutHookFoldInitWritesOneBashEntry(t *testing.T) {
	for _, c := range []struct {
		name            string
		userCfg         string
		testRun         bool
		wantTestRun     bool
		wantReadoutHook bool
	}{
		{"both on", foldReadoutConfig, true, false, true},
		{"only test_run", "", true, true, false},
		{"only readouts", foldReadoutConfig, false, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			readoutHookEnv(t, c.userCfg)
			root := t.TempDir()
			cfg := "[hooks]\ntest_run = false\n"
			if c.testRun {
				cfg = "[hooks]\ntest_run = true\n"
			}
			writeRepoFile(t, root, ".fathom/config.toml", cfg)
			got := markersOf(fathomHookSpecsAt(root))
			targets, err := initTargetsFor(harness.ClaudeCode, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
			var bodies []string
			for _, k := range spec.Keys {
				bodies = append(bodies, k.Body)
			}
			planned := strings.Join(bodies, "\n")
			for _, m := range foldTestRunMarkers {
				if slices.Contains(got, m) != c.wantTestRun || strings.Contains(planned, strings.TrimPrefix(m, "fathom ")) != c.wantTestRun {
					t.Errorf("%s: table carries %q = %v, init plans it = %v, want %v", c.name, m, slices.Contains(got, m), strings.Contains(planned, strings.TrimPrefix(m, "fathom ")), c.wantTestRun)
				}
			}
			for _, m := range foldReadoutMarkers {
				if slices.Contains(got, m) != c.wantReadoutHook {
					t.Errorf("%s: table carries %q = %v, want %v", c.name, m, slices.Contains(got, m), c.wantReadoutHook)
				}
			}
			// One fathom entry per Bash event for the pair: the readout
			// hook's, or the test-run hook's, never both.
			if c.name == "both on" {
				for _, ev := range []string{"PostToolUse", "PostToolUseFailure"} {
					n := 0
					for _, s := range fathomHookSpecsAt(root) {
						if s.event == ev && s.matcher == "Bash" && (slices.Contains(foldTestRunMarkers, s.marker) || slices.Contains(foldReadoutMarkers, s.marker)) {
							n++
						}
					}
					if n != 1 {
						t.Errorf("%s(Bash): %d test-run and readout entries, want 1", ev, n)
					}
				}
			}
		})
	}
	// The user layer alone decides a user-scope merge the same way.
	readoutHookEnv(t, foldReadoutConfig+"[hooks]\ntest_run = true\n")
	settings, _, _, err := mergeHookSettings(nil, "fathom")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range foldTestRunMarkers {
		if bytes.Contains(settings, []byte(m)) {
			t.Errorf("a user-scope merge with both on writes %q", m)
		}
	}
	for _, m := range foldReadoutMarkers {
		if !bytes.Contains(settings, []byte(m)) {
			t.Errorf("a user-scope merge with both on lacks %q", m)
		}
	}
	// The default table, which detection and every caller passing no
	// opt-in reads, still carries the old markers.
	for _, m := range foldTestRunMarkers {
		if !slices.Contains(markersOf(fathomHookSpecsAll()), m) || !slices.Contains(markersOf(fathomHookSpecs), m) {
			t.Errorf("detection lost %q", m)
		}
	}
}

func TestReadoutHookRunsTheTestRunTierLikeTheStandaloneHook(t *testing.T) {
	for _, c := range []struct{ readoutEvent, testRunEvent string }{{"post-tool", "test-run"}, {"tool-failure", "failed-test-run"}} {
		t.Run(c.readoutEvent, func(t *testing.T) {
			repo := foldEnv(t, "")
			payload := testRunPostPayload(repo, "go test ./x 2>&1 | tail -30", foldTailFail)
			if c.readoutEvent == "tool-failure" {
				payload = testRunFailurePayload(repo, "go test ./x", foldTailFail)
			}

			// Today: the test-run entry alone, the readout hook off.
			var standalone, errb bytes.Buffer
			runWithStdin([]string{"session-hook", "--event", c.testRunEvent}, strings.NewReader(payload), &standalone, &errb)
			wantRows := foldRows(t)
			if standalone.Len() == 0 || !strings.Contains(standalone.String(), "TestX") || len(wantRows) != 1 {
				t.Fatalf("the standalone hook: output %q, rows %v; want a line naming TestX and one row", standalone.String(), wantRows)
			}

			// Folded: the readout hook alone, both on.
			setUserConfig(t, foldReadoutConfig)
			freshHookLog(t)
			code, folded, _, steps := runReadoutHook(t, []string{"--event", c.readoutEvent}, payload)
			if code != 0 {
				t.Fatalf("readout hook exit %d", code)
			}
			if folded != standalone.String() {
				t.Errorf("folded output differs:\n got %q\nwant %q", folded, standalone.String())
			}
			if got := foldRows(t); len(got) != 1 || !maps.Equal(got[0], wantRows[0]) {
				t.Errorf("folded test-run row differs:\n got %v\nwant %v", got, wantRows)
			}
			if !slices.Contains(steps, "test-run") {
				t.Errorf("steps %v: the test-run stage did not run", steps)
			}
			ro := readoutRows(t)
			if len(ro) != 1 || ro[0].Meta["emitted"] != "false" || ro[0].Meta["mode"] != "observe" {
				t.Errorf("readout rows = %+v, want one observe row, emitted false", ro)
			}
		})
	}
}

func TestTestRunOldMarkerStillRuns(t *testing.T) {
	// The old markers are still wired by default and still run the tier
	// while the readout hook is off.
	for _, m := range foldTestRunMarkers {
		if !slices.Contains(markersOf(hookSpecsFor(harness.ClaudeCode)), m) {
			t.Errorf("the default table lacks %q", m)
		}
	}
	repo := foldEnv(t, "")
	payload := testRunPostPayload(repo, "go test ./x 2>&1 | tail -30", foldTailFail)
	var out, errb bytes.Buffer
	runWithStdin([]string{"session-hook", "--event", "test-run"}, strings.NewReader(payload), &out, &errb)
	if !strings.Contains(out.String(), "TestX") || len(foldRows(t)) != 1 {
		t.Fatalf("the old marker with the readout hook off: output %q, rows %d", out.String(), len(foldRows(t)))
	}

	// A settings file wired before the fold carries both entries: with both
	// on the pair still runs the tier once, one line and one row in all.
	setUserConfig(t, foldReadoutConfig)
	freshHookLog(t)
	out.Reset()
	runWithStdin([]string{"session-hook", "--event", "test-run"}, strings.NewReader(payload), &out, &errb)
	_, folded, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, payload)
	all := out.String() + folded
	if strings.Count(all, "TestX") != 1 || strings.Count(all, "\n") != 1 {
		t.Errorf("both entries wired, both on: output %q, want one line", all)
	}
	if n := len(foldRows(t)); n != 1 {
		t.Errorf("both entries wired, both on: %d test-run rows, want 1", n)
	}

	// Outside [readouts] roots the readout hook still runs the tier: the
	// roots gate the readout, not the test-run hook.
	setUserConfig(t, foldReadoutConfig+"roots = [\"/nowhere/at/all\"]\n")
	freshHookLog(t)
	_, folded, _, _ = runReadoutHook(t, []string{"--event", "post-tool"}, payload)
	if !strings.Contains(folded, "TestX") || len(foldRows(t)) != 1 || len(readoutRows(t)) != 0 {
		t.Errorf("outside roots: output %q, test-run rows %d, readout rows %d; want the line, 1, 0", folded, len(foldRows(t)), len(readoutRows(t)))
	}
}
