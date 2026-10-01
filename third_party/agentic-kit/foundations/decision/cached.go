package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"maps"
	"slices"
	"strconv"
	"sync"
)

// CacheKey identifies one cached judgment: the question template and the
// content it was asked about. See CacheKeyFor.
type CacheKey struct {
	// Template is TemplateDigest of the question and the caller's template
	// version.
	Template string
	// Content is ContentHash of the Request's State.
	Content string
}

// CacheEntry is one cached answer, stamped with the Provider and Model of
// the Response that produced it.
type CacheEntry struct {
	// Answer is the cached answer; its ID is the question ID it was first
	// asked under, and Cached re-keys it to the asking question on a hit.
	Answer Answer
	// Provider is the producing Response's Provider.
	Provider string
	// Model is the producing Response's Model.
	Model string
}

// CacheStore is the storage a Cached decorator reads and writes, injected
// by the caller. MemoryCacheStore is the in-process implementation; a
// durable one waits for the reference consumer's RT-M1 reading (decision
// 0017, fast-judgment.md). Implementations must be safe for concurrent use.
type CacheStore interface {
	// Get returns the entry under key and whether there is one. An error is
	// disclosed by Cached and read as a miss.
	Get(ctx context.Context, key CacheKey) (CacheEntry, bool, error)
	// Put stores entry under key, replacing any entry there. An error is
	// disclosed by Cached and never fails the Decide call.
	Put(ctx context.Context, key CacheKey, entry CacheEntry) error
}

// MemoryCacheStore is an in-process CacheStore: a map guarded by a mutex,
// with no eviction, no expiry, no file and no background work. It lives
// as long as the process.
//
// Concurrency: MemoryCacheStore is safe for concurrent use.
type MemoryCacheStore struct {
	mu      sync.Mutex
	entries map[CacheKey]CacheEntry
}

// NewMemoryCacheStore returns an empty MemoryCacheStore.
func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{entries: map[CacheKey]CacheEntry{}}
}

// Get returns a copy of the entry under key. It never fails.
func (s *MemoryCacheStore) Get(_ context.Context, key CacheKey) (CacheEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if ok {
		e.Answer = cloneAnswer(e.Answer)
	}
	return e, ok, nil
}

// Put stores a copy of entry under key. It never fails.
func (s *MemoryCacheStore) Put(_ context.Context, key CacheKey, entry CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.Answer = cloneAnswer(entry.Answer)
	s.entries[key] = entry
	return nil
}

// Keys returns every key held, sorted by Template then Content.
func (s *MemoryCacheStore) Keys() []CacheKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := slices.Collect(maps.Keys(s.entries))
	slices.SortFunc(keys, func(a, b CacheKey) int {
		if a.Template != b.Template {
			if a.Template < b.Template {
				return -1
			}
			return 1
		}
		if a.Content < b.Content {
			return -1
		}
		if a.Content > b.Content {
			return 1
		}
		return 0
	})
	return keys
}

