package semantic

// anchors.go — SU-21 (ADR-041 decision 7, docs/vision/31-substrate.md §3
// D6): a claim's citation, when it resolves to exactly one symbol
// definition in the index, gets a SECOND link binding it to that symbol's
// own identity (core.SymbolKey) beside the line-range citation — so a
// later rename or a pure line-shift can be told apart from a real content
// change, and governance never goes silent on a rename (rename_impact lists
// claims anchored to it; Sweep re-anchors a drifted citation through it
// when the literal-text search — T5-1/O13 — cannot).

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// SymbolIndex is a service's indexed symbols, queryable by enclosing byte
// range (EnclosingSymbol, used at claim mint) or by canonical SymbolKey
// (ByKey, used by Sweep's re-anchor and rename_impact's claims_anchored
// receipt). Load with LoadSymbolIndex.
type SymbolIndex struct {
	byFile map[string][]core.SymbolNode
	byKey  map[string]core.SymbolNode
}

// LoadSymbolIndex builds a SymbolIndex from every declared symbol the
// service's observations carry — one QueryObservations scan, the same cost
// symbolFileLookup (contract.go) and SymbolNamesForFile already pay per
// call. A store read failure degrades to an empty (never nil-panicking)
// index — callers treat "no symbol resolved" as an honest absence, exactly
// like ContractScope's own lookup.
func LoadSymbolIndex(ctx context.Context, st store.Store, serviceID string) (SymbolIndex, error) {
	idx := SymbolIndex{byFile: map[string][]core.SymbolNode{}, byKey: map[string]core.SymbolNode{}}
	obs, err := st.QueryObservations(ctx, serviceID, "")
	if err != nil {
		return idx, err
	}
	for _, o := range obs {
		if o.Symbol == "" || o.SourceRef == "" || o.Line <= 0 {
			continue
		}
		n := o.ToSymbolNode()
		// SourceRef sometimes carries a ":line" suffix (symbolFileLookup's
		// own normalization) — stripped so byFile keys on the bare path.
		file := o.SourceRef
		if i := strings.LastIndex(file, ":"); i > 0 {
			if _, aerr := strconv.Atoi(file[i+1:]); aerr == nil {
				file = file[:i]
			}
		}
		n.FilePath = file
		if n.EndLine < n.Line {
			n.EndLine = n.Line
		}
		idx.byFile[file] = append(idx.byFile[file], n)
		idx.byKey[n.Key()] = n
	}
	return idx, nil
}

// EnclosingSymbol returns the SINGLE symbol definition in path whose
// declared range [Line, EndLine] contains [start, end], or ok=false when
// zero or more than one symbol overlaps — an ambiguous or absent match
// anchors to nothing rather than guessing which one is "the" enclosing
// definition (the same no-guess discipline ContractScope's own doc comment
// states). A file's module symbol (K18, core.ModuleSymbolName) spans the
// whole file, which is the citation's own granularity: it is never an
// anchor and never makes a citation ambiguous.
func (idx SymbolIndex) EnclosingSymbol(path string, start, end int) (core.SymbolNode, bool) {
	var (
		match core.SymbolNode
		count int
	)
	for _, n := range idx.byFile[path] {
		if n.IsModuleSymbol() {
			continue
		}
		if start >= n.Line && end <= n.EndLine {
			match = n
			count++
		}
	}
	if count != 1 {
		return core.SymbolNode{}, false
	}
	return match, true
}

// ByKey resolves a canonical core.SymbolKey to its CURRENT symbol node —
// Sweep's re-anchor and rename_impact's claims_anchored receipt both read
// through this to ask "does the symbol this claim anchored to still exist,
// and if so where".
func (idx SymbolIndex) ByKey(key string) (core.SymbolNode, bool) {
	n, ok := idx.byKey[key]
	return n, ok
}

// AnchorSymbol resolves claimID's cites span (citeRef, "path#Lstart-Lend")
// to the single enclosing symbol definition in idx and, when found, returns
// the "anchors" ClaimLink binding the claim to it: TargetRef is the
// symbol's core.SymbolKey, TargetHash is the symbol's OWN current content
// fingerprint (core.HashLines over its declared [Line, EndLine] range,
// fetched via fetch — the same fetch(path) contract every other claim-link
// hash in this package uses). ok is false when the span does not resolve
// to exactly one symbol, or the symbol's file cannot be read — an anchor
// is never written on a guess.
func AnchorSymbol(idx SymbolIndex, claimID, citeRef string, fetch func(path string) ([]byte, bool)) (core.ClaimLink, bool) {
	path, start, end, ok := core.ParseCiteRef(citeRef)
	if !ok {
		return core.ClaimLink{}, false
	}
	sym, ok := idx.EnclosingSymbol(path, start, end)
	if !ok {
		return core.ClaimLink{}, false
	}
	data, found := fetch(sym.FilePath)
	if !found {
		return core.ClaimLink{}, false
	}
	hash, hok := core.HashLines(data, sym.Line, sym.EndLine)
	if !hok {
		return core.ClaimLink{}, false
	}
	return core.ClaimLink{ClaimID: claimID, Rel: core.RelAnchors, TargetRef: sym.Key(), TargetHash: hash}, true
}

// ClaimsAnchoredTo returns the sorted, deduplicated ClaimIDs of every claim
// with an "anchors" link naming symbolKey (SU-21, ADR-041 decision 7) — the
// governance-on-rename receipt rename_impact serves as `claims_anchored`.
// nil (never an error) when the service has no claims at all or none
// anchored to symbolKey — an honest absence, not a failure.
func ClaimsAnchoredTo(ctx context.Context, st store.Store, serviceID, symbolKey string) ([]string, error) {
	if symbolKey == "" {
		return nil, nil
	}
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return nil, err
	}
	if len(claims) == 0 {
		return nil, nil
	}
	ids := make([]string, len(claims))
	for i, cl := range claims {
		ids[i] = cl.ClaimID
	}
	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var anchored []string
	for _, l := range links {
		if l.Rel == core.RelAnchors && l.TargetRef == symbolKey && !seen[l.ClaimID] {
			seen[l.ClaimID] = true
			anchored = append(anchored, l.ClaimID)
		}
	}
	sort.Strings(anchored)
	return anchored, nil
}

// anchorSymbolResolver returns a memoized SU-21 resolver over the
// service's symbol index, loaded at MOST once on first actual use — most
// compiled layers govern prose documents, which resolve to zero symbols
// and never trigger the load at all (the same pay-only-when-used
// discipline symbolFileLookup, contract.go, already applies to its own
// service-wide scan).
func anchorSymbolResolver(ctx context.Context, st store.Store, serviceID string) func(claimID, citeRef string, fetch func(string) ([]byte, bool)) (core.ClaimLink, bool) {
	var (
		loaded bool
		idx    SymbolIndex
	)
	return func(claimID, citeRef string, fetch func(string) ([]byte, bool)) (core.ClaimLink, bool) {
		if !loaded {
			loaded = true
			idx, _ = LoadSymbolIndex(ctx, st, serviceID)
		}
		return AnchorSymbol(idx, claimID, citeRef, fetch)
	}
}
