package docindex

// linkpass.go — the production link pass (KR-6, docs/design/
// knowledge-retrieval.md §6): the ingest links every unit it (re)indexed.
// internal/fusion's indexDocs passes WithOnWrite a callback that runs
// LinkUnits with a NewStoreResolver, so doc_links is written by the same
// ingest that writes doc_units, never by a second job.
//
// The hash gate decides which units are linked. A unit is re-linked only
// when its file is re-indexed; an unchanged unit keeps the links and the
// link-time target hashes it has, which is what KR-7's drift compares a
// fresh resolution against. docLinkPassVersion is folded into every unit's
// content hash so that units indexed before this pass existed are
// re-indexed, and so linked, exactly once.

import (
	"context"
	"fmt"
	"regexp"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// docLinkPassVersion is folded into each repo doc unit's content hash
// (unitContentHash). Changing it re-indexes, and so re-links, every unit
// once on the next ingest; it changes only when the link pass itself
// changes what it would write for unchanged text.
const docLinkPassVersion = "doc-link-pass/1"

// unitContentHash is the gate's content hash for a unit body.
func unitContentHash(body string) string {
	return sha256Hex([]byte(docLinkPassVersion + "\x00" + body))
}

// WithOnWrite registers fn, called once per file IngestDocUnits
// (re)writes, right after its units are stored: written are the units now
// stored for path (with their bodies), removed the ids of units the path
// held before and no longer does. A file deleted from the corpus calls fn
// with no written units. An error from fn stops the ingest the way a
// store error does.
func WithOnWrite(fn func(ctx context.Context, path string, written []store.DocUnit, removed []string) error) IngestOption {
	return func(c *ingestConfig) { c.onWrite = fn }
}

// removedUnitIDs is before's ids that after does not hold.
func removedUnitIDs(before, after []store.DocUnit) []string {
	keep := make(map[string]bool, len(after))
	for _, u := range after {
		keep[u.UnitID] = true
	}
	var out []string
	for _, u := range before {
		if !keep[u.UnitID] {
			out = append(out, u.UnitID)
		}
	}
	return out
}

// LinkUnits links each written unit's spans (LinkDocSpans against r) and
// reconciles them into doc_links (store.SyncDocLinksForUnit: an open link
// the unit still makes is left untouched, a new one opens at now, one it no
// longer makes closes at now), and closes every link of each removed unit.
// It returns the number of links the written units hold. now is UnixNano,
// the ingest's convention.
func LinkUnits(ctx context.Context, sq *store.SQLiteStore, serviceID string, written []store.DocUnit, removed []string, r Resolver, now int64) (int, error) {
	links := 0
	for _, u := range written {
		if err := ctx.Err(); err != nil {
			return links, err
		}
		found := LinkDocSpans(Unit{HeadingPath: u.HeadingPath, LineStart: u.LineStart, LineEnd: u.LineEnd, Kind: u.Kind, Body: u.Body}, r)
		fresh := make([]store.DocLink, 0, len(found))
		for _, l := range found {
			fresh = append(fresh, store.DocLink{TargetKind: l.TargetKind, Target: l.Target, Line: l.Line, LinkTier: l.Tier, TargetHashAtLink: l.TargetHash})
		}
		if err := sq.SyncDocLinksForUnit(ctx, serviceID, u.UnitID, fresh, now); err != nil {
			return links, fmt.Errorf("link %s: %w", u.Path, err)
		}
		links += len(fresh)
	}
	for _, id := range removed {
		if err := sq.SyncDocLinksForUnit(ctx, serviceID, id, nil, now); err != nil {
			return links, fmt.Errorf("close links of a removed unit: %w", err)
		}
	}
	return links, nil
}

// symbolShapedRE is the only span text the store-backed resolver looks up
// as a symbol: an identifier, optionally dot-qualified. Anything else (a
// call, a flag, an expression) cannot be exactly one symbol's name, and
// refusing it without a query keeps a first ingest's backfill bounded.
var symbolShapedRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

type resolution struct {
	hash string
	ok   bool
}

// storeResolver resolves spans against the service's live index exactly
// as StalenessDocDrift re-resolves a link's target later: a path through
// its source fingerprint, a symbol through resolveSymbolNow's
// exactly-one-symbol rule, and a contract key never (ok=false) until
// SG-2's contract_keys table lands. Link time and drift time therefore
// compare the same hash. Lookups are memoized for the pass it serves.
type storeResolver struct {
	ctx       context.Context
	sq        *store.SQLiteStore
	serviceID string
	paths     map[string]resolution
	symbols   map[string]resolution
}

// NewStoreResolver is the Resolver the ingest's link pass uses for
// serviceID; use one per pass (its memo assumes the index does not move
// under it).
func NewStoreResolver(ctx context.Context, sq *store.SQLiteStore, serviceID string) Resolver {
	return &storeResolver{ctx: ctx, sq: sq, serviceID: serviceID, paths: map[string]resolution{}, symbols: map[string]resolution{}}
}

func (r *storeResolver) ResolvePath(path string) (string, bool) {
	if res, ok := r.paths[path]; ok {
		return res.hash, res.ok
	}
	hash, found, err := r.sq.Fingerprint(r.ctx, r.serviceID, path)
	res := resolution{hash: hash, ok: err == nil && found}
	r.paths[path] = res
	return res.hash, res.ok
}

func (r *storeResolver) ResolveSymbol(name string) (string, bool) {
	if !symbolShapedRE.MatchString(name) {
		return "", false
	}
	if res, ok := r.symbols[name]; ok {
		return res.hash, res.ok
	}
	now := resolveSymbolNow(r.ctx, r.sq, r.serviceID, name)
	res := resolution{hash: now.Hash, ok: now.Exists}
	r.symbols[name] = res
	return res.hash, res.ok
}

// ResolveContractKey returns ok=false for every key until SG-2's
// contract_keys table exists (Resolver's documented degrade).
func (r *storeResolver) ResolveContractKey(string) (string, bool) { return "", false }
