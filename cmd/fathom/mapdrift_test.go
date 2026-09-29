package main

// mapdrift_test.go — adversarial checks on the read side of exploration maps
// (XM-2, XM-5, XM-9, XM-11): an edit inside an anchored function is flagged at
// every read and at every budget, a revert reads fresh again (nothing is
// cached), and include cycles that reach the log behind the write-time check
// (two writers, each validating its own snapshot) are refused at read time
// rather than looped over or served.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMapShowFlagsAnEditedAnchoredFunction edits the body of an anchored
// function in place (same line count, so the filed span still exists and the
// symbol still resolves to it) and asserts `map show` flags both the span entry
// and the symbol entry stale with a diff size, in the text at every budget and
// in the JSON, and that the edit hook's pointer line counts them as changed.
// Reverting the edit reads fresh again: every read re-resolves.
func TestMapShowFlagsAnEditedAnchoredFunction(t *testing.T) {
	repo := mapRepo(t)
	oldDir := mapPointerDir
	mapPointerDir = t.TempDir()
	t.Cleanup(func() { mapPointerDir = oldDir })
	// The index still resolves a.Handle to lines 3-5 after the edit: the span
	// did not move, only its text changed.
	syms := fakeSyms{"a.Handle": {"a.go", 3, 5}}
	withMapSymbols(t, syms)

	if code, _, errs := runCLI(t, mapCLI, "new", "edits", "--purpose", "Where does Handle decide its error?", "--repo", repo); code != 0 {
		t.Fatalf("map new: %s", errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "append", "edits", "--anchor", "a.go#L3-L5", "--why", "the handler body", "--repo", repo); code != 0 {
		t.Fatalf("map append span: %s", errs)
	}
	if _, err := appendMapEntry(mapEntryRequest{Map: "edits", Anchor: "a.Handle", Why: "the handler by name", Root: repo, Now: time.Now()}, syms); err != nil {
		t.Fatalf("append symbol: %v", err)
	}

	show := func(args ...string) string {
		t.Helper()
		code, out, errs := runCLI(t, mapCLI, append([]string{"show", "edits", "--repo", repo}, args...)...)
		if code != 0 {
			t.Fatalf("map show %v: exit %d: %s", args, code, errs)
		}
		return out
	}
	if out := show(); !strings.Contains(out, "2 entries from 1 map: 2 fresh, 0 re-resolved, 0 stale") {
		t.Fatalf("before the edit both entries read fresh:\n%s", out)
	}

	body := func(ret string) string {
		return strings.Replace(mapFixtureGo, "\treturn nil\n", "\t"+ret+"\n", 1)
	}
	edited := body(`return errString("denied")`)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	out := show()
	if !strings.Contains(out, "2 entries from 1 map: 0 fresh, 0 re-resolved, 2 stale") {
		t.Errorf("after the edit the counts must say stale:\n%s", out)
	}
	if strings.Contains(out, "[fresh]") {
		t.Errorf("an edited function is served as fresh:\n%s", out)
	}
	if strings.Count(out, "[STALE +1/-1 lines since filing") != 2 {
		t.Errorf("both stale entries carry their diff size (+1/-1):\n%s", out)
	}
	if !strings.Contains(out, "a.go#L3-L5 a.Handle — the handler by name [STALE +1/-1 lines since filing: symbol a.Handle changed since it was filed]") {
		t.Errorf("the symbol entry resolves to its unchanged span and is still stale:\n%s", out)
	}

	// JSON: the same verdict as data.
	var v struct {
		Counts struct{ Fresh, Stale int } `json:"counts"`
		Items  []struct {
			State          string
			Added, Removed int
			DiffKnown      bool `json:"diff_known"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(show("--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Counts.Fresh != 0 || v.Counts.Stale != 2 || len(v.Items) != 2 {
		t.Errorf("json counts = %+v over %d items, want 0 fresh, 2 stale", v.Counts, len(v.Items))
	}
	for _, it := range v.Items {
		if it.State != "stale" || !it.DiffKnown || it.Added != 1 || it.Removed != 1 {
			t.Errorf("json item = %+v, want stale with a known +1/-1 diff", it)
		}
	}

	// Every budget: the stale verdict is never cut while an entry is shown,
	// and once the header fits the counts always say stale.
	full := show("--budget", "0")
	head, _, _ := strings.Cut(full, "\n")
	for budget := len(head) + 1; budget <= len(full)+10; budget++ {
		out := show("--budget", fmt.Sprint(budget))
		if len(out) > budget {
			t.Fatalf("budget %d: %d bytes", budget, len(out))
		}
		if strings.Contains(out, "[fresh]") {
			t.Fatalf("budget %d: an edited entry served as fresh:\n%s", budget, out)
		}
		if strings.Contains(out, "a.go#L3-L5") && !strings.Contains(out, "a.go#L3-L5 (stale)") && !strings.Contains(out, "[STALE") {
			t.Fatalf("budget %d: the entry is shown without its stale mark:\n%s", budget, out)
		}
		if strings.Count(out, "\n") >= 2 && !strings.Contains(out, "2 stale") {
			t.Fatalf("budget %d: the counts line is shown without the stale count:\n%s", budget, out)
		}
	}

	// The edit hook's pointer: the file re-hash counts the entry as changed.
	line := mapPointerLine(repo, "sess-edit", mapTouch{Files: []string{"a.go"}}, time.Now())
	if !strings.Contains(line, "fathom map edits:") || !strings.Contains(line, "changed since filed") || strings.Contains(line, "all fresh") {
		t.Errorf("the pointer line on an edit inside the footprint must name the drift: %q", line)
	}

	// Revert: the same map reads fresh again. Nothing was cached.
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(mapFixtureGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := show(); !strings.Contains(out, "0 re-resolved, 0 stale") || strings.Contains(out, "STALE") {
		t.Errorf("after the revert the entry reads fresh again:\n%s", out)
	}
}

// TestMapShowSymbolEntryStaleWhenOnlyTheSymbolResolves pins the harder case:
// the function grew and moved, the filed span now holds other text, and the
// index follows the symbol to its new span, whose text also differs. The entry
// is stale at its new span with a diff size, never "moved" (a move keeps the
// text) and never fresh.
func TestMapShowSymbolEntryStaleWhenOnlyTheSymbolResolves(t *testing.T) {
	repo := mapRepo(t)
	if _, err := appendMapEntry(mapEntryRequest{Map: "sym", Anchor: "a.Handle", Why: "the handler", Root: repo, Now: time.Now()},
		fakeSyms{"a.Handle": {"a.go", 3, 5}}); err != nil {
		t.Fatal(err)
	}
	src := "package a\n\nimport \"errors\"\n\nfunc Handle() error {\n\tif true {\n\t\treturn errors.New(\"x\")\n\t}\n\treturn nil\n}\n\nfunc Other() {}\n"
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	withMapSymbols(t, fakeSyms{"a.Handle": {"a.go", 5, 10}})
	code, out, errs := runCLI(t, mapCLI, "show", "sym", "--repo", repo)
	if code != 0 {
		t.Fatalf("show: %s", errs)
	}
	if !strings.Contains(out, "a.go#L5-L10 a.Handle — the handler [STALE +") || !strings.Contains(out, "symbol a.Handle changed since it was filed") {
		t.Errorf("a changed symbol at a new span is stale there, with its diff size:\n%s", out)
	}
	if strings.Contains(out, "moved from") || strings.Contains(out, "[fresh]") {
		t.Errorf("a changed symbol is neither moved nor fresh:\n%s", out)
	}
}

// TestMapReadsRefuseACycleThatReachedTheLog: `map include` refuses a cycle
// against the log it read, but two writers can each pass that check (A
// includes B, B includes A). The read side must not loop, must not serve the
// cycle, and must not crash a hook.
func TestMapReadsRefuseACycleThatReachedTheLog(t *testing.T) {
	repo := mapRepo(t)
	oldDir := mapPointerDir
	mapPointerDir = t.TempDir()
	t.Cleanup(func() { mapPointerDir = oldDir })
	for _, id := range []string{"ca", "cb"} {
		if code, _, errs := runCLI(t, mapCLI, "append", id, "--anchor", "a.go#L1-L1", "--why", "the package clause", "--repo", repo); code != 0 {
			t.Fatalf("append %s: %s", id, errs)
		}
	}
	// Writer one: ca includes cb, checked and stored.
	if code, _, errs := runCLI(t, mapCLI, "include", "ca", "cb", "--repo", repo); code != 0 {
		t.Fatalf("include ca cb: %s", errs)
	}
	// The CLI refuses the closing edge.
	if code, _, errs := runCLI(t, mapCLI, "include", "cb", "ca", "--repo", repo); code == 0 || !strings.Contains(errs, "include cycle") {
		t.Fatalf("the CLI must refuse the closing edge: exit %d %q", code, errs)
	}
	// Writer two raced it: its header (checked against a snapshot without the
	// first edge) lands anyway.
	set := loadMapSet(repo, time.Now())
	cb := set.maps["cb"]
	cb.Includes = []string{"ca"}
	row := mapHeaderRow(cb, set.headers["cb"], "", repo, time.Now())
	if err := appendThreadItem(row); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		code, out, errs := runCLI(t, mapCLI, "show", "ca", "--repo", repo)
		if code == 0 || !strings.Contains(errs, "include cycle") {
			t.Errorf("show over a stored cycle must refuse it: exit %d\nout: %s\nerr: %s", code, out, errs)
		}
		if code, _, errs := runCLI(t, mapCLI, "compose", "ca", "cb", "--id-only", "--repo", repo); code == 0 {
			t.Errorf("compose over a stored cycle must refuse it: %s", errs)
		}
		if code, _, errs := runCLI(t, mapCLI, "list", "--repo", repo); code != 0 {
			t.Errorf("list over a stored cycle: exit %d %s", code, errs)
		}
		_ = mapPointerLine(repo, "sess-cycle", mapTouch{Files: []string{"a.go"}}, time.Now())
		_ = subagentMapLine(repo, "sess-parent", "agent-1", time.Now())
		_ = briefMapLines(nil, repo, time.Now())
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("a stored include cycle hung a map read")
	}
}
