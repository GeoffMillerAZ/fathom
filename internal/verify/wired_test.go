package verify_test

// wired_test.go — "done means proven AND wired" (ADR-018 amendment
// 2026-09-28): a claimed row whose symbol a test reaches but no product
// entry point does is library-only — Unknown, so the default gate reports
// it and --strict refuses it — unless the row declares `library: true`
// with a reason that predates the claim.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// productEvidence adds verify.ProductEvidence to fakeEvidence.
type productEvidence struct {
	fakeEvidence
	entries int
	reached map[string]string // symbol -> entry that reaches it
}

func (p productEvidence) ProductEntryCount() int { return p.entries }
func (p productEvidence) ProductReachedBy(s string) (string, bool) {
	e, ok := p.reached[s]
	return e, ok
}

func goFunc(name, file string) core.SymbolNode {
	return core.SymbolNode{Name: name, FilePath: file, Line: 1, Tier: core.TierVerified, Kind: core.KindGoFunc}
}

// wiredFixture: Wired is called from cmd/app's main; TestOnly and API are
// reached only by TestLib.
func wiredFixture() productEvidence {
	return productEvidence{
		fakeEvidence: fakeEvidence{
			symbols: map[string][]core.SymbolNode{
				"Wired":    {goFunc("Wired", "lib/lib.go")},
				"TestOnly": {goFunc("TestOnly", "lib/lib.go")},
				"API":      {goFunc("API", "lib/lib.go")},
			},
			tests: map[string][]string{
				"Wired":    {"svc::lib/lib.TestLib"},
				"TestOnly": {"svc::lib/lib.TestLib"},
				"API":      {"svc::lib/lib.TestLib"},
			},
		},
		entries: 1,
		reached: map[string]string{"Wired": "svc::cmd/app/main.main"},
	}
}

func verdict(t *testing.T, rep verify.Report, id string) verify.TaskVerdict {
	t.Helper()
	for _, v := range rep.Tasks {
		if v.TaskID == id {
			return v
		}
	}
	t.Fatalf("no verdict for %s", id)
	return verify.TaskVerdict{}
}

func TestWired_TestOnlyRowIsLibraryOnlyAndFailsStrict(t *testing.T) {
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "W-1", Claimed: true, Expect: verify.Expect{Symbol: "Wired"}},
		{ID: "W-2", Claimed: true, Expect: verify.Expect{Symbol: "TestOnly"}},
		{ID: "W-3", Claimed: true, Reason: "the API is the deliverable", Expect: verify.Expect{Symbol: "API", Library: true}},
	}}
	rep := verify.VerifyPlan(plan, wiredFixture())

	if v := verdict(t, rep, "W-1"); v.Status != verify.Pass || v.Label != "verified" {
		t.Errorf("wired row: got %s/%s, want pass/verified", v.Status, v.Label)
	} else if c, _ := findCrit(v, "wired"); c.Status != verify.Pass || !strings.Contains(c.Detail, "cmd/app/main.main") {
		t.Errorf("wired row: wired criterion %+v, want pass naming the entry point", c)
	}
	v := verdict(t, rep, "W-2")
	if v.Status != verify.Unknown || v.Label != "library-only" {
		t.Errorf("test-only row: got %s/%s, want unknown/library-only", v.Status, v.Label)
	}
	if c, _ := findCrit(v, "wired"); c.Status != verify.Unknown {
		t.Errorf("test-only row: wired criterion %+v, want unknown", c)
	}
	if v := verdict(t, rep, "W-3"); v.Status != verify.Pass {
		t.Errorf("library row: got %s/%s, want pass", v.Status, v.Label)
	} else if c, ok := findCrit(v, "library"); !ok || c.Status != verify.Pass || !strings.Contains(c.Detail, "the API is the deliverable") {
		t.Errorf("library row: library criterion %+v, want pass carrying the reason", c)
	}

	strict := verify.Gate(rep, true)
	if strict.Pass || len(strict.Failures) != 1 || !strings.HasPrefix(strict.Failures[0], "W-2 ") || !strings.Contains(strict.Failures[0], "library-only") {
		t.Errorf("strict gate = %+v, want exactly W-2 refused as library-only", strict)
	}
	if loose := verify.Gate(rep, false); !loose.Pass {
		t.Errorf("default gate = %+v, want pass (library-only is Unknown, reported, not a failure)", loose)
	}
}

