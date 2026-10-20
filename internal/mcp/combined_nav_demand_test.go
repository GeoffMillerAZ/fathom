package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// These public-tool regressions use only APIs present before the repair, so
// their tests-only overlay can establish unnecessary builds on the old source.
type graphDemandFake struct {
	patchFake
	typed         []core.Edge
	typedCalls    int
	typedFailures int
	mark          string
	listErr       error
}

func (f *graphDemandFake) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	f.loads++
	return f.nodes, f.listErr
}

func (f *graphDemandFake) StoreWatermark(context.Context, string) (string, error) {
	return f.mark, nil
}

func (f *graphDemandFake) TypedEdges(context.Context, string) ([]core.Edge, error) {
	f.typedCalls++
	if f.typedFailures > 0 {
		f.typedFailures--
		return nil, errTypedBusy
	}
	return f.typed, nil
}

func newGraphDemandFake() *graphDemandFake {
	n := func(pkg, name, file string, kind core.SourceKind, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, FilePath: file, Line: 1, EndLine: 2, Kind: kind, Calls: calls}
	}
	f := &graphDemandFake{mark: "demand-1"}
	f.nodes = []core.SymbolNode{
		n("Store", "Store.Target", "store.go", core.KindGoMethod),
		n("p", "Config", "store.go", core.KindGoType),
		n("app", "Real", "app.go", core.KindGoFunc, "item.Target", "p.Config"),
		n("app", "Ghost", "app.go", core.KindGoFunc, "remote.Target"),
		n("app", "TestReal", "app_test.go", core.KindGoFunc, "Real"),
	}
	f.typed = []core.Edge{
		{FromService: "svc", FromFile: "app.go", FromSymbol: f.nodes[2].Key(), ToSymbol: f.nodes[0].Key(), Kind: core.EdgeDirect},
		{FromService: "svc", FromFile: "app_test.go", FromSymbol: f.nodes[4].Key(), ToSymbol: f.nodes[2].Key(), Kind: core.EdgeDirect},
	}
	return f
}

func graphDemandFlights() int64 {
	var n int64
	for _, stat := range memo.Stats() {
		if stat.Name == "graphstore" {
			n += stat.Flights
		}
	}
	return n
}

func graphDemandArgs(tool string) map[string]any {
	if tool == "review_diff" {
		return map[string]any{"service": "svc", "changed_files": []string{"store.go"}}
	}
	return map[string]any{"service": "svc", "symbol": "Store.Target", "snippet": false}
}

func TestCombinedNav_PublicToolsBuildOnlyConsumedGraph(t *testing.T) {
	for _, tool := range []string{"symbol_context", "review_diff"} {
		t.Run(tool, func(t *testing.T) {
			f := newGraphDemandFake()
			s := NewServer(f)
			t.Cleanup(s.DropCaches)
			before := graphDemandFlights()
			first := callToolArgs(t, s, tool, graphDemandArgs(tool))
			if built := graphDemandFlights() - before; built != 1 {
				t.Fatalf("%s built %d graph kinds for one combined-graph answer; want 1", tool, built)
			}
			second := callToolArgs(t, s, tool, graphDemandArgs(tool))
			expectedTyped := 1
			if tool == "review_diff" {
				// Review input reads typed evidence once per request as well as
				// the navigation graph's single memoized typed-edge load.
				expectedTyped = 3
			}
			if built := graphDemandFlights() - before; built != 1 || f.loads != 1 || f.typedCalls != expectedTyped {
				t.Fatalf("warm answer rebuilt: graphs=%d loads=%d typed=%d", built, f.loads, f.typedCalls)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("cold/warm public answer differs:\ncold=%v\nwarm=%v", first, second)
			}
		})
	}
}

func TestCombinedNav_PublicAnswersMatchFullGraphControl(t *testing.T) {
	for _, tool := range []string{"symbol_context", "review_diff"} {
		t.Run(tool, func(t *testing.T) {
			control := newGraphDemandFake()
			legacy, err := loadNavGraphsErr(context.Background(), control, "svc")
			if err != nil || legacy == nil || !legacy.TypedAvailable {
				t.Fatalf("full-graph control: %+v %v", legacy, err)
			}
			controlServer := NewServer(control)
			t.Cleanup(controlServer.DropCaches)
			want := callToolArgs(t, controlServer, tool, graphDemandArgs(tool))
			f := newGraphDemandFake()
			s := NewServer(f)
			t.Cleanup(s.DropCaches)
			got := callToolArgs(t, s, tool, graphDemandArgs(tool))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("public %s answer changed from full-graph control:\ngot=%v\nwant=%v", tool, got, want)
			}
			if tool == "symbol_context" {
				if callers := got["callers"]; !reflect.DeepEqual(callers, []any{"svc::app.Real"}) {
					t.Fatalf("typed-file name guess survived: callers=%v", callers)
				}
			}
		})
	}
}

