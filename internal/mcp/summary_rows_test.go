package mcp_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// summaryFleet serves summary rows and counts every other read.
type summaryFleet struct {
	fakeFleet
	rows          map[string]store.ServiceSummary
	symbolLoads   atomic.Int32
	unsupported   atomic.Int32
	summaryReads  atomic.Int32
	absentReasons map[string]string
}

func (f *summaryFleet) SymbolsFor(service string) []core.SymbolNode {
	f.symbolLoads.Add(1)
	return f.fakeFleet.SymbolsFor(service)
}

func (f *summaryFleet) UnsupportedLanguagesFor(service string) map[string]int {
	f.unsupported.Add(1)
	return nil
}

func (f *summaryFleet) StoredSummary(_ context.Context, service string) (store.ServiceSummary, error) {
	f.summaryReads.Add(1)
	if r, ok := f.absentReasons[service]; ok {
		return store.ServiceSummary{}, errors.New(r)
	}
	return f.rows[service], nil
}

// TestServiceSummaries_ReadsSummaryRowsOnly is RT-79 (§4.4, §6):
// list_services answers each service from its summary row — count,
// languages, unsupported languages — and never loads a symbol table or
// walks a tree; a service whose row is not yet derived is a named absence
// (the count is unknown, never zero, never a fallback).
func TestServiceSummaries_ReadsSummaryRowsOnly(t *testing.T) {
	f := &summaryFleet{
		fakeFleet: fakeFleet{svcs: map[string][]core.SymbolNode{"api": {{Name: "X"}}, "web": {{Name: "Y"}}}},
		rows: map[string]store.ServiceSummary{
			"api": {ServiceID: "api", SymbolCount: 1234, SourceKinds: []core.SourceKind{core.KindGoFunc, core.KindPyFunc}, UnsupportedLangs: map[string]int{"swift": 3}},
		},
		absentReasons: map[string]string{"web": "summary not yet derived for web: an ingest or backfill holds the ingest slot"},
	}
	sums := mcp.ServiceSummaries(context.Background(), nil, f)
	if len(sums) != 2 {
		t.Fatalf("sums = %+v", sums)
	}
	api, web := sums[0], sums[1]
	if api.SymbolCount != 1234 || len(api.Languages) != 2 || api.Unsupported["swift"] != 3 || api.CountErr != "" {
		t.Fatalf("api = %+v", api)
	}
	if web.CountErr == "" || web.SymbolCount != 0 {
		t.Fatalf("web absence not named: %+v", web)
	}
	if n := f.symbolLoads.Load(); n != 0 {
		t.Fatalf("list_services loaded %d symbol table(s)", n)
	}

	srv := mcp.NewServerWithFleet(nil, f)
	out := callTool(t, srv, "list_services", map[string]any{})
	rows := out["services"].([]any)
	first := rows[0].(map[string]any)
	if first["symbol_count"].(float64) != 1234 {
		t.Fatalf("list_services api = %v", first)
	}
	un := first["unsupported_languages"].([]any)
	if len(un) != 1 || un[0].(map[string]any)["language"] != "swift" {
		t.Fatalf("unsupported = %v", un)
	}
	second := rows[1].(map[string]any)
	if _, has := second["symbol_count"]; has || second["symbol_count_error"] == nil {
		t.Fatalf("list_services web = %v (want a named absence)", second)
	}
	if n := f.unsupported.Load(); n != 0 {
		t.Fatalf("list_services walked for unsupported languages %d time(s)", n)
	}
	if n := f.symbolLoads.Load(); n != 0 {
		t.Fatalf("list_services loaded %d symbol table(s)", n)
	}
}

// TestListServices_ClassificationCounts is LB-F9/LB-F10's list_services
// disclosure (language-breadth §5): the summary row's sniffed, ambiguous
// and skipped_generated counts are named under "classification"; the
// other exclusion reasons (oversize) are not, and a service with none
// carries no key.
func TestListServices_ClassificationCounts(t *testing.T) {
	f := &summaryFleet{
		fakeFleet: fakeFleet{svcs: map[string][]core.SymbolNode{"api": {{Name: "X"}}, "web": {{Name: "Y"}}}},
		rows: map[string]store.ServiceSummary{
			"api": {ServiceID: "api", SymbolCount: 1, ExcludedCounts: map[string]int{
				"classified_by_sniff": 4, "ambiguous_unclassified": 1, "skipped_generated": 7, "oversize": 2}},
			"web": {ServiceID: "web", SymbolCount: 1, ExcludedCounts: map[string]int{"oversize": 2}},
		},
	}
	srv := mcp.NewServerWithFleet(nil, f)
	out := callTool(t, srv, "list_services", map[string]any{})
	rows := out["services"].([]any)
	cls, ok := rows[0].(map[string]any)["classification"].(map[string]any)
	if !ok {
		t.Fatalf("api row has no classification: %v", rows[0])
	}
	want := map[string]float64{"classified_by_sniff": 4, "ambiguous_unclassified": 1, "skipped_generated": 7}
	if len(cls) != len(want) {
		t.Fatalf("classification = %v, want %v", cls, want)
	}
	for k, v := range want {
		if cls[k] != v {
			t.Fatalf("classification[%s] = %v, want %v", k, cls[k], v)
		}
	}
	if _, has := rows[1].(map[string]any)["classification"]; has {
		t.Fatalf("web row carries classification with nothing to disclose: %v", rows[1])
	}
}

// TestServiceSummaries_TypeScriptFromFileClasses pins O1086: a service
// whose only .ts file holds functions (the shared JS* kinds) lists
// typescript and not javascript; a mixed .js+.ts service lists both.
func TestServiceSummaries_TypeScriptFromFileClasses(t *testing.T) {
	f := &summaryFleet{
		fakeFleet: fakeFleet{svcs: map[string][]core.SymbolNode{"tsonly": nil, "mixed": nil, "legacy": nil}},
		rows: map[string]store.ServiceSummary{
			"tsonly": {ServiceID: "tsonly", SymbolCount: 3, SourceKinds: []core.SourceKind{core.KindJSFunc}, FileLangs: []string{"ts"}},
			"mixed":  {ServiceID: "mixed", SymbolCount: 3, SourceKinds: []core.SourceKind{core.KindJSFunc}, FileLangs: []string{"js", "ts"}},
			"legacy": {ServiceID: "legacy", SymbolCount: 3, SourceKinds: []core.SourceKind{core.KindJSFunc}},
		},
	}
	got := map[string][]string{}
	for _, s := range mcp.ServiceSummaries(context.Background(), nil, f) {
		got[s.Service] = s.Languages
	}
	want := map[string][]string{
		"tsonly": {"typescript"},
		"mixed":  {"javascript", "typescript"},
		"legacy": {"javascript"},
	}
	for svc, w := range want {
		if fmt.Sprint(got[svc]) != fmt.Sprint(w) {
			t.Errorf("%s languages = %v, want %v", svc, got[svc], w)
		}
	}
}
