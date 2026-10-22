package mcp

// cold_graph.go — what symbol_context and search answer while a service's
// symbol snapshot or call graph is not resident (O1446, O1447), and
// symbol_context's answer to a name it cannot resolve (O1451).
//
// Every ingest and every daemon start evicts a service's snapshot and
// graphs (memo.EvictStoreService), so on an actively developed repository
// the next graph-backed call pays a cold build. On a large service that
// build is as long as the call's deadline: the call was abandoned with
// nothing. Here symbol_context answers what the store lookup holds, named
// partial, and search serves its hits without the optional receipts. Each
// leaves the build it started or joined running for the next caller: it
// stays on the flight at least memo.NavKeepAfter, past which Group.leave
// keeps a flight instead of cancelling it.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// notComputedColdGraph is a neighbourhood list symbol_context did not
// compute because the call graph was cold: never an empty list (I38).
const notComputedColdGraph = "not computed (cold graph)"

// coldKeepMargin is added to memo.NavKeepAfter so a caller leaving a flight
// it started is past the keep threshold, not on it.
const coldKeepMargin = 100 * time.Millisecond

// coldGraph is why a symbol_context call answers without its call graph.
type coldGraph struct {
	scope memo.Scope
	// kept is true when the build keeps running for the next caller: this
	// call stayed on its flights past memo.NavKeepAfter. False when no
	// build was started (the deadline left no time, or the memo declined).
	kept bool
	// unmeasured is true when no build of the service has completed since
	// the process started, so its cost is unknown and taken as longer than
	// the deadline (memo.NavBuildMeasured).
	unmeasured bool
}

// navGraphsWithin is loadNavGraphsErr at sc bounded by ctx's deadline
// (O1446). With no deadline, an uncacheable scope (I21) or everything
// resident it is loadNavGraphsAt. When the measured build
// (memo.NavBuildCost) fits in the deadline less a reserve for the answer
// (verify_plan's stage reserve) it waits up to that; when it does not, or
// no build of the service is measured since the process started
// (memo.NavBuildMeasured: ExpectedCost says nothing about a large service,
// whose cold build ran past the deadline), it warms the build stage by
// stage (warmNavStages) and stops at the first stage still building. A
// cold result means the graphs are not ready and the caller answers its
// partial; an error is the load's own.
func navGraphsWithin(ctx context.Context, backend QueryInterface, service string, sc memo.Scope) (*navGraphs, *coldGraph, error) {
	dl, hasDeadline := ctx.Deadline()
	cost, resident := memo.NavBuildCost(sc)
	if !hasDeadline || resident || sc.Watermark == "" {
		ng, err := loadNavGraphsAt(ctx, backend, service, sc)
		return ng, nil, err
	}
	remaining := time.Until(dl)
	wait := remaining - min(max(remaining/verifyStageReserveShare, verifyStageReserveMin), verifyStageReserveMax)
	cold := &coldGraph{scope: sc}
	if wait <= 0 {
		return nil, cold, nil
	}
	if cost <= wait && memo.NavBuildMeasured(sc) {
		wctx, cancel := context.WithTimeout(ctx, wait)
		defer cancel()
		ng, err := loadNavGraphsAt(wctx, backend, service, sc)
		switch {
		case err == nil:
			return ng, nil, nil
		case ctx.Err() != nil:
			return nil, nil, err
		case errors.Is(err, memo.ErrWouldNotFinish):
			return nil, cold, nil
		case wctx.Err() != nil:
			cold.kept = wait >= memo.NavKeepAfter()
			return nil, cold, nil
		}
		return nil, nil, err
	}
	cold.unmeasured = cost <= wait
	done, kept, err := warmNavStages(ctx, backend, service, sc, time.Now().Add(wait))
	if err != nil {
		return nil, nil, err
	}
	if !done {
		cold.kept = kept
		return nil, cold, nil
	}
	ng, err := loadNavGraphsAt(ctx, backend, service, sc)
	return ng, nil, err
}

