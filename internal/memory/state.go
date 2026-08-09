package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// state.go — the lifecycle, evaluated at READ time (§3, §3.1).
//
// There is no sweep and no timer. Three reasons, all load-bearing: drift
// cannot be missed while the daemon is down; it costs one hash comparison per
// anchor on a code path that already loads them; and a memory is never wrong
// merely because a scheduled job had not run yet.

// State is a memory's anchor-derived lifecycle state.
type State string

const (
	// StateFresh — every anchor's hash matches what it was at the last
	// confirmation. Surfaces normally.
	StateFresh State = "fresh"
	// StateSuspect — an anchor's content changed. Still surfaces, LABELED,
	// with its tier dropped one step: a suspect memory is usually still
	// true, and suppressing it silently would lose more than it saves.
	StateSuspect State = "suspect"
	// StateOrphaned — an anchor no longer exists. Stops surfacing and moves
	// to the review queue; never deleted, because the anchor's
	// disappearance may itself be the point.
	StateOrphaned State = "orphaned"
	// StateRetired — a review resolved the memory "no longer true". Never
	// surfaces again; still retrievable by id.
	StateRetired State = "retired"
)

// DriftStatus is one anchor's verdict within an evaluation.
type DriftStatus string

const (
	DriftUnchanged DriftStatus = "unchanged"
	DriftChanged   DriftStatus = "changed"
	DriftMissing   DriftStatus = "missing"
	// DriftUnknown marks an anchor with no recorded baseline hash — a
	// memory written before its anchor could be probed. Law 3: it is named,
	// never silently treated as unchanged.
	DriftUnknown DriftStatus = "unknown"
)

// AnchorDrift is one anchor's contribution to a memory's state.
type AnchorDrift struct {
	Anchor Anchor      `json:"anchor"`
	Status DriftStatus `json:"status"`
	// Was and Now are the baseline and current hashes. Now is empty when
	// the anchor is missing.
	Was string `json:"was,omitempty"`
	Now string `json:"now,omitempty"`
	// Covers is AnchorProbe.Covers — what the hash actually covers, so a
	// label never claims more precision than fathom has.
	Covers string `json:"covers,omitempty"`
	// Detail is the one-line "what changed and when" a reviewer reads.
	Detail string `json:"detail,omitempty"`
}

// Evaluation is a memory as it should be SURFACED right now: its state, the
// tier that state permits, and the label that must accompany it.
type Evaluation struct {
	ID      string `json:"id"`
	Service string `json:"service"`
	Claim   string `json:"claim"`
	State   State  `json:"state"`
	// BaseTier is the memory's stored tier; Tier is the effective tier
	// after the suspect penalty. They differ exactly when State is
	// suspect.
	BaseTier core.EvidenceTier `json:"base_tier"`
	Tier     core.EvidenceTier `json:"tier"`
	// Confidence is re-capped against the effective tier.
	Confidence int `json:"confidence"`
	// Generation is the index generation this evaluation was computed
	// against — the cache key, surfaced so a stale answer is attributable.
	Generation string `json:"generation"`
	// Drift is per-anchor, sorted by anchor key.
	Drift []AnchorDrift `json:"drift"`
	// Surfaces is false for orphaned and retired memories.
	Surfaces bool `json:"surfaces"`
	// Specificity is the sharpest anchor's rank — the second sort key
	// after tier when several memories are surfaced together.
	Specificity     int       `json:"specificity"`
	LastConfirmedAt time.Time `json:"last_confirmed_at"`
}

// Line renders the memory for surfacing. It ALWAYS carries tier and state:
// an unlabelled memory is a claim fathom would be making on its own
// authority, and fathom does not have that authority (§9).
func (e Evaluation) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s · %s", e.Tier, e.State)
	if e.State == StateSuspect {
		b.WriteString(" ⚠")
	}
	b.WriteString("] ")
	b.WriteString(e.Claim)
	if !e.LastConfirmedAt.IsZero() {
		fmt.Fprintf(&b, " (confirmed %s)", e.LastConfirmedAt.UTC().Format(time.RFC3339))
	}
	for _, d := range e.Drift {
		if d.Status == DriftUnchanged {
			continue
		}
		b.WriteString(" — ")
		b.WriteString(d.Detail)
	}
	return b.String()
}