func TestCombinedNav_TypedFailureRetriesWithoutCachingAbsence(t *testing.T) {
	f := newGraphDemandFake()
	f.typedFailures = 1
	s := NewServer(f)
	t.Cleanup(s.DropCaches)
	first := callToolArgs(t, s, "symbol_context", graphDemandArgs("symbol_context"))
	if got := first["callers"]; !reflect.DeepEqual(got, []any{"svc::app.Ghost", "svc::app.Real"}) {
		t.Fatalf("failed typed read must retain the syntax answer for this call: %v", got)
	}
	second := callToolArgs(t, s, "symbol_context", graphDemandArgs("symbol_context"))
	if got := second["callers"]; !reflect.DeepEqual(got, []any{"svc::app.Real"}) {
		t.Fatalf("recovered typed read did not remove the name guess: %v", got)
	}
	if f.typedCalls != 2 || f.loads != 1 {
		t.Fatalf("typed failure cached or snapshot reloaded: typed=%d loads=%d", f.typedCalls, f.loads)
	}
}

func TestCombinedNav_WatermarkInvalidatesAndEmptyNeverCaches(t *testing.T) {
	for _, cacheable := range []bool{true, false} {
		t.Run(fmt.Sprint(cacheable), func(t *testing.T) {
			f := newGraphDemandFake()
			if !cacheable {
				f.mark = ""
			}
			s := NewServer(f)
			t.Cleanup(s.DropCaches)
			callToolArgs(t, s, "symbol_context", graphDemandArgs("symbol_context"))
			typedAfterFirst := f.typedCalls
			// Remove the real edge from a new store state. A cached old graph
			// must not turn that absence into a resolved caller at the new mark.
			f.typed = nil
			f.nodes[2].Calls = nil
			f.nodes[3].Calls = nil
			if cacheable {
				f.mark = "demand-2"
			}
			got := callToolArgs(t, s, "symbol_context", graphDemandArgs("symbol_context"))
			if callers, ok := got["callers"].([]any); !ok || len(callers) != 0 {
				t.Fatalf("new state kept stale callers: %v", got["callers"])
			}
			if f.loads != 2 || f.typedCalls <= typedAfterFirst {
				t.Fatalf("state did not reload: loads=%d typed=%d", f.loads, f.typedCalls)
			}
			if cacheable && f.typedCalls != 2 {
				t.Fatalf("watermark-scoped typed state should load once per mark: %d", f.typedCalls)
			}
		})
	}
}

func TestCombinedNav_PublicRefusalAndCancellationAreNotEmptySuccess(t *testing.T) {
	for _, want := range []error{errors.New("over budget: tree producer halted at refuse"), context.Canceled} {
		f := newGraphDemandFake()
		f.listErr = want
		s := NewServer(f)
		raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "symbol_context", "arguments": graphDemandArgs("symbol_context")}})
		if err != nil {
			t.Fatal(err)
		}
		response, ok := s.HandleRequest(context.Background(), raw)
		if !ok || !strings.Contains(string(response), want.Error()) {
			t.Fatalf("named load refusal disappeared: %s", response)
		}
		var envelope map[string]any
		if err := json.Unmarshal(response, &envelope); err != nil || envelope["error"] == nil {
			t.Fatalf("load refusal became a successful empty answer: %s (decode=%v)", response, err)
		}
		s.DropCaches()
	}
}

// This measures public-tool allocations in a bounded synthetic service. Run
// the same tests-only file on base and repaired sources; it is not a peak-
// footprint measurement and never changes GC, tenancy or cache budgets.
func BenchmarkCombinedNav_PublicCold(b *testing.B) {
	for _, tool := range []string{"symbol_context", "review_diff"} {
		b.Run(tool, func(b *testing.B) {
			f := newGraphDemandFake()
			for i := 0; i < 4096; i++ {
				n := core.SymbolNode{Service: "svc", Package: "p", Name: fmt.Sprintf("Extra%04d", i), FilePath: "extra/nodes.go", Kind: core.KindGoFunc, Line: i + 1}
				f.nodes = append(f.nodes, n)
			}
			s := NewServer(f)
			defer s.DropCaches()
			raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": graphDemandArgs(tool)}})
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var last []byte
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s.DropCaches()
				b.StartTimer()
				response, ok := s.HandleRequest(context.Background(), raw)
				if !ok || len(response) == 0 {
					b.Fatal("public tool returned no response")
				}
				last = response
			}
			b.StopTimer()
			var envelope struct {
				Error  json.RawMessage `json:"error"`
				Result *struct {
					IsError bool `json:"isError"`
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal(last, &envelope); err != nil || envelope.Result == nil || (len(envelope.Error) > 0 && string(envelope.Error) != "null") || envelope.Result.IsError || len(envelope.Result.Content) == 0 {
				b.Fatalf("benchmark measured a failed public result: %s (decode=%v)", last, err)
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &payload); err != nil {
				b.Fatalf("benchmark result is not a decoded answer: %v", err)
			}
			if tool == "symbol_context" && payload["def"] == nil {
				b.Fatal("benchmark context omitted its known definition")
			}
			if tool == "review_diff" && payload["dimensions"] == nil {
				b.Fatal("benchmark review omitted its dimensions")
			}
		})
	}
}
