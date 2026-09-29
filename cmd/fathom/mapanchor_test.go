package main

// mapanchor_test.go — XM-31 (O147): a map anchor resolves the way find_symbol
// does, through the store or the daemon, and an index that could not be
// reached is named as that, never blamed on the anchor.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/explmap"
)

// anchorTable is an index over a node table: the ctxIndex the plans verbs use,
// plus the candidate list a map anchor resolves against.
type anchorTable struct{ nodes []core.SymbolNode }

func (anchorTable) resolveSymbol(string, string) (symbolnav.Context, bool) {
	return symbolnav.Context{}, false
}

func (a anchorTable) anchorNodes(string) ([]core.SymbolNode, error) { return a.nodes, nil }

func tableNode(pkg, name, file string, line, end int) core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: pkg, Name: name, FilePath: file, Line: line, EndLine: end, Kind: core.KindGoFunc}
}

// mapIndexHarness replaces the index seams for one test: the service lookup,
// the index opener and the clock the bounded wait reads. attempts counts opens.
type mapIndexHarness struct {
	attempts, lookups int
	slept             time.Duration
	open              func(attempt int, service, store, url string) (ctxIndex, error)
	service           string
}

func withMapIndex(t *testing.T, h *mapIndexHarness) {
	t.Helper()
	pOpen, pSvc, pSleep, pNow, pWait, pPoll := mapOpenIndex, mapServiceOf, mapIndexSleep, mapIndexNow, mapIndexWait, mapIndexPoll
	base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	mapOpenIndex = func(service, store, url string) (ctxIndex, error) {
		h.attempts++
		return h.open(h.attempts, service, store, url)
	}
	mapServiceOf = func(string) string { h.lookups++; return h.service }
	mapIndexSleep = func(d time.Duration) { h.slept += d }
	mapIndexNow = func() time.Time { return base.Add(h.slept) }
	mapIndexWait, mapIndexPoll = 3*time.Second, time.Second
	t.Cleanup(func() {
		mapOpenIndex, mapServiceOf, mapIndexSleep, mapIndexNow, mapIndexWait, mapIndexPoll = pOpen, pSvc, pSleep, pNow, pWait, pPoll
	})
}

func fixedIndex(nodes ...core.SymbolNode) func(int, string, string, string) (ctxIndex, error) {
	return func(int, string, string, string) (ctxIndex, error) { return anchorTable{nodes: nodes}, nil }
}

// TestMapAppendResolvesSymbolAnchorsLikeFindSymbol pins XM-31 through the
// product verbs: the spellings find_symbol accepts each file the same entry on
// the canonical key, an ambiguous name lists its candidates and files nothing
// (not even the map), and a name no tier matches says so.
func TestMapAppendResolvesSymbolAnchorsLikeFindSymbol(t *testing.T) {
	repo := mapRepo(t)
	withMapIndex(t, &mapIndexHarness{service: "svc", open: fixedIndex(
		tableNode("a", "Handle", "a.go", 3, 5),
		tableNode("a", "Other", "a.go", 7, 7),
		tableNode("a", "Twin", "a.go", 1, 1),
		tableNode("b", "Twin", "sub/b.go", 1, 1),
	)})

	for i, spelling := range []string{"Handle", "a.Handle", "handle", "svc::a.Handle", "(*a).Handle"} {
		id := fmt.Sprintf("m%d", i)
		code, out, errs := runCLI(t, mapCLI, "append", id, "--anchor", spelling, "--why", "the handler", "--repo", repo, "--service", "svc")
		if code != 0 {
			t.Fatalf("append %q: exit %d: %s", spelling, code, errs)
		}
		if !strings.Contains(out, "a.go#L3-L5") {
			t.Errorf("append %q = %q, want the span a.go#L3-L5", spelling, out)
		}
	}
	for _, e := range mapRows(t, mapEntryKind) {
		if e.Pointers["symbol"] != "a.Handle" || e.Pointers["span"] != "a.go#L3-L5" {
			t.Errorf("entry on %s: symbol %q span %q; want the canonical a.Handle at a.go#L3-L5 whatever the spelling", e.Pointers["map"], e.Pointers["symbol"], e.Pointers["span"])
		}
	}
	// The typed spelling is kept as the anchor the filer named.
	for _, e := range mapRows(t, mapEntryKind) {
		if e.Pointers["map"] == "m2" && e.Pointers["anchor"] != "handle" {
			t.Errorf("m2's anchor = %q, want the spelling the filer typed", e.Pointers["anchor"])
		}
	}

	before := len(loadThreadRowsRaw(threadsPath()))
	code, _, errs := runCLI(t, mapCLI, "append", "amb", "--anchor", "Twin", "--why", "w", "--repo", repo, "--service", "svc")
	if code == 0 {
		t.Fatalf("an ambiguous anchor filed an entry")
	}
	for _, want := range []string{"a.Twin (a.go:1)", "sub.Twin (sub/b.go:1)", "nothing was filed", "2 symbols"} {
		if !strings.Contains(errs, want) {
			t.Errorf("the ambiguity error lacks %q: %s", want, errs)
		}
	}
	if strings.Contains(errs, "does not resolve") {
		t.Errorf("an ambiguous name is not 'not found': %s", errs)
	}
	if after := len(loadThreadRowsRaw(threadsPath())); after != before {
		t.Errorf("an ambiguous anchor wrote %d row(s), want none (not even the map's header)", after-before)
	}

	code, _, errs = runCLI(t, mapCLI, "append", "miss", "--anchor", "NoSuchThing", "--why", "w", "--repo", repo, "--service", "svc")
	if code == 0 || !strings.Contains(errs, "no symbol matches") || !strings.Contains(errs, "find_symbol") {
		t.Errorf("a miss: exit %d, %q; want 'no symbol matches' naming the tiers", code, errs)
	}

	// The read side resolves the same way: a stored key finds its span, an
	// ambiguous one finds nothing.
	r := ctxSymbolResolver{idx: anchorTable{nodes: []core.SymbolNode{
		tableNode("a", "Handle", "a.go", 3, 5), tableNode("a", "Twin", "a.go", 1, 1), tableNode("b", "Twin", "sub/b.go", 1, 1)}}}
	if p, s, e, ok := r.symbolSpan("a.Handle"); !ok || p != "a.go" || s != 3 || e != 5 {
		t.Errorf("symbolSpan(a.Handle) = %s %d %d %v, want a.go 3 5", p, s, e, ok)
	}
	if _, _, _, ok := r.symbolSpan("Twin"); ok {
		t.Errorf("symbolSpan(Twin) resolved an ambiguous name")
	}
}

