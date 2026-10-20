package main

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestPortHeldByUnsupervisedHolderExitsNonZeroUncounted (§8.4 step 5, O1363):
// a same-label managed holder whose record says it is supervised, or whose
// record predates the field, is "already served" (exit 0, as before); one
// whose record says `supervised: false` (a hand-started managed daemon)
// exits non-zero so the supervisor relaunches and takes the port over once
// it is free — under a reason neither the breaker nor RT-M1's day summary
// counts.
func TestPortHeldByUnsupervisedHolderExitsNonZeroUncounted(t *testing.T) {
	const label = "com.example.fathom"
	yes, no := true, false
	facts := func(sup *bool) portHolderFacts {
		return portHolderFacts{ListenerPID: 500, ListenerCmd: "fathom serve-http --managed", Responds: true,
			Record: &console.DaemonRecord{ID: label, Managed: true, PID: 500, Supervised: sup}}
	}
	if code, reason, detail := resolvePortHolder(true, label, facts(&yes)); code != 0 || reason != tenancy.ExitPortHeld || !strings.Contains(detail, "already served by pid 500") {
		t.Fatalf("supervised holder: %d %s %q", code, reason, detail)
	}
	if code, reason, _ := resolvePortHolder(true, label, facts(nil)); code != 0 || reason != tenancy.ExitPortHeld {
		t.Fatalf("record without the field: %d %s, want today's exit 0", code, reason)
	}
	code, reason, detail := resolvePortHolder(true, label, facts(&no))
	if code == 0 || reason != tenancy.ExitPortHeldUnsupervised || !strings.Contains(detail, "pid 500") || !strings.Contains(detail, "no supervisor") {
		t.Fatalf("unsupervised holder: %d %s %q", code, reason, detail)
	}

	// The breaker: five relaunches that each found the hand-started holder
	// count nothing, name no unclean predecessor and write no breaker row;
	// the same exits as a foreign port_held would count.
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	ledgerWith := func(reason string) string {
		ledger := filepath.Join(t.TempDir(), label, "lifecycle.jsonl")
		for i := 0; i < 5; i++ {
			at := t0.Add(time.Duration(i) * time.Minute)
			st := tenancy.NewLifecycleRow(tenancy.RowStart, at, 700+i, "v")
			st.Managed = true
			ex := tenancy.NewLifecycleRow(tenancy.RowExit, at.Add(time.Second), 700+i, "v")
			ex.Code, ex.Reason = tenancy.IntPtr(code), reason
			for _, r := range []tenancy.LifecycleRow{st, ex} {
				if err := appendLifecycle(ledger, r); err != nil {
					t.Fatal(err)
				}
			}
		}
		return ledger
	}
	ledger := ledgerWith(tenancy.ExitPortHeldUnsupervised)
	out := beginLifecycle(lcStart(ledger, 800, t0.Add(6*time.Minute)))
	if out.verdict.Action != tenancy.BreakerRun || out.verdict.Counted != 0 || out.verdict.CountedSafe != 0 || out.unclean != nil || out.stop {
		t.Fatalf("breaker counted an unsupervised-holder exit: %+v", out)
	}
	if kinds := rowKinds(readRowsT(t, ledger)); strings.Contains(kinds, "breaker") || strings.Contains(kinds, "unclean") {
		t.Fatalf("ledger = %s", kinds)
	}
	if v := tenancy.EvaluateBreaker(readRowsT(t, ledgerWith(tenancy.ExitPortHeld)), t0.Add(6*time.Minute), tenancy.BreakerSettings{}); v.Counted == 0 {
		t.Fatalf("control: a foreign port_held exit is counted, got %+v", v)
	}

	// RT-M1: the day summary shows no unclean exit and no exit rung for it,
	// and `fathom resources` counts none toward a crash loop.
	var entries []lifecycleEntry
	for _, r := range readRowsT(t, ledger) {
		entries = append(entries, lifecycleEntry{Row: r.Row, At: r.At, PID: r.PID, Reason: r.Reason, Detail: r.Detail})
	}
	days := mergeLedgerIntoDays([]tenancy.DaySummary{{Date: "2026-10-19"}}, entries, false)
	if len(days) != 1 || days[0].ExitRungs != 0 || days[0].UncleanExits != 0 || !days[0].ExitsKnown {
		t.Fatalf("day summary = %+v", days)
	}
	if l := summarizeLifecycle(entries, ledger, t0.Add(6*time.Minute)); l.CountedExits30m != 0 || l.CrashLooping || l.Unclean24h != 0 {
		t.Fatalf("resources lifecycle = %+v", l)
	}
}

