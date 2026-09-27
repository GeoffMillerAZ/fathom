package main

// metrics_tenancy.go — /metrics' tenancy families (docs/design/
// responsible-tenant.md §13.3, RT-124): the governor's ladder, budget,
// pressure, swap, footprint and children (governorMetricLines); admission's
// in-flight, queue and rejected counters (Gate.MetricLines); and the rest —
// volume free per volume, the oldest in-flight age per class, abandoned
// calls by tool and cause, bytes read by tool, cache bytes by group, starts
// by reason, memory released to the OS and SQLite's own allocator. A
// reading this process could not take is omitted or named in a comment,
// never a zero (I38).

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// writeTenancyMetrics writes every tenancy family (srv may be nil: no gate).
func writeTenancyMetrics(w io.Writer, srv *mcp.Server) {
	io.WriteString(w, governorMetricLines()) //nolint:errcheck
	if srv != nil {
		io.WriteString(w, admissionMetricLines(srv)) //nolint:errcheck
	} else {
		io.WriteString(w, testCallerMetricLine()) //nolint:errcheck
	}
	if g := activeGovernor.Load(); g != nil {
		io.WriteString(w, g.extraMetricLines()) //nolint:errcheck
	}
	io.WriteString(w, tenancy.ToolCounterLines()) //nolint:errcheck
	if v, ok := store.SQLiteMemoryUsed(); ok {
		fmt.Fprintf(w, "fathom_sqlite_memory_used_bytes %d\n", v)
	} else {
		io.WriteString(w, "# fathom_sqlite_memory_used_bytes: not counted — this SQLite build keeps no memory status (SQLITE_DEFAULT_MEMSTATUS=0); its page cache is inside fathom_footprint_bytes\n") //nolint:errcheck
	}
}

// extraMetricLines are the governor-held families governorMetricLines
// predates: volumes, released memory, oldest in-flight, starts by reason.
func (g *governor) extraMetricLines() string {
	var b strings.Builder
	g.mu.Lock()
	s, have := g.sample, g.haveSmp
	g.mu.Unlock()
	if have {
		for _, v := range []struct {
			vol string
			n   int64
		}{{"store", s.StoreFree}, {"tmp", s.TmpFree}, {"swap", s.SwapVolumeFree}} {
			if v.n >= 0 {
				fmt.Fprintf(&b, "fathom_volume_free_bytes{vol=%q} %d\n", v.vol, v.n)
			}
		}
		if s.HeapReleased >= 0 {
			fmt.Fprintf(&b, "fathom_memory_released_bytes %d\n", s.HeapReleased)
		}
	}
	now := g.now()
	oldest := map[tenancy.Class]time.Duration{}
	for _, e := range g.inflight.Snapshot() {
		if age := now.Sub(e.Started); age > oldest[e.Class] {
			oldest[e.Class] = age
		}
	}
	for _, c := range tenancy.Classes {
		fmt.Fprintf(&b, "fathom_oldest_inflight_seconds{class=%q} %.3f\n", c.String(), oldest[c].Seconds())
	}
	if starts, err := g.startsByReason(); err != nil {
		fmt.Fprintf(&b, "# fathom_lifecycle_starts_total: lifecycle ledger unreadable (%v)\n", err)
	} else if starts != nil {
		reasons := make([]string, 0, len(starts))
		for r := range starts {
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		for _, r := range reasons {
			fmt.Fprintf(&b, "fathom_lifecycle_starts_total{reason=%q} %d\n", r, starts[r])
		}
	}
	return b.String()
}

// startsLedgerCache memoizes the ledger's starts by reason on the ledger's
// size and mtime: a scrape a second (the stress harness) re-reads nothing.
type startsLedgerCache struct {
	mu     sync.Mutex
	key    string
	starts map[string]int
}

var startsCache startsLedgerCache

// startsByReason is the ledger's start rows by reason (nil: no ledger).
func (g *governor) startsByReason() (map[string]int, error) {
	if g.ledger == "" {
		return nil, nil
	}
	key := ""
	for _, p := range []string{g.ledger + ".1", g.ledger} {
		if st, err := os.Stat(p); err == nil {
			key += fmt.Sprintf("%s:%d:%d;", p, st.Size(), st.ModTime().UnixNano())
		}
	}
	if key == "" {
		return map[string]int{}, nil
	}
	startsCache.mu.Lock()
	defer startsCache.mu.Unlock()
	if startsCache.key == key {
		return startsCache.starts, nil
	}
	f, done, err := openLedgerGenerations(g.ledger)
	if err != nil {
		return nil, err
	}
	defer done()
	rows, _, err := tenancy.ReadLedger(f)
	if err != nil {
		return nil, err
	}
	startsCache.key, startsCache.starts = key, tenancy.StartsByReason(rows)
	return startsCache.starts, nil
}