func TestWired_LibraryWaiverNeedsAReasonAndMayNotBeSelfDeclared(t *testing.T) {
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "L-bare", Claimed: true, Expect: verify.Expect{Symbol: "API", Library: true}},
		{ID: "L-self", Claimed: true, Reason: "because", SelfLibrary: true, Expect: verify.Expect{Symbol: "API", Library: true}},
	}}
	rep := verify.VerifyPlan(plan, wiredFixture())
	for id, want := range map[string]string{"L-bare": "no reason", "L-self": "may not grade its own row"} {
		v := verdict(t, rep, id)
		c, _ := findCrit(v, "wired")
		if v.Status != verify.Unknown || v.Label != "library-only" || !strings.Contains(c.Detail, want) {
			t.Errorf("%s: got %s/%s %q, want unknown/library-only naming %q", id, v.Status, v.Label, c.Detail, want)
		}
	}
}

func TestWired_NotApplicableIsNamed(t *testing.T) {
	ev := wiredFixture()
	ev.symbols["TestHelper"] = []core.SymbolNode{goFunc("TestHelper", "lib/lib_test.go")}
	ev.tests["TestHelper"] = []string{"svc::lib/lib.TestLib"}
	ev.symbols["Graph"] = []core.SymbolNode{{Name: "Graph", FilePath: "lib/graph.go", Tier: core.TierVerified, Kind: core.KindGoType}}
	ev.tests["Graph"] = []string{"svc::lib/lib.TestLib"}
	ev.symbols["render"] = []core.SymbolNode{{Name: "render", FilePath: "web/app.ts", Tier: core.TierVerified, Kind: core.KindJSFunc}}
	ev.tests["render"] = []string{"svc::web/app.test"}
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "N-test", Claimed: true, Expect: verify.Expect{Symbol: "TestHelper"}},
		{ID: "N-type", Claimed: true, Expect: verify.Expect{Symbol: "Graph"}},
		{ID: "N-js", Claimed: true, Expect: verify.Expect{Symbol: "render"}},
	}}
	for _, v := range verify.VerifyPlan(plan, ev).Tasks {
		c, ok := findCrit(v, "wired")
		if !ok || c.Status != verify.NA || c.Detail == "" || v.Status != verify.Pass {
			t.Errorf("%s: wired %+v, status %s — want a named n/a and the row still verified", v.TaskID, c, v.Status)
		}
	}

	// A service with no product entry point at all (a library) has nothing
	// to wire into: n/a, never library-only.
	lib := wiredFixture()
	lib.entries = 0
	v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{{ID: "N-lib", Claimed: true, Expect: verify.Expect{Symbol: "TestOnly"}}}}, lib).Tasks[0]
	if c, _ := findCrit(v, "wired"); c.Status != verify.NA || v.Status != verify.Pass {
		t.Errorf("library service: wired %+v status %s, want n/a and pass", c, v.Status)
	}
}

// TestWired_AmbiguousNameIsWiredWhenAnyFunctionIs: a package: filter that
// leaves two methods of one name (MemoryStore.Put, SQLiteStore.Put)
// resolves to nothing in the call graph; the row is wired when any of
// the functions it names is, and a type sharing the name never counts.
func TestWired_AmbiguousNameIsWiredWhenAnyFunctionIs(t *testing.T) {
	mem := core.SymbolNode{Service: "svc", Package: "store", Name: "MemoryStore.Put", FilePath: "store/memory.go", Tier: core.TierVerified, Kind: core.KindGoMethod}
	sql := core.SymbolNode{Service: "svc", Package: "store", Name: "SQLiteStore.Put", FilePath: "store/sqlite.go", Tier: core.TierVerified, Kind: core.KindGoMethod}
	typ := core.SymbolNode{Service: "svc", Package: "store", Name: "Put", FilePath: "store/put.go", Tier: core.TierVerified, Kind: core.KindGoType}
	ev := productEvidence{
		fakeEvidence: fakeEvidence{symbols: map[string][]core.SymbolNode{"Put": {mem, sql}, "Solo": {mem, typ}}},
		entries:      1,
		reached:      map[string]string{sql.Key(): "svc::cmd/app/main.main", typ.Key(): "svc::cmd/app/main.main"},
	}
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "A-1", Claimed: true, Expect: verify.Expect{Symbol: "Put", Package: "store", SkipTest: true, SkipReason: "contract suite"}},
		{ID: "A-2", Claimed: true, Expect: verify.Expect{Symbol: "Solo", Package: "store", SkipTest: true, SkipReason: "contract suite"}},
	}}
	rep := verify.VerifyPlan(plan, ev)
	if c, _ := findCrit(verdict(t, rep, "A-1"), "wired"); c.Status != verify.Pass || !strings.Contains(c.Detail, sql.Key()) {
		t.Errorf("A-1: wired %+v, want pass naming %s", c, sql.Key())
	}
	if v := verdict(t, rep, "A-2"); v.Label != "library-only" {
		t.Errorf("A-2: a reached TYPE must not wire the method: got %s/%s", v.Status, v.Label)
	}
}