// TestManagedHandStartRecordsUnsupervised (§8.4 step 5, O1363): a managed
// start with no launchd/systemd marker and a parent other than init is
// unsupervised — its start row and its daemon record say supervised false,
// and it logs one stderr line naming the supervisor's restart command. It
// still starts (here it meets a held port and exits).
func TestManagedHandStartRecordsUnsupervised(t *testing.T) {
	const label = "com.example.fathom"
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	for _, c := range []struct {
		name, goos string
		env        map[string]string
		ppid       int
		want       bool
	}{
		{"launchd job", "darwin", map[string]string{"XPC_SERVICE_NAME": label}, 4242, true},
		{"darwin init parent", "darwin", nil, 1, true},
		{"darwin terminal", "darwin", map[string]string{"XPC_SERVICE_NAME": "application.com.example.term"}, 4242, false},
		{"darwin other label", "darwin", map[string]string{"XPC_SERVICE_NAME": "com.example.other"}, 4242, false},
		{"systemd unit", "linux", map[string]string{"INVOCATION_ID": "0123abcd"}, 4242, true},
		{"linux init parent", "linux", nil, 1, true},
		{"linux shell", "linux", nil, 4242, false},
	} {
		if got := detectSupervised(c.goos, label, env(c.env), c.ppid); got != c.want {
			t.Errorf("%s: supervised = %v, want %v", c.name, got, c.want)
		}
	}
	if s := supervisorRestartCommand("darwin", label); s != "launchctl kickstart -k gui/$(id -u)/"+label {
		t.Errorf("darwin restart = %q", s)
	}
	if s := supervisorRestartCommand("linux", label); !strings.HasPrefix(s, "systemctl --user restart ") {
		t.Errorf("linux restart = %q", s)
	}

	// The record carries the field.
	withIsolatedConfigHome(t)
	no := false
	rec, p, err := recordServingDaemonAs(daemonMeta{Managed: true, Label: label, Supervised: &no}, "127.0.0.1:0", "", filepath.Join(t.TempDir(), "s.sqlite"), "abc", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if back, rerr := console.ReadDaemonRecord(p); rerr != nil || back.Supervised == nil || *back.Supervised || rec.Supervised == nil {
		t.Fatalf("record supervised = %v (%v)", back.Supervised, rerr)
	}

	// A real hand start: no marker, the test binary as parent.
	home, _ := withIsolatedConfigHome(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), testHelperEnv+"=rt-handstart", "RT_STORE="+filepath.Join(t.TempDir(), "s.sqlite"), "RT_ADDR="+ln.Addr().String(), "XPC_SERVICE_NAME=", "INVOCATION_ID=")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	rc := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run: %v", err)
		}
		rc = ee.ExitCode()
	}
	if rc != tenancy.ExitRetryLater {
		t.Fatalf("rc = %d, want %d (a foreign holder)\n%s", rc, tenancy.ExitRetryLater, stderr.String())
	}
	ledgers, _ := filepath.Glob(filepath.Join(home, ".fathom", "daemon", "*", "lifecycle.jsonl"))
	if len(ledgers) != 1 {
		t.Fatalf("ledgers = %v", ledgers)
	}
	var start *tenancy.LifecycleRow
	for _, r := range readRowsT(t, ledgers[0]) {
		if r.Row == tenancy.RowStart {
			r := r
			start = &r
		}
	}
	if start == nil || !start.Managed || start.Supervised == nil || *start.Supervised {
		t.Fatalf("start row = %+v", start)
	}
	want := supervisorRestartCommand(runtime.GOOS, start.LedgerID)
	if n := strings.Count(stderr.String(), want); n != 1 {
		t.Fatalf("stderr names %q %d times, want once:\n%s", want, n, stderr.String())
	}
}

// TestDaemonDownTextNamesSupervisorRestart (O1363): the text an agent sees
// when the daemon is down (the SessionStart/prime primer and the
// tool-failure note) names the supervisor's restart command and says not to
// start a managed serve-http by hand.
func TestDaemonDownTextNamesSupervisorRestart(t *testing.T) {
	const tail = "never start `fathom serve-http --managed` by hand"
	kick := "launchctl kickstart -k gui/$(id -u)/com.example.fathom"
	if got := daemonDownHintFor("darwin", kick); !strings.HasPrefix(got, kick) || !strings.Contains(got, tail) {
		t.Fatalf("darwin: %q", got)
	}
	got := daemonDownHintFor("linux", "fathom serve-http --store s.sqlite --addr :7171   # no launchd on this platform; run it under your own supervisor")
	if !strings.HasPrefix(got, "systemctl --user restart ") || strings.Contains(got, "--store") || !strings.Contains(got, tail) {
		t.Fatalf("linux: %q", got)
	}
	if got := daemonDownHintFor("linux", "fathom daemon start   # stopped on purpose"); !strings.HasPrefix(got, "fathom daemon start") {
		t.Fatalf("a kill-switch hint is kept: %q", got)
	}

	withIsolatedConfigHome(t)
	if line := buildSessionContext("http://127.0.0.1:1", "fathom"); !strings.Contains(line, "not responding") || !strings.Contains(line, tail) {
		t.Fatalf("primer: %q", line)
	}
	if note := toolFailureAdvice("http://127.0.0.1:1", t.TempDir(), "mcp__fathom__find_symbol", "connection refused"); !strings.Contains(note, "not responding") || !strings.Contains(note, tail) {
		t.Fatalf("tool-failure note: %q", note)
	}
}
