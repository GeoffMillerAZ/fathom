package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestMetricsCarryCacheEventsByGroup is RT-188 (§5, §13.3): every memo
// group's counters reach /metrics as fathom_cache_events_total{group,event}
// — hits, misses, flights, uncached, abandoned (the last waiter left),
// refuse_cancelled (a refuse drop ended the build), declined (a build its
// caller could not use was not started) and evictions by reason — sorted
// by group, then in one fixed event order, so a scrape is deterministic.
func TestMetricsCarryCacheEventsByGroup(t *testing.T) {
	var b bytes.Buffer
	writeCacheEvents(&b, []memo.GroupStats{
		{Name: "zeta", Hits: 1},
		{Name: "graphstore", Hits: 5, Misses: 2, Flights: 2, Uncached: 1, Abandoned: 3, RefuseCancelled: 4, Declined: 6,
			Evictions: map[memo.DropReason]int64{memo.ReasonShed: 7, memo.ReasonBudget: 8}},
	})
	want := `# HELP fathom_cache_events_total memo cache events by group: hits, misses, flights started, uncached builds, abandoned (the last waiter left), refuse_cancelled (a refuse drop ended the build), declined (a build that would not finish inside its caller's deadline was not started), evicted_<reason>
fathom_cache_events_total{group="graphstore",event="hits"} 5
fathom_cache_events_total{group="graphstore",event="misses"} 2
fathom_cache_events_total{group="graphstore",event="flights"} 2
fathom_cache_events_total{group="graphstore",event="uncached"} 1
fathom_cache_events_total{group="graphstore",event="abandoned"} 3
fathom_cache_events_total{group="graphstore",event="refuse_cancelled"} 4
fathom_cache_events_total{group="graphstore",event="declined"} 6
fathom_cache_events_total{group="graphstore",event="evicted_budget"} 8
fathom_cache_events_total{group="graphstore",event="evicted_idle"} 0
fathom_cache_events_total{group="graphstore",event="evicted_max_entries"} 0
fathom_cache_events_total{group="graphstore",event="evicted_refuse"} 0
fathom_cache_events_total{group="graphstore",event="evicted_shed"} 7
fathom_cache_events_total{group="graphstore",event="evicted_watermark"} 0
fathom_cache_events_total{group="zeta",event="hits"} 1
fathom_cache_events_total{group="zeta",event="misses"} 0
fathom_cache_events_total{group="zeta",event="flights"} 0
fathom_cache_events_total{group="zeta",event="uncached"} 0
fathom_cache_events_total{group="zeta",event="abandoned"} 0
fathom_cache_events_total{group="zeta",event="refuse_cancelled"} 0
fathom_cache_events_total{group="zeta",event="declined"} 0
fathom_cache_events_total{group="zeta",event="evicted_budget"} 0
fathom_cache_events_total{group="zeta",event="evicted_idle"} 0
fathom_cache_events_total{group="zeta",event="evicted_max_entries"} 0
fathom_cache_events_total{group="zeta",event="evicted_refuse"} 0
fathom_cache_events_total{group="zeta",event="evicted_shed"} 0
fathom_cache_events_total{group="zeta",event="evicted_watermark"} 0
`
	if got := b.String(); got != want {
		t.Errorf("writeCacheEvents:\n%s\nwant:\n%s", got, want)
	}

	// No group: a comment naming why, never a zero (I38).
	b.Reset()
	writeCacheEvents(&b, nil)
	if got := b.String(); !strings.HasPrefix(got, "# fathom_cache_events_total: ") || strings.Contains(got, "} ") {
		t.Errorf("no groups: %q", got)
	}

	// The daemon's /metrics carries the family for the registered groups,
	// beside the families it already wrote.
	h, _ := gatedTestHandler(t, tenancy.ModeEnforce)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`fathom_cache_events_total{group="graphstore",event="abandoned"} `,
		`fathom_cache_events_total{group="graphstore",event="refuse_cancelled"} `,
		`fathom_cache_events_total{group="graphstore",event="declined"} `,
		`fathom_cache_events_total{group="snapshot",event="evicted_shed"} `,
		"fathom_graph_memo_entries ", "fathom_requests_total ", "fathom_open_stores ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics lacks %q", want)
		}
	}
}
