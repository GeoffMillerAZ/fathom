package docindex

// docdrift.go — docDrift (KR-7, docs/design/knowledge-retrieval.md §6: "At
// link time the target's content hash is stored. When a later ingest
// changes the target..., the span is possibly stale. `staleness_report`
// gains a `doc_drift` section: span, target, the commit that changed it,
// and how long ago. It never says a doc is wrong; it says the thing it
// describes moved" — §11 restates the same refusal). Pure and
// deterministic (internal/CLAUDE.md): the actual re-resolution and commit
// lookup are the caller's functions, matching link.go's (KR-6) own
// Resolver-injection shape — docDrift itself does no I/O; store.OpenDocLinks
// (KR-6) is what actually reads doc_links.
//
// DriftedDocLink deliberately carries no verdict field ("wrong", "stale",
// pass/fail) — only what moved (LinkedHash vs CurrentHash) and, when
// known, which commit and when. A caller renders that as "possibly stale"
// (§6's own word); docDrift itself never says more.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TargetResolution is one open link's target, resolved RIGHT NOW: whether
// it still exists at all, its fresh content hash, and — when the target
// has a single owning file (a symbol's defining file, or a path target
// itself; a contract key has none) — that file, so docDrift can look up
// the commit that most recently touched it.
type TargetResolution struct {
	Exists bool
	Hash   string
	File   string // "" when the target has no single owning file
}

// CurrentTargetFunc resolves (targetKind, target) — the same pair every
// store.DocLinkView row already carries — to its current state. The real,
// store-backed implementation degrades exactly like Resolver's
// ResolveContractKey does before SG-2 lands: a "key" target with no
// contract_keys table to check returns Exists: false only when the
// grammar itself refuses it, never a guess.
type CurrentTargetFunc func(targetKind, target string) TargetResolution

// CommitForPathFunc names the most recent commit (of the service's own
// stored commits rows — internal/store.Commit, the same substrate
// TQ-1's file_churn already reads, never a fresh `git log` at request time
// per RT-76's "whole-repository facts come from ingest" doctrine) that
// touched path. ok=false when the service has no stored commits, or none
// touched path.
type CommitForPathFunc func(path string) (hash, dateRFC3339 string, ok bool)

// DriftedDocLink is one open doc_links row whose target moved: either its
// content hash no longer matches TargetHashAtLink, or it stopped
// resolving entirely (moved so far it vanished — still "moved", never
// "wrong", §11). ChangedCommit/ChangedAt are "" when no owning file is
// known for the target, or no commit touching it is on record.
type DriftedDocLink struct {
	Path          string
	HeadingPath   string
	UnitID        string
	TargetKind    string
	Target        string
	Line          int
	LinkTier      string
	LinkedHash    string
	CurrentHash   string
	ChangedCommit string
	ChangedAt     string
}

// docDrift compares every links row against current's fresh resolution and
// returns the ones whose target moved — hash mismatch, or the target no
// longer resolving at all — attributed to the most recent commit touching
// the target's owning file when commitForPath can name one. Unchanged
// links (Exists && Hash == TargetHashAtLink) produce nothing: an
// unresolved/never-drifted link is not noise. Results are sorted by path,
// then line, then target (I2 determinism) — never the input order, which
// OpenDocLinks already sorts the same way but this function does not rely
// on that.
func docDrift(links []store.DocLinkView, current CurrentTargetFunc, commitForPath CommitForPathFunc) []DriftedDocLink {
	var out []DriftedDocLink
	for _, l := range links {
		res := current(l.TargetKind, l.Target)
		if res.Exists && res.Hash == l.TargetHashAtLink {
			continue
		}
		d := DriftedDocLink{
			Path:        l.Path,
			HeadingPath: l.HeadingPath,
			UnitID:      l.UnitID,
			TargetKind:  l.TargetKind,
			Target:      l.Target,
			Line:        l.Line,
			LinkTier:    l.LinkTier,
			LinkedHash:  l.TargetHashAtLink,
			CurrentHash: res.Hash,
		}
		if res.File != "" && commitForPath != nil {
			if hash, at, ok := commitForPath(res.File); ok {
				d.ChangedCommit = hash
				d.ChangedAt = at
			}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Target < out[j].Target
	})
	return out
}

