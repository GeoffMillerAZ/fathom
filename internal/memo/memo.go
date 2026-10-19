// Package memo is fathom's one byte-budgeted, single-flighted cache
// registry (docs/design/responsible-tenant.md §5, invariant I51, ADR-047).
//
// Every derived in-memory cache in the serving daemon is a Group: a build
// for a key runs once no matter how many callers ask for it at the same
// time (single flight), a waiter leaves at its own context's end instead of
// waiting out someone else's build, and the build itself is cancelled when
// its last waiter leaves (unless the group marks flights Keep and the build
// is past half its expected cost). All groups share one byte budget — the
// cache fraction of the tenancy budget — and one LRU across groups; a
// watermark-scoped group never caches under an empty watermark (I21) and a
// service's entries are evicted, in every group, the moment a newer
// watermark for that service is seen or EvictService names it.
//
// The ladder acts on the registry, never on individual caches: at shed the
// registry trims by LRU to ShedTrim (50%) of the cache budget, keeps its
// resident bytes under that while shed, and admits one rebuild flight
// process-wide (Trim); refuse and idle drop everything (Drop). Dropping
// every cache at shed would make each following request rebuild the
// snapshot and graphs and push the footprint straight back into shed.
//
// I21's "no whole tool answer is cached" stands: no group caches an MCP
// tool's response.
package memo

