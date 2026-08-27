package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// index.go — resolving an anchor against hashes fathom ALREADY maintains.
//
// Nothing here computes a new hash over source content: the file fingerprint
// (the ingest hash gate) and the observation rows are the whole substrate.
// That is what makes read-time drift evaluation affordable enough to run on
// every surfacing (§3.1).

// AnchorProbe is one anchor's current state in the index.
type AnchorProbe struct {
	// Hash is the anchor's current content hash. Empty when Exists is false.
	Hash string
	// Exists is false when the anchor no longer resolves at all — the
	// symbol was deleted, the file removed, the glob matches nothing. This
	// is the orphaning signal.
	Exists bool
	// Covers names, in a short phrase, WHAT the hash actually covers. Law 3
	// applied to granularity: fathom stores file-level content hashes, so a
	// symbol anchor's drift signal is honestly "the file holding it
	// changed", and the label must say so rather than claiming to have
	// diffed a function body.
	Covers string
	// Ambiguous carries the sorted fully-qualified candidates when a BARE
	// symbol name matches more than one package. The first implementation
	// picked "the first row in the store's deterministic order" — but
	// QueryObservations orders only by ingested_at, which a whole ingest
	// run shares, so the winner tracked INSERT order: two stores seeded
	// with the same observations resolved the same anchor differently,
	// and a per-file re-ingest could flip the winner and fabricate drift
	// naming a file the memory was never about. An ambiguous anchor now
	// refuses to bind rather than guessing (review finding W1).
	Ambiguous []string
}

// Index resolves anchors and reports the index generation they were resolved
// against. Implementations must be safe for concurrent use.
type Index interface {
	// Generation returns an opaque token that changes whenever the
	// service's indexed content could have changed. Drift evaluations are
	// cached per generation.
	Generation(ctx context.Context, service string) (string, error)
	// Probe returns the anchor's current state.
	Probe(ctx context.Context, service string, a Anchor) (AnchorProbe, error)
}

// Reader is the slice of internal/store.Store that StoreIndex needs.
// *store.SQLiteStore and the memory fake both satisfy it.
type Reader interface {
	QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error)
	Fingerprint(ctx context.Context, serviceID, sourceRef string) (hash string, found bool, err error)
	ListSourceRefs(ctx context.Context, serviceID string) ([]string, error)
	GetServiceMeta(ctx context.Context, serviceID string) (store.ServiceMeta, bool, error)
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
}

// StoreIndex resolves anchors against a fathom store.
//
// Concurrency: safe for concurrent use via sync.Mutex; the per-generation
// observation snapshot is loaded at most once per (service, generation).
type StoreIndex struct {
	r Reader

	mu   sync.Mutex
	gen  map[string]string
	snap map[string]symbolSnapshot // service -> resolved snapshot
}

// symbolSnapshot is one service's resolved symbol table: exact facts per
// name/key, plus the bare names that CANNOT bind because they collide
// across packages.
type symbolSnapshot struct {
	facts     map[string]symbolFacts
	ambiguous map[string][]string // bare name -> sorted qualified keys
}

type symbolFacts struct {
	sourceRef   string
	contentHash string
	signature   string
}

// NewStoreIndex returns an Index backed by r.
func NewStoreIndex(r Reader) *StoreIndex {
	return &StoreIndex{r: r, gen: map[string]string{}, snap: map[string]symbolSnapshot{}}
}

// Generation returns the service's indexed commit, falling back to the
// indexed-at timestamp when the corpus has no git identity, and to the
// explicit token "unindexed" when the service has no metadata at all.
//
// Law 3: "unindexed" is a named reason, not a fabricated empty generation
// that would silently share a cache slot with every other unindexed service.
func (s *StoreIndex) Generation(ctx context.Context, service string) (string, error) {
	meta, found, err := s.r.GetServiceMeta(ctx, service)
	if err != nil {
		return "", err
	}
	if !found {
		return "unindexed", nil
	}
	if meta.IndexedCommit != "" {
		// A non-git corpus records its watermark as "manifest:<hash>"
		// (internal/fusion's watermark), which already names its own
		// scheme — prefixing it again would produce the flatly untrue
		// "commit:manifest:…". Law 3 applies to labels too.
		if strings.Contains(meta.IndexedCommit, ":") {
			return meta.IndexedCommit, nil
		}
		return "commit:" + meta.IndexedCommit, nil
	}
	if meta.IndexedAt != 0 {
		return "indexed-at:" + strconv.FormatInt(meta.IndexedAt, 10), nil
	}
	return "unindexed", nil
}

