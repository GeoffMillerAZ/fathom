package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// writeRouterThinFixture writes a .fathom/concerns.cue carrying a
// router_thin-shaped max_lines concern against CLAUDE.md at budget lines,
// and a CLAUDE.md whose only content is n filler "chrome" lines (nothing
// fathom owns yet) ending in a trailing newline — the fresh-insert shape
// `fathom init` sees on a repo it has never wired.
func writeRouterThinFixture(t *testing.T, root string, budget, chromeLines int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	cue := fmt.Sprintf(`concerns: {
	router_thin: {
		want:  "the root CLAUDE.md stays a router"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: %d}
	}
}
`, budget)
	if err := os.WriteFile(filepath.Join(root, ".fathom", "concerns.cue"), []byte(cue), 0o644); err != nil {
		t.Fatal(err)
	}
	var chrome strings.Builder
	for i := 0; i < chromeLines; i++ {
		fmt.Fprintf(&chrome, "chrome line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(chrome.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// applyStanzaOnce runs exactly what cmdInit's stanza step runs — build the
// regions via stanzaRegionsFit, apply them — against a single file, and
// returns the notes plus the resulting line count.
func applyStanzaOnce(t *testing.T, root string) (lines int, notes []string) {
	t.Helper()
	regions, stanzaNotes := stanzaRegionsFit(root, []string{"CLAUDE.md"}, harness.ClaudeCode, "svc", false)
	spec := wiring.Spec{Regions: regions}
	if _, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{}); err != nil {
		t.Fatalf("ApplyWiring: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	return countTextLines(string(data)), stanzaNotes
}

// TestStanzaRefresh_IdempotentLineCount pins O57 item 3's first property:
// re-rendering the fathom region over an existing region must not change
// the file's line count when the stanza itself has not changed. 73 chrome
// lines is exactly what a fresh insert of the untrimmed (drop-0) stanza —
// 46 lines with its markers plus the one blank separator line a non-empty
// base file needs, as of the identifier-clean extended-disclosure rewrite
// (the config custodian's stanza_tools_served check flags any off-surface
// tool cited by name, so the disclosure sentence describes the wider verb
// set instead of naming it, then the 2026-10-15 rewrite without internal ids) — fills to a 120-line router_thin budget —
// this repo's own CLAUDE.md shape — so the first apply must land at 120
// and the second, re-run over what the first one wrote, must leave it at
// 120 too (and must not report anything dropped, since drop 0 already
// fit).
func TestStanzaRefresh_IdempotentLineCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeRouterThinFixture(t, root, 120, 73)

	lines1, notes1 := applyStanzaOnce(t, root)
	if lines1 != 120 {
		t.Fatalf("first render: got %d lines, want 120", lines1)
	}
	if len(notes1) != 0 {
		t.Errorf("first render: unexpected trim notes %v (nothing should need dropping at exactly-budget size)", notes1)
	}

	before, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines2, notes2 := applyStanzaOnce(t, root)
	if lines2 != 120 {
		t.Fatalf("second render: got %d lines, want 120 (line count must not drift on an unchanged stanza)", lines2)
	}
	if len(notes2) != 0 {
		t.Errorf("second render: unexpected trim notes %v", notes2)
	}
	after, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("second render rewrote bytes that the first render already got right — not idempotent")
	}
}

// TestStanzaRefresh_TrimsToFitRouterThinBudget pins O57 item 3's second
// property: a file close enough to a router_thin budget that the FULL
// stanza would trip it (here, 3 lines past what a fresh insert of the
// untrimmed stanza would need to land exactly on budget) gets a trimmed
// stanza instead — one that fits — and the generator names what it
// dropped rather than silently shrinking the stanza's actual rules.
func TestStanzaRefresh_TrimsToFitRouterThinBudget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	// 73 chrome lines exactly fits an untrimmed insert to 120 (see the
	// idempotency test above); 76 is 3 lines past that, so the untrimmed
	// stanza would land the file at 123 — over the 120-line budget.
	writeRouterThinFixture(t, root, 120, 76)

	lines, notes := applyStanzaOnce(t, root)
	if lines > 120 {
		t.Fatalf("got %d lines, want <= 120 (router_thin budget) — the stanza should have been trimmed to fit", lines)
	}
	if len(notes) == 0 {
		t.Fatal("stanza was trimmed but nothing was reported")
	}
	joined := strings.Join(notes, " ")
	if !strings.Contains(joined, "live-example") {
		t.Errorf("trim report %q does not name the live-example (opening-moves) drop", joined)
	}
	if strings.Contains(joined, "continuity paragraph") {
		t.Errorf("trim report %q drops the continuity paragraph too — only the first tier (live-example) should have been needed to fit", joined)
	}

	// The body actually written must be missing the opening-moves table
	// and must still carry everything load-bearing: the routing
	// paragraph, the Obligations contract, and the verify command.
	data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "Opening moves by task shape") {
		t.Error("the trimmed stanza still carries the opening-moves table")
	}
	for _, want := range []string{
		"This repo is indexed by fathom as service",
		"Obligations — governing rules for the file you are about to touch",
		"fathom verify --service",
		"Continuity: run `fathom prime`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("trimmed stanza lost required text %q", want)
		}
	}
}