// Len returns the number of entries held.
func (s *MemoryCacheStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// TemplateDigest returns "sha256:" and the hex SHA-256 of a canonical
// encoding of q's template: every field except ID (Kind with the alias
// "noul" folded into KindPredicate, Instructions, Options, Min, Max,
// Rubric, Levels, TrueCriteria and FalseCriteria) and the caller's
// template version, which a caller bumps to re-grade a question whose
// wording it changed elsewhere. Two questions differing only in ID share a
// digest.
func TemplateDigest(q Question, version string) string {
	h := sha256.New()
	writeField(h, "decision.template/1")
	writeField(h, version)
	writeTemplate(h, q)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// ContentHash returns "sha256:" and the hex SHA-256 of state.
func ContentHash(state string) string {
	sum := sha256.Sum256([]byte(state))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CacheKeyFor returns the CacheKey of question q asked about state under
// template version version.
func CacheKeyFor(q Question, state, version string) CacheKey {
	return CacheKey{Template: TemplateDigest(q, version), Content: ContentHash(state)}
}

// writeField writes s to h length-prefixed, so no two field sequences
// share an encoding.
func writeField(h hash.Hash, s string) {
	h.Write([]byte(strconv.Itoa(len(s))))
	h.Write([]byte{':'})
	h.Write([]byte(s))
}

// writeFloat writes v to h in the shortest form that round-trips.
func writeFloat(h hash.Hash, v float64) {
	writeField(h, strconv.FormatFloat(v, 'g', -1, 64))
}

// writeTemplate writes every field of q except ID to h, canonically.
func writeTemplate(h hash.Hash, q Question) {
	writeField(h, string(canonicalKind(q.Kind)))
	writeField(h, q.Instructions)
	writeField(h, strconv.Itoa(len(q.Options)))
	for _, o := range q.Options {
		writeField(h, o.Name)
		writeField(h, o.Criteria)
	}
	writeFloat(h, q.Min)
	writeFloat(h, q.Max)
	writeField(h, q.Rubric)
	writeField(h, strconv.Itoa(len(q.Levels)))
	for _, l := range q.Levels {
		writeField(h, l)
	}
	writeField(h, q.TrueCriteria)
	writeField(h, q.FalseCriteria)
}

// CachedOptions configures NewCached. It is plain data apart from Store,
// so a composer can build it from configuration.
type CachedOptions struct {
	// Name is what the Cached's Name returns and the Provider of a Response
	// whose answers do not share one Provider and Model. Empty means
	// "cached".
	Name string
	// Store holds the entries. Required.
	Store CacheStore
	// TemplateVersion is the caller's question-template version, part of
	// every key (see TemplateDigest). It may be empty.
	TemplateVersion string
	// Observe, when set, receives the report of every Decide call. It may
	// be called concurrently from concurrent Decide calls.
	Observe func(CachedReport)
}

// CachedReport discloses one Cached call: which answers came from the
// store and which from the provider.
type CachedReport struct {
	// Hits lists, in request order, the questions served from the store.
	Hits []string
	// Misses lists, in request order, the questions sent to the inner
	// Decider in one call.
	Misses []string
	// Regraded lists, in request order, the questions that hit an entry
	// stamped with the previous Provider and Model and were re-asked
	// because this call's miss revealed a new one.
	Regraded []string
	// Sources maps every question ID in the returned Response's Answers to
	// where its answer came from; Cached is true for a hit.
	Sources map[string]AnswerSource
	// Provider and Model are those of the last inner call, empty when every
	// question hit.
	Provider, Model string
	// StoreErrors lists, in order, every Get and Put failure: a failed Get
	// reads as a miss and a failed Put leaves the answer uncached.
	StoreErrors []string
}

// Fresh returns resp, a Response this report describes, with only the
// answers the provider produced during the call: the hits are removed, so
// a caller that logs Records or feeds a calibration reading never counts a
// cached judgment as a new one. Invalid, Usage and Latency are resp's
// (hits contribute none). Its Provider and Model are those the fresh
// answers share (resp's when they do not share one; the report's when
// there are none).
func (r CachedReport) Fresh(resp Response) Response {
	out := Response{Provider: r.Provider, Model: r.Model, Answers: map[string]Answer{}, Invalid: maps.Clone(resp.Invalid), Usage: resp.Usage, Latency: resp.Latency}
	fresh := map[string]AnswerSource{}
	for id, a := range resp.Answers {
		if src, ok := r.Sources[id]; ok && !src.Cached {
			out.Answers[id] = a
			fresh[id] = src
		}
	}
	if len(fresh) > 0 {
		if p, m, ok := sharedSource(fresh); ok {
			out.Provider, out.Model = p, m
		} else {
			out.Provider, out.Model = resp.Provider, resp.Model
		}
	}
	if out.Invalid == nil {
		out.Invalid = map[string]string{}
	}
	return out
}

// cacheStamp is the Provider and Model a cache entry is stamped with.
type cacheStamp struct {
	provider, model string
}

// Cached is a Decider that serves repeated judgments from an injected
// CacheStore, keyed by (question template, content hash) — see CacheKeyFor
// — and stamped with the Provider and Model that produced them, so a
// better model re-grades cleanly. Per question, a hit is served and the
// misses are batched into one inner call; only valid answers are stored.
//
// Stamps: the current stamp is the Provider and Model of the inner
// Decider's most recent Response, and an entry stamped otherwise is a
// miss. A new Cached starts with no stamp, so its first call asks every
// question. When a miss reveals a new stamp (a rolling model alias moved),
// the questions that hit under the old stamp in the same call are re-asked
// (CachedReport.Regraded), so a Response never mixes models; should the
// re-ask reveal yet another stamp, the Response carries the Cached's Name
// and an empty Model and CachedReport.Sources says who answered each.
//
// Apart from proof: hits are disclosed in CachedReport (Hits, Sources, and
// Fresh, which drops them), which DecideReport returns and
// CachedOptions.Observe receives. Cached writes no Records, ledger, file
// or table: the in-process store is the only state, per the TABLE FREEZE
// until the reference consumer's RT-M1 reading. There is no expiry; a
// caller bumps CachedOptions.TemplateVersion or builds a new store.
//
// Concurrency: Cached guards its stamp with a mutex and is safe for
// concurrent use when its inner Decider and Store are.
type Cached struct {
	name    string
	inner   Decider
	store   CacheStore
	version string
	observe func(CachedReport)

	mu    sync.Mutex
	stamp cacheStamp
	known bool
}

// NewCached returns a Cached over inner. It fails when inner or
// opts.Store is nil.
func NewCached(inner Decider, opts CachedOptions) (*Cached, error) {
	if isNilDecider(inner) {
		return nil, errors.New("decision: NewCached: inner Decider is nil")
	}
	if opts.Store == nil {
		return nil, errors.New("decision: NewCached: Store is required")
	}
	name := opts.Name
	if name == "" {
		name = "cached"
	}
	return &Cached{name: name, inner: inner, store: opts.Store, version: opts.TemplateVersion, observe: opts.Observe}, nil
}

// Name returns CachedOptions.Name, or "cached".
func (c *Cached) Name() string { return c.name }

// Capabilities declares the inner Decider's Capabilities unchanged: a miss
// costs what the inner Decider costs. It makes no call.
func (c *Cached) Capabilities() Capabilities { return CapabilitiesOf(c.inner) }

// Decide is DecideReport without the report (Observe still receives it).
func (c *Cached) Decide(ctx context.Context, req Request) (Response, error) {
	resp, _, err := c.DecideReport(ctx, req)
	return resp, err
}

// DecideReport answers req from the store and the inner Decider (see
// Cached) and returns the merged Response with its CachedReport. It fails
// when req fails Validate (the store and the inner Decider are not
// touched) or the inner Decider returns an error (that error, wrapped;
// nothing is stored).
func (c *Cached) DecideReport(ctx context.Context, req Request) (Response, CachedReport, error) {
	resp, report, err := c.decide(ctx, req)
	if c.observe != nil {
		c.observe(report)
	}
	return resp, report, err
}

// currentStamp returns the current stamp and whether there is one.
func (c *Cached) currentStamp() (cacheStamp, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stamp, c.known
}

// setStamp makes s the current stamp.
func (c *Cached) setStamp(s cacheStamp) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stamp, c.known = s, true
}

// decide is DecideReport before Observe.
func (c *Cached) decide(ctx context.Context, req Request) (Response, CachedReport, error) {
	report := CachedReport{Sources: map[string]AnswerSource{}}
	if err := Validate(req); err != nil {
		return Response{}, report, err
	}
	cur, known := c.currentStamp()
	content := ContentHash(req.State)
	keys := make(map[string]CacheKey, len(req.Questions))
	answers := map[string]Answer{}
	invalid := map[string]string{}
	var hits, misses []Question
	for _, q := range req.Questions {
		key := CacheKey{Template: TemplateDigest(q, c.version), Content: content}
		keys[q.ID] = key
		if known {
			e, ok, err := c.store.Get(ctx, key)
			switch {
			case err != nil:
				report.StoreErrors = append(report.StoreErrors, fmt.Sprintf("get %s: %v", q.ID, err))
			case ok && e.Provider == cur.provider && e.Model == cur.model:
				a := cloneAnswer(e.Answer)
				a.ID = q.ID
				answers[q.ID] = a
				report.Sources[q.ID] = AnswerSource{Provider: e.Provider, Model: e.Model, Cached: true}
				hits = append(hits, q)
				continue
			}
		}
		misses = append(misses, q)
	}

	merged := Response{}
	ask := func(qs []Question) error {
		resp, err := c.inner.Decide(ctx, Request{State: req.State, Questions: qs})
		if err != nil {
			return fmt.Errorf("decision: cached: %s: %w", c.inner.Name(), err)
		}
		c.setStamp(cacheStamp{provider: resp.Provider, model: resp.Model})
		report.Provider, report.Model = resp.Provider, resp.Model
		merged.Usage.InputTokens += resp.Usage.InputTokens
		merged.Usage.OutputTokens += resp.Usage.OutputTokens
		merged.Latency += resp.Latency
		for _, q := range qs {
			a, ok := resp.Answers[q.ID]
			if !ok {
				why, named := resp.Invalid[q.ID]
				if !named {
					why = "no answer"
				}
				invalid[q.ID] = why
				continue
			}
			answers[q.ID] = a
			report.Sources[q.ID] = AnswerSource{Provider: resp.Provider, Model: resp.Model}
			entry := CacheEntry{Answer: cloneAnswer(a), Provider: resp.Provider, Model: resp.Model}
			if err := c.store.Put(ctx, keys[q.ID], entry); err != nil {
				report.StoreErrors = append(report.StoreErrors, fmt.Sprintf("put %s: %v", q.ID, err))
			}
		}
		return nil
	}

	if len(misses) > 0 {
		report.Misses = questionIDs(misses)
		if err := ask(misses); err != nil {
			return Response{}, report, err
		}
		if len(hits) > 0 && (report.Provider != cur.provider || report.Model != cur.model) {
			report.Regraded = questionIDs(hits)
			for _, q := range hits {
				delete(answers, q.ID)
				delete(report.Sources, q.ID)
			}
			if err := ask(hits); err != nil {
				return Response{}, report, err
			}
			hits = nil
		}
	}
	report.Hits = questionIDs(hits)

	merged.Answers = answers
	merged.Invalid = invalid
	switch {
	case len(report.Sources) > 0:
		merged.Provider, merged.Model = labelBySources(report.Sources, c.name)
	case len(misses) > 0:
		merged.Provider, merged.Model = report.Provider, report.Model
	default:
		merged.Provider = c.name
	}
	return merged, report, nil
}

// cloneAnswer returns a with its own Probabilities map.
func cloneAnswer(a Answer) Answer {
	a.Probabilities = maps.Clone(a.Probabilities)
	return a
}
