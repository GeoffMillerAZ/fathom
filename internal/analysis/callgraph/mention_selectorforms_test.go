package callgraph_test

// mention_selectorforms_test.go — DG-21 (O39, .fathom/observations.md): a
// full-stack reproduction (real fusion.Pipeline.Ingest + real
// fusion.EnrichTypedEdges, no hand-built core.Edge/core.SymbolNode values)
// of the three call shapes the owner's amendment named as the reproduction
// target: a method called via a selector on a VALUE, the SAME method
// called via a selector on a POINTER, and an INTERFACE-dispatch call.
// Exercises callgraph.MentionConfidence directly — the exact code path
// internal/mcp/references_tool.go's exact mode uses (SB-45) — against the
// typedfixture module's selectorforms.go and adapter.go.
//
// Every one of the four selector-call sites here (two receiver shapes ×
// two call syntaxes) and the one interface-dispatch call site resolve to
// EdgeConfidenceResolved: this test found MentionConfidence's own
// mechanism sound for all five shapes against a real ingest, so it stands
// as a NEGATIVE result for the "receiver-shape mismatch" hypothesis this
// amendment asked to check — see the O39 Status update this test's name is
// recorded under for what that rules out and what remains open.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// mentionFixtureIngest ingests the shared typedfixture module (real
// extraction + real typed-edge resolution) and returns its nodes and typed
// edges — the same real-pipeline shape internal/verify/
// typededge_residue_test.go uses, reused here because selectorforms.go and
// adapter.go already live in that fixture.
func mentionFixtureIngest(t *testing.T) ([]core.SymbolNode, []core.Edge) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "fusion", "testdata", "typedfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	svc := core.ServiceRef{ID: "typedfixture", Path: dir}
	if _, err := pipe.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, "typedfixture", "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("no observations stored — Ingest found no Go source at the fixture path")
	}
	nodes := make([]core.SymbolNode, len(obs))
	for i, o := range obs {
		nodes[i] = o.ToSymbolNode()
	}
	typed, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(typed) == 0 {
		t.Fatal("no typed edges stored — the typed pass did not run against the fixture module")
	}
	return nodes, typed
}

// findNode returns the (unique, top-level) node named name, failing the
// test if it is not found — used to read the REAL line goextract recorded
// for a caller function, rather than hardcoding it.
func findNode(t *testing.T, nodes []core.SymbolNode, name string) core.SymbolNode {
	t.Helper()
	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node named %q in the fixture's ingested symbols", name)
	return core.SymbolNode{}
}

// TestMentionConfidence_SelectorOnValueAndPointer is DG-21 (O39): the
// value-receiver method Greeter.Greet, called once through a value
// variable (GreetViaValue) and once through a pointer variable
// (GreetViaPointer) — the caller's own enclosing-symbol lookup and the
// typed edge's target key must agree for BOTH call syntaxes, since
// namedTypeName (internal/fusion/typedcalls.go) canonicalizes the receiver
// key by stripping the pointer.
func TestMentionConfidence_SelectorOnValueAndPointer(t *testing.T) {
	nodes, typed := mentionFixtureIngest(t)
	greet := findNode(t, nodes, "Greeter.Greet")
	conf := callgraph.MentionConfidence(nodes, typed, []string{greet.Key()})

	viaValue := findNode(t, nodes, "GreetViaValue")
	if got := conf(viaValue.FilePath, viaValue.Line+1); got != core.EdgeConfidenceResolved {
		t.Errorf("GreetViaValue's g.Greet() call = %v, want resolved (selector on a VALUE)", got)
	}

	viaPointer := findNode(t, nodes, "GreetViaPointer")
	if got := conf(viaPointer.FilePath, viaPointer.Line+1); got != core.EdgeConfidenceResolved {
		t.Errorf("GreetViaPointer's g.Greet() call = %v, want resolved (selector on a POINTER, same value-receiver method)", got)
	}
}

// TestMentionConfidence_PointerReceiverViaPointerAndAddressableValue is
// DG-21 (O39)'s pointer-receiver counterpart: Greeter.SetName, called once
// through a pointer variable and once through an addressable value
// parameter (Go auto-takes the address).
func TestMentionConfidence_PointerReceiverViaPointerAndAddressableValue(t *testing.T) {
	nodes, typed := mentionFixtureIngest(t)
	setName := findNode(t, nodes, "Greeter.SetName")
	conf := callgraph.MentionConfidence(nodes, typed, []string{setName.Key()})

	viaPointer := findNode(t, nodes, "SetNameViaPointer")
	if got := conf(viaPointer.FilePath, viaPointer.Line+1); got != core.EdgeConfidenceResolved {
		t.Errorf("SetNameViaPointer's g.SetName() call = %v, want resolved (selector on a POINTER)", got)
	}

	viaValue := findNode(t, nodes, "SetNameViaAddressableValue")
	if got := conf(viaValue.FilePath, viaValue.Line+1); got != core.EdgeConfidenceResolved {
		t.Errorf("SetNameViaAddressableValue's g.SetName() call = %v, want resolved (selector on an addressable VALUE, same pointer-receiver method)", got)
	}
}

// TestMentionConfidence_InterfaceDispatch is DG-21 (O39)'s third named
// shape: a.Complete(prompt) in RunAdapter, dispatched through the Adapter
// interface to two concrete implementers (AdapterA, AdapterB) —
// EdgeIfaceDispatch, not EdgeDirect.
func TestMentionConfidence_InterfaceDispatch(t *testing.T) {
	nodes, typed := mentionFixtureIngest(t)
	implKeys := []string{
		findNode(t, nodes, "AdapterA.Complete").Key(),
		findNode(t, nodes, "AdapterB.Complete").Key(),
	}

	conf := callgraph.MentionConfidence(nodes, typed, implKeys)
	runAdapter := findNode(t, nodes, "RunAdapter")
	if got := conf(runAdapter.FilePath, runAdapter.Line+1); got != core.EdgeConfidenceResolved {
		t.Errorf("RunAdapter's a.Complete() call = %v, want resolved (interface dispatch to a known implementer)", got)
	}
}