// Probe resolves one anchor against the store.
func (s *StoreIndex) Probe(ctx context.Context, service string, a Anchor) (AnchorProbe, error) {
	switch a.Kind {
	case AnchorSymbol:
		return s.probeSymbol(ctx, service, a.Ref)
	case AnchorFile:
		return s.probeFile(ctx, service, a.Ref)
	case AnchorPath:
		return s.probePath(ctx, service, a.Ref)
	case AnchorCommit:
		// A commit is a historical fact: it neither drifts nor orphans.
		return AnchorProbe{Hash: "commit:" + a.Ref, Exists: true, Covers: "a commit sha, which is immutable"}, nil
	case AnchorClaim:
		return s.probeClaim(ctx, service, a.Ref)
	default:
		return AnchorProbe{}, nil
	}
}

// probeSymbol resolves a symbol anchor. The hash combines the symbol's
// signature with the content hash of the file that defines it — both already
// stored on every observation row.
//
// GRANULARITY, stated plainly because the design pre-registered it (§7): this
// is FILE granularity, not function-body granularity. Fathom stores one
// content hash per source file (the ingest hash gate), so an unrelated edit
// elsewhere in the same file marks the symbol suspect. That is a false
// positive in the noisy direction, never in the silent direction, and
// AnchorProbe.Covers says so at the point of use rather than letting a label
// claim more precision than exists.
func (s *StoreIndex) probeSymbol(ctx context.Context, service, ref string) (AnchorProbe, error) {
	snap, err := s.symbols(ctx, service)
	if err != nil {
		return AnchorProbe{}, err
	}
	if cands, amb := snap.ambiguous[ref]; amb {
		shown := cands
		if len(shown) > 4 {
			shown = append(append([]string{}, cands[:4]...), "…")
		}
		return AnchorProbe{
			Exists:    true,
			Ambiguous: cands,
			Covers: "the bare name " + ref + " matches " + strconv.Itoa(len(cands)) +
				" symbols (" + strings.Join(shown, ", ") + ") — it cannot bind; use a qualified key",
		}, nil
	}
	f, ok := snap.facts[ref]
	if !ok {
		return AnchorProbe{Exists: false, Covers: "no symbol by that name in the index"}, nil
	}
	return AnchorProbe{
		Hash:   hashParts("symbol", f.signature, f.contentHash),
		Exists: true,
		Covers: "the signature of " + ref + " and the content hash of " + f.sourceRef + " (file granularity)",
	}, nil
}

func (s *StoreIndex) probeFile(ctx context.Context, service, ref string) (AnchorProbe, error) {
	h, found, err := s.r.Fingerprint(ctx, service, ref)
	if err != nil {
		return AnchorProbe{}, err
	}
	if !found {
		return AnchorProbe{Exists: false, Covers: "no fingerprint for that file in the index"}, nil
	}
	return AnchorProbe{Hash: hashParts("file", h), Exists: true, Covers: "the content hash of " + ref}, nil
}

