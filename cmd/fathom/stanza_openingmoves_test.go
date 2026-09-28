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
	stanza := fathomStanza("fathom", false)

	// SR-34 (surface reckoning F-86, design §1.1) rewrote the table's
	// arrow-chains to name only the default eight-tool floor
	// (TestStanzaNamesOnlyDefaultSurfaceTools pins that property directly);
	// this test keeps pinning the table's presence, position and the
	// session-start upgrade WF-9 added.
	for _, want := range []string{
		"Opening moves by task shape (first 2-3 calls, then edit):",
		"bug report / stack trace / failing test: search -> symbol_context -> read_source",
		"feature/change: search -> obligations -> symbol_context",
		"review/verify: review_diff -> obligations -> verify_plan",
		"refactor/delete: blast_radius -> review_diff -> verify_plan",
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

	// Position: the table sits between the tool list and the auto-refresh
	// paragraph (SR-34 replaced the old `ingest` instruction with one
	// explaining that a read auto-refreshes — nothing to call).
	toolListIdx := strings.Index(stanza, "Read files to *edit* them")
	tableIdx := strings.Index(stanza, "Opening moves by task shape")
	refreshIdx := strings.Index(stanza, "The index refreshes itself")
	if toolListIdx < 0 || tableIdx < 0 || refreshIdx < 0 {
		t.Fatalf("could not locate all three anchors: toolList=%d table=%d refresh=%d", toolListIdx, tableIdx, refreshIdx)
	}
	if !(toolListIdx < tableIdx && tableIdx < refreshIdx) {
		t.Errorf("opening-moves table must sit between the tool list and the auto-refresh paragraph: toolList=%d table=%d refresh=%d", toolListIdx, tableIdx, refreshIdx)
	}
}