import (
	"container/list"
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// DropReason names why entries left the registry; Stats counts evictions by
// it.
type DropReason string

const (
	// ReasonBudget is an LRU eviction to stay under the byte budget.
	ReasonBudget DropReason = "budget"
	// ReasonShed is the shed rung's LRU trim (Trim).
	ReasonShed DropReason = "shed"
	// ReasonRefuse is the refuse rung's drop (Drop).
	ReasonRefuse DropReason = "refuse"
	// ReasonIdle is the idle drop after IdleDrop without tickets.
	ReasonIdle DropReason = "idle"
	// ReasonWatermark is a service's entries leaving on a newer watermark.
	ReasonWatermark DropReason = "watermark"
	// ReasonMaxEntries is a group's own entry cap.
	ReasonMaxEntries DropReason = "max_entries"
)

// DefaultShedTrim is the fraction of the cache budget the shed rung trims
// to (§5, [tenancy.caches] shed_trim).
const DefaultShedTrim = 0.5

// Scoped is implemented by keys that belong to one service at one store
// watermark. A WatermarkScoped group requires it: a key whose watermark is
// empty is built but never cached (I21), and a key naming a newer
// watermark than the registry last saw for its service evicts that
// service's older entries in every group.
type Scoped interface {
	MemoScope() (service, watermark string)
}

// Options configure a group.
type Options struct {
	// MaxEntries caps the group's entries (0 = only the byte budget).
	MaxEntries int
	// WatermarkScoped groups key by (service, watermark): see Scoped.
	WatermarkScoped bool
	// Priority orders eviction ties: a lower priority's entries go first
	// among entries equally old. Reserved; the LRU ignores it today.
	Priority int
	// Keep marks the group's flights as worth finishing: once a build has
	// run for at least half ExpectedCost, its last waiter leaving does not
	// cancel it, and the result is cached for the next caller. Keep with no
	// ExpectedCost keeps nothing: a flight is held past its callers only on
	// a stated cost. The refuse drop cancels kept flights too.
	Keep         bool
	ExpectedCost time.Duration
}

// GroupStats is one group's counters.
type GroupStats struct {
	Name    string
	Entries int
	Bytes   int64
	Hits    int64
	Misses  int64
	Flights int64
	// Abandoned counts flights cancelled because their last waiter left
	// (a build nobody would read); RefuseCancelled counts flights a
	// refuse drop ended — a floor refuse is not a doomed build.
	Abandoned       int64
	RefuseCancelled int64
	// Declined counts Gets that started no flight because their deadline
	// could not hold it (ErrWouldNotFinish).
	Declined  int64
	Uncached  int64 // builds not cached: empty watermark, oversize, or failed
	Evictions map[DropReason]int64
}

// Registry is the shared byte budget and LRU. The package-level Default is
// the daemon's; tests build their own with NewRegistry.
type Registry struct {
	mu       sync.Mutex
	budget   int64 // cache budget in bytes; 0 = unlimited
	shedTrim float64
	total    int64
	lru      *list.List // front = most recent; values are *entryRef
	groups   []groupHandle
	marks    map[string]string // service → newest watermark seen
	level    func() tenancy.Level
	rebuild  chan struct{} // one rebuild flight process-wide while shed
	now      func() time.Time
}

// groupHandle is what the registry needs from a group without its type
// parameters.
type groupHandle interface {
	name() string
	evictLocked(e *entryRef, reason DropReason)
	cancelFlightsLocked()
	stats() GroupStats
}

// entryRef is one cached value's registry bookkeeping.
type entryRef struct {
	g       groupHandle
	key     any
	service string
	mark    string
	bytes   int64
	elem    *list.Element
}

// NewRegistry returns an empty registry with no byte limit; the level
// source is the published tenancy ladder level.
func NewRegistry() *Registry {
	return &Registry{
		shedTrim: DefaultShedTrim,
		lru:      list.New(),
		marks:    make(map[string]string),
		level:    tenancy.CurrentLevel,
		rebuild:  make(chan struct{}, 1),
		now:      time.Now,
	}
}

// Default is the daemon's registry.
var Default = NewRegistry()

// SetBudget sets the cache byte budget (0 = unlimited) and the shed trim
// fraction (0 = DefaultShedTrim), evicting by LRU at once if the resident
// bytes are over it.
func (r *Registry) SetBudget(bytes int64, shedTrim float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.budget = bytes
	if shedTrim <= 0 || shedTrim > 1 {
		shedTrim = DefaultShedTrim
	}
	r.shedTrim = shedTrim
	r.fitLocked(ReasonBudget)
}

// SetLevelSource replaces the ladder level source (tests).
func (r *Registry) SetLevelSource(f func() tenancy.Level) {
	r.mu.Lock()
	r.level = f
	r.mu.Unlock()
}

// Budget reports the cache budget and the resident bytes.
func (r *Registry) Budget() (budget, resident int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.budget, r.total
}

// limitLocked is the byte limit in force: the budget, or ShedTrim of it
// while the ladder is at shed or above — so the registry cannot refill to
// the footprint that caused the shed while it lasts.
func (r *Registry) limitLocked() int64 {
	if r.budget <= 0 {
		return 0
	}
	if r.level != nil && r.level() >= tenancy.LevelShed {
		return int64(float64(r.budget) * r.shedTrim)
	}
	return r.budget
}

// fitLocked evicts least-recently-used entries until the resident bytes
// fit the limit in force.
func (r *Registry) fitLocked(reason DropReason) {
	limit := r.limitLocked()
	if limit <= 0 {
		return
	}
	r.trimToLocked(limit, reason)
}

func (r *Registry) trimToLocked(limit int64, reason DropReason) {
	for r.total > limit {
		back := r.lru.Back()
		if back == nil {
			return
		}
		e := back.Value.(*entryRef)
		e.g.evictLocked(e, reason)
		r.removeLocked(e)
	}
}

func (r *Registry) removeLocked(e *entryRef) {
	if e.elem != nil {
		r.lru.Remove(e.elem)
		e.elem = nil
		r.total -= e.bytes
	}
}

// noteWatermarkLocked records service's newest watermark and evicts its
// entries at any other watermark, in every group. Reports whether mark is
// the newest (a key at an older mark is built but not cached).
func (r *Registry) noteWatermarkLocked(service, mark string) {
	if prev, ok := r.marks[service]; ok && prev == mark {
		return
	}
	r.marks[service] = mark
	r.evictServiceLocked(service, mark)
}

func (r *Registry) evictServiceLocked(service, keepMark string) {
	var victims []*entryRef
	for el := r.lru.Front(); el != nil; el = el.Next() {
		e := el.Value.(*entryRef)
		if e.service == service && (keepMark == "" || e.mark != keepMark) {
			victims = append(victims, e)
		}
	}
	for _, e := range victims {
		e.g.evictLocked(e, ReasonWatermark)
		r.removeLocked(e)
	}
}

// EvictService drops every entry of service in every group (the refresh
// path's WatermarkBumped): the next reader rebuilds at the new watermark.
func (r *Registry) EvictService(service string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.marks, service)
	r.evictServiceLocked(service, "")
}