// TestWired_NoProductEvidenceAddsNothing: an Evidence without the
// capability judges a row exactly as before.
func TestWired_NoProductEvidenceAddsNothing(t *testing.T) {
	v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{{ID: "X", Claimed: true, Expect: verify.Expect{Symbol: "TestOnly"}}}}, wiredFixture().fakeEvidence).Tasks[0]
	if _, ok := findCrit(v, "wired"); ok || v.Status != verify.Pass {
		t.Errorf("got %s with criteria %+v, want pass and no wired criterion", v.Status, v.Criteria)
	}
}

func TestWired_LibraryKeyParsesAndLints(t *testing.T) {
	md := "- [x] api {id: L-1, symbol: API, library: true, reason: \"the API is the deliverable\"}\n" +
		"- [x] bare {id: L-2, symbol: API, library: true}\n" +
		"- [x] no symbol {id: L-3, oracle: lexical, path: go.mod, pattern: \"^module\", library: true, reason: \"x\"}\n"
	p := verify.ParseMarkdownPlan("svc", []byte(md))
	if !p.Tasks[0].Expect.Library || p.Tasks[0].Reason != "the API is the deliverable" {
		t.Fatalf("library/reason not parsed: %+v", p.Tasks[0])
	}
	lint := strings.Join(verify.LintPlan(p), "\n")
	if !strings.Contains(lint, "L-2: library: true without a reason") {
		t.Errorf("lint must name a bare library waiver:\n%s", lint)
	}
	if !strings.Contains(lint, "L-3: library: true applies only to a symbol: row") {
		t.Errorf("lint must name a library waiver on a non-symbol row:\n%s", lint)
	}
	if strings.Contains(lint, "L-1:") {
		t.Errorf("a reasoned library waiver is not a lint finding:\n%s", lint)
	}
}

// TestWired_SelfLibraryFromHistory: the O56 comparison marks a claimed
// row whose library: true is new since the prior plan, and the HTTP
// overlay withdraws a waiver the daemon granted without git access.
func TestWired_SelfLibraryFromHistory(t *testing.T) {
	prior := verify.ParseMarkdownPlan("svc", []byte("- [x] a {id: A, symbol: API}\n- [x] b {id: B, symbol: API, library: true, reason: \"r\"}\n"))
	cur := verify.ParseMarkdownPlan("svc", []byte("- [x] a {id: A, symbol: API, library: true, reason: \"r\"}\n- [x] b {id: B, symbol: API, library: true, reason: \"r\"}\n"))
	got := verify.AnnotateSelfSkip(cur, &prior)
	if !got.Tasks[0].SelfLibrary || got.Tasks[1].SelfLibrary {
		t.Fatalf("SelfLibrary = %v/%v, want true for A (gained) and false for B (predates)", got.Tasks[0].SelfLibrary, got.Tasks[1].SelfLibrary)
	}
	if d := verify.AnnotateSelfDeclared(cur, &prior); strings.Join(d.Tasks[0].SelfDeclared, ",") != "library" || !d.Tasks[0].SelfLibrary {
		t.Errorf("--since: A self-declared %v SelfLibrary %v, want [library] and true", d.Tasks[0].SelfDeclared, d.Tasks[0].SelfLibrary)
	}

	// The daemon, with no git, waived A; the overlay withdraws it.
	daemon := verify.VerifyPlan(cur, wiredFixture())
	verify.OverlaySelfSkip(&daemon, got)
	if v := verdict(t, daemon, "A"); v.Status != verify.Unknown || v.Label != "library-only" {
		t.Errorf("overlay: A got %s/%s, want unknown/library-only", v.Status, v.Label)
	}
	if v := verdict(t, daemon, "B"); v.Status != verify.Pass {
		t.Errorf("overlay: B got %s/%s, want pass (its waiver predates the claim)", v.Status, v.Label)
	}
}
