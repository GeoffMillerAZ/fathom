package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// fixtureServiceRoot is the root the console fixture's registry records for
// svc: the one root a ledger reader counts for it (PE-24).
func fixtureServiceRoot(t *testing.T, f *consoleFixture, svc string) string {
	t.Helper()
	root, _, found, err := f.client.ServiceMeta(t.Context(), svc)
	if err != nil || !found || root == "" {
		t.Fatalf("%s has no recorded root: %v", svc, err)
	}
	return root
}

// unreadRoots is the registered roots a test that does not exercise the
// PE-24 rule passes: unread, so every row is kept (I38).
var unreadRoots = ledgerroots.Unread("")

// ledgerRootsFixture is PE-24's fixture: a verify ledger with rows under a
// registered root, under a symlinked spelling of it, under a registered root
// that sits in the scratch dir, under a worktree of the registered root, and
// under an unregistered root.
type ledgerRootsFixture struct {
	base, reg, link, scratch, scratchClone, worktree, other string
	ledger                                                  string
	roots                                                   ledgerroots.Roots
}

func newLedgerRootsFixture(t *testing.T) ledgerRootsFixture {
	t.Helper()
	var f ledgerRootsFixture
	f.base = t.TempDir()
	f.reg = filepath.Join(f.base, "repos", "app")
	f.scratch = filepath.Join(f.base, "scratch")
	f.scratchClone = filepath.Join(f.scratch, "clone")
	f.worktree = filepath.Join(f.reg, ".claude", "worktrees", "agent-1")
	f.other = filepath.Join(f.base, "elsewhere", "other")
	for _, d := range []string{f.reg, f.scratchClone, f.worktree, f.other, filepath.Join(f.base, "links")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.link = filepath.Join(f.base, "links", "app")
	if err := os.Symlink(f.reg, f.link); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(f.reg, "docs", "plans", "p.md"), "# P\n\n- [x] charge {symbol: validateCharge}\n")

	now := time.Now().UTC()
	at := func(minutes int) string { return now.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339) }
	row := func(root, when string, claimed, verified, failing int) string {
		b, _ := json.Marshal(verifyLedgerRow{SchemaVersion: 2, At: when, RepoRoot: root, Service: "app", Plan: "docs/plans/p.md",
			Commit: "c1", Total: 3, Claimed: claimed, Verified: verified, Failing: failing, Origin: verifyOriginCLI})
		return string(b)
	}
	lines := []string{
		row(f.reg, at(60), 2, 1, 1),          // registered, a refusal
		row(f.reg, at(50), 2, 2, 0),          // registered
		row(f.link, at(40), 2, 2, 0),         // the registered root through a symlink
		row(f.other, at(30), 3, 0, 3),        // unregistered, a refusal
		row(f.other, at(20), 3, 0, 3),        // unregistered, a refusal
		row(f.scratchClone, at(10), 3, 0, 3), // registered but scratch
		row(f.worktree, at(5), 3, 0, 3),      // a worktree under the registered root
	}
	f.ledger = filepath.Join(f.base, "fathom-home", "verify-ledger.jsonl")
	mustWrite(t, f.ledger, strings.Join(lines, "\n")+"\n")
	t.Setenv("FATHOM_VERIFY_LEDGER", f.ledger)
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(f.base, "fathom-home", "sessions.jsonl"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(f.base, "fathom-home", "hooks.jsonl"))
	f.roots = ledgerroots.RegisteredRootsWith([]string{f.reg, f.scratchClone}, []string{f.scratch})
	return f
}

// wantIgnoredPhrase is the fixture's ignored rows, named.
const wantIgnoredPhrase = "4 verify-ledger rows under 3 unregistered roots (1 under a scratch dir) ignored"