// probePath resolves a glob anchor over every source_ref the service knows,
// hashing the sorted (ref, fingerprint) pairs so that adding, removing or
// editing any matching file drifts the anchor.
func (s *StoreIndex) probePath(ctx context.Context, service, glob string) (AnchorProbe, error) {
	refs, err := s.r.ListSourceRefs(ctx, service)
	if err != nil {
		return AnchorProbe{}, err
	}
	var parts []string
	for _, ref := range refs {
		if !matchGlob(glob, ref) {
			continue
		}
		h, found, ferr := s.r.Fingerprint(ctx, service, ref)
		if ferr != nil {
			return AnchorProbe{}, ferr
		}
		if !found {
			continue
		}
		parts = append(parts, ref+"="+h)
	}
	if len(parts) == 0 {
		return AnchorProbe{Exists: false, Covers: "no indexed file matches " + glob}, nil
	}
	sort.Strings(parts)
	return AnchorProbe{
		Hash:   hashParts(append([]string{"path"}, parts...)...),
		Exists: true,
		Covers: "the content hashes of " + strconv.Itoa(len(parts)) + " files matching " + glob,
	}, nil
}

// probeClaim resolves a claim anchor: the anchored claim's STATUS is the
// hash, so a claim going stale, contradicted or refuted drifts every memory
// that leans on it.
func (s *StoreIndex) probeClaim(ctx context.Context, service, claimID string) (AnchorProbe, error) {
	claims, err := s.r.ClaimsForService(ctx, service, "")
	if err != nil {
		return AnchorProbe{}, err
	}
	for _, c := range claims {
		if c.ClaimID != claimID {
			continue
		}
		return AnchorProbe{
			Hash:   hashParts("claim", string(c.Status)),
			Exists: true,
			Covers: "the status of claim " + claimID,
		}, nil
	}
	return AnchorProbe{Exists: false, Covers: "no claim with that id in the index"}, nil
}

// symbols loads (and caches per generation) the service's symbol facts,
// indexed by both bare symbol name and fully-qualified SymbolKey so an anchor
// written either way resolves.
func (s *StoreIndex) symbols(ctx context.Context, service string) (symbolSnapshot, error) {
	gen, err := s.Generation(ctx, service)
	if err != nil {
		return symbolSnapshot{}, err
	}
	s.mu.Lock()
	if s.gen[service] == gen {
		if snap, ok := s.snap[service]; ok {
			s.mu.Unlock()
			return snap, nil
		}
	}
	s.mu.Unlock()

	obs, err := s.r.QueryObservations(ctx, service, "")
	if err != nil {
		return symbolSnapshot{}, err
	}
	snap := symbolSnapshot{
		facts:     make(map[string]symbolFacts, len(obs)*2),
		ambiguous: map[string][]string{},
	}
	// candidates: bare name -> qualified key set. Built first, decided
	// after, so the outcome cannot depend on the order rows arrive in —
	// QueryObservations orders only by ingested_at, which a whole ingest
	// run shares, so row order tracks insert order and is NOT a total
	// order (review finding W1: two stores with identical content resolved
	// the same bare anchor differently).
	candidates := map[string]map[string]symbolFacts{}
	for _, o := range obs {
		if o.Symbol == "" {
			continue
		}
		f := symbolFacts{sourceRef: o.SourceRef, contentHash: o.ContentHash, signature: o.Claim}
		key := core.SymbolKey(o.ServiceID, o.Package, o.Symbol)
		snap.facts[key] = f
		if candidates[o.Symbol] == nil {
			candidates[o.Symbol] = map[string]symbolFacts{}
		}
		candidates[o.Symbol][key] = f
	}
	for bare, byKey := range candidates {
		if _, taken := snap.facts[bare]; taken {
			// The bare string IS a qualified key for some symbol —
			// vanishingly unlikely, but the qualified meaning wins and
			// the bare form is not additionally bound.
			continue
		}
		if len(byKey) == 1 {
			for _, f := range byKey {
				snap.facts[bare] = f
			}
			continue
		}
		keys := make([]string, 0, len(byKey))
		for k := range byKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		snap.ambiguous[bare] = keys
	}

	s.mu.Lock()
	s.gen[service] = gen
	s.snap[service] = snap
	s.mu.Unlock()
	return snap, nil
}

// matchGlob matches ref against a glob. "**" spans directory separators;
// everything else is path.Match semantics.
func matchGlob(glob, ref string) bool { return pathglob.Match(glob, ref) }

// hashParts is the anchor hash constructor: sha256 over NUL-joined parts, so
// no part boundary can be forged by embedding the separator.
func hashParts(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}
