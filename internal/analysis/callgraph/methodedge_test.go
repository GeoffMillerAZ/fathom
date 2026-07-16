package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// The Go extractor names a method node "Recv.Name" with Package=Recv (key
// "svc::Recv.Recv.Name"), while a call site is recorded as "<var>.Name"
// (e.g. "h.RunOnce"). Neither byName, byPkgName, nor "."+raw suffix matching
// can map "<var>.Name" to that key, so every test -> method edge is dropped
// and test_reaches wrongly fails on tested methods. The resolver must map
// "<var>.Method" to a method node when exactly one node key ends in
// "."+Method AND that node has the Recv.Method shape — mirroring the
// "."+symbol suffix matching internal/query/engine.go callsSymbol does.

// TestMethodEdge_ReceiverVarCallResolves pins root cause 4: a Test* node
// calling "h.RunOnce" must reach the "Harness.RunOnce" method node.
func TestMethodEdge_ReceiverVarCallResolves(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Harness", Name: "Harness.RunOnce",
			Kind: core.KindGoMethod, FilePath: "harness/harness.go",
		},
		{
			Service: "svc", Package: "harness_test", Name: "TestHarnessRunOnce",
			Kind: core.KindGoFunc, FilePath: "harness/harness_test.go",
			Calls: []string{"h.RunOnce"},
		},
	}
	g := callgraph.Build(nodes)

	want := []string{"svc::harness_test.TestHarnessRunOnce"}
	if got := callgraph.TestImpact(g, "Harness.RunOnce"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(Harness.RunOnce) = %v, want %v", got, want)
	}
	if !callgraph.Reachable(g, "TestHarnessRunOnce", "Harness.RunOnce") {
		t.Error("Reachable(TestHarnessRunOnce, Harness.RunOnce) = false, want true")
	}
	if got := callgraph.RenameImpact(g, "Harness.RunOnce"); !reflect.DeepEqual(got, want) {
		t.Errorf("RenameImpact(Harness.RunOnce) = %v, want %v", got, want)
	}
}

// TestMethodEdge_AmbiguousMethodResolvesToNothing: a method name defined on
// two receivers must keep resolving to NOTHING — never first-match-wins.
func TestMethodEdge_AmbiguousMethodResolvesToNothing(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "File", Name: "File.Close",
			Kind: core.KindGoMethod, FilePath: "fs/file.go",
		},
		{
			Service: "svc", Package: "Conn", Name: "Conn.Close",
			Kind: core.KindGoMethod, FilePath: "net/conn.go",
		},
		{
			Service: "svc", Package: "fs_test", Name: "TestClose",
			Kind: core.KindGoFunc, FilePath: "fs/file_test.go",
			Calls: []string{"f.Close"},
		},
	}
	g := callgraph.Build(nodes)

	if got := callgraph.TestImpact(g, "svc::File.File.Close"); len(got) != 0 {
		t.Errorf("TestImpact(File.Close) = %v, want [] (ambiguous method)", got)
	}
	if got := callgraph.TestImpact(g, "svc::Conn.Conn.Close"); len(got) != 0 {
		t.Errorf("TestImpact(Conn.Close) = %v, want [] (ambiguous method)", got)
	}
	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount() = %d, want 0 (ambiguous 'f.Close' must not resolve)", g.EdgeCount())
	}
}

// TestMethodEdge_PlainFunctionNotClaimedByVarCall: when the sole ".Method"
// suffix match is a plain function (not Recv.Method shape), a "<var>.Method"
// callee must NOT resolve to it — a receiver-variable call cannot target a
// package-level function.
func TestMethodEdge_PlainFunctionNotClaimedByVarCall(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "alpha", Name: "RunOnce",
			Kind: core.KindGoFunc, FilePath: "alpha/alpha.go",
		},
		{
			Service: "svc", Package: "beta", Name: "OtherFunc",
			Kind: core.KindGoFunc, FilePath: "beta/beta.go",
		},
		{
			Service: "svc", Package: "gamma_test", Name: "TestRun",
			Kind: core.KindGoFunc, FilePath: "gamma/gamma_test.go",
			Calls: []string{"h.RunOnce"}, // some external receiver's method
		},
	}
	g := callgraph.Build(nodes)
	if got := callgraph.TestImpact(g, "svc::alpha.RunOnce"); len(got) != 0 {
		t.Errorf("TestImpact(alpha.RunOnce) = %v, want [] (h.RunOnce is not the function alpha.RunOnce)", got)
	}
}

