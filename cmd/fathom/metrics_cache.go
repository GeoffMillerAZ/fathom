package main

// metrics_cache.go — /metrics' fathom_cache_events_total{group,event}
// (docs/design/responsible-tenant.md §5, §13.3): every memo group's
// counters, so an abandoned build (its last waiter left), a refuse drop's
// cancellation and a declined flight (a build its caller's deadline could
// not hold) read apart.

import (
	"fmt"
	"io"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// cacheEvictionReasons is every memo drop reason, sorted: each group
// reports all of them, a zero included, so a series never appears only once
// its first eviction happens.
var cacheEvictionReasons = []memo.DropReason{
	memo.ReasonBudget, memo.ReasonIdle, memo.ReasonMaxEntries,
	memo.ReasonRefuse, memo.ReasonShed, memo.ReasonWatermark,
}

// writeCacheEvents writes fathom_cache_events_total for groups, sorted by
// group and then in one fixed event order; a reason a group counted that
// cacheEvictionReasons does not name follows the known ones, sorted. With
// no group it writes a comment naming why, never a zero (I38).
func writeCacheEvents(w io.Writer, groups []memo.GroupStats) {
	if len(groups) == 0 {
		io.WriteString(w, "# fathom_cache_events_total: no memo cache group registered in this process\n") //nolint:errcheck
		return
	}
	groups = append([]memo.GroupStats(nil), groups...)
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	io.WriteString(w, "# HELP fathom_cache_events_total memo cache events by group: hits, misses, flights started, uncached builds, abandoned (the last waiter left), refuse_cancelled (a refuse drop ended the build), declined (a build that would not finish inside its caller's deadline was not started), evicted_<reason>\n") //nolint:errcheck
	for _, g := range groups {
		line := func(event string, n int64) {
			fmt.Fprintf(w, "fathom_cache_events_total{group=%q,event=%q} %d\n", g.Name, event, n)
		}
		line("hits", g.Hits)
		line("misses", g.Misses)
		line("flights", g.Flights)
		line("uncached", g.Uncached)
		line("abandoned", g.Abandoned)
		line("refuse_cancelled", g.RefuseCancelled)
		line("declined", g.Declined)
		known := make(map[memo.DropReason]bool, len(cacheEvictionReasons))
		for _, r := range cacheEvictionReasons {
			known[r] = true
			line("evicted_"+string(r), g.Evictions[r])
		}
		var other []string
		for r := range g.Evictions {
			if !known[r] {
				other = append(other, string(r))
			}
		}
		sort.Strings(other)
		for _, r := range other {
			line("evicted_"+r, g.Evictions[memo.DropReason(r)])
		}
	}
}
