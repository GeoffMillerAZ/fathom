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
	victim = "TestNoSuchTestAnywhereInTheTree_IV8"
	c.Invariants[0].Tests = append(append([]string{}, c.Invariants[0].Tests...), victim)
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
	// Every invariant the live corpus marks `violated` is listed as U3 debt —
	// exactly those, and zero is the state the corpus exists to reach (I35,
	// the last one, was answered by ADR-016's 2026-09-24 amendment).
	violated := 0
	for _, inv := range c.Invariants {
		if inv.Rung == intent.RungViolated {
			violated++
		}
	}
	if a.ByKind["U3"] != violated {
		t.Errorf("U3 = %d, want one per violated invariant in the live corpus (%d): %v", a.ByKind["U3"], violated, a.ByKind)
	}
	// The U3 path stays proven when the live corpus has none: mark one
	// invariant violated on a copy and it must be listed with its evidence.
	if len(c.Invariants) > 0 {
		cv := c
		cv.Invariants = append([]intent.Invariant(nil), c.Invariants...)
		cv.Invariants[0].Rung = intent.RungViolated
		av := intent.RunAudit(intent.AuditInput{Corpus: cv, Symbols: fakeOracle{known: known}, Now: time.Now().UTC()})
		found := false
		for _, cl := range av.Unproven {
			if cl.Kind == "U3" && cl.Ref == cv.Invariants[0].ID && cl.Remedy != "" && cl.Evidence != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("a violated invariant (%s) is not listed as U3 debt with remedy and evidence: %+v", cv.Invariants[0].ID, av.Unproven)
		}
	}

	// `stated` is honest, not debt. The live corpus may have none left
	// (PC-14 lifted the last two, I19 and I36, to tests on 2026-09-02) —
	// zero is the state the rung ladder exists to reach, never a failure;
	// what is pinned is that an unenforced invariant is never ALSO listed
	// as U3 debt.
	t.Logf("unenforced (`stated`/`reviewed`) invariants in the live corpus: %d", len(a.Unenforced))
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

// TestAudit_U4ReadsTheRepositoryNotAStaleIndex (IV-8, O1136, I38): an index
// that does not know a test the repository's own test files define must
// not read as a missing test; a name no test file defines still does, and
// a family name ("TestX_*") is answered by a member in the tree.
func TestAudit_U4ReadsTheRepositoryNotAStaleIndex(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Invariants) == 0 {
		t.Fatal("no invariants parsed")
	}
	c.Invariants = c.Invariants[:1]
	c.Surfaces = nil
	c.Invariants[0].GateSteps = nil
	c.Invariants[0].Tests = []string{
		"TestEnforcementAuditOneRPC",      // exists in internal/intent/audit_test.go
		"TestUnprovenClaimsEachNamed*",    // a family with a member in the tree
		"TestNoSuchTestAnywhere_IV8Ghost", // defined nowhere
	}
	a := intent.RunAudit(intent.AuditInput{
		Corpus: c, Symbols: fakeOracle{known: map[string]bool{}}, // a stale index knows nothing
		Now: time.Now().UTC(),
	})
	var missing []string
	for _, cl := range a.Unproven {
		if cl.Kind == "U4" {
			missing = append(missing, cl.Detail)
		}
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "TestNoSuchTestAnywhere_IV8Ghost") {
		t.Fatalf("U4 findings = %v, want exactly the ghost test", missing)
	}
}
