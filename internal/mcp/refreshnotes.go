package mcp

// refreshnotes.go — RT-150 (docs/design/responsible-tenant.md §7.1): a read
// whose auto-refresh was deferred (shed, safe mode, warm-up, quarantine)
// answers from the index as it stands and says so. ensure() records the
// reason per service on the request's collector; dispatch renders it into
// the response envelope as "refresh": {service: "deferred (…) — …"} beside
// "suppressed" — never a stale answer that looks fresh (I38).

import (
	"context"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/refresh"
)

type refreshNotesKey struct{}

// refreshNotes collects one request's deferred-refresh reasons.
type refreshNotes struct {
	mu sync.Mutex
	m  map[string]string
}

// withRefreshNotes attaches a collector to ctx.
func withRefreshNotes(ctx context.Context) (context.Context, *refreshNotes) {
	n := &refreshNotes{}
	return context.WithValue(ctx, refreshNotesKey{}, n), n
}

// noteRefreshDeferred records service's deferral on ctx's collector, if any.
func noteRefreshDeferred(ctx context.Context, service, reason string) {
	n, _ := ctx.Value(refreshNotesKey{}).(*refreshNotes)
	if n == nil || reason == "" {
		return
	}
	n.mu.Lock()
	if n.m == nil {
		n.m = map[string]string{}
	}
	n.m[service] = reason
	n.mu.Unlock()
}

// render is the envelope's "refresh" block, or nil when nothing was
// deferred.
func (n *refreshNotes) render() map[string]string {
	if n == nil {
		return nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(n.m))
	for k := range n.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = n.m[k]
	}
	return out
}

// noteRefreshQueued records that service's re-ingest is queued or running
// (RT-99, design §7.1): the read was answered from the index at indexed
// while HEAD is head — index_stale, disclosed, never a stale answer that
// looks fresh.
func noteRefreshQueued(ctx context.Context, service string, state refresh.State, indexed, head string) {
	verb := "queued"
	if state == refresh.StateRunning {
		verb = "refreshing"
	}
	noteRefreshDeferred(ctx, service, verb+" (index_stale) — served from the index at "+shortSHA(indexed)+"; HEAD is "+shortSHA(head)+"; the refresh runs in the background")
}
