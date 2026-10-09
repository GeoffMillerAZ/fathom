package symbolnav_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// buildFixture returns a hand-crafted set of SymbolNodes and a Graph built
// from them.  The topology is:
//
//	main → A → B → C
//	            ↘ D
//	TestA → A
//	TestB → B
//
// Packages: main and lib.  Files: main.go (main, TestA, TestB) and lib.go (A, B, C, D).
// Interface Writer with method Write; type MyWriter implements Write.
func buildFixture() ([]core.SymbolNode, *callgraph.Graph) {
	svc := "svc"
	nodes := []core.SymbolNode{
		{
			Service:   svc,
			Package:   "main",
			Name:      "main",
			Kind:      core.KindGoFunc,
			FilePath:  "main.go",
			Line:      5,
			Signature: "func main()",
			Calls:     []string{"svc::lib.A"},
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "lib",
			Name:      "A",
			Kind:      core.KindGoFunc,
			FilePath:  "lib.go",
			Line:      10,
			Signature: "func A()",
			Calls:     []string{"svc::lib.B"},
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "lib",
			Name:      "B",
			Kind:      core.KindGoFunc,
			FilePath:  "lib.go",
			Line:      20,
			Signature: "func B()",
			Calls:     []string{"svc::lib.C", "svc::lib.D"},
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "lib",
			Name:      "C",
			Kind:      core.KindGoFunc,
			FilePath:  "lib.go",
			Line:      30,
			Signature: "func C()",
			Calls:     nil,
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "lib",
			Name:      "D",
			Kind:      core.KindGoFunc,
			FilePath:  "lib.go",
			Line:      40,
			Signature: "func D()",
			Calls:     nil,
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "main",
			Name:      "TestA",
			Kind:      core.KindGoFunc,
			FilePath:  "main.go",
			Line:      50,
			Signature: "func TestA(t *testing.T)",
			Calls:     []string{"svc::lib.A"},
			Tier:      core.TierVerified,
		},
		{
			Service:   svc,
			Package:   "main",
			Name:      "TestB",
			Kind:      core.KindGoFunc,
			FilePath:  "main.go",
			Line:      60,
			Signature: "func TestB(t *testing.T)",
			Calls:     []string{"svc::lib.B"},
			Tier:      core.TierVerified,
		},
		// HTTP handler entrypoint.
		{
			Service:   svc,
			Package:   "main",
			Name:      "ServeHTTP",
			Kind:      core.KindGoMethod,
			FilePath:  "server.go",
			Line:      70,
			Signature: "func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)",
			Calls:     nil,
			Tier:      core.TierVerified,
		},
		// Interface node.
		{
			Service:   svc,
			Package:   "lib",
			Name:      "Writer",
			Kind:      core.KindGoInterface,
			FilePath:  "lib.go",
			Line:      80,
			Signature: "type Writer interface",
			Calls:     []string{"Writer.Write"},
			Tier:      core.TierVerified,
		},
		// Method of Writer (interface method).
		{
			Service:   svc,
			Package:   "lib",
			Name:      "Writer.Write",
			Kind:      core.KindGoMethod,
			FilePath:  "lib.go",
			Line:      82,
			Signature: "Write(p []byte) (int, error)",
			Calls:     nil,
			Tier:      core.TierVerified,
		},
		// Implementor: MyWriter.Write.
		{
			Service:   svc,
			Package:   "lib",
			Name:      "MyWriter.Write",
			Kind:      core.KindGoMethod,
			FilePath:  "mywriter.go",
			Line:      10,
			Signature: "Write(p []byte) (int, error)",
			Calls:     nil,
			Tier:      core.TierVerified,
		},
	}
	g := callgraph.Build(nodes)
	return nodes, g
}

// key is a helper to build "svc::pkg.Name" keys for assertions.
func key(pkg, name string) string {
	return core.SymbolKey("svc", pkg, name)
}

// ─── CallHierarchy ───────────────────────────────────────────────────────────

