package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// demoCLIHelperEnv turns this test binary into the fathom CLI, so the
// demo's child steps run the real verbs: initDemo execs its binary, and in
// a test that binary is this one.
const demoCLIHelperEnv = "FATHOM_DEMO_CLI_HELPER"

func init() {
	if os.Getenv(demoCLIHelperEnv) == "1" {
		os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
	}
}

var demoWiredLine = regexp.MustCompile(`(?m)^ok wired the copy with fathom init: (\d+) files written`)

// TestInitDemoWiresFauxtownAndReachesAFirstProvenAnswer runs the demo end
// to end through real child processes: init wires the fixture copy, ingest
// indexes a service into a scratch store, verify proves the fixture plan,
// and one line carries the elapsed time. The HOME the test process holds
// (standing in for the operator's) is byte-identical afterwards, ~/.fathom
// is never created in it, and the throwaway directory is gone.
func TestInitDemoWiresFauxtownAndReachesAFirstProvenAnswer(t *testing.T) {
	realHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(realHome, "keep.txt"), []byte("the operator's home\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", realHome)
	t.Setenv("XDG_STATE_HOME", filepath.Join(realHome, ".local", "state"))
	t.Setenv("FATHOM_HOME", filepath.Join(realHome, ".fathom"))
	before := snapshotTree(t, realHome)

	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	tmpRoot := t.TempDir()
	base := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	calls := 0
	d := initDemoDeps{
		exe:     exe,
		extra:   []string{demoCLIHelperEnv + "=1"},
		tmpRoot: tmpRoot,
		fleet:   connector.Fauxtown,
		now: func() time.Time {
			calls++
			if calls == 1 {
				return base
			}
			return base.Add(1500 * time.Millisecond)
		},
	}

	var out, errb bytes.Buffer
	if code := initDemo(context.Background(), d, &out, &errb); code != 0 {
		t.Fatalf("initDemo exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
	}
	got := out.String()

	m := demoWiredLine.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("no wiring line:\n%s", got)
	}
	if n, _ := strconv.Atoi(m[1]); n == 0 {
		t.Errorf("init wrote no file into the copy:\n%s", got)
	}
	for _, want := range []string{
		"fauxtown fixture fleet (9 services)",
		"ok ingested checkout-service into a scratch store",
		"ok verified a two-row fixture plan",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	var timing []string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "first proven answer in ") {
			timing = append(timing, line)
		}
	}
	if len(timing) != 1 || !strings.HasPrefix(timing[0], "first proven answer in 1.5s ") {
		t.Errorf("want exactly one timing line reading 1.5s, got %q", timing)
	}
	if ids := internalIDs(got); len(ids) > 0 {
		t.Errorf("the demo's output names internal ids %v:\n%s", ids, got)
	}

	if after := snapshotTree(t, realHome); len(touchedFiles(before, after)) != 0 || len(after) != len(before) {
		t.Errorf("the demo touched the real HOME: %v", touchedFiles(before, after))
	}
	if _, err := os.Stat(filepath.Join(realHome, ".fathom")); !os.IsNotExist(err) {
		t.Errorf("~/.fathom exists in the real HOME after the demo (stat err %v)", err)
	}
	if left, _ := os.ReadDir(tmpRoot); len(left) != 0 {
		t.Errorf("the throwaway directory was left behind: %d entries in %s", len(left), tmpRoot)
	}
}

// TestInitDemoRefusesDryRun: the demo writes only into its own copy, so a
// dry run of it has nothing to list; the combination is refused before any
// step runs.
func TestInitDemoRefusesDryRun(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdInit([]string{"--demo", "--dry-run"}, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1; stderr %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "do not combine") {
		t.Errorf("stderr does not name the refusal: %s", errb.String())
	}
}
