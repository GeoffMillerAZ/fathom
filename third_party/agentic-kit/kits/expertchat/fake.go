package expertchat

import (
	"context"
	"sync"
)

// FakeRetriever is a deterministic, scriptable Retriever for tests: it
// returns Items (optionally truncated to a caller's limit) and records every
// query it was asked, in order, for later assertion. No network, no
// randomness. Safe for concurrent use.
type FakeRetriever struct {
	// Items is returned verbatim (subject to limit) by every Retrieve call,
	// unless Err is set.
	Items []EvidenceItem

	// Err, when non-nil, is returned by Retrieve instead of Items.
	Err error

	mu      sync.Mutex
	queries []string
}

// Retrieve records query and returns up to limit items from f.Items (or
// f.Err, if set). limit <= 0 returns every item.
func (f *FakeRetriever) Retrieve(_ context.Context, query string, limit int) ([]EvidenceItem, error) {
	f.mu.Lock()
	f.queries = append(f.queries, query)
	f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}
	items := f.Items
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]EvidenceItem, len(items))
	copy(out, items)
	return out, nil
}

// Queries returns a copy of every query Retrieve was called with, in order.
func (f *FakeRetriever) Queries() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.queries))
	copy(out, f.queries)
	return out
}