// Trim is the shed rung: evict by LRU to fraction × the cache budget
// (fraction ≤ 0 uses the registry's ShedTrim). With no budget set it trims
// nothing.
func (r *Registry) Trim(reason DropReason, fraction float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.budget <= 0 {
		return
	}
	if fraction <= 0 {
		fraction = r.shedTrim
	}
	r.trimToLocked(int64(float64(r.budget)*fraction), reason)
}

// Drop evicts every entry in every group (refuse, idle). Refuse also
// cancels every build in flight: a flight's context is detached from its
// callers' (so a Keep flight can outlive them), so cancelling the
// callers' tickets alone would let the build run on and refill the cache.
func (r *Registry) Drop(reason DropReason) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reason == ReasonRefuse {
		for _, g := range r.groups {
			g.cancelFlightsLocked()
		}
	}
	r.trimToLocked(0, reason)
	for el := r.lru.Front(); el != nil; el = r.lru.Front() {
		e := el.Value.(*entryRef)
		e.g.evictLocked(e, reason)
		r.removeLocked(e)
	}
}

// Stats is every registered group's counters, sorted by name.
func (r *Registry) Stats() []GroupStats {
	r.mu.Lock()
	groups := append([]groupHandle(nil), r.groups...)
	r.mu.Unlock()
	out := make([]GroupStats, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.stats())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Trim trims the Default registry (the shed rung).
func Trim(reason DropReason, fraction float64) { Default.Trim(reason, fraction) }

// Drop drops the Default registry (refuse, idle).
func Drop(reason DropReason) { Default.Drop(reason) }

// EvictService evicts service from the Default registry.
func EvictService(service string) { Default.EvictService(service) }

// Stats reports the Default registry's groups.
func Stats() []GroupStats { return Default.Stats() }

// shed reports whether the ladder is at shed or above.
func (r *Registry) shed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.level != nil && r.level() >= tenancy.LevelShed
}

// acquireRebuild takes the one rebuild token (Get takes it only while
// shed), waiting no longer than ctx. release is never nil.
func (r *Registry) acquireRebuild(ctx context.Context) (release func(), err error) {
	select {
	case r.rebuild <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-r.rebuild }) }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

// Group is one named cache of V by K.
type Group[K comparable, V any] struct {
	reg  *Registry
	nm   string
	opts Options

	mu       sync.Mutex
	entries  map[K]*entry[V]
	flights  map[K]*flight[V]
	counters GroupStats
	onEvict  func(K, V) // SetOnEvict; nil for heap-only values
	// lastBuild is each service's last completed build (wouldAbandon): a
	// measurement in place of ExpectedCost, not a cache — it holds no
	// derived data and serves no reader.
	lastBuild map[string]buildRecord
}

// buildRecord is one completed build's duration and size.
type buildRecord struct {
	took time.Duration
	size int64
}

// SetOnEvict registers fn to be told of every value that leaves the cache
// (an LRU, shed or refuse eviction, a watermark eviction, Reset, Remove, a
// replacement), once per value, so a group whose values hold resources
// outside the Go heap — compiled grammar code (LB-F15) — can release them.
// fn runs with the registry and group locks held: it must be quick and must
// not call back into memo. Set it before the group is used.
func (g *Group[K, V]) SetOnEvict(fn func(K, V)) {
	g.mu.Lock()
	g.onEvict = fn
	g.mu.Unlock()
}

