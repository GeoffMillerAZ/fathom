package main

// ingest_passtimes_test.go — EW-48 (wave ic; O362): `fathom ingest` names
// each pass's wall and CPU time, one human line and `pass_times` in
// --json, and under a zero clock its output is byte-identical to the
// output before pass times existed.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// steppingPassClock advances one millisecond of wall and one microsecond of
// CPU per reading: deterministic, never zero.
func steppingPassClock() fusion.PassClock {
	var mu sync.Mutex
	var wall, cpu time.Duration
	base := time.Unix(1_700_000_000, 0)
	return fusion.PassClock{
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			wall += time.Millisecond
			return base.Add(wall)
		},
		CPU: func() time.Duration {
			mu.Lock()
			defer mu.Unlock()
			cpu += time.Microsecond
			return cpu
		},
	}
}

// ingestWithClock runs one ingest of root into a fresh store under clock.
func ingestWithClock(t *testing.T, clock fusion.PassClock, root string, args ...string) string {
	t.Helper()
	saved := ingestPassClock
	ingestPassClock = clock
	t.Cleanup(func() { ingestPassClock = saved })
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	all := append([]string{"--service", "passprobe", "--path", root, "--store", store}, args...)
	if code := cmdIngest(all, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func passProbeCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("def main():\n    return 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestIngestPassTimesLineAndJSON(t *testing.T) {
	root := passProbeCorpus(t)

	text := ingestWithClock(t, steppingPassClock(), root)
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "pass-times") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("want exactly one pass-times line, got %d:\n%s", len(lines), text)
	}
	for _, pass := range []string{"discover ", "extract ", "store_writes ", "typed_go ", "cue ", "file_resolved ", "doc_xref ", "route_ref ", "doc_index ", "centrality ", "commit "} {
		if !strings.Contains(lines[0], pass) {
			t.Errorf("pass-times line does not name %q: %s", strings.TrimSpace(pass), lines[0])
		}
	}

	var env ingestJSON
	out := ingestWithClock(t, steppingPassClock(), root, "--json")
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(env.PassTimes) == 0 {
		t.Fatalf("pass_times missing:\n%s", out)
	}
	for _, p := range env.PassTimes {
		if p.Name == "" || p.WallNS <= 0 || p.CPUNS <= 0 {
			t.Errorf("pass_times entry %+v: want a name and positive wall_ns/cpu_ns", p)
		}
	}
	if env.PassTimes[0].Name != "discover" || env.PassTimes[len(env.PassTimes)-1].Name != "commit" {
		t.Errorf("pass_times run discover..commit in execution order: %+v", env.PassTimes)
	}
}

// TestIngestPassTimesZeroClockByteIdentical: the zero clock prints no
// pass-times line and no pass_times key, and the timed output differs from
// it by exactly that line or key.
func TestIngestPassTimesZeroClockByteIdentical(t *testing.T) {
	root := passProbeCorpus(t)

	zeroText := ingestWithClock(t, fusion.PassClock{}, root)
	timedText := ingestWithClock(t, steppingPassClock(), root)
	if strings.Contains(zeroText, "pass-times") {
		t.Fatalf("zero clock printed a pass-times line:\n%s", zeroText)
	}
	var kept []string
	for _, l := range strings.SplitAfter(timedText, "\n") {
		if !strings.HasPrefix(l, "pass-times") {
			kept = append(kept, l)
		}
	}
	if got := strings.Join(kept, ""); got != zeroText {
		t.Fatalf("timed output less its pass-times line differs from the zero-clock output:\n%s\n---\n%s", got, zeroText)
	}

	zeroJSON := ingestWithClock(t, fusion.PassClock{}, root, "--json")
	timedJSON := ingestWithClock(t, steppingPassClock(), root, "--json")
	if strings.Contains(zeroJSON, "pass_times") {
		t.Fatalf("zero clock emitted pass_times:\n%s", zeroJSON)
	}
	var zero, timed map[string]any
	if err := json.Unmarshal([]byte(zeroJSON), &zero); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(timedJSON), &timed); err != nil {
		t.Fatal(err)
	}
	if _, ok := timed["pass_times"]; !ok {
		t.Fatalf("timed --json has no pass_times:\n%s", timedJSON)
	}
	delete(timed, "pass_times")
	if !reflect.DeepEqual(zero, timed) {
		t.Fatalf("timed --json less pass_times differs from the zero-clock envelope:\n%s\n---\n%s", timedJSON, zeroJSON)
	}
}
