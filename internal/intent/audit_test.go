package intent_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

type fakeOracle struct {
	known map[string]bool
	err   error
}

func (f fakeOracle) Known(names []string) (map[string]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]bool{}
	for _, n := range names {
		out[n] = f.known[n]
	}
	return out, nil
}

// TestEnforcementAuditOneRPC (EP-7): an invariant that names a test or a
// gate step is CHECKED — the named test must exist in the index and the
// gate step in Taskfile/lefthook. A claim that fails is listed, never
// silently downgraded: the doc keeps saying what its author meant, and the
// audit says the claim is unbacked.
func TestEnforcementAuditOneRPC(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Every test this repo's invariants name really exists — so with a
	// truthful oracle there must be no U4 finding at all.
	truthful := map[string]bool{}
	var asked []string
	for _, inv := range c.Invariants {
		for _, tn := range inv.Tests {
			truthful[tn] = true
			asked = append(asked, tn)
		}
	}
	for _, s := range c.Surfaces {
		for _, tn := range s.Tests {
			truthful[tn] = true
		}
	}
	if len(asked) < 20 {
		t.Fatalf("only %d tests named across the invariants — the rung parser regressed", len(asked))
	}

	a := intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{known: truthful},
		PlanRows: []intent.PlanRowState{{Plan: "p", ID: "X-1"}}, Now: time.Now().UTC(),
	})
	for _, cl := range a.Unproven {
		if cl.Kind == "U4" && strings.Contains(cl.Detail, "does not know that test") {
			t.Errorf("a truthful oracle must produce no U4 test findings, got %+v", cl)
		}
	}

	// A LYING corpus — the oracle does not know one named test — is caught,
	// with the remedy and the file:line.
	lying := map[string]bool{}
	for k := range truthful {
		lying[k] = true
	}
	var victim string
	for _, inv := range c.Invariants {
		if len(inv.Tests) > 0 {
			victim = inv.Tests[0]
			break
		}
	}
	delete(lying, victim)
	a = intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{known: lying},
		PlanRows: []intent.PlanRowState{{Plan: "p", ID: "X-1"}}, Now: time.Now().UTC(),
	})
	found := false
	for _, cl := range a.Unproven {
		if cl.Kind == "U4" && strings.Contains(cl.Detail, victim) {
			found = true
			if cl.Remedy == "" || cl.Evidence == "" {
				t.Errorf("every finding carries a remedy and evidence: %+v", cl)
			}
		}
	}
	if !found {
		t.Errorf("an invariant naming a test the index does not know must be listed (test %q)", victim)
	}

	// No oracle at all is UNCOMPUTABLE, never zero findings.
	a = intent.RunAudit(intent.AuditInput{Corpus: c, Now: time.Now().UTC()})
	if len(a.Uncomputable) == 0 {
		t.Error("a missing index must be named as uncomputable, never rendered as 'nothing wrong'")
	}
	a = intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{err: errors.New("daemon down")},
		Now: time.Now().UTC(),
	})
	if !strings.Contains(strings.Join(a.Uncomputable, " "), "daemon down") {
		t.Errorf("an oracle error must surface its cause: %v", a.Uncomputable)
	}
}

// TestUnprovenClaimsEachNamedWithRemedy (EP-8): every source produces
// findings with a kind, a ref, a remedy and evidence — and `stated` rows are
// listed as unenforced, NOT as debt. Making an honest rung into debt is how
// authors learn to fake rungs.
func TestUnprovenClaimsEachNamedWithRemedy(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, inv := range c.Invariants {
		for _, tn := range inv.Tests {
			known[tn] = true
		}
	}
	for _, s := range c.Surfaces {
		for _, tn := range s.Tests {
			known[tn] = true
		}
	}

	a := intent.RunAudit(intent.AuditInput{
		Corpus:  c,
		Symbols: fakeOracle{known: known},
		PlanRows: []intent.PlanRowState{
			{Plan: "docs/plans/x.md", ID: "X-1", Claimed: true, Failing: true, MeasuredAt: "2026-08-26T00:00:00Z"},
			{Plan: "docs/plans/x.md", ID: "X-2", Claimed: true, Unknown: true, MeasuredAt: "2026-08-26T00:00:00Z"},
			{Plan: "docs/plans/x.md", ID: "X-3", Claimed: true},
		},
		Now: time.Now().UTC(),
	})

	for _, cl := range a.Unproven {
		if cl.Kind == "" || cl.Ref == "" || cl.Remedy == "" {
			t.Errorf("every claim needs kind, ref and remedy: %+v", cl)
		}
	}
	if a.ByKind["U1"] != 1 || a.ByKind["U2"] != 1 {
		t.Errorf("a failing row is U1 and a contract-less claim is U2: %v", a.ByKind)
	}
	if a.ByKind["U3"] == 0 {
		t.Errorf("this repo has VIOLATED invariants; they must be listed: %v", a.ByKind)
	}

	// `stated` is honest, not debt.
	if len(a.Unenforced) == 0 {
		t.Error("this repo has `stated` invariants; they belong in Unenforced")
	}
	for _, inv := range a.Unenforced {
		for _, cl := range a.Unproven {
			if cl.Ref == inv.ID && cl.Kind == "U3" {
				t.Errorf("invariant %s is `%s` — honest, not debt", inv.ID, inv.Rung)
			}
		}
	}

	// The output is sorted, so two runs read the same.
	b := intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{known: known},
		PlanRows: []intent.PlanRowState{{Plan: "p", ID: "X-1"}}, Now: time.Now().UTC(),
	})
	c2 := intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{known: known},
		PlanRows: []intent.PlanRowState{{Plan: "p", ID: "X-1"}}, Now: time.Now().UTC(),
	})
	if len(b.Unproven) != len(c2.Unproven) {
		t.Fatalf("two runs disagree: %d vs %d", len(b.Unproven), len(c2.Unproven))
	}
	for i := range b.Unproven {
		if b.Unproven[i] != c2.Unproven[i] {
			t.Errorf("output is not deterministic at %d: %+v vs %+v", i, b.Unproven[i], c2.Unproven[i])
		}
	}

	t.Logf("audit: %v; unenforced %d; uncomputable %v", b.ByKind, len(b.Unenforced), b.Uncomputable)
}