// TestVerifyLedgerReadersIgnoreUnregisteredRoots pins PE-24: every reader
// of the verify ledger — the gate figure and catches, the flywheel, the
// impact page, proof history, the Projects runs, plans trend and burn-up,
// the realization ladder, the learn pipeline and ask samples, the intent
// audit's plan rows and assess gate results — reads only rows whose
// repo_root is a registered service root, names how many it ignored, keeps
// every row when the registry is unread (and says why), and never rewrites
// the ledger file.
func TestVerifyLedgerReadersIgnoreUnregisteredRoots(t *testing.T) {
	f := newLedgerRootsFixture(t)
	before, err := os.ReadFile(f.ledger)
	if err != nil {
		t.Fatal(err)
	}
	all := readVerifyLedger()
	if len(all) != 7 {
		t.Fatalf("fixture ledger holds %d rows, want 7", len(all))
	}

	t.Run("the one filter", func(t *testing.T) {
		kept, ign := verifyLedgerRegisteredRows(all, f.roots)
		if len(kept) != 3 || ign.Rows != 4 || ign.ScratchRows != 1 || len(ign.Roots) != 3 {
			t.Fatalf("kept %d, ignored %+v; want 3 kept and 4 ignored under 3 roots", len(kept), ign)
		}
		if p := ign.Phrase(verifyLedgerNoun); p != wantIgnoredPhrase {
			t.Fatalf("phrase = %q, want %q", p, wantIgnoredPhrase)
		}
		un, uign := verifyLedgerRegisteredRows(all, ledgerroots.Unread("the daemon did not answer"))
		if len(un) != 7 || uign.Rows != 0 || !strings.Contains(uign.Phrase(verifyLedgerNoun), "the daemon did not answer") {
			t.Fatalf("unread registry kept %d, ignored %+v; want all 7 and the reason", len(un), uign)
		}
	})

	t.Run("gate figure (research report, adoption, impact)", func(t *testing.T) {
		fig := registeredVerifyFigure(all, f.roots)
		if fig.Runs != 3 || fig.Refusals != 1 || !strings.Contains(fig.Sentence(), wantIgnoredPhrase) {
			t.Fatalf("figure %d runs %d refusals: %s", fig.Runs, fig.Refusals, fig.Sentence())
		}
		un := registeredVerifyFigure(all, ledgerroots.Unread("the daemon did not answer"))
		if un.Runs != 7 || !strings.Contains(un.Sentence(), "none ignored: the daemon did not answer") {
			t.Fatalf("unread: %d runs: %s", un.Runs, un.Sentence())
		}
		kept, ign := verifyLedgerRegisteredRows(all, f.roots)
		in := impactInputsFrom(impactSources{verify: kept, verifyIgnored: ign})
		if in.GateRuns != 3 || !strings.Contains(in.GateNote, wantIgnoredPhrase) {
			t.Fatalf("impact gate: %d runs, note %q", in.GateRuns, in.GateNote)
		}
	})

	t.Run("flywheel verify-gate loop", func(t *testing.T) {
		c := checkLoopVerifyGate(obsImpact{}, f.roots)
		if !strings.Contains(c.Evidence, "3 ledger'd gate run(s)") || !strings.Contains(c.Evidence, wantIgnoredPhrase) {
			t.Fatalf("evidence %q", c.Evidence)
		}
	})

	t.Run("catches timeline", func(t *testing.T) {
		src := freshCatchSources(nil, console.CallLogFacts{}, strings.ToLower, console.RegistryUnread(""), f.roots)
		in := catchInputsFrom(src)
		if len(in.VerifyRuns) != 3 || !strings.Contains(in.VerifySource, wantIgnoredPhrase) {
			t.Fatalf("catches: %d runs, source %q", len(in.VerifyRuns), in.VerifySource)
		}
	})

	t.Run("proof history through the model's roots", func(t *testing.T) {
		t.Setenv("TMPDIR", f.scratch) // the OS temp dir is the fixture's scratch dir
		m := observatoryModel{registry: console.RegistryOf([]string{"app", "clone"})}
		m.Services = []obsServiceRow{{fleetServiceRow: fleetServiceRow{Service: "app", Path: f.link}}, {fleetServiceRow: fleetServiceRow{Service: "clone", Path: f.scratchClone}}}
		h, read := console.ProofHistory(context.Background(), f.ledger, console.DefaultHistoryTail, m.ledgerRoots())
		if len(h.Readings) != 3 || !strings.Contains(read.Reason, "4 verify-ledger readings under 3 unregistered roots") {
			t.Fatalf("proof history: %d readings, reason %q", len(h.Readings), read.Reason)
		}
		if roots := (observatoryModel{}).ledgerRoots(); roots.Read || roots.Note == "" {
			t.Fatalf("a model whose registry was not read gave roots %+v; want unread with a reason", roots)
		}
	})

	t.Run("projects runs and gate figure", func(t *testing.T) {
		runs, read := readVerifyRuns(f.ledger)
		kept, ign := console.RegisteredRuns(runs, f.roots)
		v := gateRefusalFigure(kept, read, ign)
		if len(kept) != 3 || v.V != 1 || !strings.Contains(v.Denominator, wantIgnoredPhrase) {
			t.Fatalf("projects: %d runs, figure %+v", len(kept), v)
		}
	})

	t.Run("plans trend and burn-up", func(t *testing.T) {
		ledger := readLedgerRuns(f.roots)
		if len(ledger.Runs) != 3 || ledger.Ignored.Rows != 4 {
			t.Fatalf("ledger runs %d, ignored %+v", len(ledger.Runs), ledger.Ignored)
		}
		tr := plansTrendOf([]serviceRoot{{Service: "app", Root: f.reg}}, ledger)
		if !strings.Contains(tr.Source, wantIgnoredPhrase) {
			t.Fatalf("trend source %q", tr.Source)
		}
		b := planBurnupFrom("app", filepath.Join(f.reg, "docs", "plans", "p.md"), nil, 0, "", &ledger)
		if !strings.Contains(b.Proven.Source, wantIgnoredPhrase) {
			t.Fatalf("burn-up source %q", b.Proven.Source)
		}
	})

	t.Run("realization gating rung", func(t *testing.T) {
		now := time.Now().UTC()
		runs, ignored, note := windowVerifyRuns(now.Add(-24*time.Hour), now.Add(time.Hour), f.roots)
		if note != "" || runs["app"] != 3 || ignored["app"] != 4 {
			t.Fatalf("window runs %v ignored %v note %q", runs, ignored, note)
		}
		l := console.RealizationLadder(console.LadderFacts{Service: "app", HasPlans: true, VerifyRuns: runs["app"], VerifyIgnored: ignored["app"], WindowDays: 14})
		found := false
		for _, r := range l.Rungs {
			if r.Name == "gating" {
				found = strings.Contains(r.Evidence, "3 verify run(s)") && strings.Contains(r.Evidence, "4 more under an unregistered root ignored")
			}
		}
		if !found {
			t.Fatalf("gating rung does not name the ignored runs: %+v", l.Rungs)
		}
	})

	t.Run("learn pipeline and ask sample", func(t *testing.T) {
		g := readVerifyLedgerFor(f.ledger, "app", f.reg, f.roots)
		if g.runs.V != 3 || !strings.Contains(g.runs.Denominator, wantIgnoredPhrase) {
			t.Fatalf("learn gate runs %+v", g.runs)
		}
		choice := learnChoice{Service: "app"}
		choice.Row.Path = f.reg
		row, note, err := samplePlanRow(choice, f.roots)
		if err != nil || row == "" || !strings.Contains(note, wantIgnoredPhrase) {
			t.Fatalf("ask sample %q %q %v", row, note, err)
		}
	})

	t.Run("intent audit plan rows and assess gate results", func(t *testing.T) {
		_, note := planRowStates(f.reg, f.roots)
		if !strings.Contains(note, wantIgnoredPhrase) {
			t.Fatalf("plan row note %q", note)
		}
		// A unit at the unregistered root: its two runs are not gate results.
		now := time.Now().UTC()
		ev, ign := gatherUnitEvidence(f.other, "s1", now.Add(-2*time.Hour), now, f.roots)
		if len(ev.Gates) != 0 || ign.Rows != 2 {
			t.Fatalf("assess at an unregistered root: %d gates, ignored %+v", len(ev.Gates), ign)
		}
		ev, ign = gatherUnitEvidence(f.reg, "s1", now.Add(-2*time.Hour), now, f.roots)
		if len(ev.Gates) != 3 || ign.Rows != 0 {
			t.Fatalf("assess at the registered root: %d gates, ignored %+v", len(ev.Gates), ign)
		}
	})

	after, err := os.ReadFile(f.ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("a reader rewrote the verify ledger")
	}
}