// DriftCache memoizes anchor probes per index generation, so a session that
// surfaces the same memory forty times hashes its anchors once (§3.1). The
// cache for a service is dropped wholesale the moment its generation changes
// — a per-entry expiry would let one stale anchor outlive a re-ingest.
//
// Concurrency: safe for concurrent use via sync.Mutex.
type DriftCache struct {
	mu     sync.Mutex
	gen    map[string]string
	probes map[string]AnchorProbe
}

// NewDriftCache returns an empty cache.
func NewDriftCache() *DriftCache {
	return &DriftCache{gen: map[string]string{}, probes: map[string]AnchorProbe{}}
}

func cacheKey(service, generation, anchorKey string) string {
	return service + "\x00" + generation + "\x00" + anchorKey
}

// probe returns the cached probe for (service, generation, anchor), calling
// idx.Probe at most once per generation.
func (c *DriftCache) probe(ctx context.Context, idx Index, service, generation string, a Anchor) (AnchorProbe, error) {
	key := cacheKey(service, generation, a.Key())

	c.mu.Lock()
	if c.gen[service] != generation {
		// Generation moved: every probe for this service is stale.
		for k := range c.probes {
			if strings.HasPrefix(k, service+"\x00") {
				delete(c.probes, k)
			}
		}
		c.gen[service] = generation
	}
	p, hit := c.probes[key]
	c.mu.Unlock()
	if hit {
		return p, nil
	}

	p, err := idx.Probe(ctx, service, a)
	if err != nil {
		return AnchorProbe{}, err
	}

	c.mu.Lock()
	// Re-check the generation: a concurrent evaluation may have rolled it
	// forward while this probe was in flight, and caching against a
	// superseded generation would resurrect a stale answer.
	if c.gen[service] == generation {
		c.probes[key] = p
	}
	c.mu.Unlock()
	return p, nil
}

// EvaluateState computes a memory's current state from its anchors, at read
// time, against the index generation now in effect.
//
// cache may be nil, in which case every anchor is probed afresh — correct but
// unmemoized. now must be non-nil.
func EvaluateState(ctx context.Context, idx Index, cache *DriftCache, now func() time.Time, m Memory) (Evaluation, error) {
	if idx == nil {
		return Evaluation{}, errors.New("memory: EvaluateState requires an index")
	}
	if now == nil {
		return Evaluation{}, errors.New("memory: EvaluateState requires an injected clock (now func() time.Time)")
	}

	ev := Evaluation{
		ID:              m.ID,
		Service:         m.Service,
		Claim:           m.Claim,
		BaseTier:        m.Tier,
		Tier:            m.Tier,
		Confidence:      m.Confidence,
		Specificity:     m.Specificity(),
		LastConfirmedAt: m.LastConfirmedAt,
	}

	// A retired memory is retired regardless of what its anchors are doing:
	// the judgment was "no longer true", not "the code moved".
	if m.Retired() {
		ev.State = StateRetired
		ev.Surfaces = false
		return ev, nil
	}

	generation, err := idx.Generation(ctx, m.Service)
	if err != nil {
		return Evaluation{}, fmt.Errorf("index generation for %s: %w", m.Service, err)
	}
	ev.Generation = generation

	anyMissing, anyChanged := false, false
	for _, a := range m.Anchors {
		var p AnchorProbe
		if cache != nil {
			p, err = cache.probe(ctx, idx, m.Service, generation, a)
		} else {
			p, err = idx.Probe(ctx, m.Service, a)
		}
		if err != nil {
			return Evaluation{}, fmt.Errorf("probe anchor %s: %w", a.Key(), err)
		}

		d := AnchorDrift{Anchor: a, Was: m.AnchorHashes[a.Key()], Now: p.Hash, Covers: p.Covers}
		switch {
		case !p.Exists:
			anyMissing = true
			d.Status = DriftMissing
			d.Now = ""
			d.Detail = fmt.Sprintf("%s no longer exists in the index (%s); last confirmed %s",
				a.Key(), p.Covers, confirmedPhrase(m))
		case d.Was == "":
			d.Status = DriftUnknown
			d.Detail = fmt.Sprintf("%s has no recorded baseline hash, so drift cannot be judged for it", a.Key())
		case d.Was != d.Now:
			anyChanged = true
			d.Status = DriftChanged
			d.Detail = fmt.Sprintf("%s changed since this was confirmed (%s; covers %s; index now at %s)",
				a.Key(), confirmedPhrase(m), p.Covers, ShortWatermark(generation))
		default:
			d.Status = DriftUnchanged
		}
		ev.Drift = append(ev.Drift, d)
	}
	sort.Slice(ev.Drift, func(i, j int) bool { return ev.Drift[i].Anchor.Key() < ev.Drift[j].Anchor.Key() })

	switch {
	case anyMissing:
		ev.State = StateOrphaned
		ev.Surfaces = false
	case anyChanged:
		ev.State = StateSuspect
		ev.Surfaces = true
		// Law 14 while suspect: the machine check that verified this ran
		// against different code, so it is not verified any more.
		ev.Tier = dropOne(m.Tier)
		ev.Confidence = capConfidence(m.Confidence, ev.Tier)
	default:
		ev.State = StateFresh
		ev.Surfaces = true
	}
	return ev, nil
}

