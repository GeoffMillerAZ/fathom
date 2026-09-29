package main

// mapindex.go — the index a map anchor resolves through (XM-31, O147): which
// service to ask, a bounded wait for a daemon that is still warming, the
// candidate lists behind explmap.ResolveAnchorSymbol (the store's symbols, the
// daemon's find_symbol), and an error that says which thing could not be
// reached instead of blaming the anchor.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/explmap"
)

// The seams an append reaches the index through, swapped by tests: the service
// the daemon names for a repository, the index opener, and the clock the bounded
// wait reads.
var (
	mapServiceOf  = func(root string) string { return threadScopeFor(root).service }
	mapOpenIndex  = openCtxIndex
	mapIndexSleep = time.Sleep
	mapIndexNow   = time.Now
	// mapIndexWait bounds how long an anchor append waits for the daemon: a
	// daemon that has just been deployed answers late while it warms (O147).
	// A store never warms, so a store is tried once.
	mapIndexWait = 8 * time.Second
	mapIndexPoll = 500 * time.Millisecond
)

// anchorResolver resolves a symbol anchor the way find_symbol does.
type anchorResolver interface {
	resolveAnchor(query string) (explmap.AnchorMatch, error)
}

// anchorIndex is an index that lists the candidates for an anchor query: the
// store lists every symbol of its service, the daemon answers find_symbol.
type anchorIndex interface {
	anchorNodes(query string) ([]core.SymbolNode, error)
}

// errMapNoService is an append that could not learn which service to resolve in.
var errMapNoService = errors.New("no service to resolve the anchor in")

// mapIndexError is an anchor append that could not reach what resolves it.
type mapIndexError struct {
	Cause  error
	Waited time.Duration
	Store  bool
}

func (e *mapIndexError) Unwrap() error { return e.Cause }

func (e *mapIndexError) Error() string {
	switch {
	case errors.Is(e.Cause, errMapNoService):
		return "no service to resolve the anchor in: the daemon did not name this repository's service (it is down, still warming after a deploy, or does not index this checkout); " +
			"pass --service S (the id the repository is indexed under), retry in a minute, or anchor by path#Lx-Ly, which needs no index"
	case e.Store:
		return fmt.Sprintf("the store could not be used (%v): pass --store a scratch ingest of this service with --service S, or anchor by path#Lx-Ly, which needs no index", e.Cause)
	}
	waited := ""
	if e.Waited > 0 {
		waited = " after waiting " + e.Waited.Round(time.Second).String()
	}
	return fmt.Sprintf("the index is not reachable (%v)%s: a daemon that was just deployed answers late while it warms, so retry in a minute; "+
		"or pass --store a scratch ingest with --service S, or anchor by path#Lx-Ly, which needs no index", e.Cause, waited)
}

// openMapAnchorIndex opens what a symbol anchor resolves through. The service is
// the caller's (--service) or the one the daemon names for root; a daemon that
// does not answer yet is waited for, up to mapIndexWait, and then named as
// unreachable with the retry. A store is tried once.
func openMapAnchorIndex(root, service, storePath, url string) (mapSymbolResolver, error) {
	wait := mapIndexWait
	if storePath != "" {
		wait = 0
	}
	start := mapIndexNow()
	var last error
	for {
		svc := service
		if svc == "" {
			svc = mapServiceOf(root)
		}
		if svc == "" {
			last = errMapNoService
		} else {
			idx, err := mapOpenIndex(svc, storePath, url)
			if err == nil {
				return ctxSymbolResolver{idx: idx}, nil
			}
			last = err
		}
		if mapIndexNow().Sub(start)+mapIndexPoll > wait {
			break
		}
		mapIndexSleep(mapIndexPoll)
	}
	return nil, &mapIndexError{Cause: last, Waited: mapIndexNow().Sub(start), Store: storePath != ""}
}

// downIndex stands in for an index that could not be opened: an anchor
// resolved through it says why, never that the anchor is unknown.
type downIndex struct{ err error }

func (d downIndex) symbolSpan(string) (string, int, int, bool) { return "", 0, 0, false }

func (d downIndex) resolveAnchor(string) (explmap.AnchorMatch, error) {
	return explmap.AnchorMatch{}, d.err
}

// resolveAnchor resolves query over the index's candidates.
func (r ctxSymbolResolver) resolveAnchor(query string) (explmap.AnchorMatch, error) {
	ai, ok := r.idx.(anchorIndex)
	if !ok {
		return explmap.AnchorMatch{}, errors.New("this index cannot list candidates for an anchor")
	}
	nodes, err := ai.anchorNodes(query)
	if err != nil {
		return explmap.AnchorMatch{}, err
	}
	return explmap.ResolveAnchorSymbol(nodes, query)
}

// anchorNodes is every symbol of the store's service; the tiers filter them.
func (idx *storeCtxIndex) anchorNodes(string) ([]core.SymbolNode, error) { return idx.nodes, nil }

// anchorNodes asks the daemon's find_symbol for query and, when it is
// qualified, for its bare name too: the daemon's exact tier knows a name, and a
// qualified spelling reaches its symbol through the name's candidates. The tiers
// then decide over what came back.
func (idx *httpCtxIndex) anchorNodes(query string) ([]core.SymbolNode, error) {
	q := explmap.NormalizeSymbol(query)
	queries := []string{q}
	if i := strings.LastIndex(q, "."); i > 0 && i < len(q)-1 {
		queries = append(queries, q[i+1:])
	}
	seen := map[string]bool{}
	var out []core.SymbolNode
	for _, name := range queries {
		nodes, err := idx.findSymbol(name)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			k := fmt.Sprintf("%s:%d:%s", n.FilePath, n.Line, n.Name)
			if !seen[k] {
				seen[k] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// findSymbol is one find_symbol call to the daemon.
func (idx *httpCtxIndex) findSymbol(name string) ([]core.SymbolNode, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": idx.service, "name": name}},
	})
	if err != nil {
		return nil, err
	}
	req, err := originHeader(idx.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("find_symbol: %w", err)
	}
	resp, err := idx.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("find_symbol on %s: %w", idx.baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("find_symbol: reading the answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("find_symbol on %s: %s", idx.baseURL, resp.Status)
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return nil, fmt.Errorf("find_symbol: %w", err)
	}
	var payload struct {
		Symbols []struct {
			Service  string `json:"service"`
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			FilePath string `json:"file_path"`
			Line     int    `json:"line"`
			EndLine  int    `json:"end_line"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, fmt.Errorf("find_symbol: reading the answer: %w", err)
	}
	nodes := make([]core.SymbolNode, 0, len(payload.Symbols))
	for _, s := range payload.Symbols {
		nodes = append(nodes, core.SymbolNode{
			Service: s.Service, Name: s.Name, Kind: core.SourceKind(s.Kind),
			FilePath: s.FilePath, Line: s.Line, EndLine: s.EndLine,
		})
	}
	return nodes, nil
}
