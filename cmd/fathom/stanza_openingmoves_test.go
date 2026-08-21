package main

// stanza_openingmoves_test.go — WF-9 (docs/plans/work-frontier.md /
// docs/vision/24-work-frontier.md §5): the stanza gains a task-shaped
// opening-moves table between the tool list and the ingest paragraph, and
// its Codex session-start instruction upgrades from `fathom thread brief`
// to `fathom prime`.

import (
	"strings"
	"testing"
)

func TestStanzaCarriesOpeningMovesAndPrime(t *testing.T) {
	// Exercise the rendering function, not the raw const — the rendered
	// stanza is what applyStanza writes to CLAUDE.md/AGENTS.md, and the
	// call is what wires WF-9's reachability contract.
	stanza := fathomStanza("fathom")

	for _, want := range []string{
		"Opening moves by task shape (first 2-3 calls, then edit):",
		"debug/trace: find_symbol -> find_callers/references -> symbol_context",
		"feature/change: repo_map -> pack_context(concern) -> references",
		"review/verify: verify_plan -> obligations -> references on changed symbols",
		"refactor: references -> outline -> symbol_context",
		"fathom prime",
	} {
		if !strings.Contains(stanza, want) {
			t.Errorf("stanza missing %q", want)
		}
	}

	// The old Codex session-start instruction must be gone, not merely
	// supplemented — WF-9 upgrades it, it does not duplicate it.
	if strings.Contains(stanza, "run `fathom thread brief` at session start") {
		t.Error("stanza still carries the old `fathom thread brief` session-start instruction — it must upgrade to `fathom prime`")
	}

	// Position: the table sits between the tool list and the ingest
	// paragraph.
	toolListIdx := strings.Index(stanza, "Read files to *edit* them")
	tableIdx := strings.Index(stanza, "Opening moves by task shape")
	ingestIdx := strings.Index(stanza, "Refresh the index via the `ingest` MCP tool")
	if toolListIdx < 0 || tableIdx < 0 || ingestIdx < 0 {
		t.Fatalf("could not locate all three anchors: toolList=%d table=%d ingest=%d", toolListIdx, tableIdx, ingestIdx)
	}
	if !(toolListIdx < tableIdx && tableIdx < ingestIdx) {
		t.Errorf("opening-moves table must sit between the tool list and the ingest paragraph: toolList=%d table=%d ingest=%d", toolListIdx, tableIdx, ingestIdx)
	}
}