type entry[V any] struct {
	val V
	ref *entryRef
	gen uint64 // bumped by each Update commit (Update's compare-and-swap)
}

type flight[V any] struct {
	done    chan struct{}
	val     V
	err     error
	waiters int
	cancel  context.CancelFunc
	started time.Time
	// refused is set when a refuse drop cancelled the flight: a waiter
	// leaving afterwards is not an abandonment.
	refused bool
	// stale is set when Update or Remove invalidated the key while this
	// flight was building: its waiters get its value, but it is never
	// cached — it may predate the refresh that invalidated the key.
	stale bool
}

// NewGroup registers a group in the Default registry.
func NewGroup[K comparable, V any](name string, opts Options) *Group[K, V] {
	return NewGroupIn[K, V](Default, name, opts)
}

// NewGroupIn registers a group in reg.
func NewGroupIn[K comparable, V any](reg *Registry, name string, opts Options) *Group[K, V] {
	g := &Group[K, V]{
		reg:       reg,
		nm:        name,
		opts:      opts,
		entries:   make(map[K]*entry[V]),
		flights:   make(map[K]*flight[V]),
		counters:  GroupStats{Name: name, Evictions: map[DropReason]int64{}},
		lastBuild: make(map[string]buildRecord),
	}
	reg.mu.Lock()
	reg.groups = append(reg.groups, g)
	reg.mu.Unlock()
	if reg == Default {
		// RT-124: /metrics reads the group's bytes as
		// fathom_cache_bytes{group}.
		tenancy.RegisterCacheBytes(name, func() int64 { return g.stats().Bytes })
	}
	return g
}

func (g *Group[K, V]) name() string { return g.nm }

// evictLocked is called with the registry lock held.
func (g *Group[K, V]) evictLocked(e *entryRef, reason DropReason) {
	g.mu.Lock()
	k := e.key.(K)
	if cur, ok := g.entries[k]; ok && cur.ref == e {
		delete(g.entries, k)
		if g.onEvict != nil {
			g.onEvict(k, cur.val)
		}
	}
	g.counters.Evictions[reason]++
	g.mu.Unlock()
}

func (g *Group[K, V]) stats() GroupStats {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.counters
	s.Entries = len(g.entries)
	var b int64
	for _, e := range g.entries {
		b += e.ref.bytes
	}
	s.Bytes = b
	s.Evictions = make(map[DropReason]int64, len(g.counters.Evictions))
	for k, v := range g.counters.Evictions {
		s.Evictions[k] = v
	}
	return s
}

// scopeOf returns the key's (service, watermark) for a watermark-scoped
// group; ok is false when the key must not be cached (no Scoped, or an
// empty watermark — I21).
func (g *Group[K, V]) scopeOf(key K) (service, mark string, ok bool) {
	if !g.opts.WatermarkScoped {
		return "", "", true
	}
	sc, isScoped := any(key).(Scoped)
	if !isScoped {
		return "", "", false
	}
	service, mark = sc.MemoScope()
	return service, mark, mark != ""
}

// ErrOversize is never returned; an entry larger than the limit in force
// is simply not cached (counted as Uncached).
var errNilBuild = errors.New("memo: nil build")

// ErrWouldNotFinish is a Keep group's Get declining to start a build its
// caller's deadline cannot hold (wouldAbandon): no flight started, no
// rebuild token taken. The caller answers with a named partial (I38).
var ErrWouldNotFinish = errors.New("the build would not finish inside the caller's deadline; no build was started")