// StalenessDocDrift is docDrift's store-backed entrypoint: every still-open
// doc_links row for serviceID (store.OpenDocLinks, KR-6), re-resolved
// against st's live index right now, attributed to the most recent of the
// service's own stored commits (ListCommits, already newest-first) that
// touched the target's file. Returns ErrDocIndexUnavailable for a
// non-SQLite store, matching RankDocs' (KR-4) and Resolver's own refusal
// shape rather than reporting an empty, indistinguishable "nothing
// drifted" (I38) when there is in fact no doc index to check at all.
//
// Symbol resolution mirrors LinkDocSpans' Resolver contract exactly —
// "exactly one indexed symbol" — via the store's own ObservationsForSymbol
// candidate lookup narrowed to the same bare/suffix name rule
// internal/query/engine.go's symbolMatches applies (this package does not
// import internal/query to reuse it byte-for-byte, to avoid coupling a
// pure-comparison package to the query engine for one filter; the rule
// itself — exact name, or a ".name" suffix match either direction — is
// simple enough to restate here). A "key" target always resolves
// Exists: false until SG-2's contract_keys table lands, same as
// Resolver.ResolveContractKey's own documented degrade.
func StalenessDocDrift(ctx context.Context, st store.Store, serviceID string) ([]DriftedDocLink, error) {
	sq, ok := st.(*store.SQLiteStore)
	if !ok {
		return nil, ErrDocIndexUnavailable
	}
	links, err := sq.OpenDocLinks(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("StalenessDocDrift: open links: %w", err)
	}
	if len(links) == 0 {
		return nil, nil
	}

	commits, err := sq.ListCommits(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("StalenessDocDrift: commits: %w", err)
	}
	commitForPath := func(path string) (string, string, bool) {
		for _, c := range commits { // ListCommits: ORDER BY date DESC, so the first match is the most recent
			for _, f := range c.Files {
				if f == path {
					return c.Hash, time.Unix(0, c.Date).UTC().Format(time.RFC3339), true
				}
			}
		}
		return "", "", false
	}

	current := func(kind, target string) TargetResolution {
		switch kind {
		case "path":
			hash, found, ferr := sq.Fingerprint(ctx, serviceID, target)
			if ferr != nil || !found {
				return TargetResolution{}
			}
			return TargetResolution{Exists: true, Hash: hash, File: target}
		case "symbol":
			return resolveSymbolNow(ctx, sq, serviceID, target)
		default: // "key": no contract_keys table to check yet (SG-2)
			return TargetResolution{}
		}
	}

	return docDrift(links, current, commitForPath), nil
}

// resolveSymbolNow narrows ObservationsForSymbol's candidate superset to
// bare/suffix name matches and returns a live resolution only when exactly
// one DISTINCT symbol name matches — multiple observations of the very
// same symbol (re-extracted across passes) are not ambiguity.
func resolveSymbolNow(ctx context.Context, sq *store.SQLiteStore, serviceID, name string) TargetResolution {
	obs, err := sq.ObservationsForSymbol(ctx, serviceID, name)
	if err != nil {
		return TargetResolution{}
	}
	type hit struct{ hash, file string }
	seen := map[string]hit{}
	for _, o := range obs {
		if !(o.Symbol == name || strings.HasSuffix(o.Symbol, "."+name) || strings.HasSuffix(name, "."+o.Symbol)) {
			continue
		}
		file := o.SourceRef
		if i := strings.LastIndexByte(file, ':'); i >= 0 {
			file = file[:i]
		}
		seen[o.Symbol] = hit{hash: o.ContentHash, file: file}
	}
	if len(seen) != 1 {
		return TargetResolution{}
	}
	for _, h := range seen {
		return TargetResolution{Exists: true, Hash: h.hash, File: h.file}
	}
	return TargetResolution{} // unreachable
}
