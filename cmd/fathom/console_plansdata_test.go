package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestConsolePlans_HeadOnlyChangeReusesDaemonVerdicts pins the plan cache's
// cost on a repository that keeps committing: the daemon's verdicts depend
// on the plan's content and the index watermark, not on HEAD, so a commit
// that moves only HEAD re-applies the working-tree overlays to the verdicts
// already fetched — no daemon call (the daemon here is unreachable) — while
// a changed plan asks the daemon again. The cached verdicts are never
// written through by an overlay, and a failed verification is asked again
// once it is older than consolePlansRetry.
func TestConsolePlans_HeadOnlyChangeReusesDaemonVerdicts(t *testing.T) {
	root := t.TempDir()
	plans := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plans, "window.md")
	plan := "# Window\n\n- [x] The runbook names the window {id: W-1, oracle: lexical, path: README.md, pattern: \"^## Window$\"}\n"
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# Runbook\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := &consoleDeps{selfURL: "http://127.0.0.1:1", now: time.Now}
	p := &consolePlans{deps: deps, entries: map[string]*planEntry{}, wake: make(chan struct{}, 1),
		heads: map[string]plansStamp{}, stale: map[string]plansStamp{}, graphs: map[string]*graphEntry{},
		hist: map[string]historyEntry{}}
	sum := sha256.Sum256([]byte(plan))
	rawKey := hex.EncodeToString(sum[:8]) + "|"
	daemonSaid := &verify.Report{Service: "svc", Tasks: []verify.TaskVerdict{{
		TaskID: "W-1", Title: "The runbook names the window", Status: verify.Unknown, Label: "oracle-unevaluated", Claimed: true,
	}}}
	p.entries[planPath] = &planEntry{
		service: "svc", root: root, path: planPath, key: rawKey + "|head-1", rawKey: rawKey,
		done: make(chan struct{}), finished: true, raw: daemonSaid,
	}
	moveHead := func(v string) {
		p.mu.Lock()
		p.heads[root] = plansStamp{value: v, at: time.Now()}
		p.stale["svc"] = plansStamp{at: time.Now()}
		p.mu.Unlock()
	}
	settle := func(e *planEntry) *planEntry {
		t.Helper()
		select {
		case <-e.done:
		case <-time.After(30 * time.Second):
			t.Fatal("the plan never settled")
		}
		return e
	}
	w1 := func(e *planEntry) verify.TaskVerdict {
		t.Helper()
		if e.err != "" {
			t.Fatalf("verification failed: %s", e.err)
		}
		for _, v := range e.result.Report.Tasks {
			if v.TaskID == "W-1" {
				return v
			}
		}
		t.Fatalf("W-1 missing from %+v", e.result.Report.Tasks)
		return verify.TaskVerdict{}
	}
	sr := serviceRoot{Service: "svc", Root: root}

	// HEAD moved, the plan and the index did not: the verdicts are reused
	// and the lexical overlay reads the tree as it is now (no heading).
	moveHead("head-2")
	_, e := p.plan(context.Background(), sr, planPath)
	if e.prevRaw != daemonSaid {
		t.Fatal("a HEAD-only change must carry the daemon's verdicts forward")
	}
	if v := w1(settle(e)); v.Status != verify.Fail {
		t.Errorf("W-1 with no heading = %s/%s, want the overlay's fail", v.Status, v.Label)
	}
	if daemonSaid.Tasks[0].Status != verify.Unknown || daemonSaid.Tasks[0].Label != "oracle-unevaluated" {
		t.Errorf("an overlay wrote through the cached verdicts: %+v", daemonSaid.Tasks[0])
	}

	// The tree gains the heading, HEAD moves again: still no daemon call,
	// and the overlay now passes — the carried verdicts are the daemon's,
	// not the previous overlay's.
	if err := os.WriteFile(readme, []byte("# Runbook\n\n## Window\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moveHead("head-3")
	_, e2 := p.plan(context.Background(), sr, planPath)
	if e2 == e || e2.prevRaw == nil || e2.prevRaw.Tasks[0].Label != "oracle-unevaluated" {
		t.Fatalf("second HEAD move carried %+v", e2.prevRaw)
	}
	if v := w1(settle(e2)); v.Status != verify.Pass {
		t.Errorf("W-1 with the heading = %s/%s, want pass", v.Status, v.Label)
	}

	// The plan itself changed: the daemon is asked again (and here it is
	// unreachable, so the entry says so rather than reusing old verdicts).
	if err := os.WriteFile(planPath, []byte(plan+"- [ ] another row {id: W-2, symbol: Window}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, e3 := p.plan(context.Background(), sr, planPath)
	if e3.prevRaw != nil {
		t.Fatal("a changed plan must not reuse the old plan's verdicts")
	}
	if settle(e3); e3.err == "" || !strings.Contains(e3.err, "daemon") {
		t.Errorf("changed plan against an unreachable daemon = err %q", e3.err)
	}

	// A failure stands (with its reason) for consolePlansRetry, then the
	// next read asks again — a transient failure is never cached for good.
	if _, again := p.plan(context.Background(), sr, planPath); again != e3 {
		t.Error("a fresh failure must stand until the retry interval passes")
	}
	p.mu.Lock()
	e3.at = time.Now().Add(-consolePlansRetry - time.Second)
	p.mu.Unlock()
	if _, e4 := p.plan(context.Background(), sr, planPath); e4 == e3 || e4.prevRaw != nil {
		t.Error("a failure older than the retry interval must be verified again")
	}
}

// TestConsolePlans_UnreadablePlansDirIsAnError pins the console's side of
// I38: a docs/plans that exists and cannot be read makes the repository's
// plans an error with its reason (the portfolio then names it), never zero
// plans — the browser proof saw "0 plans across 0 repositories" while the
// volume's reads were refused.
func TestConsolePlans_UnreadablePlansDirIsAnError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("- [ ] x {id: A-1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	p := &consolePlans{deps: &consoleDeps{now: time.Now}, entries: map[string]*planEntry{}, wake: make(chan struct{}, 1),
		heads: map[string]plansStamp{}, stale: map[string]plansStamp{}, graphs: map[string]*graphEntry{},
		hist: map[string]historyEntry{}}
	sp, _ := p.servicePlans(context.Background(), serviceRoot{Service: "svc", Root: root}, "")
	if sp.Err == "" || len(sp.Plans) != 0 {
		t.Errorf("console discovery of an unreadable docs/plans = %+v, want its error", sp)
	}
}

// TestTruthStateLine_ReasonWrapsAnywhere: a truth state's reason often
// carries a path or a SHA; at 390 px one widened its panel (the OC-12 row's
// oracle error, the refactor page's reading set) until it wrapped anywhere.
func TestTruthStateLine_ReasonWrapsAnywhere(t *testing.T) {
	for _, s := range []console.State{console.StateAbsent, console.StateError, console.StateStale, console.StatePartial} {
		if line := truthStateLine(s, "open /a/very/long/path/that/does/not/break: no such file"); !strings.Contains(line, "[overflow-wrap:anywhere]") {
			t.Errorf("%s line does not wrap: %s", s, line)
		}
	}
}

// TestConsolePlans_VerifyEventsOnlyWhenVerdictsChange pins the plans'
// /events traffic to design §4 and "live, gently": a background check
// tells the plans panels when a plan's verdicts land or change, never on a
// recomputation that changed nothing. One commit to a busy checkout
// re-keyed its 64 plans and published 64 verify events, turning over the
// whole 64-event replay ring and marking every open panel "Updated" with
// nothing new to show.
func TestConsolePlans_VerifyEventsOnlyWhenVerdictsChange(t *testing.T) {
	root := t.TempDir()
	plans := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plans, "window.md")
	plan := "# Window\n\n- [x] The runbook names the window {id: W-1, oracle: lexical, path: README.md, pattern: \"^## Window$\"}\n"
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# Runbook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bus := console.NewEventBus(console.BusOptions{})
	deps := &consoleDeps{selfURL: "http://127.0.0.1:1", now: time.Now, bus: bus}
	p := &consolePlans{deps: deps, entries: map[string]*planEntry{}, wake: make(chan struct{}, 1),
		heads: map[string]plansStamp{}, stale: map[string]plansStamp{}, graphs: map[string]*graphEntry{},
		hist: map[string]historyEntry{}}
	sum := sha256.Sum256([]byte(plan))
	rawKey := hex.EncodeToString(sum[:8]) + "|"
	daemonSaid := &verify.Report{Service: "svc", Tasks: []verify.TaskVerdict{{
		TaskID: "W-1", Title: "The runbook names the window", Status: verify.Unknown, Label: "oracle-unevaluated", Claimed: true,
	}}}
	// The first verdicts, as a cold verification would leave them.
	first := &planEntry{service: "svc", root: root, path: planPath, key: rawKey + "|head-0", rawKey: rawKey, done: make(chan struct{})}
	p.entries[planPath] = first
	sr := serviceRoot{Service: "svc", Root: root}
	moveHead := func(v string) {
		p.mu.Lock()
		p.heads[root] = plansStamp{value: v, at: time.Now()}
		p.stale["svc"] = plansStamp{at: time.Now()}
		p.mu.Unlock()
	}
	run := func(head string) *planEntry {
		t.Helper()
		moveHead(head)
		_, e := p.plan(context.Background(), sr, planPath)
		select {
		case <-e.done:
		case <-time.After(30 * time.Second):
			t.Fatal("the plan never settled")
		}
		return e
	}
	// Seed the landed verdicts through the same path: a HEAD-only change
	// over the daemon's report (no daemon is reachable here).
	first.finished, first.raw = true, daemonSaid
	close(first.done)
	run("head-1")
	landed := bus.LastID()
	if landed == 0 {
		t.Fatal("verdicts landing for a plan must be announced")
	}

	// HEAD moves twice more, the tree does not: the same verdicts, no event.
	run("head-2")
	run("head-3")
	if got := bus.LastID(); got != landed {
		t.Errorf("a recomputation that changed nothing published %d verify event(s)", got-landed)
	}

	// The tree gains the heading: W-1's verdict changes, one event.
	if err := os.WriteFile(readme, []byte("# Runbook\n\n## Window\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("head-4")
	if got := bus.LastID(); got != landed+1 {
		t.Errorf("a changed verdict published %d event(s), want 1", got-landed)
	}
}