// Get returns the cached value for key, or joins (or starts) the one
// build in flight for it. build returns the value and its estimated size
// in bytes. A waiter whose ctx ends leaves at once with ctx.Err(); the
// build is cancelled when its last waiter leaves unless the group keeps
// it. A failed build is never cached.
//
// While the ladder is at shed a caller that must start a flight first
// takes the process-wide rebuild token, waiting for it no longer than its
// own ctx; the flight returns the token when its build ends. A Get made
// from inside a build (a nested flight: GraphFor loading typed edges) is
// part of that rebuild and never takes the token — its parent holds it —
// so a rebuild cannot wait on itself. Joining a flight never needs it.
//
// On a Keep group a caller that would start a flight it cannot use — its
// deadline too short for the build and the build not one that would be
// kept for the next caller (wouldAbandon) — gets ErrWouldNotFinish at once
// instead, before any rebuild token. A nested Get, or one with no
// deadline, always starts.
func (g *Group[K, V]) Get(ctx context.Context, key K, build func(context.Context) (V, int64, error)) (V, error) {
	var zero V
	if build == nil {
		return zero, errNilBuild
	}
	service, mark, cacheable := g.scopeOf(key)
	if g.opts.WatermarkScoped && cacheable {
		g.reg.mu.Lock()
		g.reg.noteWatermarkLocked(service, mark)
		g.reg.mu.Unlock()
	}

	nested := ctx.Value(inBuildKey{}) != nil
	var token func() // the rebuild token this caller holds, if any
	defer func() {
		if token != nil {
			token()
		}
	}()
	var f *flight[V]
	_, hasDeadline := ctx.Deadline()
	mayDecline := g.opts.Keep && !nested && hasDeadline
	for {
		needToken := token == nil && !nested && g.reg.shed()
		g.reg.mu.Lock()
		g.mu.Lock()
		if e, ok := g.entries[key]; ok {
			g.counters.Hits++
			if e.ref.elem != nil {
				g.reg.lru.MoveToFront(e.ref.elem)
			}
			v := e.val
			g.mu.Unlock()
			g.reg.mu.Unlock()
			return v, nil
		}
		var limit int64
		if mayDecline {
			limit = g.reg.limitLocked()
		}
		g.reg.mu.Unlock()
		if cur, ok := g.flights[key]; ok {
			g.counters.Hits++
			f = cur
			break
		}
		if mayDecline && g.wouldAbandon(ctx, service, limit) {
			g.counters.Declined++
			g.mu.Unlock()
			return zero, ErrWouldNotFinish
		}
		if needToken {
			g.mu.Unlock()
			release, err := g.reg.acquireRebuild(ctx)
			if err != nil {
				return zero, err
			}
			token = release
			continue // the entry or a flight may have appeared meanwhile
		}
		g.counters.Misses++
		g.counters.Flights++
		fctx, cancel := context.WithCancel(context.WithValue(context.WithoutCancel(ctx), inBuildKey{}, true))
		f = &flight[V]{done: make(chan struct{}), cancel: cancel, started: g.reg.now()}
		g.flights[key] = f
		go g.run(fctx, key, f, build, service, mark, cacheable, token)
		token = nil // the flight returns it when its build ends
		break
	}
	f.waiters++
	g.mu.Unlock()
	if token != nil {
		token() // joined a flight after all: no rebuild of ours
		token = nil
	}

	select {
	case <-f.done:
		return f.val, f.err
	case <-ctx.Done():
		g.leave(key, f)
		return zero, ctx.Err()
	}
}

// inBuildKey marks a build's context: a Get under it is a nested flight.
type inBuildKey struct{}

// wouldAbandon reports a flight its caller cannot use (§5 "No doomed
// flights", O1014): ctx's deadline d is shorter than the last completed
// build of service in this group (ExpectedCost when none is measured) and
// the flight would not be kept for the next caller either — kept needs d
// at least half ExpectedCost and the last build's size within limit, the
// byte limit in force (0 = none; an unmeasured size fits). No deadline is
// never declined. Group lock held.
func (g *Group[K, V]) wouldAbandon(ctx context.Context, service string, limit int64) bool {
	dl, ok := ctx.Deadline()
	if !ok {
		return false
	}
	d := dl.Sub(g.reg.now())
	rec, known := g.lastBuild[service]
	took := g.opts.ExpectedCost
	if known {
		took = rec.took
	}
	if d >= took {
		return false
	}
	fits := !known || limit <= 0 || rec.size <= limit
	kept := g.opts.ExpectedCost > 0 && d >= g.opts.ExpectedCost/2 && fits
	return !kept
}

