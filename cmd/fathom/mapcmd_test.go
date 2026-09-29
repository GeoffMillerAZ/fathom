package main

// mapcmd_test.go — exploration maps on the thread store (XM-1, XM-14) and the
// `map` verbs that reach the model (XM-2 to XM-7 through the product).

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/explmap"
)

const mapFixtureGo = "package a\n\nfunc Handle() error {\n\treturn nil\n}\n\nfunc Other() {}\n"

// mapRepo is a committed git repository with one Go file, and a private
// threads log; the session is fixed so provenance is deterministic.
func mapRepo(t *testing.T) string {
	t.Helper()
	threadEnv(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-parent")
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(mapFixtureGo), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"},
		{"add", "a.go"}, {"commit", "-q", "-m", "fixture"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func runCLI(t *testing.T, f func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := f(args, &out, &errb)
	return code, out.String(), errb.String()
}

func threadCLI(args []string, out, errb *bytes.Buffer) int { return cmdThread(args, out, errb) }
func mapCLI(args []string, out, errb *bytes.Buffer) int    { return cmdMap(args, out, errb) }

func mapRows(t *testing.T, kind string) []threadItem {
	t.Helper()
	var out []threadItem
	for _, it := range loadThreadRowsRaw(threadsPath()) {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

type fakeSyms map[string][3]any

func (f fakeSyms) symbolSpan(key string) (string, int, int, bool) {
	v, ok := f[key]
	if !ok {
		return "", 0, 0, false
	}
	return v[0].(string), v[1].(int), v[2].(int), true
}

// TestThreadNoteMapEntryAnchoredWithHash pins XM-1: `thread note --map` files
// an entry that is an anchor (a span, or a symbol key resolved to its span) plus
// the content hash of that span (core.HashLines, the layer claims' contract)
// plus a why line, on a thread whose map header carries `completes_when`; and
// the thread verbs never serve map rows as thread items.
func TestThreadNoteMapEntryAnchoredWithHash(t *testing.T) {
	repo := mapRepo(t)
	code, out, errs := runCLI(t, threadCLI, "note", "--map", "run-1", "--anchor", "a.go#L3-L5",
		"--why", "the request handler", "--completes-when", "file_exists:DONE", "--repo", repo)
	if code != 0 {
		t.Fatalf("thread note --map exit %d: %s", code, errs)
	}
	if !strings.Contains(out, `on map "run-1"`) || !strings.Contains(out, "a.go#L3-L5") {
		t.Errorf("confirmation = %q", out)
	}

	entries := mapRows(t, mapEntryKind)
	if len(entries) != 1 {
		t.Fatalf("entry rows = %d, want 1", len(entries))
	}
	e := entries[0]
	want, _ := core.HashLines([]byte(mapFixtureGo), 3, 5)
	if e.Pointers["hash"] != want {
		t.Errorf("hash = %q, want core.HashLines over a.go 3-5 (%q)", e.Pointers["hash"], want)
	}
	if e.Pointers["anchor"] != "a.go#L3-L5" || e.Pointers["span"] != "a.go#L3-L5" || e.Text != "the request handler" {
		t.Errorf("entry = anchor %q span %q why %q", e.Pointers["anchor"], e.Pointers["span"], e.Text)
	}
	if e.Pointers["map"] != "run-1" || e.Commit == "" || e.Thread == "" {
		t.Errorf("entry map %q commit %q thread %q: want the map, the filing HEAD and a thread", e.Pointers["map"], e.Commit, e.Thread)
	}

	headers := mapRows(t, mapHeaderKind)
	if len(headers) != 1 {
		t.Fatalf("header rows = %d, want 1 (the map created on first use)", len(headers))
	}
	h := headers[0]
	if h.CompletesWhen == nil || h.CompletesWhen.Kind != "file_exists" || h.CompletesWhen.Path != "DONE" {
		t.Errorf("the map's completes_when = %+v, want file_exists DONE", h.CompletesWhen)
	}
	if h.Thread != e.Thread || h.Pointers["map_kind"] != string(explmap.KindEphemeral) || h.Pointers["created_by"] != "sess-parent" {
		t.Errorf("header = thread %q kind %q creator %q", h.Thread, h.Pointers["map_kind"], h.Pointers["created_by"])
	}

	// A map created without a condition lives until its session ends.
	if code, _, errs := runCLI(t, threadCLI, "note", "the other one", "--map", "run-2", "--anchor", "a.go#L7-L7", "--repo", repo); code != 0 {
		t.Fatalf("second map: %s", errs)
	}
	for _, hh := range mapRows(t, mapHeaderKind) {
		if hh.Pointers["map"] == "run-2" && (hh.CompletesWhen == nil || hh.CompletesWhen.Kind != "session_ended" || hh.CompletesWhen.SessionID != "sess-parent") {
			t.Errorf("run-2's completes_when = %+v, want session_ended for sess-parent", hh.CompletesWhen)
		}
	}

	// A symbol anchor records the key and hashes the span it resolves to.
	it, err := appendMapEntry(mapEntryRequest{Map: "run-1", Anchor: "a.Handle", Why: "entry", Root: repo, Now: time.Now()},
		fakeSyms{"a.Handle": {"a.go", 3, 5}})
	if err != nil {
		t.Fatalf("symbol anchor: %v", err)
	}
	if it.Pointers["symbol"] != "a.Handle" || it.Pointers["span"] != "a.go#L3-L5" || it.Pointers["hash"] != want {
		t.Errorf("symbol entry = %v", it.Pointers)
	}

	// Refusals: no why, a span past the file, an unknown symbol, no index.
	for _, c := range []struct {
		req  mapEntryRequest
		syms mapSymbolResolver
		want string
	}{
		{mapEntryRequest{Map: "run-1", Anchor: "a.go#L3-L5", Root: repo}, nil, "--why"},
		{mapEntryRequest{Map: "run-1", Anchor: "a.go#L3-L99", Why: "w", Root: repo}, nil, "no lines"},
		{mapEntryRequest{Map: "run-1", Anchor: "a.Gone", Why: "w", Root: repo}, fakeSyms{}, "does not resolve"},
		{mapEntryRequest{Map: "run-1", Anchor: "a.Handle", Why: "w", Root: repo}, nil, "no index"},
		{mapEntryRequest{Map: "run-1", Anchor: "../x.go#L1-L1", Why: "w", Root: repo}, nil, "outside the repository"},
		{mapEntryRequest{Map: "bad id", Anchor: "a.go#L1-L1", Why: "w", Root: repo}, nil, "map id"},
	} {
		if _, err := appendMapEntry(c.req, c.syms); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("appendMapEntry(%+v) err = %v, want %q", c.req, err, c.want)
		}
	}

	// Map rows are not thread items: the brief and the list never serve them.
	for _, item := range loadThreadItems(threadsPath()) {
		if isMapItem(item) {
			t.Errorf("loadThreadItems served map row %s", item.ID)
		}
	}
	if brief := renderThreadBrief(loadThreadItems(threadsPath()), repo, threadBriefBudgetBytes, time.Now()); strings.Contains(brief, "request handler") {
		t.Errorf("the thread brief served a map entry:\n%s", brief)
	}
}

// TestSubagentAppendCarriesAgentIDProvenance pins XM-14: a sub-agent appends
// to a map with its agent id as provenance, an append never edits an existing
// entry (the parent's row is byte-identical afterwards, and the same anchor
// with another why is a second entry), a composite refuses entries, and a
// composite a sub-agent mints and passes down carries references only.
func TestSubagentAppendCarriesAgentIDProvenance(t *testing.T) {
	repo := mapRepo(t)
	if code, _, errs := runCLI(t, threadCLI, "note", "--map", "plan", "--anchor", "a.go#L3-L5", "--why", "the handler", "--repo", repo); code != 0 {
		t.Fatalf("parent entry: %s", errs)
	}
	before := mapRows(t, mapEntryKind)[0]
	beforeJSON, _ := json.Marshal(before)

	if code, _, errs := runCLI(t, threadCLI, "note", "--map", "plan", "--anchor", "a.go#L3-L5", "--why", "where errors are swallowed",
		"--agent", "agent-7", "--repo", repo); code != 0 {
		t.Fatalf("sub-agent entry: %s", errs)
	}
	entries := mapRows(t, mapEntryKind)
	if len(entries) != 2 {
		t.Fatalf("entry rows = %d, want 2: an append adds, never edits", len(entries))
	}
	if entries[0].ID == entries[1].ID {
		t.Fatalf("the sub-agent's append reused the parent's row id %s", entries[0].ID)
	}
	afterJSON, _ := json.Marshal(entries[0])
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Errorf("the parent's entry changed:\nbefore %s\nafter  %s", beforeJSON, afterJSON)
	}
	if entries[1].Pointers["agent_id"] != "agent-7" || entries[0].Pointers["agent_id"] != "" {
		t.Errorf("provenance = parent %q, sub-agent %q; want none and agent-7", entries[0].Pointers["agent_id"], entries[1].Pointers["agent_id"])
	}

	// Both annotations survive resolution, with their provenance.
	set := loadMapSet(repo, time.Now())
	if got := len(set.maps["plan"].Entries); got != 2 {
		t.Fatalf("map entries = %d, want 2", got)
	}

	// The sub-agent files its own map and mints a composite to pass down.
	if code, _, errs := runCLI(t, threadCLI, "note", "--map", "mine", "--anchor", "a.go#L7-L7", "--why", "a sibling", "--agent", "agent-7", "--repo", repo); code != 0 {
		t.Fatalf("own map: %s", errs)
	}
	code, out, errs := runCLI(t, mapCLI, "compose", "plan", "mine", "--agent", "agent-7", "--repo", repo, "--url", "http://127.0.0.1:1")
	if code != 0 {
		t.Fatalf("compose: %s", errs)
	}
	id := strings.Fields(out)[1]
	// The parent's and the sub-agent's entries name the same span and hash:
	// one entry, two annotations; plus the sub-agent's own.
	if !strings.HasPrefix(id, explmap.CompositePrefix) || !strings.Contains(out, "2 entries") || !strings.Contains(out, "1 with conflicting annotations") {
		t.Fatalf("compose output = %q, want a minted id, 2 entries, 1 conflict", out)
	}
	set = loadMapSet(repo, time.Now())
	c := set.maps[id]
	if c.Kind != explmap.KindComposite || len(c.Entries) != 0 || c.CreatedBy != "agent-7" {
		t.Errorf("composite = kind %q, %d entries, by %q; want references only, by agent-7", c.Kind, len(c.Entries), c.CreatedBy)
	}

	// A composite refuses entries.
	if code, _, errs := runCLI(t, threadCLI, "note", "--map", id, "--anchor", "a.go#L1-L1", "--why", "x", "--agent", "agent-7", "--repo", repo); code == 0 || !strings.Contains(errs, "references only") {
		t.Errorf("appending to a composite: exit %d, %q; want a refusal", code, errs)
	}
}

// TestMapVerbsReachTheModel pins the product paths of XM-2, XM-6 and XM-7:
// `map include` refuses a cycle, `map name` refuses a named map without a
// purpose, `map list` reports lifetimes, and a dropped include restarts the
// base's idle clock.
func TestMapVerbsReachTheModel(t *testing.T) {
	repo := mapRepo(t)
	for _, id := range []string{"base", "top"} {
		if code, _, errs := runCLI(t, threadCLI, "note", "--map", id, "--anchor", "a.go#L3-L5", "--why", "w "+id, "--repo", repo); code != 0 {
			t.Fatalf("%s: %s", id, errs)
		}
	}
	if code, _, errs := runCLI(t, mapCLI, "include", "top", "base", "--repo", repo); code != 0 {
		t.Fatalf("include: %s", errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "include", "base", "top", "--repo", repo); code == 0 || !strings.Contains(errs, "cycle") {
		t.Errorf("a cycle: exit %d %q, want refused", code, errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "name", "top", "--repo", repo); code == 0 || !strings.Contains(errs, "purpose") {
		t.Errorf("a name without a purpose: exit %d %q, want refused", code, errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "name", "top", "--purpose", "Where does a request get handled?", "--repo", repo); code != 0 {
		t.Fatalf("name: %s", errs)
	}

	code, out, errs := runCLI(t, mapCLI, "list", "--json", "--repo", repo)
	if code != 0 {
		t.Fatalf("list: %s", errs)
	}
	var got struct {
		Maps []struct {
			ID, Lifetime string
			PinnedBy     []string `json:"pinned_by"`
		} `json:"maps"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("list json: %v\n%s", err, out)
	}
	life := map[string]string{}
	for _, m := range got.Maps {
		life[m.ID] = m.Lifetime
	}
	if life["base"] != "pinned" || life["top"] != "permanent" {
		t.Errorf("lifetimes = %v, want base pinned (top includes it) and top permanent (named)", life)
	}

	if code, _, errs := runCLI(t, mapCLI, "include", "top", "--drop", "base", "--repo", repo); code != 0 {
		t.Fatalf("drop: %s", errs)
	}
	set := loadMapSet(repo, time.Now())
	if len(set.maps["top"].Drops) != 1 || set.maps["top"].Drops[0].ID != "base" {
		t.Errorf("drops = %+v, want base dropped", set.maps["top"].Drops)
	}
	if lt := explmap.Lifetimes(set.maps)["base"]; lt.State != explmap.Open || lt.IdleSince.IsZero() {
		t.Errorf("base after the drop = %+v, want open with its clock restarted", lt)
	}
}

// TestMapNameRestatementKeepsWhatItDoesNotRestate: a header is restated whole on
// every change, so `map name` given only a new purpose must carry the map's
// name and when-to-use line forward, not reset the name to the id and drop the
// when line.
func TestMapNameRestatementKeepsWhatItDoesNotRestate(t *testing.T) {
	repo := mapRepo(t)
	if code, _, errs := runCLI(t, mapCLI, "new", "auth", "--purpose", "Where is the token checked?", "--when", "before touching auth", "--repo", repo); code != 0 {
		t.Fatalf("new: %s", errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "name", "auth", "--as", "Auth surface", "--purpose", "Where is the token checked?", "--repo", repo); code != 0 {
		t.Fatalf("name: %s", errs)
	}
	if code, _, errs := runCLI(t, mapCLI, "name", "auth", "--purpose", "Where is the token checked and refreshed?", "--repo", repo); code != 0 {
		t.Fatalf("rename purpose: %s", errs)
	}
	m := loadMapSet(repo, time.Now()).maps["auth"]
	if m.Name != "Auth surface" || m.WhenToUse != "before touching auth" || m.Purpose != "Where is the token checked and refreshed?" {
		t.Errorf("after a purpose-only restatement: name %q, when %q, purpose %q; want the name and when line kept", m.Name, m.WhenToUse, m.Purpose)
	}
}

// TestMapNamedByThePlannerOutlivesTheSessionThatFiledIt pins XM-32 (O143)
// through the product: a map filed with the default session-end condition and
// then named stays listed, and is served by the hooks, after that session's
// end note exists; a map named only after its session ended is durable from
// then; an unnamed map filed the same way ends.
func TestMapNamedByThePlannerOutlivesTheSessionThatFiledIt(t *testing.T) {
	repo := mapRepo(t)
	for _, id := range []string{"kept", "late", "scratch"} {
		if code, _, errs := runCLI(t, mapCLI, "new", id, "--purpose", "Where is "+id+" handled?", "--repo", repo); code != 0 {
			t.Fatalf("new %s: %s", id, errs)
		}
		if code, _, errs := runCLI(t, threadCLI, "note", "--map", id, "--anchor", "a.go#L3-L5", "--why", "the handler", "--repo", repo); code != 0 {
			t.Fatalf("entry on %s: %s", id, errs)
		}
	}
	if code, out, errs := runCLI(t, mapCLI, "name", "kept", "--purpose", "Where is kept handled?", "--repo", repo); code != 0 {
		t.Fatalf("name kept: %s", errs)
	} else if !strings.Contains(out, "durable") {
		t.Errorf("name output = %q, want it to say the map is durable now", out)
	}

	// The planner's session ends.
	seedItem(t, "se-parent", "session-end", "session ended", func(it *threadItem) {
		it.Pointers = map[string]string{"session_id": "sess-parent"}
	})

	list := func() map[string]string {
		t.Helper()
		code, out, errs := runCLI(t, mapCLI, "list", "--json", "--repo", repo)
		if code != 0 {
			t.Fatalf("list: %s", errs)
		}
		var got struct {
			Maps []struct{ ID, Lifetime string } `json:"maps"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("list json: %v\n%s", err, out)
		}
		life := map[string]string{}
		for _, m := range got.Maps {
			life[m.ID] = m.Lifetime
		}
		return life
	}
	life := list()
	if life["kept"] != "permanent" {
		t.Errorf("kept (named before the session ended) = %q in %v, want permanent and listed", life["kept"], life)
	}
	if _, listed := life["scratch"]; listed {
		t.Errorf("scratch (never named) is still listed after its session ended: %v", life)
	}
	if _, listed := life["late"]; listed {
		t.Errorf("late (not named yet) is still listed after its session ended: %v", life)
	}

	// Naming a map after its session ended makes it durable from then.
	if code, _, errs := runCLI(t, mapCLI, "name", "late", "--purpose", "Where is late handled?", "--repo", repo); code != 0 {
		t.Fatalf("name late: %s", errs)
	}
	life = list()
	if life["late"] != "permanent" {
		t.Errorf("late (named after the session ended) = %q in %v, want permanent", life["late"], life)
	}

	// The next session hears of the durable maps (the hooks that deliver maps
	// read this set: the brief, the pre-compact resume, the location pointer)
	// and not of the ended one.
	set := loadMapSet(repo, time.Now())
	states := explmap.Lifetimes(set.maps)
	if states["kept"].State != explmap.Permanent || states["late"].State != explmap.Permanent || states["scratch"].State != explmap.Ended {
		t.Errorf("lifetimes = kept %s, late %s, scratch %s; want permanent, permanent, ended",
			states["kept"].State, states["late"].State, states["scratch"].State)
	}
	_, alive := loadAliveMaps(repo, time.Now())
	if strings.Join(alive, " ") != "kept late" && strings.Join(alive, " ") != "late kept" {
		t.Errorf("the hooks' alive maps = %v, want kept and late only", alive)
	}
}
