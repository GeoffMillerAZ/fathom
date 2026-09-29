package main

// mapshow_test.go — `fathom map show` (XM-9) and the verbs an orchestrator uses
// to pass one map id: `map new`, `map append`, `map compose --id-only`.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// gitIn runs git in repo, failing the test on error.
func gitIn(t *testing.T, repo string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// withMapSymbols points the map verbs' symbol lookups at a table for the test,
// so no test opens the index or probes a daemon.
func withMapSymbols(t *testing.T, syms fakeSyms) {
	t.Helper()
	prev := openMapSymbols
	openMapSymbols = func(root, storePath, url, service string) mapSymbolResolver { return syms }
	t.Cleanup(func() { openMapSymbols = prev })
}

// showFixture files the maps the show tests read:
//
//	base:  E1 a.go#L3-L5 (Handle), E2 the symbol a.Other (line 7), E3 a.go#L1-L1
//	top:   T1 b.go#L1-L2, includes base
//	other: O1 a.go#L1-L1 (E3's span, another why), includes base
//
// and returns the composite id over top and other, so base is reached by two
// routes.
func showFixture(t *testing.T) (repo, composite string) {
	t.Helper()
	repo = mapRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "b.go"), []byte("package a\n\nfunc Helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "b.go")
	gitIn(t, repo, "commit", "-q", "-m", "add b.go")

	at := func(mapID, anchor, why string, syms mapSymbolResolver) {
		t.Helper()
		if _, err := appendMapEntry(mapEntryRequest{Map: mapID, Anchor: anchor, Why: why, Root: repo, Now: time.Now()}, syms); err != nil {
			t.Fatalf("append %s %s: %v", mapID, anchor, err)
		}
	}
	at("base", "a.go#L3-L5", "the handler", nil)
	at("base", "a.Other", "the sibling", fakeSyms{"a.Other": {"a.go", 7, 7}})
	at("base", "a.go#L1-L1", "package clause", nil)
	at("top", "b.go#L1-L2", "the helper file", nil)
	at("other", "a.go#L1-L1", "where the package is declared", nil)
	for _, top := range []string{"top", "other"} {
		if code, _, errs := runCLI(t, mapCLI, "include", top, "base", "--repo", repo); code != 0 {
			t.Fatalf("include %s: %s", top, errs)
		}
	}
	code, out, errs := runCLI(t, mapCLI, "compose", "top", "other", "--id-only", "--repo", repo)
	if code != 0 {
		t.Fatalf("compose: %s", errs)
	}
	return repo, strings.TrimSpace(out)
}

// drift edits a.go so Handle's span no longer hashes (line 4 becomes two
// lines) and Other moves from line 7 to line 9 unchanged.
func drift(t *testing.T, repo string) {
	t.Helper()
	src := "package a\n\nfunc Handle() error {\n\tv := 1\n\t_ = v\n\treturn nil\n}\n\nfunc Other() {}\n"
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

var driftedSyms = fakeSyms{"a.Other": {"a.go", 9, 9}}

// TestCmdMapShowReResolvesWithinBudget pins XM-9 through the product verb:
// `map show ID --budget N` re-resolves every anchor at read time (fresh, moved
// by its symbol key, or stale with its diff size), expands a base reached by two
// routes once, never exceeds the budget, and names what it cut.
func TestCmdMapShowReResolvesWithinBudget(t *testing.T) {
	repo, id := showFixture(t)
	withMapSymbols(t, driftedSyms)

	// Read time one: nothing has changed, so nothing is stale.
	code, out, errs := runCLI(t, mapCLI, "show", id, "--repo", repo)
	if code != 0 {
		t.Fatalf("show: exit %d: %s", code, errs)
	}
	if !strings.Contains(out, "4 entries from 4 maps: 4 fresh, 0 re-resolved, 0 stale") {
		t.Errorf("before any edit the counts should be all fresh:\n%s", out)
	}

	// The code drifts. The same id, read again, says so: no cached answer.
	drift(t, repo)
	code, out, errs = runCLI(t, mapCLI, "show", id, "--repo", repo)
	if code != 0 {
		t.Fatalf("show after drift: exit %d: %s", code, errs)
	}
	if !strings.Contains(out, "4 entries from 4 maps: 2 fresh, 1 re-resolved, 1 stale; 1 with conflicting annotations") {
		t.Errorf("the counts after drift:\n%s", out)
	}
	for _, want := range []string{
		"a.go#L3-L5 — the handler [STALE +2/-2 lines",             // Handle changed: never served as if fresh
		"a.go#L9-L9 a.Other — the sibling [moved from a.go#L7-L7", // re-resolved by symbol key
		"b.go#L1-L2 — the helper file [fresh]",                    // untouched
		"a.go#L1-L1 — where the package is declared [fresh]",      // the nearest include's why leads
		"also: package clause (map base)",                         // the farther why is kept with its provenance
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}
	// base is included by both top and other: its entries appear once.
	if n := strings.Count(out, "the handler"); n != 1 {
		t.Errorf("the shared base's entry appears %d times, want once:\n%s", n, out)
	}
	if !strings.HasPrefix(out, "map "+id+" — composite of top, other\n") {
		t.Errorf("the header of a composite names its references:\n%s", out)
	}
	if strings.Contains(out, "cut ") {
		t.Errorf("a default budget of %d bytes fits four entries, yet the output cut:\n%s", mapShowDefaultBudget, out)
	}
	if len(out) > mapShowDefaultBudget {
		t.Errorf("output is %d bytes, over the default %d", len(out), mapShowDefaultBudget)
	}

	// Every budget is kept, every entry is accounted for, and the footer names
	// what it cut. The counts still cover the whole map: freshness is checked
	// for every entry, shown or not.
	_, full, _ := runCLI(t, mapCLI, "show", id, "--budget", "0", "--repo", repo)
	labels := []string{"b.go#L1-L2", "a.go#L1-L1", "a.go#L3-L5", "a.go#L9-L9"}
	cutSeen := 0
	for budget := 250; budget <= len(full); budget += 25 {
		code, out, errs := runCLI(t, mapCLI, "show", id, "--budget", fmt.Sprint(budget), "--repo", repo)
		if code != 0 {
			t.Fatalf("budget %d: exit %d: %s", budget, code, errs)
		}
		if len(out) > budget {
			t.Errorf("budget %d: output is %d bytes:\n%s", budget, len(out), out)
		}
		body, footer, _ := strings.Cut(out, "\ncut ")
		if !strings.Contains(body, "4 entries from 4 maps: 2 fresh, 1 re-resolved, 1 stale") {
			t.Errorf("budget %d: the counts must cover the whole map, shown or not:\n%s", budget, out)
		}
		shown := 0
		for _, l := range labels {
			if strings.Contains(body, l) {
				shown++
			}
		}
		if shown == 4 {
			if footer != "" {
				t.Errorf("budget %d: everything shown, yet a cut footer:\n%s", budget, out)
			}
			continue
		}
		cutSeen++
		m := regexp.MustCompile(`cut (\d+) of 4 entries to stay within (\d+) bytes`).FindStringSubmatch("cut " + footer)
		if m == nil || m[1] != fmt.Sprint(4-shown) || m[2] != fmt.Sprint(budget) {
			t.Errorf("budget %d: %d of 4 entries shown, footer %q does not say cut %d of 4 within %d:\n%s", budget, shown, footer, 4-shown, budget, out)
		}
	}
	if cutSeen < 3 {
		t.Fatalf("the budget sweep cut in only %d cases; the fixture no longer exercises the cut", cutSeen)
	}

	// A cut that has room names what it cut, with its state: one byte under
	// the full text cuts only the last entry and names it.
	_, out, _ = runCLI(t, mapCLI, "show", id, "--budget", fmt.Sprint(len(full)-1), "--repo", repo)
	body, footer, found := strings.Cut(out, "\ncut ")
	if !found || !strings.Contains(footer, "of 4 entries") || !strings.Contains(footer, "a.go#L9-L9") {
		t.Fatalf("one byte under the full text must cut the last entry and name it:\n%s", out)
	}
	if strings.Count(body, "\n") >= strings.Count(full, "\n") {
		t.Errorf("the cut kept every line of the full text:\n%s", out)
	}
	// A stale entry that is cut is named as stale, never as if it were fresh.
	_, out, _ = runCLI(t, mapCLI, "show", id, "--budget", "330", "--repo", repo)
	if _, footer, found := strings.Cut(out, "\ncut "); !found || (strings.Contains(footer, "a.go#L3-L5") && !strings.Contains(footer, "a.go#L3-L5 (stale)")) {
		t.Errorf("a cut stale entry must be named stale:\n%s", out)
	}

	// The deterministic contract: the same read prints the same bytes.
	if _, again, _ := runCLI(t, mapCLI, "show", id, "--budget", "0", "--repo", repo); again != full {
		t.Errorf("two reads differ:\n%s\n---\n%s", full, again)
	}

	// --budget 0 lifts the cap; a negative one is refused; an unknown map is named.
	if strings.Contains(full, "\ncut ") {
		t.Errorf("--budget 0 must show everything:\n%s", full)
	}
	if code, _, errs := runCLI(t, mapCLI, "show", id, "--budget", "-5", "--repo", repo); code == 0 || !strings.Contains(errs, "budget") {
		t.Errorf("a negative budget: exit %d, %q", code, errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "show", "nope", "--repo", repo); code == 0 || !strings.Contains(errs, `unknown map "nope"`) {
		t.Errorf("an unknown map: exit %d, %q", code, errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "show", "--repo", repo); code == 0 || !strings.Contains(errs, "usage") {
		t.Errorf("show without an id: exit %d, %q", code, errs)
	}
}

// TestCmdMapShowJSONAndWideMap reads a map far over the default budget: the
// default budget holds, the JSON view says which entries were shown and which
// were cut, and every entry is accounted for either way.
func TestCmdMapShowJSONAndWideMap(t *testing.T) {
	repo := mapRepo(t)
	var b strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "// line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(repo, "wide.go"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "wide.go")
	gitIn(t, repo, "commit", "-q", "-m", "wide")
	if code, _, errs := runCLI(t, mapCLI, "new", "wide", "--purpose", "Which lines of wide.go matter?", "--repo", repo); code != 0 {
		t.Fatalf("new: %s", errs)
	}
	for i := 1; i <= 30; i++ {
		if code, _, errs := runCLI(t, mapCLI, "append", "wide", "--anchor", fmt.Sprintf("wide.go#L%d-L%d", i, i),
			"--why", fmt.Sprintf("line %d is one of thirty reasons this map is over its budget", i), "--repo", repo); code != 0 {
			t.Fatalf("append %d: %s", i, errs)
		}
	}

	code, out, errs := runCLI(t, mapCLI, "show", "wide", "--repo", repo)
	if code != 0 {
		t.Fatalf("show: %s", errs)
	}
	if len(out) > 1200 {
		t.Errorf("default budget: %d bytes", len(out))
	}
	if !strings.HasPrefix(out, "map wide — Which lines of wide.go matter?\n30 entries from 1 map: 30 fresh, 0 re-resolved, 0 stale\n") {
		t.Errorf("header and counts:\n%s", out)
	}
	if !strings.Contains(out, "\ncut ") || !strings.Contains(out, " of 30 entries to stay within 1200 bytes") {
		t.Errorf("a 30-entry map cannot fit 1200 bytes; the cut must be named:\n%s", out)
	}

	code, js, errs := runCLI(t, mapCLI, "show", "wide", "--json", "--repo", repo)
	if code != 0 {
		t.Fatalf("show --json: %s", errs)
	}
	var v struct {
		ID      string `json:"id"`
		Purpose string `json:"purpose"`
		Budget  int    `json:"budget"`
		Bytes   int    `json:"bytes"`
		Counts  struct {
			Entries, Fresh, ReResolved, Stale int
		} `json:"counts"`
		Items []struct {
			Anchor, Why, State string
		} `json:"items"`
		Cut []struct {
			Anchor, State string
		} `json:"cut"`
	}
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		t.Fatalf("json: %v\n%s", err, js)
	}
	if v.ID != "wide" || v.Budget != 1200 || v.Bytes != len(out) || v.Counts.Entries != 30 || v.Counts.Fresh != 30 {
		t.Errorf("json header = %+v (text was %d bytes)", v, len(out))
	}
	if len(v.Items)+len(v.Cut) != 30 || len(v.Cut) == 0 || v.Items[0].Anchor != "wide.go#L1-L1" || v.Items[0].State != "fresh" {
		t.Errorf("json items %d + cut %d, first %+v", len(v.Items), len(v.Cut), v.Items)
	}
	code, js, errs = runCLI(t, mapCLI, "show", "wide", "--budget", "0", "--json", "--repo", repo)
	var whole struct {
		Items []json.RawMessage `json:"items"`
		Cut   []json.RawMessage `json:"cut"`
	}
	if err := json.Unmarshal([]byte(js), &whole); code != 0 || err != nil || len(whole.Items) != 30 || len(whole.Cut) != 0 {
		t.Errorf("--budget 0 --json: exit %d, err %v, %d items, %d cut, %s", code, err, len(whole.Items), len(whole.Cut), errs)
	}
}

// TestMapNewAppendComposeIDOnly is the orchestrator's path: create a map with
// its purpose line, append entries as a sub-agent, and mint the one id.
func TestMapNewAppendComposeIDOnly(t *testing.T) {
	repo := mapRepo(t)

	// A purpose line is required and is one line; an id is not reused; the
	// composite prefix is reserved.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"new", "run-1"}, "purpose"},
		{[]string{"new", "run-1", "--purpose", "   "}, "purpose"},
		{[]string{"new", "run-1", "--purpose", "two\nlines"}, "one line"},
		{[]string{"new", "xm-abc", "--purpose", "q?"}, "reserved"},
		{[]string{"new", "bad id", "--purpose", "q?"}, "map id"},
		{[]string{"new", "--purpose", "q?"}, "usage"},
	} {
		code, _, errs := runCLI(t, mapCLI, append(c.args, "--repo", repo)...)
		if code == 0 || !strings.Contains(errs, c.want) {
			t.Errorf("map %v: exit %d, %q; want a refusal naming %q", c.args, code, errs, c.want)
		}
	}
	if len(mapRows(t, mapHeaderKind)) != 0 {
		t.Fatalf("a refused `map new` left a header behind")
	}

	code, out, errs := runCLI(t, mapCLI, "new", "run-1", "--purpose", "Where does a request get handled?", "--when", "before touching the handler", "--repo", repo)
	if code != 0 {
		t.Fatalf("new: %s", errs)
	}
	if !strings.Contains(out, `map "run-1"`) || !strings.Contains(out, "Where does a request get handled?") {
		t.Errorf("new printed %q", out)
	}
	h := mapRows(t, mapHeaderKind)
	if len(h) != 1 || h[0].Pointers["purpose"] != "Where does a request get handled?" || h[0].Pointers["when"] != "before touching the handler" ||
		h[0].Pointers["map_kind"] != "ephemeral" || h[0].Pointers["name"] != "" {
		t.Fatalf("header = %+v, want an unnamed ephemeral map carrying its purpose", h)
	}
	if h[0].CompletesWhen == nil || h[0].CompletesWhen.Kind != "session_ended" {
		t.Errorf("completes_when = %+v, want the session's end by default", h[0].CompletesWhen)
	}
	if code, _, errs := runCLI(t, mapCLI, "new", "run-1", "--purpose", "again", "--repo", repo); code == 0 || !strings.Contains(errs, "already exists") {
		t.Errorf("a second `map new` on one id: exit %d, %q", code, errs)
	}

	// A sub-agent appends with its id; entries need the anchor and the why.
	if code, _, errs := runCLI(t, mapCLI, "append", "run-1", "--repo", repo); code == 0 || !strings.Contains(errs, "usage: fathom map append") {
		t.Errorf("append without an anchor: exit %d, %q", code, errs)
	}
	code, out, errs = runCLI(t, mapCLI, "append", "run-1", "--anchor", "a.go#L3-L5", "--why", "the handler", "--agent", "agent-9", "--repo", repo)
	if code != 0 || !strings.Contains(out, "by agent-9") {
		t.Fatalf("append: exit %d, %q, %q", code, out, errs)
	}
	if e := mapRows(t, mapEntryKind); len(e) != 1 || e[0].Pointers["agent_id"] != "agent-9" || e[0].Pointers["map"] != "run-1" {
		t.Errorf("entry = %+v", e)
	}
	if got := len(mapRows(t, mapHeaderKind)); got != 1 {
		t.Errorf("appending to an existing map wrote another header (%d rows)", got)
	}

	// The map's own show leads with its purpose and its when-to-use line.
	code, out, errs = runCLI(t, mapCLI, "show", "run-1", "--repo", repo)
	if code != 0 || !strings.HasPrefix(out, "map run-1 — Where does a request get handled?\n") ||
		!strings.Contains(out, "a.go#L3-L5 — the handler (agent-9) [fresh]") {
		t.Errorf("show: exit %d %q %q", code, out, errs)
	}
	if !strings.Contains(out, "use when: before touching the handler") {
		t.Errorf("show should carry the when-to-use line:\n%s", out)
	}

	// compose --id-only prints exactly the id, on one line, and the id shows.
	if code, _, errs := runCLI(t, mapCLI, "new", "run-2", "--purpose", "Which helper backs it?", "--repo", repo); code != 0 {
		t.Fatalf("new run-2: %s", errs)
	}
	code, out, errs = runCLI(t, mapCLI, "compose", "run-1", "run-2", "--id-only", "--repo", repo)
	id := strings.TrimSpace(out)
	if code != 0 || !regexp.MustCompile(`^xm-[0-9a-f]+$`).MatchString(id) || strings.Contains(id, " ") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("compose --id-only: exit %d, %q, %q", code, out, errs)
	}
	if code, out, _ := runCLI(t, mapCLI, "show", id, "--repo", repo); code != 0 || !strings.Contains(out, "a.go#L3-L5 — the handler") {
		t.Errorf("the composite id does not show its maps' entries: exit %d\n%s", code, out)
	}
	// One map is already one id: --id-only hands it back unchanged.
	if _, out, _ := runCLI(t, mapCLI, "compose", "run-1", "--id-only", "--repo", repo); strings.TrimSpace(out) != "run-1" {
		t.Errorf("compose of one map --id-only = %q, want run-1", out)
	}

	// Anything else to append to a composite is refused with the way through.
	if code, _, errs := runCLI(t, mapCLI, "append", id, "--anchor", "a.go#L1-L1", "--why", "x", "--repo", repo); code == 0 || !strings.Contains(errs, "references only") {
		t.Errorf("append to a composite: exit %d, %q", code, errs)
	}
}

// TestMapShowSaysWhenTheIndexIsUnreachable: a drifted symbol entry is
// re-resolved through the index; with the index out of reach the read is
// flagged stale and says why, rather than claiming the symbol no longer
// resolves.
func TestMapShowSaysWhenTheIndexIsUnreachable(t *testing.T) {
	repo := mapRepo(t)
	if _, err := appendMapEntry(mapEntryRequest{Map: "sym", Anchor: "a.Other", Why: "the sibling", Root: repo, Now: time.Now()},
		fakeSyms{"a.Other": {"a.go", 7, 7}}); err != nil {
		t.Fatal(err)
	}
	// Nothing drifted: the index is never needed, so nothing is said about it.
	if code, out, errs := runCLI(t, mapCLI, "show", "sym", "--repo", repo, "--url", "http://127.0.0.1:1"); code != 0 ||
		strings.Contains(out, "note:") || !strings.Contains(out, "[fresh]") {
		t.Fatalf("fresh read: exit %d %s\n%s", code, errs, out)
	}
	drift(t, repo)
	code, out, errs := runCLI(t, mapCLI, "show", "sym", "--repo", repo, "--url", "http://127.0.0.1:1")
	if code != 0 {
		t.Fatalf("show: %s", errs)
	}
	if !strings.Contains(out, "note: "+mapShowIndexDownNote+"\n") || !strings.Contains(out, "[STALE") {
		t.Errorf("an unreachable index must be named beside the stale entry:\n%s", out)
	}
}

// TestMapShowFocusListsTheFocusFirst pins XM-35 (O177): `map show --focus
// <path|package>[,...]` orders the entries whose resolved file sits under a
// focus path (the file itself, or a directory and everything below it) first,
// in the map's own order, and marks every other entry "outside focus": none is
// hidden, the counts stay the whole map's, the budget cuts from the tail, and
// the same map, tree and focus print the same bytes.
func TestMapShowFocusListsTheFocusFirst(t *testing.T) {
	repo := mapRepo(t)
	for p, src := range map[string]string{
		"cmd/x/x.go":        "package x\n\nfunc X() {}\n",
		"internal/y/y.go":   "package y\n\nfunc Y() {}\n",
		"internal/yy/yy.go": "package yy\n\nfunc YY() {}\n",
	} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, p), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, repo, "add", p)
	}
	gitIn(t, repo, "commit", "-q", "-m", "packages")
	for _, e := range [][2]string{
		{"a.go#L3-L5", "the root handler"},
		{"cmd/x/x.go#L3-L3", "the command"},
		{"internal/yy/yy.go#L3-L3", "the neighbour package"},
		{"internal/y/y.go#L3-L3", "the package in question"},
	} {
		if _, err := appendMapEntry(mapEntryRequest{Map: "wave", Anchor: e[0], Why: e[1], Root: repo, Now: time.Now()}, nil); err != nil {
			t.Fatalf("append %s: %v", e[0], err)
		}
	}
	withMapSymbols(t, fakeSyms{})
	show := func(args ...string) string {
		t.Helper()
		code, out, errs := runCLI(t, mapCLI, append([]string{"show", "wave", "--repo", repo}, args...)...)
		if code != 0 {
			t.Fatalf("show %v: exit %d: %s", args, code, errs)
		}
		return out
	}
	entryLines := func(out string) []string {
		var ls []string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, ".go#L") {
				ls = append(ls, l)
			}
		}
		return ls
	}

	plain := show()
	if strings.Contains(plain, "focus") {
		t.Errorf("no --focus, no focus marks:\n%s", plain)
	}

	out := show("--focus", "./internal/y/,"+filepath.Join(repo, "cmd", "x"))
	lines := entryLines(out)
	if len(lines) != 4 {
		t.Fatalf("an entry was hidden: %d entry lines\n%s", len(lines), out)
	}
	for i, want := range []string{"cmd/x/x.go#L3-L3", "internal/y/y.go#L3-L3", "a.go#L3-L5", "internal/yy/yy.go#L3-L3"} {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("entry %d = %q, want %s first (focus first, then the rest, each in the map's order)\n%s", i, lines[i], want, out)
		}
		outside := strings.Contains(lines[i], "outside focus")
		if outside != (i >= 2) {
			t.Errorf("entry %q: outside-focus mark %v, want %v", lines[i], outside, i >= 2)
		}
	}
	if !strings.Contains(out, "focus cmd/x, internal/y: 2 of 4 entries inside, listed first") {
		t.Errorf("the header names the focus and how many entries sit in it:\n%s", out)
	}
	if !strings.Contains(out, "4 entries from 1 map: 4 fresh") {
		t.Errorf("the counts stay the whole map's:\n%s", out)
	}
	if again := show("--focus", "./internal/y/,"+filepath.Join(repo, "cmd", "x")); again != out {
		t.Errorf("not deterministic:\n%s\n---\n%s", out, again)
	}

	// A file is a focus too; a focus that names nothing says so and hides nothing.
	if l := entryLines(show("--focus", "a.go")); !strings.HasPrefix(l[0], "a.go#L3-L5") || strings.Contains(l[0], "outside focus") {
		t.Errorf("a file focus: %q", l)
	}
	none := show("--focus", "docs")
	if len(entryLines(none)) != 4 || !strings.Contains(none, "0 of 4 entries inside") {
		t.Errorf("a focus matching nothing:\n%s", none)
	}

	// The budget cuts from the tail, so the focus survives a tight budget.
	tight := show("--focus", "internal/y", "--budget", "420")
	if tl := entryLines(tight); len(tl) == 0 || !strings.HasPrefix(tl[0], "internal/y/y.go#L3-L3") || len(tight) > 420 {
		t.Errorf("a tight budget (%d bytes) lost the focus:\n%s", len(tight), tight)
	}

	// The JSON says it too.
	code, js, errs := runCLI(t, mapCLI, "show", "wave", "--repo", repo, "--focus", "internal/y", "--json")
	if code != 0 {
		t.Fatalf("show --json: %s", errs)
	}
	var v struct {
		Focus   []string `json:"focus"`
		InFocus int      `json:"in_focus"`
		Items   []struct {
			Anchor  string `json:"anchor"`
			Outside bool   `json:"outside_focus"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Focus) != 1 || v.Focus[0] != "internal/y" || v.InFocus != 1 || len(v.Items) != 4 || v.Items[0].Anchor != "internal/y/y.go#L3-L3" || v.Items[0].Outside || !v.Items[1].Outside {
		t.Errorf("json = %+v", v)
	}
}