// leave drops one waiter; the last one out cancels the flight unless the
// group keeps it.
func (g *Group[K, V]) leave(key K, f *flight[V]) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f.waiters--
	if f.waiters > 0 {
		return
	}
	select {
	case <-f.done:
		return
	default:
	}
	if g.opts.Keep && g.opts.ExpectedCost > 0 && g.reg.now().Sub(f.started) >= g.opts.ExpectedCost/2 {
		return
	}
	if f.refused {
		return // a refuse drop already ended it and counted it
	}
	f.cancel()
	g.counters.Abandoned++
	// A cancelled flight no longer serves new callers: they start their own.
	if cur, ok := g.flights[key]; ok && cur == f {
		delete(g.flights, key)
	}
}

// run is the flight: the build (holding the rebuild token its starter
// took, if any, until the build ends) and the insert.
func (g *Group[K, V]) run(ctx context.Context, key K, f *flight[V], build func(context.Context) (V, int64, error), service, mark string, cacheable bool, token func()) {
	defer f.cancel()
	val, size, err := build(ctx)
	took := g.reg.now().Sub(f.started)
	if token != nil {
		token()
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	f.val, f.err = val, err

	g.reg.mu.Lock()
	g.mu.Lock()
	if cur, ok := g.flights[key]; ok && cur == f {
		delete(g.flights, key)
	}
	if err == nil {
		g.lastBuild[service] = buildRecord{took: took, size: size}
	}
	stored := false
	if err == nil && cacheable && !f.stale {
		stored = g.insertLocked(key, val, size, service, mark)
	}
	if !stored {
		g.counters.Uncached++
	}
	g.mu.Unlock()
	g.reg.mu.Unlock()
	close(f.done)
}

// cancelFlightsLocked cancels every flight in progress (refuse): kept or
// not, its build's ctx ends, its waiters get the cancellation, and a new
// caller starts afresh. Registry lock held.
func (g *Group[K, V]) cancelFlightsLocked() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, f := range g.flights {
		f.cancel()
		f.refused = true
		g.counters.RefuseCancelled++
		delete(g.flights, k)
	}
}

// insertLocked caches val under key when it fits the limit in force and
// its watermark is still the service's newest. Both locks held.
func (g *Group[K, V]) insertLocked(key K, val V, size int64, service, mark string) bool {
	r := g.reg
	if size < 0 {
		size = 0
	}
	if g.opts.WatermarkScoped {
		if cur, ok := r.marks[service]; ok && cur != mark {
			return false // a newer watermark arrived during the build
		}
		if _, ok := r.marks[service]; !ok {
			// EvictService ran during the build: this value may predate it.
			return false
		}
	}
	limit := r.limitLocked()
	if limit > 0 && size > limit {
		return false
	}
	if old, ok := g.entries[key]; ok {
		r.removeLocked(old.ref)
		if g.onEvict != nil {
			g.onEvict(key, old.val)
		}
	}
	ref := &entryRef{g: g, key: key, service: service, mark: mark, bytes: size}
	ref.elem = r.lru.PushFront(ref)
	r.total += size
	g.entries[key] = &entry[V]{val: val, ref: ref}
	if g.opts.MaxEntries > 0 && len(g.entries) > g.opts.MaxEntries {
		g.evictOldestOwnLocked(ref)
	}
	if limit > 0 && r.total > limit {
		// Evict others first; the new entry is at the front.
		g.mu.Unlock()
		r.trimToLocked(limit, ReasonBudget)
		g.mu.Lock()
	}
	return true
}

