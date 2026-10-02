package docindex

// linkpass.go — the production link pass (KR-6, docs/design/
// knowledge-retrieval.md §6): the ingest links every unit it (re)indexed.
// internal/fusion's indexDocs passes WithOnWrite a callback that runs
// a LinkPass with a NewStoreResolver, so doc_links is written by the same
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

// LinkPass is one ingest's link pass over a service (EW-50; O362): every
// file's written and removed units go through one store.DocLinkSync, which
// reads the service's open links once, where SyncDocLinksForUnit read a
// unit's open links by scanning every link of the service (units times
// links per pass).
type LinkPass struct {
	sync *store.DocLinkSync
	r    Resolver
	now  int64
}

// NewLinkPass starts a link pass for serviceID resolving spans against r;
// now is UnixNano.
func NewLinkPass(sq *store.SQLiteStore, serviceID string, r Resolver, now int64) *LinkPass {
	return &LinkPass{sync: sq.NewDocLinkSync(serviceID), r: r, now: now}
}

// Link links each of one file's written units' spans (LinkDocSpans against
// the pass's resolver) and reconciles them into doc_links (an open link the
// unit still makes is left untouched, a new one opens at the pass's now, one
// it no longer makes closes then), and closes every link of each removed
// unit. It returns the number of links the written units hold.
func (p *LinkPass) Link(ctx context.Context, written []store.DocUnit, removed []string) (int, error) {
	links := 0
	for _, u := range written {
		if err := ctx.Err(); err != nil {
			return links, err
		}
		found := LinkDocSpans(Unit{HeadingPath: u.HeadingPath, LineStart: u.LineStart, LineEnd: u.LineEnd, Kind: u.Kind, Body: u.Body}, p.r)
		fresh := make([]store.DocLink, 0, len(found))
		for _, l := range found {
			fresh = append(fresh, store.DocLink{TargetKind: l.TargetKind, Target: l.Target, Line: l.Line, LinkTier: l.Tier, TargetHashAtLink: l.TargetHash})
		}
		if err := p.sync.SyncUnit(ctx, u.UnitID, fresh, p.now); err != nil {
			return links, fmt.Errorf("link %s: %w", u.Path, err)
		}
		links += len(fresh)
	}
	for _, id := range removed {
		if err := p.sync.SyncUnit(ctx, id, nil, p.now); err != nil {
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
//
// Symbols resolve from one read of the service's symbol table per pass
// (EW-49; O362): the first symbol-shaped span reads store.SymbolHashes
// into a symbolIndex, and every name resolves from it with a few map
// probes. A point lookup per name (resolveSymbolNow, which drift keeps)
// scanned the service's whole symbol index for each name, since its
// `symbol GLOB '*.name'` cannot range idx_observations_service_symbol:
// names times symbols per pass. The index lives as long as the resolver,
// one link pass.
type storeResolver struct {
	ctx       context.Context
	sq        *store.SQLiteStore
	serviceID string
	paths     map[string]resolution
	symbols   map[string]resolution
	index     *symbolIndex // built at the first symbol lookup

	// work counters (EW-49's linearity test): store reads and index probes
	// this pass made.
	storeReads   int
	symbolReads  int
	symbolProbes int
}

// NewStoreResolver is the Resolver the ingest's link pass uses for
// serviceID; use one per pass (its memo and symbol index assume the index
// does not move under it).
func NewStoreResolver(ctx context.Context, sq *store.SQLiteStore, serviceID string) Resolver {
	return &storeResolver{ctx: ctx, sq: sq, serviceID: serviceID, paths: map[string]resolution{}, symbols: map[string]resolution{}}
}

func (r *storeResolver) ResolvePath(path string) (string, bool) {
	if res, ok := r.paths[path]; ok {
		return res.hash, res.ok
	}
	r.storeReads++
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
	if r.index == nil {
		r.storeReads++
		r.symbolReads++
		rows, err := r.sq.SymbolHashes(r.ctx, r.serviceID)
		if err != nil {
			rows = nil // as resolveSymbolNow on a failed read: nothing resolves
		}
		r.index = newSymbolIndex(rows)
	}
	now := r.index.resolve(name, &r.symbolProbes)
	res := resolution{hash: now.Hash, ok: now.Exists}
	r.symbols[name] = res
	return res.hash, res.ok
}

// ResolveContractKey returns ok=false for every key until SG-2's
// contract_keys table exists (Resolver's documented degrade).
func (r *storeResolver) ResolveContractKey(string) (string, bool) { return "", false }

// symbolIndex answers resolveSymbolNow for every name from one read of
// the service's symbols. resolveSymbolNow matches the distinct symbols
// that equal the name, end in "."+name, or are a dotted suffix of the name
// (name ends in "."+symbol), and resolves only when exactly one matches,
// to the hash and file of that symbol's last row in SymbolHashes' order.
// exact holds each distinct symbol's last row; bySuffix counts, for each
// proper dotted suffix of a symbol ("b.C" and "C" of "a.b.C"), the
// distinct symbols ending in "."+it, so a name costs one probe for itself,
// one per dotted suffix and one for the symbols it is a suffix of.
type symbolIndex struct {
	exact    map[string]symbolHit
	bySuffix map[string]suffixHits
}

type symbolHit struct{ hash, file string }

type suffixHits struct {
	n     int
	first string // the symbol, when n == 1
}

func newSymbolIndex(rows []store.SymbolHash) *symbolIndex {
	x := &symbolIndex{exact: make(map[string]symbolHit, len(rows)), bySuffix: map[string]suffixHits{}}
	for _, row := range rows {
		_, seen := x.exact[row.Symbol]
		x.exact[row.Symbol] = symbolHit{hash: row.ContentHash, file: refFile(row.SourceRef)} // the last row wins
		if seen {
			continue
		}
		for i := 0; i < len(row.Symbol)-1; i++ {
			if row.Symbol[i] != '.' {
				continue
			}
			h := x.bySuffix[row.Symbol[i+1:]]
			if h.n == 0 {
				h.first = row.Symbol
			}
			h.n++
			x.bySuffix[row.Symbol[i+1:]] = h
		}
	}
	return x
}

// resolve is resolveSymbolNow(name) over the index; probes counts the map
// probes it made.
func (x *symbolIndex) resolve(name string, probes *int) TargetResolution {
	matched, only := 0, ""
	probe := func(sym string) {
		*probes++
		if _, ok := x.exact[sym]; ok {
			matched++
			only = sym
		}
	}
	probe(name) // the symbol equal to name
	for i := 0; i < len(name)-1; i++ {
		if name[i] == '.' {
			probe(name[i+1:]) // a symbol name ends in "."+symbol
		}
	}
	*probes++
	if h, ok := x.bySuffix[name]; ok { // symbols ending in "."+name
		matched += h.n
		only = h.first
	}
	if matched != 1 {
		return TargetResolution{}
	}
	h := x.exact[only]
	return TargetResolution{Exists: true, Hash: h.hash, File: h.file}
}