// warmNavStages builds sc's snapshot, typed edges and three graphs one
// stage at a time until end, each stage held only memo.NavKeepAfter (plus
// a margin): long enough that a flight it starts or joins is kept for the
// next caller (Group.leave), never cancelled with this one, and short
// enough that the call answers well inside its deadline. A stage that
// finishes inside its hold hands the rest of the hold to the next, so a
// fast snapshot does not spend the graph's keep window. done is true when
// every stage is built; kept when the stage it stopped at keeps building;
// err is a failure that is not the hold ending (a typed-edge read failure
// is left to loadNavGraphsAt, which degrades it).
func warmNavStages(ctx context.Context, backend QueryInterface, service string, sc memo.Scope, end time.Time) (done, kept bool, err error) {
	hold := memo.NavKeepAfter() + coldKeepMargin
	typed := typedLoader(backend, service)
	var snap *memo.Snapshot
	stages := []func(context.Context) error{
		func(c context.Context) (e error) { snap, e = serviceSnapshotAt(c, backend, service, sc); return e },
		func(c context.Context) error { _, e := memo.GraphFor(c, snap, memo.GraphSyntax, nil); return e },
		func(c context.Context) error {
			_, e := memo.TypedFor(c, sc, typed)
			if e != nil && c.Err() == nil && !errors.Is(e, memo.ErrWouldNotFinish) {
				return nil // a read failure, degraded by loadNavGraphsAt (I38)
			}
			return e
		},
		func(c context.Context) error {
			_, _, _, e := memo.GraphForTyped(c, snap, memo.GraphBoth, typed)
			return e
		},
		func(c context.Context) error {
			_, _, _, e := memo.GraphForTyped(c, snap, memo.GraphResolved, typed)
			return e
		},
	}
	for _, stage := range stages {
		if time.Until(end) < hold {
			return false, false, nil // no time to keep a stage it would start
		}
		sctx, cancel := context.WithTimeout(ctx, hold)
		e := stage(sctx)
		held := sctx.Err() != nil
		cancel()
		switch {
		case e == nil:
			continue
		case ctx.Err() != nil:
			return false, false, e
		case errors.Is(e, memo.ErrWouldNotFinish):
			return false, false, nil
		case held:
			return false, true, nil
		default:
			return false, false, e
		}
	}
	return true, false, nil
}

// coldGraphNote is a cold symbol_context answer's partial field.
func coldGraphNote(service string, c *coldGraph) string {
	why := "building it takes longer than this call's deadline leaves"
	if c.unmeasured {
		why = "no build of it has finished since this server started, so its build time is unknown and taken as longer than this call's deadline leaves"
	}
	msg := "call graph not built (cold): the call graph for service " + service +
		" is not built and " + why + "; callers, callees and related tests are not computed, and the definition comes from the symbol index"
	if c.kept {
		msg += "; the build keeps running, so a later call answers them"
	} else {
		msg += "; no build was started: a call with a longer deadline, or none, builds it"
	}
	if lvl := tenancyLevelNote(); lvl != "" {
		msg += "; " + lvl
	}
	return msg
}

// symbolContextColdAnswer is symbol_context's answer while the call graph
// is cold (O1446): the definition, signature and index commit from the
// resident snapshot or, when that is not resident either, from the store
// lookup find_symbol's exact tier uses; callers, callees and tests named
// not computed. A snippet is served only from a resident snapshot, which
// can say whether the definition's file holds values that may be secrets
// (snippetWithheld).
func symbolContextColdAnswer(ctx context.Context, backend QueryInterface, service, symbol string, wantSnippet bool, c *coldGraph) (any, *rpcError) {
	note := coldGraphNote(service, c)
	snap, resident := memo.PeekSnapshot(c.scope)
	var nodes []core.SymbolNode
	if resident {
		recordSnapshotDisclosure(ctx, service, snap)
		nodes = snap.Nodes
	} else {
		found, err := backend.FindSymbol(ctx, service, symbolLookupName(symbol))
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "symbol_context: " + note + "; the symbol index lookup failed too: " + err.Error()}
		}
		nodes = found
	}
	_, cands, ok, _ := resolveSymbolContextName(nodes, service, symbol)
	if len(cands) > 1 {
		return ambiguousSymbolContextResult(ctx, backend, service, cands, len(cands), false), nil
	}
	if !ok {
		var r map[string]any
		if resident {
			r = symbolContextNotFound(ctx, backend, nodes, service, symbol)
		} else {
			r = storeNotFound(service, symbol)
		}
		r["partial"] = note
		return r, nil
	}
	def := cands[0]
	dto := toNodeDTOs([]core.SymbolNode{def})[0]
	result := map[string]any{
		"def":           &dto,
		"callers":       notComputedColdGraph,
		"callees":       notComputedColdGraph,
		"related_tests": notComputedColdGraph,
		"partial":       note,
	}
	if ic := indexCommitFor(ctx, backend, service); ic != "" {
		result["index_commit"] = ic
	}
	if wantSnippet && resident {
		addCitationWindow(ctx, service, dto.FilePath, dto.Line, 0, symbolContextSnippetMaxLines, sourceReadMaxBytes)
	}
	bindCitation(ctx, backend, service, dto.FilePath, func(status *core.CitationStatus) {
		dto.CitationStatus = status
		if status != nil {
			result["citation_status"] = status
		} else {
			delete(result, "citation_status")
		}
	})
	if !wantSnippet {
		return result, nil
	}
	if !resident {
		result["snippet_withheld"] = "not served while the service's symbol index loads: whether the definition's file holds values that may be secrets is not yet known"
		return result, nil
	}
	if why := snippetWithheld(def, nodes); why != "" {
		result["snippet_withheld"] = why
		return result, nil
	}
	// O1466: the warm answer's changed-file check (O1450) and read.
	serveDefinitionSnippet(ctx, backend, service, def, result)
	return result, nil
}