// confirmedPhrase renders when the memory was last confirmed and by whom —
// the "and when" half of the suspect label.
func confirmedPhrase(m Memory) string {
	if m.LastConfirmedAt.IsZero() {
		return "never confirmed"
	}
	s := "confirmed " + m.LastConfirmedAt.UTC().Format(time.RFC3339)
	if m.ConfirmedBy.Kind != "" {
		s += " by " + m.ConfirmedBy.String()
	}
	if m.ConfirmedAtCommit != "" {
		s += " against " + ShortWatermark(m.ConfirmedAtCommit)
	}
	return s
}

// ShortWatermark abbreviates a commit sha or manifest hash for a label,
// keeping any scheme prefix intact. Evaluation.Generation always carries the
// full value; this is a display truncation and nothing reads it back.
func ShortWatermark(w string) string {
	scheme, hash, found := strings.Cut(w, ":")
	if !found {
		scheme, hash = "", w
	}
	if len(hash) > 12 {
		hash = hash[:12] + "…"
	}
	if scheme == "" {
		return hash
	}
	return scheme + ":" + hash
}

// EvaluateAll evaluates several memories against ONE index-generation lookup
// and one shared cache, then returns them in the deterministic surfacing
// order (tier, then specificity, then id). This is the shape the obligations
// injection path consumes.
func EvaluateAll(ctx context.Context, idx Index, cache *DriftCache, now func() time.Time, ms []Memory) ([]Evaluation, error) {
	out := make([]Evaluation, 0, len(ms))
	for _, m := range ms {
		ev, err := EvaluateState(ctx, idx, cache, now, m)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	SortEvaluations(out)
	return out, nil
}

// SortEvaluations orders evaluations deterministically: strongest tier first,
// then sharpest anchor, then id. Ties are broken by id, which is
// content-addressed, so the order is stable across processes and machines.
func SortEvaluations(evs []Evaluation) {
	sort.SliceStable(evs, func(i, j int) bool {
		a, b := evs[i], evs[j]
		if ta, tb := ladderIndex(a.Tier), ladderIndex(b.Tier); ta != tb {
			return ta < tb
		}
		if a.Specificity != b.Specificity {
			return a.Specificity < b.Specificity
		}
		return a.ID < b.ID
	})
}

// Surfacing filters evaluations down to the ones that may appear in a
// payload, preserving order. Orphaned and retired memories are dropped.
func Surfacing(evs []Evaluation) []Evaluation {
	out := make([]Evaluation, 0, len(evs))
	for _, e := range evs {
		if e.Surfaces {
			out = append(out, e)
		}
	}
	return out
}