func TestCallHierarchy_depthZero(t *testing.T) {
	nodes, g := buildFixture()
	tree := symbolnav.CallHierarchy(g, nodes, "svc::lib.A", 0)
	if tree.Key != "svc::lib.A" {
		t.Errorf("depth-0 Key = %q", tree.Key)
	}
	if len(tree.Children) != 0 {
		t.Errorf("depth-0 Children = %v; want none", tree.Children)
	}
}

func TestCallHierarchy_depthOne(t *testing.T) {
	nodes, g := buildFixture()
	tree := symbolnav.CallHierarchy(g, nodes, "svc::lib.A", 1)
	if tree.Key != "svc::lib.A" {
		t.Fatalf("root key wrong: %q", tree.Key)
	}
	// Neighbours of A: callee B, callers main and TestA.
	want := []string{key("lib", "B"), key("main", "TestA"), key("main", "main")}
	sort.Strings(want)
	var got []string
	for _, c := range tree.Children {
		got = append(got, c.Key)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CallHierarchy(A,1) children = %v; want %v", got, want)
	}
}

func TestCallHierarchy_deterministic(t *testing.T) {
	nodes, g := buildFixture()
	a := symbolnav.CallHierarchy(g, nodes, "svc::lib.A", 2)
	b := symbolnav.CallHierarchy(g, nodes, "svc::lib.A", 2)
	if !treeEqual(a, b) {
		t.Errorf("CallHierarchy not deterministic")
	}
}

