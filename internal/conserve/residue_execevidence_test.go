package conserve

// residue_execevidence_test.go — CV-69/CV-70/CV-71 (docs/design/
// refactor-conservation.md §3.5, CV-M13): execution evidence.
// TestResidue_ExecutionEvidenceClasses pins the attested/partial/
// unexecuted classification (plus the no-changed-block "cleared" rule);
// TestResidue_UnusableCoverageFallsBack pins the global and per-function
// fallback-to-static-reach rules (I44); TestResidue_CountsReconcile pins
// CV-71's own "Changed always equals the sum of the classes plus
// unanalyzed_functions".

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestResidue_ExecutionEvidenceClasses pins design §3.5's "Classes with
// execution evidence": attested (every changed block executed), partial
// (some did, naming the unexecuted line ranges), unexecuted (none did),
// an added function's changed-block set is EVERY block in its span
// (never git-diff-filtered), and a modified function whose only changed
// LINE carries no coverage block at all (its signature line: a parameter
// renamed, the one statement block starting on the next line) clears to
// attested with no changed block counted against it. (A comment-only
// edit never gets this far: ChangedFunctions does not call it modified —
// residue_commentonly_test.go.)
func TestResidue_ExecutionEvidenceClasses(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	base := map[string]string{
		"pkg/x.go": "package pkg\n\n" +
			"func Attested() { x := 0 }\n\n" +
			"func Partial() { a := 0 }\n\n" +
			"func Unexecuted() { z := 0 }\n\n" +
			"func HeaderOnly(a int) { _ = a }\n",
	}
	head := map[string]string{
		"pkg/x.go": "package pkg\n\n" + // line 1
			"func Attested() { x := 1; _ = x }\n\n" + // line 3
			"func Partial() { a := 1; _ = a; b := 2; _ = b }\n\n" + // line 5
			"func Unexecuted() { z := 9; _ = z }\n\n" + // line 7
			"func AddedFn() { y := 5; _ = y }\n\n" + // line 9
			"func HeaderOnly(renamed int) {\n" + // line 11
			"\t_ = renamed }\n", // line 12
	}

	st := store.NewMemoryStore()
	var obs []core.Observation
	for _, name := range []string{"Attested", "Partial", "Unexecuted", "AddedFn", "HeaderOnly"} {
		obs = append(obs, core.Observation{ServiceID: svc, Package: "pkg", Symbol: name, SourceKind: core.KindGoFunc, SourceRef: "pkg/x.go", Line: 1})
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}

	cov := &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
		"pkg/x.go": {
			{StartLine: 3, StartCol: 18, EndLine: 3, EndCol: 34, NumStmt: 1, Count: 4},  // Attested: executed
			{StartLine: 5, StartCol: 18, EndLine: 5, EndCol: 32, NumStmt: 1, Count: 2},  // Partial: executed half
			{StartLine: 5, StartCol: 34, EndLine: 5, EndCol: 48, NumStmt: 1, Count: 0},  // Partial: unexecuted half
			{StartLine: 7, StartCol: 20, EndLine: 7, EndCol: 36, NumStmt: 1, Count: 0},  // Unexecuted
			{StartLine: 9, StartCol: 17, EndLine: 9, EndCol: 33, NumStmt: 1, Count: 9},  // AddedFn: executed (added -> all blocks)
			{StartLine: 12, StartCol: 2, EndLine: 12, EndCol: 16, NumStmt: 1, Count: 0}, // HeaderOnly's statement block — Count=0 on purpose: must be EXCLUDED (never used) since it never overlaps the changed signature line
		},
	}}

	in := Input{
		Service: svc, ChangedFiles: []string{"pkg/x.go"},
		BaseFile: mapFile(base), HeadFile: mapFile(head),
		Store: st, IndexAtHead: true,
		Coverage:     cov,
		CoverageMeta: CoverageMeta{Commit: "c1", Dirty: false, Passed: true},
		ReachUnchangedSince: func(commit string, files []string) (bool, error) {
			return commit == "c1", nil
		},
		// Only the "modified" functions' own single-statement line
		// changed for Attested/Partial/Unexecuted; HeaderOnly's only
		// changed line is its SIGNATURE (line 11), never its statement
		// (line 12) — the fixture design §3.5's "no changed block
		// clears" rule rests on.
		ChangedLines: func(path string) ([]LineRange, error) {
			if path != "pkg/x.go" {
				return nil, nil
			}
			return []LineRange{{Start: 3, End: 3}, {Start: 5, End: 5}, {Start: 7, End: 7}, {Start: 11, End: 11}}, nil
		},
	}

	rep := Residue(ctx, in, 0)
	if len(rep.Missing) != 0 {
		t.Fatalf("Missing = %v, want none", rep.Missing)
	}
	if rep.CoverageUnusable != "" {
		t.Fatalf("CoverageUnusable = %q, want empty (a usable profile)", rep.CoverageUnusable)
	}
	if rep.Changed != 5 {
		t.Fatalf("Changed = %d, want 5", rep.Changed)
	}

	byName := map[string]ResidueItem{}
	for _, it := range rep.Items {
		byName[it.Symbol] = it
	}
	// Attested items (Attested itself, HeaderOnly once cleared, and
	// AddedFn whose one block executed) never appear in Items — the
	// pre-existing "attested is counted, never listed" rule, unchanged
	// by execution evidence.
	for _, name := range []string{"Attested", "HeaderOnly", "AddedFn"} {
		if _, ok := byName[name]; ok {
			t.Errorf("%s must not appear in Items (class attested): %+v", name, rep.Items)
		}
	}
	if rep.Attested != 3 {
		t.Errorf("Attested = %d, want 3 (Attested, HeaderOnly cleared, AddedFn)", rep.Attested)
	}

	if it, ok := byName["Partial"]; !ok || it.Class != "partial" || it.Evidence != evidenceExecuted {
		t.Errorf("Partial item = %+v (ok=%v), want class=partial evidence=executed", it, ok)
	} else if len(it.UnexecutedLines) != 1 || it.UnexecutedLines[0] != "5-5" {
		t.Errorf("Partial.UnexecutedLines = %v, want [5-5]", it.UnexecutedLines)
	}

	if it, ok := byName["Unexecuted"]; !ok || it.Class != "unexecuted" || it.Evidence != evidenceExecuted {
		t.Errorf("Unexecuted item = %+v (ok=%v), want class=unexecuted evidence=executed", it, ok)
	}

	if rep.Partial != 1 || rep.Unexecuted != 1 {
		t.Fatalf("Partial/Unexecuted = %d/%d, want 1/1; items=%+v", rep.Partial, rep.Unexecuted, rep.Items)
	}
}

