package conserve

// residue_test.go — CV-60/CV-61/CV-62/CV-63 (design §3.4, ADR-044 amended
// 2026-09-25, Q22): the reading set. TestChangedFunctions_
// AddedModifiedMovedDeleted pins ChangedFunctions' own added/modified/
// moved/deleted split; the three TestResidue_* pin Residue's
// classification, honesty and ordering rules.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestChangedFunctions_AddedModifiedMovedDeleted exercises all four
// outcomes in one changed-file set, two files in the same package
// directory "pkg": a stable, unchanged function; one modified in place;
// one moved, unchanged, from pkg/a.go to pkg/b.go (neither added nor
// modified — excluded); one deleted outright (excluded: the compiler and
// the resurrection/absence planes cover it); and one genuinely new
// function (added).
func TestChangedFunctions_AddedModifiedMovedDeleted(t *testing.T) {
	base := map[string]string{
		"pkg/a.go": "package pkg\n\n" +
			"func Stable() {}\n\n" +
			"func ToModify() {}\n\n" +
			"func ToMove() { x := 1\n_ = x }\n\n" +
			"func ToDelete() {}\n",
		"pkg/b.go": "package pkg\n\nfunc Existing() {}\n",
	}
	head := map[string]string{
		"pkg/a.go": "package pkg\n\n" +
			"func Stable() {}\n\n" +
			"func ToModify() { _ = 1 }\n\n" +
			"func NewFunc() {}\n",
		"pkg/b.go": "package pkg\n\nfunc Existing() {}\n\nfunc ToMove() { x := 1\n_ = x }\n",
	}
	in := Input{
		ChangedFiles: []string{"pkg/a.go", "pkg/b.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}

	funcs, missing := ChangedFunctions(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}

	byName := map[string]ChangedFunction{}
	for _, f := range funcs {
		byName[f.Symbol] = f
	}

	if len(funcs) != 2 {
		t.Fatalf("ChangedFunctions = %+v, want exactly 2 (NewFunc added, ToModify modified)", funcs)
	}
	if f, ok := byName["NewFunc"]; !ok || f.Change != "added" {
		t.Errorf("NewFunc = %+v, want change=added present", f)
	}
	if f, ok := byName["ToModify"]; !ok || f.Change != "modified" {
		t.Errorf("ToModify = %+v, want change=modified present", f)
	}
	if _, ok := byName["Stable"]; ok {
		t.Errorf("Stable must not appear (unchanged body): %+v", funcs)
	}
	if _, ok := byName["ToMove"]; ok {
		t.Errorf("ToMove must not appear (moved, unchanged body, same package directory): %+v", funcs)
	}
	if _, ok := byName["ToDelete"]; ok {
		t.Errorf("ToDelete must not appear (deleted functions are never in the reading set): %+v", funcs)
	}
	if _, ok := byName["Existing"]; ok {
		t.Errorf("Existing must not appear (unchanged in an unrelated changed file): %+v", funcs)
	}
}

// TestChangedFunctions_ReadErrorIsMissing pins that an actual read error
// (never "did not exist") is reported in missing rather than silently
// dropping the file.
func TestChangedFunctions_ReadErrorIsMissing(t *testing.T) {
	in := Input{
		ChangedFiles: []string{"pkg/broken.go"},
		HeadFile: func(string) ([]byte, bool, error) {
			return nil, false, errors.New("EPERM")
		},
		BaseFile: mapFile(nil),
	}
	funcs, missing := ChangedFunctions(in)
	if len(funcs) != 0 {
		t.Errorf("funcs = %+v, want none", funcs)
	}
	if len(missing) != 1 {
		t.Fatalf("missing = %v, want exactly one entry", missing)
	}
}

// residueTestStore builds a MemoryStore seeded with the given functions
// (each present in "pkg/x.go") and test functions (in "pkg/x_test.go"),
// wired so callerToCallees[testName] names which functions that test's
// Calls reaches. Returns the store and each symbol's full key by bare
// name, for building typed edges / test runs against.
func residueTestStore(t *testing.T, service string, funcs []string, testCalls map[string][]string) (store.Store, map[string]string) {
	t.Helper()
	st := store.NewMemoryStore()
	keys := map[string]string{}

	mk := func(name, file string, calls []string) core.Observation {
		n := core.SymbolNode{Service: service, Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1}
		keys[name] = n.Key()
		return core.Observation{
			ServiceID: service, Package: "pkg", Symbol: name, SourceKind: core.KindGoFunc,
			SourceRef: file, Line: 1, Calls: calls,
		}
	}

	var obs []core.Observation
	for _, f := range funcs {
		obs = append(obs, mk(f, "pkg/x.go", nil))
	}
	for test, calls := range testCalls {
		var calleeKeys []string
		for _, c := range calls {
			calleeKeys = append(calleeKeys, keys[c]) // callees are always declared in funcs above first
		}
		obs = append(obs, mk(test, "pkg/x_test.go", calleeKeys))
	}
	if err := st.PutObservations(context.Background(), obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	return st, keys
}

// TestResidue_ClassifiesChangedFunctions pins the three-way split (design
// §3.4): Attested reaches a test whose latest run passed, is bound to a
// commit, and whose reach is unchanged since (isAttestedBy/
// ReachUnchangedSince); Unattested is reached but no such run exists;
// Unreached has no reaching test at all.
func TestResidue_ClassifiesChangedFunctions(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	st, keys := residueTestStore(t, svc, []string{"Attested", "Unattested", "Unreached"}, map[string][]string{
		"TestAttested":   {"Attested"},
		"TestUnattested": {"Unattested"},
	})

	if err := st.PutTestRuns(ctx, svc, []store.TestRun{
		{
			RunDigest: "r1", Suite: "s", StartedAt: "2026-01-01T00:00:00Z",
			Commit: "c1", Dirty: false,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestAttested", Status: "pass", TestSymbolKey: keys["TestAttested"]}},
		},
	}); err != nil {
		t.Fatalf("PutTestRuns: %v", err)
	}

	head := map[string]string{
		"pkg/x.go": "package pkg\n\n" +
			"func Attested() {}\n\n" +
			"func Unattested() {}\n\n" +
			"func Unreached() {}\n",
	}
	in := Input{
		Service: svc, ChangedFiles: []string{"pkg/x.go"},
		BaseFile: mapFile(nil), HeadFile: mapFile(head),
		Store: st, IndexAtHead: true,
		ReachUnchangedSince: func(commit string, files []string) (bool, error) {
			return commit == "c1", nil // only the bound, passing run's commit reads unchanged
		},
	}

	rep := Residue(ctx, in, 0)
	if len(rep.Missing) != 0 {
		t.Fatalf("Missing = %v, want none", rep.Missing)
	}
	if rep.Changed != 3 {
		t.Errorf("Changed = %d, want 3", rep.Changed)
	}
	if rep.Attested != 1 || rep.Unattested != 1 || rep.Unreached != 1 {
		t.Fatalf("Attested/Unattested/Unreached = %d/%d/%d, want 1/1/1; items=%+v", rep.Attested, rep.Unattested, rep.Unreached, rep.Items)
	}

	byName := map[string]ResidueItem{}
	for _, it := range rep.Items {
		byName[it.Symbol] = it
	}
	// Attested is proven, never listed among Items (design §3.4: only
	// unreached/unattested need a reviewer's attention).
	if _, ok := byName["Attested"]; ok {
		t.Errorf("Items must never list an attested function: %+v", rep.Items)
	}
	if it, ok := byName["Unattested"]; !ok || it.Class != "unattested" || len(it.Tests) != 1 || it.Tests[0] != "TestUnattested" {
		t.Errorf("Unattested item = %+v, want class=unattested tests=[TestUnattested]", it)
	}
	if it, ok := byName["Unreached"]; !ok || it.Class != "unreached" || len(it.Tests) != 0 {
		t.Errorf("Unreached item = %+v, want class=unreached, no tests", it)
	}
}

// TestResidue_UnprovenWhenIndexNotAtHead pins I44/CV-62: the reading set
// classes nothing — Items empty, every count zero — when the index is not
// at HEAD, the typed pass failed, or a changed file's base could not be
// read, in each case naming what was missing. Changed/Unanalyzed are pure
// diff facts and are still reported (never itself a reason to believe
// anything was actually classified — Missing is what signals that).
func TestResidue_UnprovenWhenIndexNotAtHead(t *testing.T) {
	ctx := context.Background()
	head := map[string]string{"pkg/x.go": "package pkg\n\nfunc New() {}\n"}
	baseIn := func() Input {
		return Input{
			Service: "svc", ChangedFiles: []string{"pkg/x.go", "README.md"},
			BaseFile: mapFile(nil), HeadFile: mapFile(head),
			Store: store.NewMemoryStore(), IndexAtHead: true,
		}
	}

	assertUnproven := func(t *testing.T, rep ResidueReport, wantMissingSubstr string, wantChanged int) {
		t.Helper()
		if len(rep.Missing) == 0 {
			t.Fatalf("Missing = %v, want at least one reason", rep.Missing)
		}
		var found bool
		for _, m := range rep.Missing {
			if strings.Contains(m, wantMissingSubstr) {
				found = true
			}
		}
		if !found {
			t.Errorf("Missing = %v, want an entry containing %q", rep.Missing, wantMissingSubstr)
		}
		if len(rep.Items) != 0 {
			t.Errorf("Items = %+v, want none (classes nothing)", rep.Items)
		}
		if rep.Attested != 0 || rep.Unattested != 0 || rep.Unreached != 0 {
			t.Errorf("Attested/Unattested/Unreached = %d/%d/%d, want 0/0/0", rep.Attested, rep.Unattested, rep.Unreached)
		}
		// The diff itself is still an honest, store-independent fact.
		if rep.Changed != wantChanged {
			t.Errorf("Changed = %d, want %d (the diff computation does not depend on the index)", rep.Changed, wantChanged)
		}
		if len(rep.Unanalyzed) != 1 || rep.Unanalyzed[0] != "README.md" {
			t.Errorf("Unanalyzed = %v, want [README.md]", rep.Unanalyzed)
		}
	}

	t.Run("index not at HEAD", func(t *testing.T) {
		in := baseIn()
		in.IndexAtHead = false
		in.IndexAtHeadReason = "the index was ingested at commit aaa, HEAD is now bbb"
		assertUnproven(t, Residue(ctx, in, 0), "HEAD is now bbb", 1)
	})

	t.Run("typed packages failed", func(t *testing.T) {
		in := baseIn()
		in.FailedTypedPackages = []string{"1/1 package(s) failed typed loading"}
		assertUnproven(t, Residue(ctx, in, 0), "1/1 package(s) failed", 1)
	})

	t.Run("base unreadable", func(t *testing.T) {
		in := baseIn()
		in.BaseFile = func(path string) ([]byte, bool, error) {
			return nil, false, fmt.Errorf("EPERM reading %s", path)
		}
		// ChangedFunctions itself cannot classify pkg/x.go's own functions
		// without base content to compare against — Changed is honestly 0
		// here, not a guess; the read failure is what Missing reports.
		assertUnproven(t, Residue(ctx, in, 0), "reading base content", 0)
	})

	t.Run("no store", func(t *testing.T) {
		in := baseIn()
		in.Store = nil
		assertUnproven(t, Residue(ctx, in, 0), "no store", 1)
	})
}

// TestResidue_OrderAndCap pins CV-63: Items list unreached before
// unattested; within a class, resolved caller count descending, then
// symbol ascending; the list is capped and the cut count reported;
// attested items never appear at all (pinned again here alongside the
// ordering, since a correct sort over an accidentally-included attested
// item would still "look" ordered).
func TestResidue_OrderAndCap(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	// Five changed, unreached-or-unattested functions plus one attested
	// one that must never appear. Distinct resolved-caller counts drive
	// the ordering; UB/UA share a caller count to pin the symbol tie-break.
	// P1a/P1b/P2a/PUB2/PUC2 are plain, dead-end callers (never Test*, never
	// themselves called by anything) — they exist ONLY to give a resolved-
	// caller count without connecting R1/R2 into the test-reachability
	// graph through UA/UB/UC (a direct call chain would make everything
	// transitively "reached").
	st, keys := residueTestStore(t, svc,
		[]string{"UA", "UB", "UC", "R1", "R2", "Attested", "P1a", "P1b", "P2a", "PUB2", "PUC2"},
		map[string][]string{
			"TestAttested": {"Attested"},
			// UA/UB/UC are all reached but never attested (no run at all).
			"TestUA": {"UA"}, "TestUB": {"UB"}, "TestUC": {"UC"},
			// R1/R2 are unreached: no test names them, directly or transitively.
		},
	)
	if err := st.PutTestRuns(ctx, svc, []store.TestRun{{
		RunDigest: "r1", Suite: "s", StartedAt: "t1", Commit: "c1", Dirty: false,
		Cases: []store.TestCase{{CaseID: "1", Name: "TestAttested", Status: "pass", TestSymbolKey: keys["TestAttested"]}},
	}}); err != nil {
		t.Fatalf("PutTestRuns: %v", err)
	}

	// Typed (resolved) edges give each changed function a distinct
	// resolved-caller count: R1=2, R2=1, UC=2, UB=2, UA=1 — UC and UB tie
	// at 2, must then order by symbol (UB < UC).
	edges := []core.Edge{
		{FromSymbol: keys["TestUA"], ToSymbol: keys["UA"], Kind: core.EdgeDirect},
		{FromSymbol: keys["TestUB"], ToSymbol: keys["UB"], Kind: core.EdgeDirect},
		{FromSymbol: keys["PUB2"], ToSymbol: keys["UB"], Kind: core.EdgeDirect}, // UB's 2nd resolved caller
		{FromSymbol: keys["TestUC"], ToSymbol: keys["UC"], Kind: core.EdgeDirect},
		{FromSymbol: keys["PUC2"], ToSymbol: keys["UC"], Kind: core.EdgeDirect}, // UC's 2nd resolved caller
		{FromSymbol: keys["P1a"], ToSymbol: keys["R1"], Kind: core.EdgeDirect},
		{FromSymbol: keys["P1b"], ToSymbol: keys["R1"], Kind: core.EdgeDirect}, // R1's 2nd resolved caller
		{FromSymbol: keys["P2a"], ToSymbol: keys["R2"], Kind: core.EdgeDirect}, // R2's 1 resolved caller
	}
	if err := st.PutTypedEdges(ctx, svc, "pkg", edges); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}

	head := map[string]string{
		"pkg/x.go": "package pkg\n\n" +
			"func UA() {}\n\nfunc UB() {}\n\nfunc UC() {}\n\n" +
			"func R1() {}\n\nfunc R2() {}\n\nfunc Attested() {}\n",
	}
	in := Input{
		Service: svc, ChangedFiles: []string{"pkg/x.go"},
		BaseFile: mapFile(nil), HeadFile: mapFile(head),
		Store: st, IndexAtHead: true,
		ReachUnchangedSince: func(commit string, files []string) (bool, error) { return commit == "c1", nil },
	}

	full := Residue(ctx, in, 0)
	if len(full.Missing) != 0 {
		t.Fatalf("Missing = %v, want none", full.Missing)
	}
	if full.Unreached != 2 || full.Unattested != 3 || full.Attested != 1 {
		t.Fatalf("Unreached/Unattested/Attested = %d/%d/%d, want 2/3/1; items=%+v", full.Unreached, full.Unattested, full.Attested, full.Items)
	}
	wantOrder := []string{"R1", "R2", "UB", "UC", "UA"} // unreached (R1 callers=2, R2=1) then unattested (UB=2,UC=2 tie->symbol, UA=1)
	if len(full.Items) != len(wantOrder) {
		t.Fatalf("Items = %+v, want %d entries in order %v", full.Items, len(wantOrder), wantOrder)
	}
	for i, sym := range wantOrder {
		if full.Items[i].Symbol != sym {
			t.Errorf("Items[%d].Symbol = %q, want %q (full order: %v)", i, full.Items[i].Symbol, sym, itemSymbols(full.Items))
		}
	}
	for _, it := range full.Items {
		if it.Symbol == "Attested" {
			t.Fatalf("Attested must never appear in Items: %+v", full.Items)
		}
	}

	capped := Residue(ctx, in, 2)
	if len(capped.Items) != 2 {
		t.Fatalf("capped Items = %+v, want exactly 2", capped.Items)
	}
	if capped.Truncated != len(wantOrder)-2 {
		t.Errorf("Truncated = %d, want %d", capped.Truncated, len(wantOrder)-2)
	}
	if capped.Items[0].Symbol != "R1" || capped.Items[1].Symbol != "R2" {
		t.Errorf("capped Items = %v, want the top 2 of the full order", itemSymbols(capped.Items))
	}
	// Counts reflect the FULL classification, never the cap.
	if capped.Unreached != 2 || capped.Unattested != 3 {
		t.Errorf("capped Unreached/Unattested = %d/%d, want 2/3 (counts are never truncated)", capped.Unreached, capped.Unattested)
	}
}

func itemSymbols(items []ResidueItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Symbol
	}
	return out
}
