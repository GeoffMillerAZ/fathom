package scope

// disclose.go — the per-call disclosure channel (SP-3).
//
// The store's read methods return slices, not envelopes, so the fact of
// filtering travels beside the call: a surface that wants to render the
// `"suppressed": {...}` block attaches a Collector to its context, and the
// store's read gate records into it. A read with no collector attached
// still filters — the disclosure is produced unconditionally by
// ApplySuppressions and only its DELIVERY is per-surface.

import (
	"context"
	"sort"
	"sync"
)

type collectorKey struct{}

// Collector accumulates per-service disclosures across the reads of one
// request. Safe for concurrent use (a request may fan out reads).
type Collector struct {
	mu        sync.Mutex
	byService map[string]*Disclosure
}

// WithCollector returns a ctx carrying a fresh Collector, and the collector
// itself.
func WithCollector(ctx context.Context) (context.Context, *Collector) {
	c := &Collector{byService: map[string]*Disclosure{}}
	return context.WithValue(ctx, collectorKey{}, c), c
}

// Record folds d into the collector attached to ctx, under service. A ctx
// with no collector is a no-op — filtering never depends on anyone
// listening. A nil d is likewise a no-op (nothing was hidden).
func Record(ctx context.Context, service string, d *Disclosure) {
	if d == nil {
		return
	}
	c, ok := ctx.Value(collectorKey{}).(*Collector)
	if !ok || c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cur, ok := c.byService[service]
	if !ok {
		cur = &Disclosure{}
		c.byService[service] = cur
	}
	cur.Merge(d)
}

// For returns a copy of the accumulated disclosure for service, or nil
// when nothing was hidden for it.
func (c *Collector) For(service string) *Disclosure {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.byService[service]
	if !ok {
		return nil
	}
	cp := *d
	cp.Reasons = append([]string(nil), d.Reasons...)
	cp.IDs = append([]string(nil), d.IDs...)
	return &cp
}

// Services returns the sorted service ids that had anything hidden.
func (c *Collector) Services() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.byService))
	for svc := range c.byService {
		out = append(out, svc)
	}
	sort.Strings(out)
	return out
}
