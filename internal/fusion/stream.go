package fusion

// stream.go — RT-102, RT-103 (docs/design/responsible-tenant.md §7.2):
// incremental, streamed ingest.
//
// Pass 1 (statChangedFiles) stats every discovered file and reads only the
// ones whose (size, mtime, inode) moved since the last ingest — a file
// whose stat and stored fingerprint both still match the record keeps its
// content hash unread. Pass 2 (fetchWindowed) reads those files with a few
// workers through an in-flight byte window (Config.InflightBytes, 64 MiB)
// and hands each to the sequential, input-ordered consumer that hashes,
// extracts and writes it, then releases its bytes: peak content memory is
// bounded by the window, whatever the corpus. Before this, pass 2 held the
// whole corpus's bytes at once (the audit's ingest-pass2-holds-whole-corpus
// hazard) and every ingest read every file.
//
// The prior stats come from the store's file_stats when the storage lane's
// source is wired (WithStatSource), else from this pipeline's record of its
// own last successful ingest of the service; the live stats are handed to
// WithStatSink at the final commit. A pipeline with neither (a one-shot CLI
// ingest) reads every file, as before; so does Config.Rehash.

import (
	"context"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// DefaultInflightBytes is the pass-2 window: content bytes read and not yet
// written (design §7.2, `[ingest] inflight_bytes`).
const DefaultInflightBytes int64 = 64 << 20

// obsBatch is how many observations accumulate before one write (§7.2).
const obsBatch = 256

// StatRecord is one file's stat and content fingerprint as the stat gate
// keeps them (the file_stats row's shape: path, size, mtime, inode, hash).
type StatRecord struct {
	Path string
	connector.FileStat
	ContentHash string // FingerprintHash of the bytes last read at this stat
}

// WithStatSource installs where pass 1's prior stats come from (the store's
// file_stats once the storage lane lands it): a map by path, or ok=false
// when the service has none (everything is read). nil restores the
// pipeline's own record of its last ingest.
func (p *Pipeline) WithStatSource(fn func(ctx context.Context, service string) (map[string]StatRecord, bool)) {
	p.statSource = fn
}

// WithStatSink installs the receiver of each successful ingest's live stats
// (every file read or stat-skipped, with its content hash), called at the
// final commit — where file_stats is written.
func (p *Pipeline) WithStatSink(fn func(ctx context.Context, service string, recs []StatRecord) error) {
	p.statSink = fn
}

// SetRehash turns the stat gate off (on) or back on for later ingests.
func (p *Pipeline) SetRehash(on bool) {
	p.mu.Lock()
	p.cfg.Rehash = on
	p.mu.Unlock()
}

// priorStats is pass 1's prior record for service, or nil.
func (p *Pipeline) priorStats(ctx context.Context, service string) map[string]StatRecord {
	if p.statSource != nil {
		m, ok := p.statSource(ctx, service)
		if !ok {
			return nil
		}
		return m
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.statMemo[service]
}

// statGate is pass 1's outcome.
type statGate struct {
	live    []StatRecord // per ref: its stat (when statted) and, once known, its content hash
	statted []bool
	toRead  []int // indexes into refs, in order
	skipped int   // files whose stat and fingerprint both matched: not read
}

// statChangedFiles is pass 1 (RT-102): stat every ref and keep for reading
// only those whose (size, mtime, inode) moved since the prior record, or
// whose stored fingerprint no longer matches the recorded hash. disabled
// (legacy anchors about to be rebuilt, or Config.Rehash) reads everything.
func (p *Pipeline) statChangedFiles(ctx context.Context, serviceID string, refs []core.SourceRef, disabled bool) statGate {
	g := statGate{live: make([]StatRecord, len(refs)), statted: make([]bool, len(refs))}
	stater, canStat := p.conn.(connector.Stater)
	var prior map[string]StatRecord
	p.mu.Lock()
	rehash := p.cfg.Rehash
	p.mu.Unlock()
	if canStat && !disabled && !rehash {
		prior = p.priorStats(ctx, serviceID)
	}
	for i, ref := range refs {
		g.live[i].Path = ref.Path
		if canStat {
			if st, err := stater.Stat(ctx, ref); err == nil {
				g.live[i].FileStat = st
				g.statted[i] = true
				if pr, ok := prior[ref.Path]; ok && pr.FileStat == st && pr.ContentHash != "" && st.Size <= connector.MaxFetchSize {
					if stored, found, ferr := p.st.Fingerprint(ctx, serviceID, ref.Path); ferr == nil && found && stored == pr.ContentHash {
						g.live[i].ContentHash = pr.ContentHash
						g.skipped++
						continue
					}
				}
			}
		}
		g.toRead = append(g.toRead, i)
	}
	return g
}

// rememberStats records a successful ingest's live stats for the next
// ingest's pass 1 and hands them to the sink. Only files statted and hashed
// this run are kept; an unreadable one is read again next time.
func (p *Pipeline) rememberStats(ctx context.Context, service string, g statGate) {
	recs := make([]StatRecord, 0, len(g.live))
	m := make(map[string]StatRecord, len(g.live))
	for i, r := range g.live {
		if !g.statted[i] || r.ContentHash == "" {
			continue
		}
		recs = append(recs, r)
		m[r.Path] = r
	}
	if p.statSource == nil {
		p.mu.Lock()
		if p.statMemo == nil {
			p.statMemo = map[string]map[string]StatRecord{}
		}
		p.statMemo[service] = m
		p.mu.Unlock()
	}
	if p.statSink != nil {
		_ = p.statSink(ctx, service, recs)
	}
}

// byteWindow bounds content bytes in flight (RT-103). One goroutine (the
// dispatcher) acquires; any goroutine releases.
type byteWindow struct {
	mu      sync.Mutex
	cap     int64
	used    int64 // reserved: reads in progress at their cap, plus content held
	content int64 // content bytes read and not yet released
	peak    int64 // the most content held at once
	wake    chan struct{}
}

func newByteWindow(capacity int64) *byteWindow {
	if capacity <= 0 {
		capacity = DefaultInflightBytes
	}
	return &byteWindow{cap: capacity, wake: make(chan struct{})}
}

// acquire reserves n bytes, waiting while they would exceed the window (a
// single reservation larger than the window is let through alone).
func (w *byteWindow) acquire(ctx context.Context, n int64) error {
	for {
		w.mu.Lock()
		if w.used+n <= w.cap || w.used == 0 {
			w.used += n
			w.mu.Unlock()
			return nil
		}
		ch := w.wake
		w.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// settle turns a read's reservation into the content it actually holds.
func (w *byteWindow) settle(reserved, got int64) {
	w.mu.Lock()
	w.used += got - reserved
	w.content += got
	if w.content > w.peak {
		w.peak = w.content
	}
	w.signalLocked()
	w.mu.Unlock()
}

// release frees content (or an unsettled reservation) of n bytes.
func (w *byteWindow) release(n int64, content bool) {
	w.mu.Lock()
	w.used -= n
	if content {
		w.content -= n
	}
	w.signalLocked()
	w.mu.Unlock()
}

func (w *byteWindow) signalLocked() {
	close(w.wake)
	w.wake = make(chan struct{})
}

// fetchWindowed is pass 2 (RT-103): it reads refs[order[k]] with up to
// workers concurrent reads — each reserving the read cap until its bytes
// are known — and hands every result to consume in order; a result's bytes
// are released when consume returns. Content in flight never exceeds
// window. consume's first error stops the reads and is returned; so is a
// cancelled ctx. peak is the most content bytes held at once.
func (p *Pipeline) fetchWindowed(ctx context.Context, refs []core.SourceRef, order []int, window int64, workers int, consume func(i int, content []byte, err error) error) (peak int64, err error) {
	if len(order) == 0 {
		return 0, nil
	}
	if workers <= 0 {
		workers = 1
	}
	w := newByteWindow(window)
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	const reserve = int64(connector.MaxFetchSize) + 1

	type job struct{ k, i int }
	type result struct {
		k, i    int
		content []byte
		err     error
		held    int64 // bytes held in the window for this result
	}
	jobs := make(chan job)
	results := make(chan result, workers)
	var wg sync.WaitGroup
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				content, ferr := p.conn.Fetch(rctx, refs[j.i])
				held := int64(len(content))
				w.settle(reserve, held)
				select {
				case results <- result{k: j.k, i: j.i, content: content, err: ferr, held: held}:
				case <-rctx.Done():
					w.release(held, true)
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for k, i := range order {
			if w.acquire(rctx, reserve) != nil {
				return
			}
			select {
			case jobs <- job{k: k, i: i}:
			case <-rctx.Done():
				w.release(reserve, false)
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	pending := map[int]result{}
	next := 0
	for next < len(order) && err == nil {
		r, ok := pending[next]
		if !ok {
			rr, open := <-results
			if !open {
				break
			}
			pending[rr.k] = rr
			continue
		}
		delete(pending, next)
		next++
		err = consume(r.i, r.content, r.err)
		w.release(r.held, true)
	}
	cancel()
	for _, r := range pending {
		w.release(r.held, true)
	}
	for r := range results {
		w.release(r.held, true)
	}
	if err == nil && next < len(order) {
		err = ctx.Err()
		if err == nil {
			err = context.Canceled
		}
	}
	w.mu.Lock()
	peak = w.peak
	w.mu.Unlock()
	return peak, err
}

// existingObservations counts service's stored observations from the
// per-(source_ref, kind) aggregate — the IT-15 refusal's "is there evidence
// to protect" question, answered without loading the rows.
func (p *Pipeline) existingObservations(ctx context.Context, service string) (int, error) {
	counts, err := p.st.SourceKindCounts(ctx, service)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range counts {
		n += c.Count
	}
	return n, nil
}