// evictOldestOwnLocked evicts this group's least recently used entry other
// than keep. Both locks held.
func (g *Group[K, V]) evictOldestOwnLocked(keep *entryRef) {
	for el := g.reg.lru.Back(); el != nil; el = el.Prev() {
		e := el.Value.(*entryRef)
		if e.g != groupHandle(g) || e == keep {
			continue
		}
		k := e.key.(K)
		if cur, ok := g.entries[k]; ok && g.onEvict != nil {
			g.onEvict(k, cur.val)
		}
		delete(g.entries, k)
		g.counters.Evictions[ReasonMaxEntries]++
		g.reg.removeLocked(e)
		return
	}
}

// Peek returns the cached value without building (no hit or miss counted).
func (g *Group[K, V]) Peek(key K) (V, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.val, true
}

// Reset drops every entry of this group only.
func (g *Group[K, V]) Reset(reason DropReason) {
	g.reg.mu.Lock()
	defer g.reg.mu.Unlock()
	g.mu.Lock()
	var refs []*entryRef
	for _, e := range g.entries {
		refs = append(refs, e.ref)
	}
	g.mu.Unlock()
	for _, ref := range refs {
		g.evictLocked(ref, reason)
		g.reg.removeLocked(ref)
	}
}

// Len is the group's entry count.
func (g *Group[K, V]) Len() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.entries)
}

// Update replaces the cached value for key with fn's result and size (a
// negative size keeps the entry's recorded size), keeping its LRU
// position; it reports whether key was cached. A flight in progress for
// key is invalidated (never cached: it may predate the change Update
// applies). fn runs with no lock held — a patch may re-index a whole
// snapshot, and every Get in the process would wait on the registry lock
// meanwhile — and its result is committed only if the entry it read is
// still the one cached, unchanged; otherwise fn runs again on the entry
// that replaced it. fn must be safe to run more than once.
func (g *Group[K, V]) Update(key K, fn func(V) (V, int64)) bool {
	for {
		g.mu.Lock()
		g.invalidateFlightLocked(key)
		e, ok := g.entries[key]
		if !ok {
			g.mu.Unlock()
			return false
		}
		old, gen := e.val, e.gen
		g.mu.Unlock()

		v, size := fn(old)

		g.reg.mu.Lock()
		g.mu.Lock()
		cur, ok := g.entries[key]
		if !ok {
			g.invalidateFlightLocked(key)
			g.mu.Unlock()
			g.reg.mu.Unlock()
			return false // evicted meanwhile: the next reader loads afresh
		}
		if cur != e || cur.gen != gen {
			g.mu.Unlock()
			g.reg.mu.Unlock()
			continue // replaced or patched meanwhile: patch what is cached now
		}
		g.invalidateFlightLocked(key)
		if size < 0 {
			size = e.ref.bytes
		}
		if e.ref.elem != nil {
			g.reg.total += size - e.ref.bytes
		}
		e.ref.bytes = size
		e.val = v
		e.gen++
		g.mu.Unlock()
		g.reg.mu.Unlock()
		return true
	}
}

// invalidateFlightLocked marks key's flight in progress, if any, stale and
// detaches it: its waiters still get its value, a new caller starts a
// fresh build, and it is never cached. Group lock held.
func (g *Group[K, V]) invalidateFlightLocked(key K) {
	if f, ok := g.flights[key]; ok {
		f.stale = true
		delete(g.flights, key)
	}
}

// Remove drops key from the group (a no-op when it is not cached) and
// invalidates a flight in progress for it (Update).
func (g *Group[K, V]) Remove(key K, reason DropReason) {
	g.reg.mu.Lock()
	defer g.reg.mu.Unlock()
	g.mu.Lock()
	g.invalidateFlightLocked(key)
	e, ok := g.entries[key]
	g.mu.Unlock()
	if !ok {
		return
	}
	g.evictLocked(e.ref, reason)
	g.reg.removeLocked(e.ref)
}