// treeEqual is a recursive equality check for Tree.
func treeEqual(a, b symbolnav.Tree) bool {
	if a.Key != b.Key {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !treeEqual(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

// ─── Community ───────────────────────────────────────────────────────────────

func TestCommunity_basic(t *testing.T) {
	nodes, g := buildFixture()
	got := symbolnav.Community(g, nodes, "svc::lib.B")
	// B's community: callee C, callee D, callers A; same package/file: A, C, D, Writer, Writer.Write
	// same file lib.go: A, C, D, Writer, Writer.Write
	// Result must contain at least A, C, D and be sorted.
	checkContains(t, "Community(B)", got, key("lib", "A"))
	checkContains(t, "Community(B)", got, key("lib", "C"))
	checkContains(t, "Community(B)", got, key("lib", "D"))
	// Must not contain B itself.
	for _, k := range got {
		if k == key("lib", "B") {
			t.Errorf("Community(B) contains B itself")
		}
	}
	// Must be sorted.
	assertSorted(t, "Community(B)", got)
}

func TestCommunity_deterministic(t *testing.T) {
	nodes, g := buildFixture()
	a := symbolnav.Community(g, nodes, "svc::lib.B")
	b := symbolnav.Community(g, nodes, "svc::lib.B")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Community not deterministic: %v vs %v", a, b)
	}
}

// ─── SymbolContext ────────────────────────────────────────────────────────────

func TestSymbolContext_basic(t *testing.T) {
	nodes, g := buildFixture()
	ctx := symbolnav.SymbolContext(nodes, g, "svc::lib.B")

	if ctx.Def.Name != "B" {
		t.Errorf("Def.Name = %q; want B", ctx.Def.Name)
	}

	// Callees of B: C, D.
	wantCallees := []string{key("lib", "C"), key("lib", "D")}
	sort.Strings(wantCallees)
	if !reflect.DeepEqual(ctx.Callees, wantCallees) {
		t.Errorf("Callees = %v; want %v", ctx.Callees, wantCallees)
	}

	// Callers of B: A and TestB (both call B directly).
	wantCallers := []string{key("lib", "A"), key("main", "TestB")}
	sort.Strings(wantCallers)
	callersCopy := make([]string, len(ctx.Callers))
	copy(callersCopy, ctx.Callers)
	sort.Strings(callersCopy)
	if !reflect.DeepEqual(callersCopy, wantCallers) {
		t.Errorf("Callers = %v; want %v", ctx.Callers, wantCallers)
	}

	// Related tests of B: TestB (TestB directly calls B).
	wantTests := []string{key("main", "TestB")}
	if !reflect.DeepEqual(ctx.RelatedTests, wantTests) {
		t.Errorf("RelatedTests = %v; want %v", ctx.RelatedTests, wantTests)
	}
	// Sanity: RelatedTests must be a subset of callers.
	callerSet := make(map[string]struct{})
	for _, c := range ctx.Callers {
		callerSet[c] = struct{}{}
	}
	for _, rt := range ctx.RelatedTests {
		if _, ok := callerSet[rt]; !ok {
			t.Errorf("RelatedTest %q not in Callers %v", rt, ctx.Callers)
		}
	}
}

func TestSymbolContext_unknown(t *testing.T) {
	nodes, g := buildFixture()
	ctx := symbolnav.SymbolContext(nodes, g, "no::such.Sym")
	if ctx.Def.Name != "" {
		t.Errorf("expected zero Def, got %v", ctx.Def)
	}
}

func TestSymbolContext_deterministic(t *testing.T) {
	nodes, g := buildFixture()
	a := symbolnav.SymbolContext(nodes, g, "svc::lib.A")
	b := symbolnav.SymbolContext(nodes, g, "svc::lib.A")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("SymbolContext not deterministic")
	}
}

// ─── Entrypoints ─────────────────────────────────────────────────────────────

func TestEntrypoints_containsMain(t *testing.T) {
	nodes, _ := buildFixture()
	eps := symbolnav.Entrypoints(nodes)
	var keys []string
	for _, n := range eps {
		keys = append(keys, n.Key())
	}
	checkContains(t, "Entrypoints", keys, key("main", "main"))
}

func TestEntrypoints_containsHTTPHandler(t *testing.T) {
	nodes, _ := buildFixture()
	eps := symbolnav.Entrypoints(nodes)
	found := false
	for _, n := range eps {
		if n.Name == "ServeHTTP" {
			found = true
		}
	}
	if !found {
		t.Errorf("Entrypoints: ServeHTTP not found; got %v", eps)
	}
}

func TestEntrypoints_sorted(t *testing.T) {
	nodes, _ := buildFixture()
	eps := symbolnav.Entrypoints(nodes)
	for i := 1; i < len(eps); i++ {
		if eps[i-1].Key() > eps[i].Key() {
			t.Errorf("Entrypoints not sorted: %q > %q", eps[i-1].Key(), eps[i].Key())
		}
	}
}

func TestEntrypoints_deterministic(t *testing.T) {
	nodes, _ := buildFixture()
	a := symbolnav.Entrypoints(nodes)
	b := symbolnav.Entrypoints(nodes)
	if len(a) != len(b) {
		t.Fatalf("Entrypoints not deterministic: lengths %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Key() != b[i].Key() {
			t.Errorf("Entrypoints not deterministic at index %d", i)
		}
	}
}

// TestEntrypoints_UnityMessages (LB-F290): a Unity message (Update,
// OnTriggerEnter, ...) of a C# class whose base list names MonoBehaviour,
// directly, qualified, or through a class of the repository that does, is an
// engine callback: an entrypoint, so the repo map and dead-code readers never
// call it unreached. The same name elsewhere (a service's Start, a
// MonoBehaviour's own helper, a Unity message that is a property) is not.
func TestEntrypoints_UnityMessages(t *testing.T) {
	cls := func(file, pkg, name, sig string) core.SymbolNode {
		return core.SymbolNode{Service: "game", Package: pkg, Name: name, Kind: "CSharpClass", FilePath: file, Line: 3, Signature: sig}
	}
	method := func(file, pkg, name, sig string) core.SymbolNode {
		return core.SymbolNode{Service: "game", Package: pkg, Name: name, Kind: "CSharpMethod", FilePath: file, Line: 5, Signature: sig}
	}
	nodes := []core.SymbolNode{
		cls("Assets/Player.cs", "Game", "Player", "[RequireComponent(typeof(Rigidbody))] public class Player : MonoBehaviour, IPointerClickHandler"),
		method("Assets/Player.cs", "Game", "Player.Update", "void Update()"),
		method("Assets/Player.cs", "Game", "Player.OnTriggerEnter", "private void OnTriggerEnter(Collider other)"),
		method("Assets/Player.cs", "Game", "Player.Jump", "public void Jump()"),
		{Service: "game", Package: "Game", Name: "Player.Start", Kind: "CSharpProperty", FilePath: "Assets/Player.cs", Line: 9},
		// a partial class: the base list is written in another file
		cls("Assets/Player.Input.cs", "Game", "Player", "public partial class Player"),
		method("Assets/Player.Input.cs", "Game", "Player.LateUpdate", "void LateUpdate()"),
		// through a class of the repository, a qualified and a generic base
		cls("Assets/Character.cs", "Game", "Character", "public abstract class Character : UnityEngine.MonoBehaviour"),
		cls("Assets/Enemy.cs", "Game.Npc", "Enemy", "public class Enemy : Character"),
		method("Assets/Enemy.cs", "Game.Npc", "Enemy.Awake", "protected override void Awake()"),
		cls("Assets/Pool.cs", "Game", "Pool", "public class Pool<T> : global::UnityEngine.MonoBehaviour where T : Component"),
		cls("Assets/Pool.cs", "Game", "Pool.Slot", "class Slot : MonoBehaviour"),
		method("Assets/Pool.cs", "Game", "Pool.Slot.OnDisable", "void OnDisable()"),
		method("Assets/Pool.cs", "Game", "Pool.Start", "IEnumerator Start()"),
		// no MonoBehaviour anywhere in the chain
		cls("Server/Service.cs", "App", "Service", "public class Service : BackgroundService"),
		method("Server/Service.cs", "App", "Service.Start", "public void Start()"),
		method("Server/Service.cs", "App", "Service.Update", "public void Update(int id)"),
	}
	var got []string
	for _, n := range symbolnav.Entrypoints(nodes) {
		got = append(got, n.FilePath+"|"+n.Name)
	}
	want := []string{
		"Assets/Enemy.cs|Enemy.Awake",
		"Assets/Player.Input.cs|Player.LateUpdate",
		"Assets/Player.cs|Player.OnTriggerEnter",
		"Assets/Player.cs|Player.Update",
		"Assets/Pool.cs|Pool.Slot.OnDisable",
		"Assets/Pool.cs|Pool.Start",
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Entrypoints:\n  %v\nwant:\n  %v", got, want)
	}
}

// ─── Outline ─────────────────────────────────────────────────────────────────

func TestOutline_libGo(t *testing.T) {
	nodes, _ := buildFixture()
	outline := symbolnav.Outline(nodes, "lib.go")
	// lib.go contains: A(10), B(20), C(30), D(40), Writer(80), Writer.Write(82), MyWriter is in mywriter.go
	if len(outline) == 0 {
		t.Fatal("Outline(lib.go) is empty")
	}
	// Must be sorted by Line.
	for i := 1; i < len(outline); i++ {
		if outline[i-1].Line > outline[i].Line {
			t.Errorf("Outline not ordered by Line: %d > %d", outline[i-1].Line, outline[i].Line)
		}
	}
	if outline[0].Name != "A" {
		t.Errorf("First symbol = %q; want A", outline[0].Name)
	}
}

func TestOutline_emptyFile(t *testing.T) {
	nodes, _ := buildFixture()
	outline := symbolnav.Outline(nodes, "nonexistent.go")
	if len(outline) != 0 {
		t.Errorf("Outline(nonexistent) = %v; want []", outline)
	}
}

func TestOutline_deterministic(t *testing.T) {
	nodes, _ := buildFixture()
	a := symbolnav.Outline(nodes, "lib.go")
	b := symbolnav.Outline(nodes, "lib.go")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Outline not deterministic")
	}
}

