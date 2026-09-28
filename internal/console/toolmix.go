package console

// toolmix.go — PE-18 (docs/design/progress-and-economics.md §5): the
// tool-call mix (each tool name's share of the calls counted) and the
// cache hit rate (cache-read tokens over cache-read-plus-fresh-input
// tokens — the share of this window's context tokens served from cache
// rather than processed fresh), per session and per plan. Pure: ToolMix
// takes counts a caller already has (PE-13's foldTranscriptCost summed
// for one session, or aggregated across a plan's sessions) and reads no
// transcript or ledger itself.

import (
	"fmt"
	"sort"
)

// ToolMixEntry is one tool name's share of the calls ToolMix was given.
type ToolMixEntry struct {
	Tool  string  `json:"tool"`
	Calls int     `json:"calls"`
	Share float64 `json:"share"`
}

// ToolMixModel is the view.
type ToolMixModel struct {
	Mix        []ToolMixEntry `json:"mix"`
	TotalCalls int            `json:"total_calls"`
	CacheHit   Value[float64] `json:"cache_hit_rate"`
}

// ToolMix computes the tool-call mix from calls (tool name to call count)
// and the cache hit rate from cacheRead and freshIn token counts. An empty
// or all-zero calls map yields no mix entries — never an empty slice
// rendered as 0% for every tool. cacheRead == freshIn == 0 likewise
// leaves CacheHit absent rather than a fabricated 0% or 100% (law 12).
func ToolMix(calls map[string]int, cacheRead, freshIn int) ToolMixModel {
	var m ToolMixModel
	for _, n := range calls {
		m.TotalCalls += n
	}
	if m.TotalCalls > 0 {
		m.Mix = make([]ToolMixEntry, 0, len(calls))
		for tool, n := range calls {
			if n == 0 {
				continue
			}
			m.Mix = append(m.Mix, ToolMixEntry{Tool: tool, Calls: n, Share: round2(float64(n) / float64(m.TotalCalls))})
		}
		sort.Slice(m.Mix, func(i, j int) bool {
			if m.Mix[i].Calls != m.Mix[j].Calls {
				return m.Mix[i].Calls > m.Mix[j].Calls
			}
			return m.Mix[i].Tool < m.Mix[j].Tool
		})
	}
	if denom := cacheRead + freshIn; denom == 0 {
		m.CacheHit = Absent[float64]("no cache-read or fresh-input tokens in this window")
	} else {
		m.CacheHit = Measured(round2(float64(cacheRead)/float64(denom)), "the transcript fold").
			WithDenominator(fmt.Sprintf("%d cache-read + fresh-input tokens", denom))
	}
	return m
}