// TestMapAppendNamesTheIndexItCouldNotReach pins XM-31's other half: the
// service and the index are what an append asks for, and when it cannot get
// them it says which one, how long it waited and what to do next, instead of
// saying the anchor does not resolve. A daemon that has just been deployed
// answers late, so an append waits a bounded time for it; a store never warms.
func TestMapAppendNamesTheIndexItCouldNotReach(t *testing.T) {
	repo := mapRepo(t)
	node := tableNode("a", "Handle", "a.go", 3, 5)

	t.Run("no service is named", func(t *testing.T) {
		h := &mapIndexHarness{service: "", open: fixedIndex(node)}
		withMapIndex(t, h)
		code, _, errs := runCLI(t, mapCLI, "append", "ns", "--anchor", "Handle", "--why", "w", "--repo", repo, "--store", "x.sqlite")
		if code == 0 || !strings.Contains(errs, "--service") || strings.Contains(errs, "does not resolve") {
			t.Errorf("exit %d, %q; want the missing service named with --service, not a blamed anchor", code, errs)
		}
		if h.attempts != 0 {
			t.Errorf("opened the index %d times with no service to open it for", h.attempts)
		}
	})

	t.Run("an explicit service skips the daemon lookup", func(t *testing.T) {
		h := &mapIndexHarness{service: "wrong", open: fixedIndex(node)}
		withMapIndex(t, h)
		if code, _, errs := runCLI(t, mapCLI, "append", "ex", "--anchor", "Handle", "--why", "w", "--repo", repo, "--service", "svc"); code != 0 {
			t.Fatalf("exit %d: %s", code, errs)
		}
		if h.lookups != 0 {
			t.Errorf("asked the daemon for the service %d times although --service named it", h.lookups)
		}
	})

	t.Run("the daemon names the service when none is given", func(t *testing.T) {
		h := &mapIndexHarness{service: "svc", open: func(_ int, service, _, _ string) (ctxIndex, error) {
			if service != "svc" {
				return nil, fmt.Errorf("asked for service %q", service)
			}
			return anchorTable{nodes: []core.SymbolNode{node}}, nil
		}}
		withMapIndex(t, h)
		if code, _, errs := runCLI(t, mapCLI, "append", "dn", "--anchor", "Handle", "--why", "w", "--repo", repo); code != 0 {
			t.Fatalf("exit %d: %s", code, errs)
		}
	})

	t.Run("a warming daemon is waited for", func(t *testing.T) {
		h := &mapIndexHarness{service: "svc", open: func(n int, _, _, _ string) (ctxIndex, error) {
			if n < 3 {
				return nil, errors.New("probing daemon: connection refused")
			}
			return anchorTable{nodes: []core.SymbolNode{node}}, nil
		}}
		withMapIndex(t, h)
		code, _, errs := runCLI(t, mapCLI, "append", "warm", "--anchor", "Handle", "--why", "w", "--repo", repo, "--service", "svc")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errs)
		}
		if h.attempts != 3 || h.slept != 2*time.Second {
			t.Errorf("attempts %d, waited %v; want 3 opens after two polls of 1s", h.attempts, h.slept)
		}
	})

	t.Run("a daemon that never answers is named, with the retry", func(t *testing.T) {
		h := &mapIndexHarness{service: "svc", open: func(int, string, string, string) (ctxIndex, error) {
			return nil, errors.New("probing daemon: connection refused")
		}}
		withMapIndex(t, h)
		before := len(loadThreadRowsRaw(threadsPath()))
		code, _, errs := runCLI(t, mapCLI, "append", "down", "--anchor", "Handle", "--why", "w", "--repo", repo, "--service", "svc")
		if code == 0 {
			t.Fatalf("filed an entry with no index")
		}
		for _, want := range []string{"index is not reachable", "connection refused", "3s", "retry", "path#Lx-Ly", "--store"} {
			if !strings.Contains(errs, want) {
				t.Errorf("the unreachable error lacks %q: %s", want, errs)
			}
		}
		if strings.Contains(errs, "does not resolve") {
			t.Errorf("an unreachable index is not a blamed anchor: %s", errs)
		}
		if h.slept != 3*time.Second {
			t.Errorf("waited %v, want the 3s budget", h.slept)
		}
		if after := len(loadThreadRowsRaw(threadsPath())); after != before {
			t.Errorf("an append with no index wrote %d row(s)", after-before)
		}
	})

	t.Run("a store is not waited for", func(t *testing.T) {
		h := &mapIndexHarness{service: "", open: func(int, string, string, string) (ctxIndex, error) {
			return nil, errors.New("opening store: no such file")
		}}
		withMapIndex(t, h)
		code, _, errs := runCLI(t, mapCLI, "append", "st", "--anchor", "Handle", "--why", "w", "--repo", repo, "--service", "svc", "--store", "gone.sqlite")
		if code == 0 || !strings.Contains(errs, "no such file") || h.attempts != 1 || h.slept != 0 {
			t.Errorf("exit %d, attempts %d, waited %v, %q; want one attempt, no wait, the cause named", code, h.attempts, h.slept, errs)
		}
	})

	t.Run("a span anchor needs no index", func(t *testing.T) {
		h := &mapIndexHarness{service: "", open: func(int, string, string, string) (ctxIndex, error) {
			return nil, errors.New("must not be opened")
		}}
		withMapIndex(t, h)
		if code, _, errs := runCLI(t, mapCLI, "append", "span", "--anchor", "a.go#L3-L5", "--why", "w", "--repo", repo); code != 0 {
			t.Fatalf("exit %d: %s", code, errs)
		}
		if h.attempts != 0 || h.lookups != 0 {
			t.Errorf("a path#Lx-Ly anchor opened the index %d times and asked for the service %d times", h.attempts, h.lookups)
		}
	})
}