// symbolContextResolveFirst resolves symbol before any wait on the call
// graph (O1446): against the resident snapshot or, when that is not
// resident, the store lookup find_symbol's exact tier uses. A name with
// more than one definition answers the ambiguity and a name with none
// answers not_found, neither of which reads the graph; nil means the
// caller goes on to the graph: a unique name (the only one that waits for
// it), a resident set (the graph path resolves without waiting), an
// uncacheable scope (the call builds its own) or a failed lookup.
//
// A name the store lookup misses is checked against the snapshot, which
// lists the nearest names: the call starts or joins its load and stays on
// it only memo.NavKeepAfter (snapshotWithinHold), so a load it starts is
// kept for the next caller. When it is still loading the answer is
// not_found without the nearest names, saying so.
func symbolContextResolveFirst(ctx context.Context, backend QueryInterface, service, symbol string, sc memo.Scope) map[string]any {
	if sc.Watermark == "" {
		return nil
	}
	if _, resident := memo.NavBuildCost(sc); resident {
		return nil
	}
	snap, resident := memo.PeekSnapshot(sc)
	if !resident {
		found, err := backend.FindSymbol(ctx, service, symbolLookupName(symbol))
		if err != nil {
			return nil
		}
		_, cands, ok, _ := resolveSymbolContextName(found, service, symbol)
		switch {
		case len(cands) > 1:
			return ambiguousSymbolContextResult(ctx, backend, service, cands, len(cands), false)
		case ok:
			return nil
		}
		if snap = snapshotWithinHold(ctx, backend, service, sc); snap == nil {
			return storeNotFound(service, symbol)
		}
	}
	recordSnapshotDisclosure(ctx, service, snap)
	_, cands, ok, _ := resolveSymbolContextName(snap.Nodes, service, symbol)
	switch {
	case len(cands) > 1:
		return ambiguousSymbolContextResult(ctx, backend, service, cands, len(cands), false)
	case !ok:
		return symbolContextNotFound(ctx, backend, snap.Nodes, service, symbol)
	}
	return nil
}

// storeNotFound is symbol_context's not_found while the service's symbol
// snapshot is not resident: the store lookup holds no exact match, and the
// nearest names (symbolContextNotFound) need the snapshot.
func storeNotFound(service, symbol string) map[string]any {
	return map[string]any{"not_found": true, "hint": fmt.Sprintf("no symbol in service %s's index matches %q exactly; the nearest names are listed once its symbol index has loaded", service, symbol)}
}

// snapshotWithinHold starts or joins sc's snapshot load and stays on it
// only memo.NavKeepAfter plus a margin: past that, Group.leave keeps a load
// it started for the next caller, and the call never waits out a long one.
// nil when the load did not finish inside the hold or failed, or the
// deadline is too short to keep a load it would start.
func snapshotWithinHold(ctx context.Context, backend QueryInterface, service string, sc memo.Scope) *memo.Snapshot {
	hold := memo.NavKeepAfter() + coldKeepMargin
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining-min(max(remaining/verifyStageReserveShare, verifyStageReserveMin), verifyStageReserveMax) < hold {
			return nil
		}
	}
	hctx, cancel := context.WithTimeout(ctx, hold)
	defer cancel()
	snap, err := serviceSnapshotAt(hctx, backend, service, sc)
	if err != nil {
		return nil
	}
	return snap
}

// resolveSymbolContextName is symbolnav.ResolveTiered with O1451's second
// spelling: a name that resolves to nothing and carries no service prefix
// is retried as service::name, so the repo-relative key `dir/file.Name`
// an ambiguous answer lists resolves within the call's service. resolved
// is the spelling that answered (symbol, or the prefixed key).
func resolveSymbolContextName(nodes []core.SymbolNode, service, symbol string) (key string, cands []core.SymbolNode, ok bool, resolved string) {
	key, cands, ok = symbolnav.ResolveTiered(nodes, symbol)
	if ok || len(cands) > 0 || strings.Contains(symbol, "::") {
		return key, cands, ok, symbol
	}
	prefixed := service + "::" + symbol
	key, cands, ok = symbolnav.ResolveTiered(nodes, prefixed)
	if ok || len(cands) > 0 {
		return key, cands, ok, prefixed
	}
	return "", nil, false, symbol
}