// TestOutline_relativePathMatchesAbsoluteStore is the O1 regression: the store
// keys FilePath absolutely, but the `outline` tool contract accepts a
// repo-relative path. A relative query must return the same symbols as the
// absolute one, and a segment-boundary suffix must not false-match a sibling.
func TestOutline_relativePathMatchesAbsoluteStore(t *testing.T) {
	abs := "/repo/orbit/app/participants/participant.go"
	nodes := []core.SymbolNode{
		{Name: "Participant", Kind: core.KindGoType, FilePath: abs, Line: 44},
		{Name: "Participant.HasAlias", Kind: core.KindGoMethod, FilePath: abs, Line: 107},
		// A sibling whose name is a substring of the query's basename: must NOT match.
		{Name: "Other", Kind: core.KindGoType, FilePath: "/repo/orbit/app/myparticipant.go", Line: 10},
	}

	rel := symbolnav.Outline(nodes, "app/participants/participant.go")
	if len(rel) != 2 {
		t.Fatalf("relative Outline got %d symbols; want 2 (%+v)", len(rel), rel)
	}
	if rel[0].Name != "Participant" || rel[1].Name != "Participant.HasAlias" {
		t.Errorf("relative Outline = %q,%q; want Participant,Participant.HasAlias", rel[0].Name, rel[1].Name)
	}

	// Absolute query returns the identical set.
	absOut := symbolnav.Outline(nodes, abs)
	if !reflect.DeepEqual(rel, absOut) {
		t.Errorf("relative and absolute Outline disagree:\n rel=%+v\n abs=%+v", rel, absOut)
	}

	// Bare basename also resolves on the segment boundary, never the sibling.
	bare := symbolnav.Outline(nodes, "participant.go")
	if len(bare) != 2 {
		t.Errorf("bare-basename Outline got %d; want 2 (sibling myparticipant.go must not match)", len(bare))
	}
}