// anchorDaemon is a daemon stand-in that lists one service and answers
// find_symbol from a table keyed by the query, recording the queries asked.
func anchorDaemon(t *testing.T, answers map[string][]map[string]any, asked *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			inner = `{"services":[{"service":"fathom","symbol_count":10}]}`
		case "find_symbol":
			q, _ := req.Params.Arguments["name"].(string)
			if asked != nil {
				*asked = append(*asked, q)
			}
			if svc, _ := req.Params.Arguments["service"].(string); svc != "fathom" {
				http.Error(w, "wrong service", 500)
				return
			}
			b, _ := json.Marshal(map[string]any{"symbols": answers[q], "matched_via": "exact"})
			inner = string(b)
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestHTTPAnchorIndexAsksFindSymbol pins that the daemon path resolves anchors
// with find_symbol's own candidates: a qualified spelling the daemon's exact
// tier does not know is tried again by its bare name, and the tiers then decide
// over what came back.
func TestHTTPAnchorIndexAsksFindSymbol(t *testing.T) {
	fusion := map[string]any{"name": "ResolveImportSpecs", "file_path": "internal/fusion/importtargets.go", "line": 55, "end_line": 97, "service": "fathom", "kind": "go_func"}
	other := map[string]any{"name": "ResolveImportSpecs", "file_path": "internal/other/x.go", "line": 3, "end_line": 9, "service": "fathom", "kind": "go_func"}
	var asked []string
	srv := anchorDaemon(t, map[string][]map[string]any{
		"ResolveImportSpecs": {fusion, other},
	}, &asked)
	idx, err := openCtxIndex("fathom", "", srv.URL)
	if err != nil {
		t.Fatalf("openCtxIndex: %v", err)
	}
	r := ctxSymbolResolver{idx: idx}

	m, err := r.resolveAnchor("internal/fusion.ResolveImportSpecs")
	if err != nil || m.Key != "internal/fusion.ResolveImportSpecs" || m.Node.Line != 55 || m.Node.EndLine != 97 {
		t.Fatalf("resolveAnchor(internal/fusion.ResolveImportSpecs) = %+v, %v", m, err)
	}
	if strings.Join(asked, ",") != "internal/fusion.ResolveImportSpecs,ResolveImportSpecs" {
		t.Errorf("find_symbol was asked %v; want the qualified query, then its bare name", asked)
	}

	var amb *explmap.AmbiguousAnchorError
	if _, err := r.resolveAnchor("ResolveImportSpecs"); !errors.As(err, &amb) || amb.Total != 2 {
		t.Errorf("the bare name of two symbols: %v, want an ambiguity error listing both", err)
	}
	if _, err := r.resolveAnchor("NoSuch"); !errors.Is(err, explmap.ErrAnchorNotFound) {
		t.Errorf("a name the daemon does not know: %v, want ErrAnchorNotFound", err)
	}

	// A daemon that fails the lookup is an unreachable index, not a miss.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "warming", 503) }))
	t.Cleanup(bad.Close)
	down := ctxSymbolResolver{idx: &httpCtxIndex{hc: bad.Client(), baseURL: bad.URL, service: "fathom"}}
	_, err = down.resolveAnchor("ResolveImportSpecs")
	if err == nil || errors.Is(err, explmap.ErrAnchorNotFound) || !strings.Contains(err.Error(), "find_symbol") {
		t.Errorf("a 503 from the daemon = %v, want an index error naming find_symbol, never ErrAnchorNotFound", err)
	}
}

// TestMapShowAndComposeTakeTheServiceToo pins that the read side names its
// service the way an append does (O147): a worker's `map show --store S
// --service X` re-resolves a drifted symbol entry through that service without
// asking the daemon which one it is, and with no service and none from the
// daemon the read says the index was out of reach instead of guessing.
func TestMapShowAndComposeTakeTheServiceToo(t *testing.T) {
	repo := mapRepo(t)
	h := &mapIndexHarness{service: "", open: func(_ int, service, _, _ string) (ctxIndex, error) {
		if service != "svc" {
			return nil, fmt.Errorf("asked for service %q", service)
		}
		return anchorTable{nodes: []core.SymbolNode{tableNode("a", "Handle", "a.go", 4, 6)}}, nil
	}}
	withMapIndex(t, h)
	// Filed while Handle sat at lines 3-5 (a fixture index for the append).
	if _, err := appendMapEntry(mapEntryRequest{Map: "sym", Anchor: "a.Handle", Why: "the handler", Root: repo, Now: time.Now()},
		fakeSyms{"a.Handle": {"a.go", 3, 5}}); err != nil {
		t.Fatal(err)
	}
	// A line is added above it: same text, new span, so the read needs the index.
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("// moved\n"+mapFixtureGo), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errs := runCLI(t, mapCLI, "show", "sym", "--repo", repo, "--store", "x.sqlite", "--service", "svc")
	if code != 0 || !strings.Contains(out, "moved from a.go#L3-L5") || strings.Contains(out, "unreachable") {
		t.Errorf("show with --service: exit %d, %q %q; want the entry found again at its new span through service svc", code, out, errs)
	}
	if h.lookups != 0 {
		t.Errorf("show asked the daemon for the service %d times although --service named it", h.lookups)
	}

	code, out, errs = runCLI(t, mapCLI, "show", "sym", "--repo", repo, "--store", "x.sqlite")
	if code != 0 || !strings.Contains(out, "unreachable") {
		t.Errorf("show with no service and none from the daemon: exit %d, %q %q; want the index named as out of reach", code, out, errs)
	}

	code, out, errs = runCLI(t, mapCLI, "compose", "sym", "--repo", repo, "--store", "x.sqlite", "--service", "svc")
	if code != 0 || !strings.Contains(out, "1 re-resolved") {
		t.Errorf("compose with --service: exit %d, %q %q; want the entry re-resolved", code, out, errs)
	}
}
