package mcp

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// indexedFleet is a fleet whose store carries RT-77's call index: every
// whole-service load is counted, and each service is call-indexed unless
// named in legacy.
type indexedFleet struct {
	obs       map[string][]core.Observation
	legacy    map[string]bool
	fullLoads atomic.Int32
	keyCalls  atomic.Int32
	nameCalls atomic.Int32
}

func (f *indexedFleet) Services() []string { return []string{"a", "b", "c"} }
func (f *indexedFleet) SymbolsFor(svc string) []core.SymbolNode {
	f.fullLoads.Add(1)
	var out []core.SymbolNode
	for _, o := range f.obs[svc] {
		out = append(out, o.ToSymbolNode())
	}
	return out
}

func (f *indexedFleet) ObservationsFor(svc string) []core.Observation {
	f.fullLoads.Add(1)
	return f.obs[svc]
}
func (f *indexedFleet) UnsupportedLanguagesFor(string) map[string]int { return nil }

func (f *indexedFleet) CallersOfKeys(_ context.Context, keys []string) ([]store.CallerRow, error) {
	f.keyCalls.Add(1)
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	var out []store.CallerRow
	for _, svc := range f.Services() {
		if f.legacy[svc] {
			continue // not call-indexed: CallersOfKeys skips it
		}
		for _, o := range f.obs[svc] {
			for _, c := range o.Calls {
				if want[c] || want[store.CalleeName(c)] {
					out = append(out, store.CallerRow{ServiceID: svc, Symbol: o.Symbol, Package: o.Package, SourceRef: o.SourceRef, Line: o.Line, CalleeKey: c})
				}
			}
		}
	}
	return out, nil
}

func (f *indexedFleet) CallsIndexed(_ context.Context, svc string) bool { return !f.legacy[svc] }

func (f *indexedFleet) SymbolSourcesForNames(_ context.Context, svc string, names []string) ([]store.SymbolSource, error) {
	f.nameCalls.Add(1)
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []store.SymbolSource
	for _, o := range f.obs[svc] {
		if want[o.Symbol] {
			out = append(out, store.SymbolSource{Package: o.Package, Symbol: o.Symbol, SourceRef: o.SourceRef})
		}
	}
	return out, nil
}

func newIndexedFleet() *indexedFleet {
	return &indexedFleet{obs: map[string][]core.Observation{
		"a": {
			{ServiceID: "a", Symbol: "Caller1", Package: "p", SourceRef: "a.go", Calls: []string{"out.FormatCents", "Other"}},
			{ServiceID: "a", Symbol: "NotACaller", Package: "p", SourceRef: "a2.go", Calls: []string{"x.FormatCentsX"}},
		},
		"b": {
			{ServiceID: "b", Symbol: "FormatCents", Package: "out", SourceRef: "b.go", Calls: []string{"strconv.Itoa"}},
			{ServiceID: "b", Symbol: "Caller2", Package: "q", SourceRef: "b2.go", Calls: []string{"b::out.FormatCents"}},
		},
		"c": {
			{ServiceID: "c", Symbol: "Caller3", Package: "r", SourceRef: "c.go", Calls: []string{"FormatCents"}},
		},
	}}
}

// RT-84 (§4.5): the wave-B proof measured symbol_context allocating
// ~3.8 GiB per call through fleetCallers, which loaded every service's
// symbols AND observations to answer "who calls X". Against a call-indexed
// store the answer is one indexed query — no service is loaded whole — and
// equals the legacy scan's answer.
func TestFleetCallers_OneIndexedQuery(t *testing.T) {
	f := newIndexedFleet()
	got := fleetCallers(context.Background(), f, "FormatCents")
	if n := f.fullLoads.Load(); n != 0 {
		t.Fatalf("fleetCallers loaded a whole service %d times against a call-indexed store, want 0", n)
	}
	if n := f.keyCalls.Load(); n != 1 {
		t.Fatalf("fleetCallers ran %d CallersOfKeys queries, want 1", n)
	}
	want := []crossServiceHit{
		{Service: "a", Symbol: "Caller1", FilePath: "a.go"},
		{Service: "b", Symbol: "Caller2", FilePath: "b2.go"},
		{Service: "c", Symbol: "Caller3", FilePath: "c.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indexed fleetCallers = %+v, want %+v", got, want)
	}
	// The legacy scan (a backend with no index) gives the same answer.
	legacy := fleetCallers(context.Background(), &plainFleet{f}, "FormatCents")
	if !reflect.DeepEqual(legacy, want) {
		t.Fatalf("legacy fleetCallers = %+v, want %+v", legacy, want)
	}

	// A service whose call index is not complete keeps the scan — for that
	// service only (§6 DerivedState.CallsIndexed).
	f2 := newIndexedFleet()
	f2.legacy = map[string]bool{"c": true}
	got = fleetCallers(context.Background(), f2, "FormatCents")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed fleetCallers = %+v, want %+v", got, want)
	}
	if n := f2.fullLoads.Load(); n != 2 {
		t.Fatalf("one unindexed service cost %d whole-service loads, want 2 (its symbols and observations)", n)
	}

	// Callees resolve each qualified key once, by name lookup, never by
	// loading another service's symbols.
	f3 := newIndexedFleet()
	def := core.SymbolNode{Name: "Caller1", Calls: []string{"out.FormatCents", "out.FormatCents", "Other", "b::out.FormatCents"}}
	callees := crossServiceCallees(context.Background(), f3, "a", def)
	if n := f3.fullLoads.Load(); n != 0 {
		t.Fatalf("crossServiceCallees loaded a whole service %d times, want 0", n)
	}
	if n := f3.nameCalls.Load(); n != 2 { // one lookup per other service, all keys at once
		t.Fatalf("crossServiceCallees ran %d name lookups, want 2 (one per other service)", n)
	}
	wantCallees := crossServiceHitsToAny([]crossServiceHit{{Service: "b", Symbol: "FormatCents", FilePath: "b.go"}})
	if !reflect.DeepEqual(callees, wantCallees) {
		t.Fatalf("crossServiceCallees = %+v, want %+v", callees, wantCallees)
	}
	if legacyCallees := crossServiceCallees(context.Background(), &plainFleet{f3}, "a", def); !reflect.DeepEqual(legacyCallees, wantCallees) {
		t.Fatalf("legacy crossServiceCallees = %+v, want %+v", legacyCallees, wantCallees)
	}
}

// plainFleet hides the index capability: the legacy scan.
type plainFleet struct{ f *indexedFleet }

func (p *plainFleet) Services() []string                            { return p.f.Services() }
func (p *plainFleet) SymbolsFor(s string) []core.SymbolNode         { return p.f.SymbolsFor(s) }
func (p *plainFleet) ObservationsFor(s string) []core.Observation   { return p.f.ObservationsFor(s) }
func (p *plainFleet) UnsupportedLanguagesFor(string) map[string]int { return nil }