// ─── Implementations ─────────────────────────────────────────────────────────

func TestImplementations_basic(t *testing.T) {
	nodes, _ := buildFixture()
	impls := symbolnav.Implementations(nodes, "Writer")
	// MyWriter has Write method, so it should appear.
	checkContains(t, "Implementations(Writer)", impls, "MyWriter")
	assertSorted(t, "Implementations(Writer)", impls)
}

func TestImplementations_noMatch(t *testing.T) {
	nodes, _ := buildFixture()
	impls := symbolnav.Implementations(nodes, "NoSuchInterface")
	if len(impls) != 0 {
		t.Errorf("Implementations(NoSuchInterface) = %v; want []", impls)
	}
}

func TestImplementations_deterministic(t *testing.T) {
	nodes, _ := buildFixture()
	a := symbolnav.Implementations(nodes, "Writer")
	b := symbolnav.Implementations(nodes, "Writer")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Implementations not deterministic: %v vs %v", a, b)
	}
}

// ─── TypeHierarchy ───────────────────────────────────────────────────────────

func TestTypeHierarchy_basic(t *testing.T) {
	nodes, _ := buildFixture()
	hier := symbolnav.TypeHierarchy(nodes)
	impls, ok := hier["Writer"]
	if !ok {
		t.Fatalf("TypeHierarchy: Writer not found; map = %v", hier)
	}
	checkContains(t, "TypeHierarchy[Writer]", impls, "MyWriter")
	assertSorted(t, "TypeHierarchy[Writer]", impls)
}

func TestTypeHierarchy_deterministic(t *testing.T) {
	nodes, _ := buildFixture()
	a := symbolnav.TypeHierarchy(nodes)
	b := symbolnav.TypeHierarchy(nodes)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("TypeHierarchy not deterministic")
	}
}

// ─── SymbolDiff ──────────────────────────────────────────────────────────────

