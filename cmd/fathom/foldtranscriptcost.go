package main

// foldtranscriptcost.go — PE-13 (docs/design/progress-and-economics.md §5,
// "outcome economics"): the cost fields a session's proven outcome is
// priced against — tokens in and out, cache reads and writes, and tool
// calls — read from the SAME incremental transcript fold the Stop hook
// already maintains per session (RT-132's offset stamp, sessionledger.go).
// This adds no new read: readTranscriptFromOffset already pays only for
// the bytes a session gained since its last Stop, so folding cost here
// costs nothing beyond what that hook already spends. Claude Code
// transcripts only; the Codex session cost fold (PE-14) lives in
// internal/harness, on the harness port.

// transcriptCost is one session's cost-relevant transcript fields.
// Available is false — and every count is 0 — when the transcript carried
// no usage data at all (transcriptSummary.TokensNote explains why), so a
// reader never mistakes "not measured" for "measured as zero" (law 12).
type transcriptCost struct {
	TokensIn          int
	TokensOut         int
	TokensCacheRead   int
	TokensCacheCreate int
	// ToolCalls counts every tool_use block in the session, any tool name
	// — the whole-session count outcome economics prices, not only the
	// exploration ledger's raw-discovery/fathom split.
	ToolCalls int
	Available bool
}

// foldTranscriptCost extracts transcriptCost for sessionID's transcript at
// path via the same per-session offset stamp readTranscriptFromOffset
// maintains for the Stop hook: repeated calls across a session's turns
// each fold only the bytes appended since the last one.
func foldTranscriptCost(sessionID, path string) transcriptCost {
	return transcriptCostFromSummary(readTranscriptFromOffset(sessionID, path))
}

// transcriptCostFromSummary derives transcriptCost from an
// already-computed transcriptSummary — incremental or whole-file, both
// fold the same way and so agree field for field.
func transcriptCostFromSummary(s transcriptSummary) transcriptCost {
	c := transcriptCost{ToolCalls: s.ToolCallsTotal}
	if s.TokensIn == nil {
		return c // TokensNote carries why; Available stays false.
	}
	c.TokensIn = *s.TokensIn
	c.TokensOut = *s.TokensOut
	c.TokensCacheRead = *s.TokensCacheRead
	c.TokensCacheCreate = *s.TokensCacheCreate
	c.Available = true
	return c
}
