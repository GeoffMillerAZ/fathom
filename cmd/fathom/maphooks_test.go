package main

// maphooks_test.go — the four ways an exploration map arrives (XM-8, XM-11,
// XM-12, XM-13; docs/design/exploration-maps.md section 4). Every budget here is
// a byte bound or a CPU-time bound, never a wall clock.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/explmap"
)

const hookMapFile = "package g\n\nfunc F() {}\n\nfunc G() {}\n"

// hookMapRepo is a committed git repository named "fathom" (the fake daemon's
// service) holding a.go and g1.go..g5.go, with a private threads log, marker
// directory and session; the parent session is sess-parent.
func hookMapRepo(t *testing.T) string {
	t.Helper()
	threadEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-parent")
	oldDir := mapPointerDir
	mapPointerDir = t.TempDir()
	t.Cleanup(func() { mapPointerDir = oldDir })
	repo := filepath.Join(t.TempDir(), "fathom")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(filepath.Dir(repo), "sessions.jsonl"))
	files := map[string]string{"a.go": mapFixtureGo}
	for i := 1; i <= 5; i++ {
		files[fmt.Sprintf("g%d.go", i)] = hookMapFile
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"},
		{"add", "."}, {"commit", "-q", "-m", "fixture"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

// fileMapEntry files one entry on a map as session, at a controlled time.
func fileMapEntry(t *testing.T, repo, id, anchor, why, session string, at time.Time) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	if _, err := appendMapEntry(mapEntryRequest{Map: id, Anchor: anchor, Why: why, Root: repo, Now: at}, nil); err != nil {
		t.Fatalf("file %s on %s: %v", anchor, id, err)
	}
}

// nameMap gives a map its purpose line.
func nameMap(t *testing.T, repo, id, purpose string) {
	t.Helper()
	if code, _, errs := runCLI(t, mapCLI, "name", id, "--purpose", purpose, "--repo", repo); code != 0 {
		t.Fatalf("map name %s: %s", id, errs)
	}
}

// fakeIndexDaemon answers list_services (the service resolves as "fathom") and
// refuses every symbol lookup at once, so a discoveries pointer renders stale
// without waiting on anything.
func fakeIndexDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name != "list_services" {
			http.Error(w, "unknown tool", 400)
			return
		}
		inner := `{"services":[{"service":"fathom","symbol_count":10,"languages":["go"]}]}`
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// hookContext decodes a hook's stdout envelope to its additionalContext.
func hookContext(t *testing.T, out string) string {
	t.Helper()
	var env struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if strings.TrimSpace(out) == "" {
		return ""
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("hook output %q: %v", out, err)
	}
	return env.HookSpecificOutput.AdditionalContext
}

func mapHeaderRows(t *testing.T) map[string]threadItem {
	t.Helper()
	out := map[string]threadItem{}
	for _, h := range loadAllThreadRows(threadsPath()) {
		if h.Kind == mapHeaderKind {
			out[h.Pointers["map"]] = h
		}
	}
	return out
}

// ─── XM-8 ────────────────────────────────────────────────────────────────

// TestSubagentStartCarriesOneMapIDWithin1200Bytes pins XM-8: the SubagentStart
// primer carries ONE map id, chosen from what the parent session filed, inside
// RC-28's 1,200-byte cap, at the head of the discoveries chunk so the brief
// yields bytes and the id never does, with the sub-agent's own id (the payload
// is the only place fathom sees it) for `--agent`. Several maps become one
// minted composite made only of references.
func TestSubagentStartCarriesOneMapIDWithin1200Bytes(t *testing.T) {
	repo := hookMapRepo(t)
	srv := fakeIndexDaemon(t)
	now := time.Now().UTC()
	recordSessionWindow(repo, sessionWindowRow{SchemaVersion: 1, SessionID: "sess-parent", Cwd: repo, Service: "fathom", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	var rows []string
	for i := 0; i < 12; i++ {
		rows = append(rows, fmt.Sprintf(`{"ts":"%s","service":"fathom","pointers":["internal/pkgname%02d.LongSymbolNameForTheBrief%02d"]}`,
			now.Add(-time.Duration(i)*time.Second).Format(time.RFC3339Nano), i, i))
	}
	writeDiscoCallLogFixture(t, callLogPathFor(repo), rows)
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-parent", now.Add(-30*time.Minute))
	nameMap(t, repo, "auth-surface", "which handlers gate the admin API")

	spawn := func() string {
		var out, errb bytes.Buffer
		payload := `{"session_id":"sess-parent","cwd":"` + repo + `","agent_id":"agent-7","agent_type":"explorer"}`
		if code := cmdSessionHookEvent("subagent-start", []string{"--url", srv.URL, "--dir", repo}, strings.NewReader(payload), &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		return hookContext(t, out.String())
	}

	primer := spawn()
	t.Logf("SubagentStart primer (%d bytes):\n%s", len(primer), primer)
	if len(primer) > subagentDiscoveriesPrimerBudget {
		t.Fatalf("primer is %d bytes, over RC-28's %d-byte cap:\n%s", len(primer), subagentDiscoveriesPrimerBudget, primer)
	}
	if n := strings.Count(primer, subagentMapLead); n != 1 {
		t.Fatalf("primer names %d maps, want exactly one id:\n%s", n, primer)
	}
	for _, want := range []string{"fathom map show auth-surface --budget 1200", "--agent agent-7", "which handlers gate the admin API"} {
		if !strings.Contains(primer, want) {
			t.Errorf("primer lacks %q:\n%s", want, primer)
		}
	}
	mapAt, briefAt := strings.Index(primer, subagentMapLead), strings.Index(primer, subagentBriefLead)
	if briefAt < 0 || mapAt > briefAt {
		t.Errorf("the map line must lead the discoveries brief (map at %d, brief at %d):\n%s", mapAt, briefAt, primer)
	}
	if !strings.Contains(primer, "more: fathom discoveries") || strings.Contains(primer, "byte cap") {
		t.Errorf("twelve long pointers do not fit beside the id: the brief yields whole pointer lines and names how many (law 3), the primer's byte cut never fires:\n%s", primer)
	}

	// A second map from the same session: still ONE id, the minted composite.
	fileMapEntry(t, repo, "pay-path", "g1.go#L3-L3", "entry point", "sess-parent", now.Add(-20*time.Minute))
	fileMapEntry(t, repo, "not-yours", "g2.go#L3-L3", "another session's map", "sess-other", now.Add(-10*time.Minute))
	primer = spawn()
	if len(primer) > subagentDiscoveriesPrimerBudget || strings.Count(primer, subagentMapLead) != 1 {
		t.Fatalf("two maps must still hand down one id inside the cap (%d bytes):\n%s", len(primer), primer)
	}
	if !strings.Contains(primer, subagentMapLead+explmap.CompositePrefix) {
		t.Fatalf("two maps hand down a composite id:\n%s", primer)
	}
	headers := mapHeaderRows(t)
	var composite threadItem
	for id, h := range headers {
		if strings.HasPrefix(id, explmap.CompositePrefix) {
			composite = h
		}
	}
	if composite.ID == "" {
		t.Fatalf("the composite was not stored: %v", headers)
	}
	if got := strings.Fields(composite.Pointers["includes"]); len(got) != 2 || got[0] != "auth-surface" || got[1] != "pay-path" {
		t.Errorf("composite includes = %v, want only the parent session's maps [auth-surface pay-path] (references, never entries, never another session's map)", got)
	}
	if composite.CompletesWhen == nil || composite.CompletesWhen.Kind != "session_ended" || composite.CompletesWhen.SessionID != "sess-parent" {
		t.Errorf("composite completes_when = %+v, want the parent session's end (it must not pin its maps past the session)", composite.CompletesWhen)
	}
	before := len(mapRows(t, mapHeaderKind))
	if again := spawn(); !strings.Contains(again, composite.Pointers["map"]) {
		t.Errorf("a second spawn with the same maps hands down the same composite %s:\n%s", composite.Pointers["map"], again)
	}
	if after := len(mapRows(t, mapHeaderKind)); after != before {
		t.Errorf("re-handing the same composite wrote %d header row(s)", after-before)
	}
	for _, e := range mapRows(t, mapEntryKind) {
		if e.Pointers["map"] == composite.Pointers["map"] {
			t.Errorf("a composite carries references only, found an entry row on it")
		}
	}
}

// TestSubagentMapLineChoosesTheParentsRoots pins the choice under the parent's
// session key: nothing filed means silence, a map another session filed is not
// handed down, an included base is reached through its includer and never named
// beside it, and an ended map is not alive.
func TestSubagentMapLineChoosesTheParentsRoots(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	if got := subagentMapLine(repo, "sess-parent", "a1", now); got != "" {
		t.Errorf("no maps filed, want silence, got %q", got)
	}
	fileMapEntry(t, repo, "other-only", "g1.go#L3-L3", "x", "sess-other", now.Add(-time.Hour))
	if got := subagentMapLine(repo, "sess-parent", "a1", now); got != "" {
		t.Errorf("only another session filed a map, want silence, got %q", got)
	}
	if got := subagentMapLine(repo, "", "a1", now); got != "" {
		t.Errorf("no parent session id keys nothing, got %q", got)
	}

	fileMapEntry(t, repo, "base", "g2.go#L3-L3", "the base", "sess-parent", now.Add(-50*time.Minute))
	fileMapEntry(t, repo, "top", "g3.go#L3-L3", "the top", "sess-parent", now.Add(-40*time.Minute))
	if code, _, errs := runCLI(t, mapCLI, "include", "top", "base", "--repo", repo); code != 0 {
		t.Fatalf("include: %s", errs)
	}
	got := subagentMapLine(repo, "sess-parent", "a1", now)
	if !strings.Contains(got, subagentMapLead+"top (2 entries)") || strings.Contains(got, "base") {
		t.Errorf("top includes base: hand down top alone (2 entries reached), got %q", got)
	}
	if !strings.Contains(got, "--agent a1") {
		t.Errorf("the line carries the sub-agent's own id, got %q", got)
	}
	if noAgent := subagentMapLine(repo, "sess-parent", "", now); strings.Contains(noAgent, "--agent") {
		t.Errorf("no agent id in the payload, none is invented: %q", noAgent)
	}

	// Ending the session (its condition holds) ends both maps: nothing to hand.
	if _, err := os.Stat(threadsPath()); err != nil {
		t.Fatal(err)
	}
	seedItem(t, "se-1", "session-end", "session ended", func(it *threadItem) {
		it.Pointers = map[string]string{"session_id": "sess-parent"}
	})
	if got := subagentMapLine(repo, "sess-parent", "a1", now.Add(time.Minute)); got != "" {
		t.Errorf("the parent session ended, its maps ended with it, want silence, got %q", got)
	}
}

// TestRenderSubagentMapLineStaysWithinItsBound pins the line's byte bound
// however long the ids and the purpose are, the read command never yielding.
func TestRenderSubagentMapLineStaysWithinItsBound(t *testing.T) {
	long := strings.Repeat("m", 64)
	maps := explmap.Maps{long: {ID: long, Kind: explmap.KindEphemeral, Purpose: strings.Repeat("a very long purpose line ", 30),
		Entries: []explmap.Entry{{ID: "e1"}, {ID: "e2"}}}}
	line := renderSubagentMapLine(maps, long, strings.Repeat("a", 40))
	if len(line) > subagentMapLineMax {
		t.Errorf("line is %d bytes, over %d: %q", len(line), subagentMapLineMax, line)
	}
	if !strings.Contains(line, "fathom map show "+long+" --budget 1200") {
		t.Errorf("the read command must survive: %q", line)
	}
	if strings.Contains(line, "\n") {
		t.Errorf("one line: %q", line)
	}
}

// TestJoinSubagentPrimerCutsTheBriefBeforeTheID pins the join: the map line
// leads the chunk, a cut takes the brief's tail and names itself, and only a
// base too long to leave the id its room yields to it.
func TestJoinSubagentPrimerCutsTheBriefBeforeTheID(t *testing.T) {
	mapLine := subagentMapLead + "xm-0123456789 (2 maps, 9 entries): fathom map show xm-0123456789 --budget 1200"
	brief := subagentBriefLead + "\n" + strings.Repeat("some/pkg.Symbol — stale (unresolved)\n", 40)
	base := strings.Repeat("b", 590)

	got := joinSubagentPrimer(base, mapLine+"\n"+brief, subagentDiscoveriesPrimerBudget)
	if len(got) > subagentDiscoveriesPrimerBudget {
		t.Errorf("joined = %d bytes, over the cap", len(got))
	}
	if !strings.Contains(got, mapLine) || strings.Index(got, mapLine) > strings.Index(got, subagentBriefLead) {
		t.Errorf("the whole map line leads the brief:\n%s", got)
	}
	if !strings.HasSuffix(got, subagentPrimerCut) {
		t.Errorf("a cut brief names the cut (law 3):\n%s", got)
	}

	// A base that leaves no room: the base yields, the id line stays whole.
	got = joinSubagentPrimer(strings.Repeat("b", 1180), mapLine+"\n"+brief, subagentDiscoveriesPrimerBudget)
	if len(got) > subagentDiscoveriesPrimerBudget || !strings.HasSuffix(got, "\n"+mapLine) {
		t.Errorf("an over-long base yields to the id (len %d):\n%s", len(got), got)
	}

	// Room for the id and nothing more: no brief, the cut named where it fits.
	tight := len(base) + 1 + len(mapLine) + 10
	got = joinSubagentPrimer(base, mapLine+"\n"+brief, tight)
	if !strings.Contains(got, mapLine) || strings.Contains(got, subagentBriefLead) || len(got) > tight+len(subagentPrimerCut) {
		t.Errorf("with no room for a brief the id ships alone (len %d, tight %d):\n%s", len(got), tight, got)
	}

	// A chunk with no map line is the old join, unchanged.
	if got := joinSubagentPrimer("base", brief, 600); got != "base\n"+brief && !strings.HasSuffix(got, subagentPrimerCut) {
		t.Errorf("a brief without a map keeps the plain join: %q", got)
	}
	if got := joinSubagentPrimer("base", "", 600); got != "base" {
		t.Errorf("nothing to append: %q", got)
	}
}

// ─── XM-11 ───────────────────────────────────────────────────────────────

// TestMapPointerLineOncePerSessionWithinBudget pins XM-11: code inside a map's
// anchored footprint gets one line (purpose, size, freshness, how to expand),
// once per session per map and within its byte bound; an untouched file, an
// ended map, and a session-less firing get nothing.
func TestMapPointerLineOncePerSessionWithinBudget(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-parent", now.Add(-time.Hour))
	fileMapEntry(t, repo, "auth-surface", "g1.go#L3-L3", "the helper", "sess-parent", now.Add(-59*time.Minute))
	nameMap(t, repo, "auth-surface", "which handlers gate the admin API")
	touchA := mapTouch{Files: []string{"a.go"}}

	line := mapPointerLine(repo, "s1", touchA, now)
	for _, want := range []string{"fathom map auth-surface:", "2 entries", "all fresh", "fathom map show auth-surface --budget 1200", "which handlers gate the admin API"} {
		if !strings.Contains(line, want) {
			t.Errorf("pointer lacks %q: %q", want, line)
		}
	}
	if len(line) > mapPointerLineMax || strings.Contains(line, "\n") {
		t.Errorf("one line within %d bytes, got %d: %q", mapPointerLineMax, len(line), line)
	}
	if again := mapPointerLine(repo, "s1", touchA, now); again != "" {
		t.Errorf("the same session is told once per map, got %q", again)
	}
	if other := mapPointerLine(repo, "s2", touchA, now); other == "" {
		t.Errorf("another session is told again")
	}
	if got := mapPointerLine(repo, "s3", mapTouch{Files: []string{"g4.go"}}, now); got != "" {
		t.Errorf("a file outside every footprint gets nothing, got %q", got)
	}
	if got := mapPointerLine(repo, "", touchA, now); got != "" {
		t.Errorf("no session id cannot be once-only, want silence, got %q", got)
	}
	if got := mapPointerLine(repo, "s4", mapTouch{}, now); got != "" {
		t.Errorf("nothing touched, got %q", got)
	}

	// Directories and symbols are footprints too.
	if got := mapPointerLine(repo, "s5", mapTouch{Dirs: []string{"."}}, now); got != "" {
		t.Errorf("a directory that is not a path prefix is no footprint, got %q", got)
	}

	// Drift is named, never served silently: change a.go's anchored lines.
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(strings.Replace(mapFixtureGo, "return nil", "return fmt.Errorf(\"x\")", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mapPointerLine(repo, "s6", touchA, now); !strings.Contains(got, "1 of 2 changed since filed") {
		t.Errorf("a drifted entry is flagged, got %q", got)
	}

	// A purpose that would not fit gives way to the bound, never the command.
	longID := strings.Repeat("l", 60)
	fileMapEntry(t, repo, longID, "g4.go#L3-L3", "the handler", "sess-parent", now.Add(-time.Minute))
	nameMap(t, repo, longID, strings.Repeat("a purpose that runs on ", 8))
	if got := mapPointerLine(repo, "s7", mapTouch{Files: []string{"g4.go"}}, now); len(got) > mapPointerLineMax || !strings.Contains(got, "fathom map show "+longID+" --budget 1200") {
		t.Errorf("an over-long purpose yields bytes, never the command (len %d): %q", len(got), got)
	}

}

// TestMapPointerLineSkipsEndedMaps pins that a map whose own condition held is
// not alive, so nothing points at it.
func TestMapPointerLineSkipsEndedMaps(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	fileMapEntry(t, repo, "scratch", "g2.go#L3-L3", "temp", "sess-parent", now.Add(-time.Minute))
	touch := mapTouch{Files: []string{"g2.go"}}
	if got := mapPointerLine(repo, "s1", touch, now); !strings.Contains(got, "fathom map scratch:") {
		t.Fatalf("an open map is named, got %q", got)
	}
	seedItem(t, "se-y", "session-end", "session ended", func(it *threadItem) {
		it.Pointers = map[string]string{"session_id": "sess-parent"}
	})
	if got := mapPointerLine(repo, "s2", touch, now.Add(2*time.Minute)); got != "" {
		t.Errorf("an unnamed map whose session ended is not alive, got %q", got)
	}
}

// TestMapPointerLineLimitsAFiringToTwoMaps pins the per-firing bound and that
// the maps left out are named on a later touch, not lost.
func TestMapPointerLineLimitsAFiringToTwoMaps(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	for i, id := range []string{"m1", "m2", "m3"} {
		fileMapEntry(t, repo, id, "a.go#L3-L5", "the handler", "sess-parent", now.Add(-time.Duration(3-i)*time.Minute))
	}
	first := mapPointerLine(repo, "s1", mapTouch{Files: []string{"a.go"}}, now)
	if n := len(strings.Split(first, "\n")); n != mapPointerMaxLines {
		t.Fatalf("a firing names at most %d maps, got %d:\n%s", mapPointerMaxLines, n, first)
	}
	second := mapPointerLine(repo, "s1", mapTouch{Files: []string{"a.go"}}, now)
	if len(strings.Split(second, "\n")) != 1 || strings.Contains(second, "fathom map m1:") == strings.Contains(first, "fathom map m1:") {
		t.Errorf("the third map is named on the next touch, and none twice:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if third := mapPointerLine(repo, "s1", mapTouch{Files: []string{"a.go"}}, now); third != "" {
		t.Errorf("all three named once, got %q", third)
	}
}

// TestEditHookNamesAMapThroughTheHook drives the real edit hook with the daemon
// down: the pointer needs no daemon, is once per session, and the file outside
// every footprint stays silent.
func TestEditHookNamesAMapThroughTheHook(t *testing.T) {
	repo := hookMapRepo(t)
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-parent", time.Now().Add(-time.Hour))
	nameMap(t, repo, "auth-surface", "which handlers gate the admin API")

	edit := func(file, session string) string {
		defer pipeStdin(t, `{"session_id":"`+session+`","tool_name":"Edit","tool_input":{"file_path":"`+filepath.Join(repo, file)+`"},"cwd":"`+repo+`"}`)()
		var out, errb bytes.Buffer
		if code := cmdEditHook([]string{"--url", "http://127.0.0.1:1"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		return hookContext(t, out.String())
	}
	got := edit("a.go", "s1")
	if !strings.Contains(got, "fathom map auth-surface:") || !strings.Contains(got, "which handlers gate the admin API") {
		t.Fatalf("the edit hook names the map whose footprint holds the file, got %q", got)
	}
	if again := edit("a.go", "s1"); again != "" {
		t.Errorf("once per session, got %q", again)
	}
	if other := edit("g3.go", "s2"); other != "" {
		t.Errorf("a file in no footprint is silent, got %q", other)
	}
}

// TestPromptHookNamesAMapForANamedPath drives the real prompt hook: a prompt
// that names a file, a directory or a symbol inside a map's footprint gets the
// pointer, once per session.
func TestPromptHookNamesAMapForANamedPath(t *testing.T) {
	repo := hookMapRepo(t)
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	now := time.Now().UTC()
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-parent", now.Add(-time.Hour))
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-parent")
	if _, err := appendMapEntry(mapEntryRequest{Map: "symbols", Anchor: "a.Handle", Why: "the entry", Root: repo, Now: now.Add(-50 * time.Minute)},
		fakeSyms{"a.Handle": {"a.go", 3, 5}}); err != nil {
		t.Fatal(err)
	}

	prompt := func(text, session string) string {
		body, _ := json.Marshal(map[string]string{"session_id": session, "prompt": text, "cwd": repo})
		defer pipeStdin(t, string(body))()
		var out, errb bytes.Buffer
		if code := cmdPromptHook([]string{"--url", "http://127.0.0.1:1"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		return hookContext(t, out.String())
	}
	got := prompt("please look at a.go and tell me what breaks", "s1")
	if !strings.Contains(got, "fathom map auth-surface:") {
		t.Fatalf("a prompt naming a file in the footprint gets the pointer, got %q", got)
	}
	if again := prompt("and a.go again", "s1"); strings.Contains(again, "auth-surface") {
		t.Errorf("once per session per map, got %q", again)
	}
	if got := prompt("what does g5.go do", "s2"); got != "" {
		t.Errorf("a prompt naming nothing in a footprint is silent, got %q", got)
	}
	if got := prompt("explain `Handle` to me", "s3"); !strings.Contains(got, "fathom map symbols:") {
		t.Errorf("a bare symbol in the footprint gets the pointer, got %q", got)
	}
}

// TestMapPointerCostsNothingWithoutMapsAndBoundedCPUWith pins the hook's work:
// a threads log with no map row is one read (no parsing), and a large log with
// maps stays under a CPU-time bound per firing.
func TestMapPointerCostsNothingWithoutMapsAndBoundedCPUWith(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	// 3,000 thread rows, written in one go (appendThreadItem forks git per row).
	var log bytes.Buffer
	identity := repoIdentity(repo)
	for i := 0; i < 3000; i++ {
		row, _ := json.Marshal(threadItem{
			SchemaVersion: 1, ID: fmt.Sprintf("th-%04d", i), Thread: "fathom", Kind: "handoff-note",
			Text:     strings.Repeat("a handoff note ", 20),
			Pointers: map[string]string{"repo": "fathom", "repo_identity": identity},
			FiledAt:  now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339),
		})
		log.Write(append(row, '\n'))
	}
	if err := os.WriteFile(threadsPath(), log.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if threadsHoldMaps() {
		t.Fatal("no map row filed yet")
	}
	// Counted in allocations, not CPU time: process CPU also bills a race
	// shard's GC and leftover goroutines (Linux CI read 256.84ms against a
	// 250ms bound for this unchanged path). Measured 7 a firing; one parse
	// of this log is 66,054.
	firing := 0
	if n := testing.AllocsPerRun(20, func() {
		firing++
		if got := mapPointerLine(repo, fmt.Sprintf("s%d", firing), mapTouch{Files: []string{"a.go"}}, now); got != "" {
			t.Fatalf("no maps, got %q", got)
		}
	}); n > 1000 {
		t.Errorf("a firing against a %d-row log with no maps made %.0f allocations, want a bare file read each", 3000, n)
	}

	for i := 0; i < 3; i++ {
		fileMapEntry(t, repo, fmt.Sprintf("m%d", i), "a.go#L3-L5", "the handler", "sess-parent", now.Add(-time.Duration(i+1)*time.Minute))
	}
	// Same measure for the with-maps path: allocations, not CPU time. A firing
	// parses the log once (loadMapSet) and measured 129,868 allocations; a
	// second whole-log parse adds ~66,000 and trips the bound.
	firing = 0
	n := testing.AllocsPerRun(20, func() {
		firing++
		mapPointerLine(repo, fmt.Sprintf("cpu%d", firing), mapTouch{Files: []string{"a.go"}}, now)
	})
	if n > 180000 {
		t.Errorf("a firing against a %d-row log with maps made %.0f allocations, over the bound", 3000, n)
	}
}

// ─── XM-12 ───────────────────────────────────────────────────────────────

// TestBriefMapsRanksNamedThenOverlapThenRecency pins the ranking on its own.
func TestBriefMapsRanksNamedThenOverlapThenRecency(t *testing.T) {
	mk := func(id string, paths ...string) explmap.Map {
		m := explmap.Map{ID: id, Kind: explmap.KindEphemeral, Purpose: "purpose " + id}
		for i, p := range paths {
			m.Entries = append(m.Entries, explmap.Entry{ID: fmt.Sprintf("%s-%d", id, i), Path: p})
		}
		return m
	}
	maps := explmap.Maps{
		"a": mk("a", "f1.go"), "b": mk("b", "f2.go", "f3.go"), "c": mk("c", "f3.go"),
		"d": mk("d", "f9.go"), "e": mk("e", "f8.go"),
	}
	// alive is most recently used first: a newest.
	shown, more := briefMaps(maps, []string{"a", "b", "c", "d", "e"}, map[string]bool{"e": true}, []string{"f2.go", "f3.go"})
	var ids []string
	for _, s := range shown {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, " ") != "e b c" || more != 2 {
		t.Errorf("shown %v (+%d more), want the note-named map, then most overlap (b: 2 files), then next overlap (c: 1) — 2 left out", ids, more)
	}
	if shown[1].Overlap != 2 || shown[2].Overlap != 1 || !shown[0].Named {
		t.Errorf("overlap/named = %+v", shown)
	}

	// Nothing named or overlapping: the three most recently used.
	shown, more = briefMaps(maps, []string{"a", "b", "c", "d", "e"}, nil, nil)
	ids = nil
	for _, s := range shown {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, " ") != "a b c" || more != 2 {
		t.Errorf("by recent use: shown %v (+%d)", ids, more)
	}

	// Never more than three, never fewer than exist.
	if shown, more := briefMaps(maps, []string{"a", "b"}, nil, nil); len(shown) != 2 || more != 0 {
		t.Errorf("two alive: shown %d (+%d)", len(shown), more)
	}
	if shown, _ := briefMaps(maps, nil, nil, nil); len(shown) != 0 {
		t.Errorf("none alive: %v", shown)
	}
}

// TestBriefNamesAtMostThreeMaps pins XM-12 through the session primer: five
// alive maps, the brief names three (the one a thread note references, then the
// two by overlap with the thread's recent files) and counts the rest, never a
// full list.
func TestBriefNamesAtMostThreeMaps(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Now().UTC()
	// `map name` restates a header at the real clock, so the entries are filed
	// ahead of it: each map's last use is then its entry's time, m5 the latest,
	// whatever second the names were written in.
	for i := 1; i <= 5; i++ {
		fileMapEntry(t, repo, fmt.Sprintf("m%d", i), fmt.Sprintf("g%d.go#L3-L3", i), "why", "sess-old", now.Add(time.Duration(i)*time.Hour))
		nameMap(t, repo, fmt.Sprintf("m%d", i), fmt.Sprintf("the question map %d answers", i))
	}
	// The thread's recent files: g2.go and g3.go are dirty.
	for _, f := range []string{"g2.go", "g3.go"} {
		if err := os.WriteFile(filepath.Join(repo, f), []byte(hookMapFile+"// edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// An open note references m1.
	if code, _, errs := runCLI(t, threadCLI, "note", "where the auth work stands", "--map-ref", "m1", "--repo", repo); code != 0 {
		t.Fatalf("thread note --map-ref: %s", errs)
	}

	primer, _, _ := composeContinuityAndCustodian("identity", repo, now.Add(6*time.Hour))
	t.Logf("session brief:\n%s", primer)
	var named []string
	for _, l := range strings.Split(primer, "\n") {
		if strings.HasPrefix(l, "  • (map ") {
			named = append(named, strings.TrimSuffix(strings.Fields(l)[2], ")"))
		}
	}
	if len(named) != maxBriefMaps {
		t.Fatalf("the brief names %d maps, want exactly %d (never a full list of 5):\n%s", len(named), maxBriefMaps, primer)
	}
	if strings.Join(named, " ") != "m1 m3 m2" {
		t.Errorf("named %v, want m1 (a note references it), then m3 and m2 (they overlap the dirty files; m3 was used last)\n%s", named, primer)
	}
	for _, left := range []string{"the question map 4", "the question map 5"} {
		if strings.Contains(primer, left) {
			t.Errorf("%q was left out of the three:\n%s", left, primer)
		}
	}
	if !strings.Contains(primer, "2 more: fathom map list") || !strings.Contains(primer, "fathom map show <id> --budget 1200") {
		t.Errorf("the rest are counted, and the expand command is named:\n%s", primer)
	}
	if !strings.Contains(primer, "[maps m1]") {
		t.Errorf("a note's map references show on the note:\n%s", primer)
	}
	if !strings.Contains(primer, "named by a thread note") {
		t.Errorf("the note-named map says why:\n%s", primer)
	}

	// The same brief through the verb.
	code, out, errs := runCLI(t, threadCLI, "brief", "--repo", repo)
	if code != 0 || strings.Count(out, "  • (map ") != maxBriefMaps {
		t.Errorf("thread brief names %d maps (exit %d, %s):\n%s", strings.Count(out, "  • (map "), code, errs, out)
	}
	// And nothing on resume/compact, whose context survives.
	if _, out, _ := runCLI(t, threadCLI, "brief", "--source", "resume", "--repo", repo); out != "" {
		t.Errorf("resume prints nothing, got %q", out)
	}
}

// TestPorcelainPathsSurvivesTheTrimmedFirstLine pins the reading of a trimmed
// `git status --porcelain`: the first line has lost its leading space.
func TestPorcelainPathsSurvivesTheTrimmedFirstLine(t *testing.T) {
	trimmed := strings.TrimSpace(" M g2.go\n M g3.go\nA  new.go\n?? scratch.txt\nR  old.go -> moved.go\n D gone.go\n?? \"a b.go\"")
	got := porcelainPaths(trimmed)
	want := []string{"g2.go", "g3.go", "new.go", "scratch.txt", "moved.go", "gone.go", "a b.go"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("paths = %v, want %v", got, want)
	}
	if got := porcelainPaths(""); len(got) != 0 {
		t.Errorf("clean tree = %v", got)
	}
}

// TestBriefHasNoMapLinesWithoutMaps pins that a repository with no map sees the
// brief exactly as before.
func TestBriefHasNoMapLinesWithoutMaps(t *testing.T) {
	repo := hookMapRepo(t)
	if code, _, errs := runCLI(t, threadCLI, "note", "where it stands", "--repo", repo); code != 0 {
		t.Fatalf("note: %s", errs)
	}
	primer, _, _ := composeContinuityAndCustodian("identity", repo, time.Now())
	if strings.Contains(primer, "(map ") || strings.Contains(primer, "fathom map") || strings.Contains(primer, "[maps") {
		t.Errorf("no maps, no map lines:\n%s", primer)
	}
}

// ─── XM-13 ───────────────────────────────────────────────────────────────

// TestPreCompactPointerCarriesActiveMapIDs pins XM-13: the pre-compact pointer
// carries the ids of the alive maps the session filed or added to (not another
// session's, not an ended one's), and a session that resumes the thread finds
// them in its brief and in the handoff.
func TestPreCompactPointerCarriesActiveMapIDs(t *testing.T) {
	repo := hookMapRepo(t)
	t.Setenv("FATHOM_CHANNELS", "")
	now := time.Now().UTC()
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-pc-1", now.Add(-30*time.Minute))
	fileMapEntry(t, repo, "pay-path", "g1.go#L3-L3", "entry", "sess-pc-1", now.Add(-10*time.Minute))
	fileMapEntry(t, repo, "someone-elses", "g2.go#L3-L3", "x", "sess-other", now.Add(-5*time.Minute))
	fileMapEntry(t, repo, "long-gone", "g3.go#L3-L3", "x", "sess-earlier", now.Add(-2*time.Hour))
	seedItem(t, "se-earlier", "session-end", "ended", func(it *threadItem) {
		it.Pointers = map[string]string{"session_id": "sess-earlier"}
	})

	raw, _ := json.Marshal(map[string]any{"session_id": "sess-pc-1", "cwd": repo, "trigger": "auto"})
	defer pipeStdin(t, string(raw))()
	var out, errb bytes.Buffer
	if code := cmdPreCompactHook(nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var pointer threadItem
	for _, it := range loadThreadItems(threadsPath()) {
		if it.Kind == "pre-compact" {
			pointer = it
		}
	}
	if pointer.ID == "" {
		t.Fatal("no pre-compact pointer was filed")
	}
	if got := pointer.Pointers[mapsPointerKey]; got != "pay-path auth-surface" {
		t.Errorf("the pointer's maps = %q, want this session's alive maps, most recently used first: %q", got, "pay-path auth-surface")
	}
	if !strings.Contains(pointer.Text, "active maps: pay-path auth-surface") {
		t.Errorf("the pointer text names them too: %q", pointer.Text)
	}

	// A fresh session on the thread: the pointer's bullet and the maps arrive.
	primer, _, _ := composeContinuityAndCustodian("identity", repo, now)
	if !strings.Contains(primer, "[maps pay-path auth-surface]") {
		t.Errorf("resuming the thread re-surfaces the pointer's maps on its bullet:\n%s", primer)
	}
	for _, id := range []string{"(map pay-path)", "(map auth-surface)"} {
		if !strings.Contains(primer, id) {
			t.Errorf("the brief re-mounts %s:\n%s", id, primer)
		}
	}
	if !strings.Contains(primer, "named by a thread note") {
		t.Errorf("a map the pointer references is ranked as named:\n%s", primer)
	}
	if code, out, _ := runCLI(t, threadCLI, "handoff", "--repo", repo); code != 0 || !strings.Contains(out, "[maps pay-path auth-surface]") {
		t.Errorf("the handoff carries the map references (exit %d):\n%s", code, out)
	}

	// A session that filed no map deposits the pointer it always did.
	raw, _ = json.Marshal(map[string]any{"session_id": "sess-none", "cwd": repo, "trigger": "manual"})
	defer pipeStdin(t, string(raw))()
	if code := cmdPreCompactHook(nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, it := range loadThreadItems(threadsPath()) {
		if it.Kind == "pre-compact" && it.Pointers["session_id"] == "sess-none" {
			if _, has := it.Pointers[mapsPointerKey]; has || strings.Contains(it.Text, "active maps") {
				t.Errorf("a session with no map carries none: %+v", it)
			}
		}
	}
}

// TestThreadNoteMapRefRefusesUnknownMaps pins the reference rule: a note names
// only maps that exist, and one flag does one job.
func TestThreadNoteMapRefRefusesUnknownMaps(t *testing.T) {
	repo := hookMapRepo(t)
	fileMapEntry(t, repo, "auth-surface", "a.go#L3-L5", "the handler", "sess-parent", time.Now().Add(-time.Minute))
	if code, _, errs := runCLI(t, threadCLI, "note", "where it stands", "--map-ref", "no-such-map", "--repo", repo); code == 0 || !strings.Contains(errs, "no such map") {
		t.Errorf("an unknown map is refused (exit %d): %s", code, errs)
	}
	if code, _, errs := runCLI(t, threadCLI, "note", "x", "--map", "auth-surface", "--map-ref", "auth-surface", "--anchor", "a.go#L3-L5", "--repo", repo); code == 0 || !strings.Contains(errs, "use one") {
		t.Errorf("--map and --map-ref together are refused (exit %d): %s", code, errs)
	}
	if code, out, errs := runCLI(t, threadCLI, "note", "where it stands", "--map-ref", "auth-surface, auth-surface", "--repo", repo); code != 0 {
		t.Fatalf("note: %s %s", out, errs)
	}
	var found bool
	for _, it := range loadThreadItems(threadsPath()) {
		if it.Kind == "handoff-note" {
			found = true
			if got := it.Pointers[mapsPointerKey]; got != "auth-surface" {
				t.Errorf("stored refs = %q, want the one map once", got)
			}
		}
	}
	if !found {
		t.Error("the note was not filed")
	}
}
