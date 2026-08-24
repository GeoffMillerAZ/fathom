package main

// cost_panel.go — BA-8 (docs/plans/ambient-fathom.md, docs/design/
// budgeted-answers.md §6): the Cost panel on /observatory/impact. Folds
// the SAME call-log entries buildObservatoryModel already reads (BA-6's
// response_bytes/elided_count fields, instrument_adoption.go's
// adoptionCallEntry) — no new read, no new log, the aggregation seam BA-6's
// design note asks for, rendered.

import "sort"

// costEstBytesPerToken is the same ~4-bytes-per-token estimate the packer
// and the mcp package use (internal/context/packer.tokenEstimate). Labeled
// ESTIMATE everywhere it renders — never presented as a measured count.
const costEstBytesPerToken = 4

// costToolRow is one tool's response-cost accounting over the call-log
// window: what fathom put into agent contexts on that tool's behalf, and
// how often it had to elide.
type costToolRow struct {
	Tool       string
	Calls      int
	TotalBytes int
	EstTokens  int // TotalBytes / costEstBytesPerToken — an ESTIMATE

	// TruncatingCalls is calls whose result carried an "elided" object at
	// all (present, whether or not it disclosed a cut) — the denominator
	// for ElisionRatePct. ElidedCalls is the numerator: calls that actually
	// elided something. A tool with TruncatingCalls==0 never renders a
	// rate — it means "not a truncating call in this window", not "0%".
	TruncatingCalls int
	ElidedCalls     int
}

// ElisionRatePct returns the elision rate as a whole-number percent and
// whether it is meaningful (TruncatingCalls > 0).
func (r costToolRow) ElisionRatePct() (int, bool) {
	if r.TruncatingCalls == 0 {
		return 0, false
	}
	return r.ElidedCalls * 100 / r.TruncatingCalls, true
}

// costPanel is BA-8's Cost panel model.
type costPanel struct {
	Tools          []costToolRow
	TotalBytes     int
	TotalEstTokens int

	// SchemaTax is BA-7's number: the standing context cost of the agent
	// surface, carried by every connected session before any work happens.
	SchemaTaxBytes int
	SchemaTaxTools int
	// SchemaTaxTrendNote states, in words, that no historical schema-byte
	// series is recorded yet (law 3: an absent trend is named, never
	// rendered as flat or zero).
	SchemaTaxTrendNote string

	// RatioTokensPerDiscoveryX10 is the one honest ratio available without
	// an A/B — tokens fathom spent per discovery action it served, ×10 for
	// one decimal of precision without floats (mirrors obsImpact.
	// ExpRatioMedianX10's convention) — nil when either side is not
	// measured. RatioNote always renders alongside it: OBSERVATIONAL, never
	// captioned as a saving (law 12), and the reason when the ratio is nil.
	RatioTokensPerDiscoveryX10 *int
	RatioNote                  string
}

// observatoryCostPanel folds already-read call-log entries into BA-8's Cost
// panel: no new read (entries is the exact slice buildObservatoryModel
// already loaded for the Q3 misses table).
func observatoryCostPanel(entries []adoptionCallEntry, schemaTools, schemaBytes int, discoveryTotal *int) costPanel {
	byTool := map[string]*costToolRow{}
	for _, e := range entries {
		if e.Tool == "" {
			continue
		}
		r := byTool[e.Tool]
		if r == nil {
			r = &costToolRow{Tool: e.Tool}
			byTool[e.Tool] = r
		}
		r.Calls++
		if e.ResponseBytes > 0 {
			r.TotalBytes += e.ResponseBytes
		}
		if e.ElidedCount != nil {
			r.TruncatingCalls++
			if *e.ElidedCount > 0 {
				r.ElidedCalls++
			}
		}
	}

	cp := costPanel{
		SchemaTaxTools: schemaTools,
		SchemaTaxBytes: schemaBytes,
		SchemaTaxTrendNote: "trend not measured yet — the panel reads the schema live on every render; " +
			"a historical series needs a recorded baseline to compare against",
	}
	rows := make([]costToolRow, 0, len(byTool))
	for _, r := range byTool {
		r.EstTokens = r.TotalBytes / costEstBytesPerToken
		cp.TotalBytes += r.TotalBytes
		cp.TotalEstTokens += r.EstTokens
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalBytes != rows[j].TotalBytes {
			return rows[i].TotalBytes > rows[j].TotalBytes
		}
		return rows[i].Tool < rows[j].Tool
	})
	cp.Tools = rows

	switch {
	case cp.TotalEstTokens == 0:
		cp.RatioNote = "no response-cost rows readable from here — the ratio needs at least one accounted call"
	case discoveryTotal == nil:
		cp.RatioNote = "OBSERVATIONAL, never a savings claim (law 12) — discovery actions are not measured, so no ratio is shown"
	case *discoveryTotal == 0:
		cp.RatioNote = "OBSERVATIONAL, never a savings claim (law 12) — 0 discovery actions recorded, so no ratio is shown"
	default:
		x10 := cp.TotalEstTokens * 10 / *discoveryTotal
		cp.RatioTokensPerDiscoveryX10 = &x10
		cp.RatioNote = "OBSERVATIONAL, never a savings claim (law 12) — tokens spent per discovery action served"
	}
	return cp
}
