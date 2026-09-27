package tenancy

// toolcounters.go — the per-tool /metrics counters (design §13.3, RT-124):
// fathom_abandoned_total{tool,cause} and fathom_bytes_read_total{tool},
// recorded by the MCP dispatch wrapper as each admitted call settles, and
// fathom_cache_bytes{group}, read from the caches that register a sizer.

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Abandonment causes a call can settle with.
const (
	AbandonGovernor = "governor"    // the refuse rung cancelled it
	AbandonClient   = "client_gone" // the client went away
	AbandonDeadline = "deadline"    // the class deadline or X-Fathom-Deadline passed
)

type toolCause struct{ tool, cause string }

var toolCounters = struct {
	sync.Mutex
	abandoned map[toolCause]int64
	bytesRead map[string]int64
	sizers    map[string]func() int64
}{abandoned: map[toolCause]int64{}, bytesRead: map[string]int64{}, sizers: map[string]func() int64{}}

// RecordAbandoned counts one call of tool abandoned for cause.
func RecordAbandoned(tool, cause string) {
	toolCounters.Lock()
	toolCounters.abandoned[toolCause{tool, cause}]++
	toolCounters.Unlock()
}

// RecordBytesRead adds n bytes read by one call of tool (n ≤ 0 is ignored).
func RecordBytesRead(tool string, n int64) {
	if n <= 0 {
		return
	}
	toolCounters.Lock()
	toolCounters.bytesRead[tool] += n
	toolCounters.Unlock()
}

// BytesReadTotal is the bytes every settled call of tool has read so far
// (fathom_bytes_read_total{tool}); the cost harness reads its delta
// around one call (RT-93).
func BytesReadTotal(tool string) int64 {
	toolCounters.Lock()
	defer toolCounters.Unlock()
	return toolCounters.bytesRead[tool]
}

// RegisterCacheBytes registers a cache group's byte reading (the memo
// registry's groups register here); a nil size unregisters it.
func RegisterCacheBytes(group string, size func() int64) {
	toolCounters.Lock()
	if size == nil {
		delete(toolCounters.sizers, group)
	} else {
		toolCounters.sizers[group] = size
	}
	toolCounters.Unlock()
}

// ToolCounterLines renders the three families, sorted. With no cache
// registered, fathom_cache_bytes is a comment naming why — never a zero
// for bytes nobody counted (I38).
func ToolCounterLines() string {
	toolCounters.Lock()
	var ab []string
	for k, n := range toolCounters.abandoned {
		ab = append(ab, fmt.Sprintf("fathom_abandoned_total{tool=%q,cause=%q} %d\n", k.tool, k.cause, n))
	}
	var br []string
	for t, n := range toolCounters.bytesRead {
		br = append(br, fmt.Sprintf("fathom_bytes_read_total{tool=%q} %d\n", t, n))
	}
	groups := make([]string, 0, len(toolCounters.sizers))
	sizers := make([]func() int64, 0, len(toolCounters.sizers))
	for g := range toolCounters.sizers {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		sizers = append(sizers, toolCounters.sizers[g])
	}
	toolCounters.Unlock()
	sort.Strings(ab)
	sort.Strings(br)
	var b strings.Builder
	for _, l := range ab {
		b.WriteString(l)
	}
	for _, l := range br {
		b.WriteString(l)
	}
	if len(groups) == 0 {
		b.WriteString("# fathom_cache_bytes: no byte-accounted cache registered in this process (the memo registry, RT-73, registers its groups)\n")
	}
	for i, g := range groups { // sizers run outside the lock
		fmt.Fprintf(&b, "fathom_cache_bytes{group=%q} %d\n", g, sizers[i]())
	}
	return b.String()
}

// resetToolCounters clears the counters (tests).
func resetToolCounters() {
	toolCounters.Lock()
	toolCounters.abandoned = map[toolCause]int64{}
	toolCounters.bytesRead = map[string]int64{}
	toolCounters.sizers = map[string]func() int64{}
	toolCounters.Unlock()
}

// ResetToolCountersForTest clears the counters.
func ResetToolCountersForTest() { resetToolCounters() }

// StartsByReason counts the ledger's start rows by why the daemon started:
// the reason of the exit row that closed the previous start ("first" for
// the ledger's first start; "unknown" when nothing closed it and no
// synthesized unclean exit was written). rows are in ledger order.
func StartsByReason(rows []LifecycleRow) map[string]int {
	out := map[string]int{}
	seenStart := false
	lastExit := ""
	for _, r := range rows {
		switch r.Row {
		case RowExit:
			lastExit = r.Reason
		case RowAdopt:
			lastExit = ExitAdopt
		case RowStart:
			switch {
			case !seenStart && lastExit == "":
				out["first"]++
			case lastExit == "":
				out["unknown"]++
			default:
				out[lastExit]++
			}
			seenStart, lastExit = true, ""
		}
	}
	return out
}