// TestResidue_UnusableCoverageFallsBack pins CV-70/I44: a profile whose
// meta is unbound, dirty or from a failing run is never used AT ALL
// (every item falls back to static reach, and the report names why); a
// profile that DOES clear that bar can still be refused for one
// SPECIFIC function whose own file changed since the profile's bound
// commit, while another function in the SAME run still gets execution
// evidence.
func TestResidue_UnusableCoverageFallsBack(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	t.Run("globally unusable (dirty meta) falls back for everything", func(t *testing.T) {
		st := store.NewMemoryStore()
		fKey := core.SymbolNode{Service: svc, Package: "pkg", Name: "F", Kind: core.KindGoFunc, FilePath: "pkg/f.go", Line: 1}.Key()
		testFKey := core.SymbolNode{Service: svc, Package: "pkg", Name: "TestF", Kind: core.KindGoFunc, FilePath: "pkg/f_test.go", Line: 1}.Key()
		if err := st.PutObservations(ctx, []core.Observation{
			{ServiceID: svc, Package: "pkg", Symbol: "F", SourceKind: core.KindGoFunc, SourceRef: "pkg/f.go", Line: 1},
			{ServiceID: svc, Package: "pkg", Symbol: "TestF", SourceKind: core.KindGoFunc, SourceRef: "pkg/f_test.go", Line: 1},
		}); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		if err := st.PutTypedEdges(ctx, svc, "pkg", []core.Edge{{FromSymbol: testFKey, ToSymbol: fKey, Kind: core.EdgeDirect}}); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}
		// Deliberately NO passing run for F: it reads "unattested" under
		// the static rule regardless of coverage, so it stays visible in
		// Items and its Evidence can be inspected directly.

		head := map[string]string{"pkg/f.go": "package pkg\n\nfunc F() { x := 1; _ = x }\n"}
		in := Input{
			Service: svc, ChangedFiles: []string{"pkg/f.go"},
			BaseFile: mapFile(nil), HeadFile: mapFile(head),
			Store: st, IndexAtHead: true,
			Coverage: &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
				"pkg/f.go": {{StartLine: 3, StartCol: 12, EndLine: 3, EndCol: 28, NumStmt: 1, Count: 5}},
			}},
			CoverageMeta: CoverageMeta{Commit: "c1", Dirty: true, Passed: true}, // dirty -> globally unusable
		}
		rep := Residue(ctx, in, 0)
		if len(rep.Missing) != 0 {
			t.Fatalf("Missing = %v, want none", rep.Missing)
		}
		if rep.CoverageUnusable == "" {
			t.Fatal("CoverageUnusable = \"\", want a reason naming the dirty meta")
		}
		byName := map[string]ResidueItem{}
		for _, it := range rep.Items {
			byName[it.Symbol] = it
		}
		it, ok := byName["F"]
		if !ok || it.Class != "unattested" || it.Evidence != evidenceStaticReach {
			t.Fatalf("F item = %+v (ok=%v), want class=unattested evidence=static-reach (fell back)", it, ok)
		}
	})

	t.Run("per-function unusable falls back for just that function", func(t *testing.T) {
		st := store.NewMemoryStore()
		gKey := core.SymbolNode{Service: svc, Package: "pkg", Name: "G", Kind: core.KindGoFunc, FilePath: "pkg/g.go", Line: 1}.Key()
		testGKey := core.SymbolNode{Service: svc, Package: "pkg", Name: "TestG", Kind: core.KindGoFunc, FilePath: "pkg/g_test.go", Line: 1}.Key()
		if err := st.PutObservations(ctx, []core.Observation{
			{ServiceID: svc, Package: "pkg", Symbol: "G", SourceKind: core.KindGoFunc, SourceRef: "pkg/g.go", Line: 1},
			{ServiceID: svc, Package: "pkg", Symbol: "TestG", SourceKind: core.KindGoFunc, SourceRef: "pkg/g_test.go", Line: 1},
			{ServiceID: svc, Package: "pkg", Symbol: "H", SourceKind: core.KindGoFunc, SourceRef: "pkg/h.go", Line: 1},
		}); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		if err := st.PutTypedEdges(ctx, svc, "pkg", []core.Edge{{FromSymbol: testGKey, ToSymbol: gKey, Kind: core.EdgeDirect}}); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}
		// G is reached but never has a passing run (unattested,
		// statically) — a wrong "coverage used anyway" bug would instead
		// read it "attested" (its one block executed) and it would
		// vanish from Items. H has no reach at all (unreached,
		// statically) but a genuinely usable, unexecuted coverage block
		// — a wrong "fell back to static" bug would read it "unreached"
		// instead of "unexecuted".

		head := map[string]string{
			"pkg/g.go": "package pkg\n\nfunc G() { x := 1; _ = x }\n",
			"pkg/h.go": "package pkg\n\nfunc H() { y := 1; _ = y }\n",
		}
		in := Input{
			Service: svc, ChangedFiles: []string{"pkg/g.go", "pkg/h.go"},
			BaseFile: mapFile(nil), HeadFile: mapFile(head),
			Store: st, IndexAtHead: true,
			Coverage: &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
				"pkg/g.go": {{StartLine: 3, StartCol: 12, EndLine: 3, EndCol: 28, NumStmt: 1, Count: 9}},
				"pkg/h.go": {{StartLine: 3, StartCol: 12, EndLine: 3, EndCol: 28, NumStmt: 1, Count: 0}},
			}},
			CoverageMeta: CoverageMeta{Commit: "covc", Dirty: false, Passed: true},
			ReachUnchangedSince: func(commit string, files []string) (bool, error) {
				if commit != "covc" {
					return false, nil
				}
				for _, f := range files {
					if f == "pkg/g.go" {
						return false, nil // simulate: g.go changed since the coverage profile's own commit
					}
				}
				return true, nil
			},
		}
		rep := Residue(ctx, in, 0)
		if len(rep.Missing) != 0 {
			t.Fatalf("Missing = %v, want none", rep.Missing)
		}
		if rep.CoverageUnusable != "" {
			t.Fatalf("CoverageUnusable = %q, want empty (globally usable)", rep.CoverageUnusable)
		}
		byName := map[string]ResidueItem{}
		for _, it := range rep.Items {
			byName[it.Symbol] = it
		}
		if it, ok := byName["G"]; !ok || it.Class != "unattested" || it.Evidence != evidenceStaticReach {
			t.Errorf("G item = %+v (ok=%v), want class=unattested evidence=static-reach (per-function fallback)", it, ok)
		}
		if it, ok := byName["H"]; !ok || it.Class != "unexecuted" || it.Evidence != evidenceExecuted {
			t.Errorf("H item = %+v (ok=%v), want class=unexecuted evidence=executed (coverage used)", it, ok)
		}
	})
}