// TestMethodEdge_FunctionAndMethodSuffixCollision: when both a function
// "pkg.Run" and a method "Recv.Run" end in ".Run", a "<var>.Run" callee is
// ambiguous at the key-suffix level and must resolve to nothing.
func TestMethodEdge_FunctionAndMethodSuffixCollision(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "Ladder", Name: "Ladder.Run",
			Kind: core.KindGoMethod, FilePath: "ladder/ladder.go",
		},
		{
			Service: "svc", Package: "runner", Name: "Run",
			Kind: core.KindGoFunc, FilePath: "runner/runner.go",
		},
		{
			Service: "svc", Package: "ladder_test", Name: "TestLadderRun",
			Kind: core.KindGoFunc, FilePath: "ladder/ladder_test.go",
			Calls: []string{"l.Run"},
		},
	}
	g := callgraph.Build(nodes)
	if got := callgraph.TestImpact(g, "svc::Ladder.Ladder.Run"); len(got) != 0 {
		t.Errorf("TestImpact(Ladder.Run) = %v, want [] (suffix .Run is ambiguous across function and method)", got)
	}
}

// TestMethodEdge_PackageSelectorStillWins: a callee "pkg.Name" where pkg is a
// real indexed package must keep resolving through byPkgName, not the method
// fallback — even when a method node with the same trailing name exists.
func TestMethodEdge_PackageSelectorStillWins(t *testing.T) {
	nodes := []core.SymbolNode{
		{
			Service: "svc", Package: "store", Name: "Open",
			Kind: core.KindGoFunc, FilePath: "store/store.go",
		},
		{
			Service: "svc", Package: "DB", Name: "DB.Open",
			Kind: core.KindGoMethod, FilePath: "db/db.go",
		},
		{
			Service: "svc", Package: "store_test", Name: "TestOpen",
			Kind: core.KindGoFunc, FilePath: "store/store_test.go",
			Calls: []string{"store.Open"},
		},
	}
	g := callgraph.Build(nodes)
	want := []string{"svc::store_test.TestOpen"}
	if got := callgraph.TestImpact(g, "svc::store.Open"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(store.Open) = %v, want %v", got, want)
	}
	if got := callgraph.TestImpact(g, "svc::DB.DB.Open"); len(got) != 0 {
		t.Errorf("TestImpact(DB.Open) = %v, want [] (store.Open is a package selector, not a DB method call)", got)
	}
}

// TestMethodEdge_ComposesWithDedup: the dedup fix (Stage 1) and the
// method-edge fix (Stage 1b) must compose — a triplicated method node still
// resolves a "<var>.Method" callee.
func TestMethodEdge_ComposesWithDedup(t *testing.T) {
	method := core.SymbolNode{
		Service: "svc", Package: "Harness", Name: "Harness.RunStream",
		Kind: core.KindGoMethod, FilePath: "harness/harness.go",
	}
	nodes := []core.SymbolNode{
		method,
		method, // duplicate observation (stale worktree copy)
		method, // duplicate observation (stale worktree copy)
		{
			Service: "svc", Package: "harness_test", Name: "TestRunStream",
			Kind: core.KindGoFunc, FilePath: "harness/harness_test.go",
			Calls: []string{"h.RunStream"},
		},
	}
	g := callgraph.Build(nodes)
	want := []string{"svc::harness_test.TestRunStream"}
	if got := callgraph.TestImpact(g, "Harness.RunStream"); !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(Harness.RunStream) = %v, want %v", got, want)
	}
}