func TestSymbolDiff_added(t *testing.T) {
	old := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
	}
	newNodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
		{Service: "s", Package: "p", Name: "B", FilePath: "b.go", Signature: "func B()"},
	}
	diff := symbolnav.SymbolDiff(old, newNodes)
	if !reflect.DeepEqual(diff.Added, []string{"s::p.B"}) {
		t.Errorf("Added = %v; want [s::p.B]", diff.Added)
	}
	if len(diff.Removed) != 0 {
		t.Errorf("Removed = %v; want []", diff.Removed)
	}
}

func TestSymbolDiff_removed(t *testing.T) {
	old := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
		{Service: "s", Package: "p", Name: "B", FilePath: "b.go", Signature: "func B()"},
	}
	newNodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
	}
	diff := symbolnav.SymbolDiff(old, newNodes)
	if !reflect.DeepEqual(diff.Removed, []string{"s::p.B"}) {
		t.Errorf("Removed = %v; want [s::p.B]", diff.Removed)
	}
}

func TestSymbolDiff_moved(t *testing.T) {
	old := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
	}
	newNodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "b.go", Signature: "func A()"},
	}
	diff := symbolnav.SymbolDiff(old, newNodes)
	if !reflect.DeepEqual(diff.Moved, []string{"s::p.A"}) {
		t.Errorf("Moved = %v; want [s::p.A]", diff.Moved)
	}
	if len(diff.Modified) != 0 {
		t.Errorf("Modified = %v; want [] (moved takes precedence)", diff.Modified)
	}
}

func TestSymbolDiff_modified(t *testing.T) {
	old := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
	}
	newNodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A() error"},
	}
	diff := symbolnav.SymbolDiff(old, newNodes)
	if !reflect.DeepEqual(diff.Modified, []string{"s::p.A"}) {
		t.Errorf("Modified = %v; want [s::p.A]", diff.Modified)
	}
	if len(diff.Moved) != 0 {
		t.Errorf("Moved = %v; want []", diff.Moved)
	}
}

func TestSymbolDiff_noChange(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
	}
	diff := symbolnav.SymbolDiff(nodes, nodes)
	if len(diff.Added)+len(diff.Removed)+len(diff.Moved)+len(diff.Modified) != 0 {
		t.Errorf("expected empty diff, got %+v", diff)
	}
}

func TestSymbolDiff_deterministic(t *testing.T) {
	old := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", FilePath: "a.go", Signature: "func A()"},
		{Service: "s", Package: "p", Name: "B", FilePath: "b.go", Signature: "func B()"},
	}
	newNodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "B", FilePath: "b.go", Signature: "func B() error"},
		{Service: "s", Package: "p", Name: "C", FilePath: "c.go", Signature: "func C()"},
	}
	a := symbolnav.SymbolDiff(old, newNodes)
	b := symbolnav.SymbolDiff(old, newNodes)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("SymbolDiff not deterministic")
	}
}

// ─── test helpers ─────────────────────────────────────────────────────────────

func checkContains(t *testing.T, label string, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("%s: %q not found in %v", label, want, slice)
}

func assertSorted(t *testing.T, label string, s []string) {
	t.Helper()
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			t.Errorf("%s: not sorted at index %d: %q > %q", label, i, s[i-1], s[i])
		}
	}
}

// TestEntrypoints_UnityMessagesFromSource (LB-F290) runs the rule on what the
// C# dialect extracts: the class's signature carries its base list.
func TestEntrypoints_UnityMessagesFromSource(t *testing.T) {
	e, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := "using UnityEngine;\n\nnamespace Game\n{\n    [RequireComponent(typeof(Rigidbody))]\n    public class Player : MonoBehaviour\n    {\n" +
		"        void Update() { Move(); }\n        void Move() { }\n        private void OnCollisionEnter(Collision c) { }\n    }\n\n" +
		"    public class Inventory\n    {\n        public void Start() { }\n    }\n}\n"
	nodes, err := e.Extract("game", "Assets/Player.cs", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range symbolnav.Entrypoints(nodes) {
		got = append(got, n.Name)
	}
	if want := []string{"Player.OnCollisionEnter", "Player.Update"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Entrypoints = %v, want %v", got, want)
	}
}