// TestResidue_PartialWhenFunctionRanButChangedBlockDidNot pins the
// design §3.5/CV-M14 distinction the pre-fix code collapsed: "unexecuted"
// means NO block of the function executed at all; "partial" means the
// function DID execute (some block, changed or not, ran) but some
// CHANGED block never did. A function whose only changed block never ran
// is misclassed "unexecuted" — with NO unexecuted_lines named at all,
// since those are only ever filled for "partial" — whenever the
// classifier looks only at the changed blocks and ignores the rest of
// the function's own span. This is exactly CV-M14's own named case:
// "runServeHTTP ran, but its one changed line never did" must read
// partial, naming the line.
func TestResidue_PartialWhenFunctionRanButChangedBlockDidNot(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	// F(a bool) { x := 1; _ = x; if a { println(1) } } at both base and
	// head, except the println argument changes from 1 to 2 — the ONLY
	// changed line sits inside the "if" body, a block that never runs;
	// the entry block (x := 1 ...) always runs.
	base := map[string]string{
		"pkg/f.go": "package pkg\n\nfunc F(a bool) {\n\tx := 1\n\t_ = x\n\tif a {\n\t\tprintln(1)\n\t}\n}\n",
	}
	head := map[string]string{
		"pkg/f.go": "package pkg\n\nfunc F(a bool) {\n\tx := 1\n\t_ = x\n\tif a {\n\t\tprintln(2)\n\t}\n}\n",
	}

	st := store.NewMemoryStore()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: svc, Package: "pkg", Symbol: "F", SourceKind: core.KindGoFunc, SourceRef: "pkg/f.go", Line: 1},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}

	// Two blocks, mirroring a real `go tool cover` split for this shape:
	// the entry statements (ran, count>0) and the if-body (never ran).
	cov := &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
		"pkg/f.go": {
			{StartLine: 3, StartCol: 16, EndLine: 6, EndCol: 7, NumStmt: 3, Count: 5},
			{StartLine: 6, StartCol: 7, EndLine: 8, EndCol: 3, NumStmt: 1, Count: 0},
		},
	}}

	in := Input{
		Service: svc, ChangedFiles: []string{"pkg/f.go"},
		BaseFile: mapFile(base), HeadFile: mapFile(head),
		Store: st, IndexAtHead: true,
		Coverage:     cov,
		CoverageMeta: CoverageMeta{Commit: "c1", Dirty: false, Passed: true},
		ReachUnchangedSince: func(commit string, files []string) (bool, error) {
			return commit == "c1", nil
		},
		ChangedLines: func(path string) ([]LineRange, error) {
			if path != "pkg/f.go" {
				return nil, nil
			}
			return []LineRange{{Start: 7, End: 7}}, nil
		},
	}

	rep := Residue(ctx, in, 0)
	if rep.CoverageUnusable != "" {
		t.Fatalf("CoverageUnusable = %q, want empty", rep.CoverageUnusable)
	}
	if rep.Unexecuted != 0 {
		t.Errorf("Unexecuted = %d, want 0 — F's entry block ran, so F executed", rep.Unexecuted)
	}
	if rep.Partial != 1 {
		t.Fatalf("Partial = %d, want 1; items=%+v", rep.Partial, rep.Items)
	}
	var f *ResidueItem
	for i := range rep.Items {
		if rep.Items[i].Symbol == "F" {
			f = &rep.Items[i]
		}
	}
	if f == nil {
		t.Fatalf("F not in Items: %+v", rep.Items)
	}
	if f.Class != "partial" || f.Evidence != evidenceExecuted {
		t.Errorf("F = %+v, want class=partial evidence=executed", f)
	}
	if len(f.UnexecutedLines) != 1 || f.UnexecutedLines[0] != "6-8" {
		t.Errorf("F.UnexecutedLines = %v, want [6-8] (naming the unrun if-body)", f.UnexecutedLines)
	}
}