// symbolLookupName is the name a store lookup (FindSymbol) matches for a
// symbol_context query: a key's service prefix and directory dropped
// ("svc::cmd/app/main.main" -> "main.main", the qualifier FindSymbol checks
// against the symbol's package), a bare name unchanged.
func symbolLookupName(symbol string) string {
	if i := strings.LastIndex(symbol, "::"); i >= 0 {
		symbol = symbol[i+2:]
	}
	if i := strings.LastIndex(symbol, "/"); i >= 0 {
		symbol = symbol[i+1:]
	}
	return symbol
}

// symbolContextNotFound is symbol_context's answer to a name no tier
// resolves (O1451): not_found with find_symbol's nearest candidates
// (case-insensitive, then substring, then edit distance), never a null
// definition beside empty callers, callees and tests that read as "exists,
// nobody calls it" (I38).
func symbolContextNotFound(ctx context.Context, backend QueryInterface, nodes []core.SymbolNode, service, symbol string) map[string]any {
	name := symbolLookupName(symbol)
	leaf := name[strings.LastIndex(name, ".")+1:]
	near, total := caseFoldMatches(nodes, name)
	if len(near) == 0 {
		near, total = substringMatches(nodes, leaf)
	}
	if len(near) == 0 {
		near = fuzzySuggestions(nodes, leaf)
		total = len(near)
	}
	result := map[string]any{"not_found": true}
	hint := fmt.Sprintf("no symbol in service %s resolves from %q", service, symbol)
	if len(near) == 0 {
		result["hint"] = hint + ", and no indexed name is near it; search finds it as text"
		return result
	}
	shown := near[:min(len(near), symbolContextCandidateCap)]
	rows := make([]any, len(shown))
	for i, n := range shown {
		rows[i] = citationCandidate(ctx, backend, service, n)
	}
	result["candidates"] = rows
	result["count"] = total
	result["hint"] = hint + "; pass a candidate's symbol_key"
	return result
}

// searchReceipts is a service search's source of sym/kind receipts
// (O1447). A resident snapshot is read in place (groupSymbolsByFile only
// reads it), never copied per call. One that is not resident is not waited
// for: the search starts or joins its load and stays only memo.NavKeepAfter
// (so the load is kept for the next caller), runs the engine, and renders
// its hits without receipts when the snapshot is still loading.
type searchReceipts struct {
	service string
	scope   memo.Scope
	byFile  map[string][]core.SymbolNode
}

// beginSearchReceipts peeks the service's snapshot, starting its load when
// it is not resident. An uncacheable scope (I21) loads it as before: the
// load is this call's own.
func beginSearchReceipts(ctx context.Context, backend QueryInterface, service string) *searchReceipts {
	sc := navScope(ctx, backend, service)
	r := &searchReceipts{service: service, scope: sc}
	if sc.Watermark == "" {
		if snap, err := serviceSnapshotAt(ctx, backend, service, sc); err == nil {
			r.byFile = groupSymbolsByFile(snap.Nodes)
		}
		return r
	}
	if r.peek(ctx) {
		return r
	}
	if snap := snapshotWithinHold(ctx, backend, service, sc); snap != nil {
		r.byFile = groupSymbolsByFile(snap.Nodes)
	}
	return r
}

// peek fills byFile from the resident snapshot.
func (r *searchReceipts) peek(ctx context.Context) bool {
	snap, ok := memo.PeekSnapshot(r.scope)
	if !ok {
		return false
	}
	recordSnapshotDisclosure(ctx, r.service, snap)
	r.byFile = groupSymbolsByFile(snap.Nodes)
	return true
}

// finish is the receipts to render after the engine ran, and the
// receipts_omitted note when there are none because the snapshot is still
// loading ("" otherwise: a load error omits them silently, as before).
func (r *searchReceipts) finish(ctx context.Context) (map[string][]core.SymbolNode, string) {
	if r.byFile != nil || r.scope.Watermark == "" || r.peek(ctx) {
		return r.byFile, ""
	}
	return nil, "sym and kind are omitted: the symbol index for service " + r.service +
		" is still loading (the first call after an ingest or a daemon start loads it); a later search carries them"
}