// TestRegisteredLedgerRootsFromTheDaemon pins PE-23's source: the roots are
// the registry's recorded paths, canonical; a daemon that does not answer
// gives unread roots with the reason.
func TestRegisteredLedgerRootsFromTheDaemon(t *testing.T) {
	f := newLedgerRootsFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"app","path":%q},{"service":"clone","path":%q}]}}`, f.link+"/", f.scratchClone)
	}))
	defer srv.Close()
	t.Setenv("TMPDIR", f.scratch)
	roots := registeredLedgerRoots(srv.Client(), srv.URL)
	resolved, _ := filepath.EvalSymlinks(f.reg)
	if !roots.Read || !roots.Registered(f.reg) || roots.Registered(f.scratchClone) || roots.Registered(f.worktree) {
		t.Fatalf("roots %v: Registered(reg)=%v scratch=%v worktree=%v", roots.List(), roots.Registered(f.reg), roots.Registered(f.scratchClone), roots.Registered(f.worktree))
	}
	if l := roots.List(); len(l) != 2 || (l[0] != resolved && l[1] != resolved) {
		t.Fatalf("List() = %v, want the symlink resolved to %s", l, resolved)
	}
	dead := registeredLedgerRoots(&http.Client{Timeout: time.Second}, "http://127.0.0.1:1")
	if dead.Read || !strings.Contains(dead.Note, "could not be read") || dead.Excludes(f.other) {
		t.Fatalf("a daemon that does not answer gave %+v; want unread, excluding nothing, with the reason", dead)
	}
}