// TestResidue_UnexecutedLinesSortedByLineNumber pins that a "partial"
// item's UnexecutedLines are ordered by START LINE as a number, never as
// a string: "100-101" is not lexicographically before "9-9", but a
// numeric-line-number reader expects it to sort after.
func TestResidue_UnexecutedLinesSortedByLineNumber(t *testing.T) {
	item := &ResidueItem{}
	fnBlocks := []CoverageBlock{
		{StartLine: 5, EndLine: 5, Count: 1},
		{StartLine: 9, EndLine: 9, Count: 0},
		{StartLine: 100, EndLine: 101, Count: 0},
	}
	classifyByExecution(item, fnBlocks, fnBlocks)
	if item.Class != "partial" {
		t.Fatalf("Class = %q, want partial; item=%+v", item.Class, item)
	}
	want := []string{"9-9", "100-101"}
	if len(item.UnexecutedLines) != len(want) || item.UnexecutedLines[0] != want[0] || item.UnexecutedLines[1] != want[1] {
		t.Errorf("UnexecutedLines = %v, want %v (numeric ascending, not lexicographic)", item.UnexecutedLines, want)
	}
}

// TestResidue_CountsReconcile pins CV-71: Changed always equals the sum
// of every class (Attested+Unattested+Unreached+Partial+Unexecuted) plus
// UnanalyzedFunctions — a changed function whose own file the index never
// analyzed at all is counted there, not silently dropped from the total.
func TestResidue_CountsReconcile(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"

	reconciles := func(t *testing.T, rep ResidueReport) {
		t.Helper()
		sum := rep.Attested + rep.Unattested + rep.Unreached + rep.Partial + rep.Unexecuted + rep.UnanalyzedFunctions
		if sum != rep.Changed {
			t.Errorf("Attested+Unattested+Unreached+Partial+Unexecuted+UnanalyzedFunctions = %d, want Changed = %d (rep=%+v)", sum, rep.Changed, rep)
		}
	}

	t.Run("static classes plus an unanalyzed file", func(t *testing.T) {
		st, keys := residueTestStore(t, svc, []string{"Attested", "Unattested", "Unreached"}, map[string][]string{
			"TestAttested":   {"Attested"},
			"TestUnattested": {"Unattested"},
		})
		if err := st.PutTypedEdges(ctx, svc, "pkg", []core.Edge{
			{FromSymbol: keys["TestAttested"], ToSymbol: keys["Attested"], Kind: core.EdgeDirect},
			{FromSymbol: keys["TestUnattested"], ToSymbol: keys["Unattested"], Kind: core.EdgeDirect},
		}); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}
		if err := st.PutTestRuns(ctx, svc, []store.TestRun{{
			RunDigest: "r1", Suite: "s", StartedAt: "t1", Commit: "c1", Dirty: false,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestAttested", Status: "pass", TestSymbolKey: keys["TestAttested"]}},
		}}); err != nil {
			t.Fatalf("PutTestRuns: %v", err)
		}

		head := map[string]string{
			"pkg/x.go":        "package pkg\n\nfunc Attested() {}\n\nfunc Unattested() {}\n\nfunc Unreached() {}\n",
			"vendor/v3p/x.go": "package v3p\n\nfunc Vendored() {}\n", // never indexed at all
		}
		in := Input{
			Service: svc, ChangedFiles: []string{"pkg/x.go", "vendor/v3p/x.go"},
			BaseFile: mapFile(nil), HeadFile: mapFile(head),
			Store: st, IndexAtHead: true,
			ReachUnchangedSince: func(commit string, files []string) (bool, error) { return commit == "c1", nil },
		}
		rep := Residue(ctx, in, 0)
		if len(rep.Missing) != 0 {
			t.Fatalf("Missing = %v, want none", rep.Missing)
		}
		if rep.Changed != 4 {
			t.Fatalf("Changed = %d, want 4 (Attested, Unattested, Unreached, Vendored)", rep.Changed)
		}
		if rep.UnanalyzedFunctions != 1 {
			t.Errorf("UnanalyzedFunctions = %d, want 1 (Vendored, in a file the index never analyzed)", rep.UnanalyzedFunctions)
		}
		reconciles(t, rep)
	})

	t.Run("execution-evidence classes plus an unanalyzed file", func(t *testing.T) {
		st := store.NewMemoryStore()
		if err := st.PutObservations(ctx, []core.Observation{
			{ServiceID: svc, Package: "pkg", Symbol: "Attested", SourceKind: core.KindGoFunc, SourceRef: "pkg/y.go", Line: 1},
			{ServiceID: svc, Package: "pkg", Symbol: "Unexecuted", SourceKind: core.KindGoFunc, SourceRef: "pkg/y.go", Line: 1},
		}); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		head := map[string]string{
			"pkg/y.go":        "package pkg\n\nfunc Attested() { x := 1; _ = x }\n\nfunc Unexecuted() { z := 1; _ = z }\n",
			"vendor/v3p/y.go": "package v3p\n\nfunc Vendored() {}\n",
		}
		in := Input{
			Service: svc, ChangedFiles: []string{"pkg/y.go", "vendor/v3p/y.go"},
			BaseFile: mapFile(nil), HeadFile: mapFile(head),
			Store: st, IndexAtHead: true,
			Coverage: &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
				"pkg/y.go": {
					{StartLine: 3, StartCol: 20, EndLine: 3, EndCol: 36, NumStmt: 1, Count: 4},
					{StartLine: 5, StartCol: 22, EndLine: 5, EndCol: 38, NumStmt: 1, Count: 0},
				},
			}},
			CoverageMeta: CoverageMeta{Commit: "c1", Dirty: false, Passed: true},
			ReachUnchangedSince: func(commit string, files []string) (bool, error) {
				return commit == "c1", nil
			},
		}
		rep := Residue(ctx, in, 0)
		if len(rep.Missing) != 0 {
			t.Fatalf("Missing = %v, want none", rep.Missing)
		}
		if rep.Changed != 3 {
			t.Fatalf("Changed = %d, want 3 (Attested, Unexecuted, Vendored)", rep.Changed)
		}
		if rep.Attested != 1 || rep.Unexecuted != 1 || rep.UnanalyzedFunctions != 1 {
			t.Fatalf("Attested/Unexecuted/UnanalyzedFunctions = %d/%d/%d, want 1/1/1; rep=%+v", rep.Attested, rep.Unexecuted, rep.UnanalyzedFunctions, rep)
		}
		reconciles(t, rep)
	})
}
