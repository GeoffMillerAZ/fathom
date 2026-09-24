package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// surface.go — the read path memories SURFACE through (docs/design/
// anchored-memory.md §4; AM-6 injection and AM-8 memory_search).
//
// One pipeline serves both consumers: list the service's memories, narrow to
// the ones ABOUT a file when a file is given, filter by a free-text query,
// evaluate drift at read time, keep only what may surface (fresh and
// suspect — retired never, orphaned goes to the review queue), and return
// them in the deterministic surfacing order (tier, then specificity, then
// id). Nothing here fabricates a label: every Evaluation returned carries
// the tier and state EvaluateState derived for it.

// SymbolLocator is the optional Index capability file-scoped surfacing
// needs: which source file defines a symbol anchor. StoreIndex implements
// it. An Index without it cannot place a symbol anchor in any file, so a
// symbol-anchored memory is simply not "about" any file for that index —
// law 3: a memory fathom cannot place is not surfaced on a guess.
type SymbolLocator interface {
	SymbolFile(ctx context.Context, service, ref string) (file string, ok bool, err error)
}

// SymbolFile implements SymbolLocator over the index snapshot: the
// source_ref of the observation defining ref (bare name or qualified key).
// ok is false for an unknown symbol AND for an ambiguous bare name (review
// finding W1: an anchor that cannot bind cannot be placed either).
func (s *StoreIndex) SymbolFile(ctx context.Context, service, ref string) (string, bool, error) {
	snap, err := s.symbols(ctx, service)
	if err != nil {
		return "", false, err
	}
	if _, amb := snap.ambiguous[ref]; amb {
		return "", false, nil
	}
	f, ok := snap.facts[ref]
	if !ok {
		return "", false, nil
	}
	return f.sourceRef, true, nil
}

// AnchorTouchesFile reports whether anchor a is ABOUT file (a repo-relative
// path): a file anchor names it, a path anchor's glob matches it (the same
// matcher StoreIndex drifts on), a symbol anchor is defined in it. Commit
// and claim anchors are never about a file.
func AnchorTouchesFile(ctx context.Context, idx Index, service string, a Anchor, file string) (bool, error) {
	switch a.Kind {
	case AnchorFile:
		return a.Ref == file, nil
	case AnchorPath:
		return matchGlob(a.Ref, file), nil
	case AnchorSymbol:
		loc, ok := idx.(SymbolLocator)
		if !ok {
			return false, nil
		}
		f, found, err := loc.SymbolFile(ctx, service, a.Ref)
		if err != nil {
			return false, err
		}
		return found && f == file, nil
	default:
		return false, nil
	}
}

// ForFile filters ms down to the memories with at least one anchor touching
// file, preserving order.
func ForFile(ctx context.Context, idx Index, service, file string, ms []Memory) ([]Memory, error) {
	out := make([]Memory, 0, len(ms))
	for _, m := range ms {
		for _, a := range m.Anchors {
			hit, err := AnchorTouchesFile(ctx, idx, service, a, file)
			if err != nil {
				return nil, fmt.Errorf("anchor %s: %w", a.Key(), err)
			}
			if hit {
				out = append(out, m)
				break
			}
		}
	}
	return out, nil
}

// MatchesQuery reports whether m matches a free-text query: case-insensitive,
// every whitespace-separated term must occur as a substring of the claim
// text or of some anchor key ("kind:ref"). An empty query matches every
// memory.
func MatchesQuery(m Memory, query string) bool {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return true
	}
	hay := []string{strings.ToLower(m.Claim)}
	for _, a := range m.Anchors {
		hay = append(hay, strings.ToLower(a.Key()))
	}
	for _, term := range terms {
		found := false
		for _, h := range hay {
			if strings.Contains(h, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// SearchQuery scopes Search. Service is required; File narrows to memories
// about that repo-relative file (AM-6's injection scope); Query is the
// free-text filter (AM-8). Both may be empty, in which case every surfacing
// memory of the service is returned.
type SearchQuery struct {
	Service string
	Query   string
	File    string
}

// Search is the one read path both memory_search and the edit hook's
// injection use. It returns only memories that SURFACE — fresh and suspect,
// each labeled by EvaluateState — in the deterministic surfacing order.
// Retired memories never appear; orphaned ones belong to the review queue.
// cache may be nil; now must be non-nil (EvaluateState's own contract).
func Search(ctx context.Context, ms Store, idx Index, cache *DriftCache, now func() time.Time, q SearchQuery) ([]Evaluation, error) {
	if ms == nil {
		return nil, fmt.Errorf("memory: Search requires a store")
	}
	if q.Service == "" {
		return nil, fmt.Errorf("memory: Search requires a service")
	}
	all, err := ms.List(ctx, q.Service)
	if err != nil {
		return nil, fmt.Errorf("list memories for %s: %w", q.Service, err)
	}
	// Drop retired memories before any anchor is probed: they never
	// surface, so hashing their anchors would be work spent on nothing.
	live := make([]Memory, 0, len(all))
	for _, m := range all {
		if m.Retired() {
			continue
		}
		if !MatchesQuery(m, q.Query) {
			continue
		}
		live = append(live, m)
	}
	if q.File != "" {
		live, err = ForFile(ctx, idx, q.Service, q.File, live)
		if err != nil {
			return nil, err
		}
	}
	evs, err := EvaluateAll(ctx, idx, cache, now, live)
	if err != nil {
		return nil, err
	}
	return Surfacing(evs), nil
}

// ShortLine renders the memory as one budgeted line: the label (tier and
// state — never omitted, §9) and the claim. A suspect memory names each
// anchor that moved in a short clause, not the full detail Line carries,
// because the edit hook's whole payload is ≤320 tokens (ADR-016, amended).
func (e Evaluation) ShortLine() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s · %s", e.Tier, e.State)
	if e.State == StateSuspect {
		b.WriteString(" ⚠")
	}
	b.WriteString("] ")
	b.WriteString(e.Claim)
	var moved []string
	for _, d := range e.Drift {
		if d.Status == DriftUnchanged {
			continue
		}
		moved = append(moved, d.Anchor.Key()+" "+string(d.Status))
	}
	if len(moved) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(moved, ", "))
		b.WriteString(")")
	}
	return b.String()
}
