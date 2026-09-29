package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// TestFleetReport pins IO-16 (vision 14 §4): one verb folds the registry,
// traffic, wiring, and observations into per-service rows — local files
// only, unmeasured cells named, never zeroed.
func TestFleetReport(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644)
	os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# x\n"+fathomStanza("alpha", false)+"\n"), 0o644) // ADR-059: the stanza's one file

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/statusz":
			w.Write([]byte(`{"services":[{"service":"alpha","indexed_commit":"deadbeef"}]}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"alpha","symbol_count":42,"languages":["go"],"path":"` + repo + `"}]}}`))
		}
	}))
	defer srv.Close()

	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	os.WriteFile(logPath, []byte(`{"ts":"2026-08-07T01:00:00Z","tool":"find_symbol","service":"alpha","surface":"agent"}
`), 0o644)
	// Route the default call-log resolution at our fixture via --json + a chdir-independent path:
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "none.jsonl"))

	var stdout, stderr bytes.Buffer
	// Use --json and point the resolver by making cwd hold the fixture log path? Simpler: the
	// call log resolution falls back to daemon candidates; here we assert the report structure
	// and wiring notes, with traffic honestly not-measured when the log isn't found.
	code := cmdFleetReport([]string{"--url", srv.URL, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v", err)
	}
	services, _ := rep["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("services = %v", rep["services"])
	}
	row := services[0].(map[string]any)
	if row["service"] != "alpha" || row["symbols"] != float64(42) {
		t.Errorf("row = %v", row)
	}
	if hooks, _ := row["hooks_note"].(string); !strings.Contains(hooks, "fathom hook") {
		t.Errorf("hooks_note = %q", hooks)
	}
	if stanza, _ := row["stanza_note"].(string); stanza != "present" {
		t.Errorf("stanza_note = %q", stanza)
	}
	if fr, _ := row["freshness"].(string); !strings.Contains(fr, "unknown") && !strings.Contains(fr, "stale") && fr != "fresh" {
		t.Errorf("freshness = %q (must be a named three-state)", fr)
	}
	if rep["schema_version"] != float64(fleetReportSchemaVersion) || rep["fathom_version"] == "" {
		t.Errorf("stamps missing: %v", rep)
	}
}

// TestFleetReport_DistinctSessionsAndUnmeasuredTraffic pins the fleet
// report's two honesty rules on the /fleet page's verb section (OC-8,
// ADR-045): session_count counts distinct sessions from the ledger the
// session-ledger resolution names (FATHOM_SESSION_LEDGER), never ledger
// rows from a hardcoded HOME path; and when the call log could not be read
// the traffic columns are omitted, never 0 beside a note saying "never
// zero" (doctrine rule 6).
func TestFleetReport_DistinctSessionsAndUnmeasuredTraffic(t *testing.T) {
	repo := t.TempDir()
	// A service whose root's directory name differs from its ID: the Stop
	// hook labels its sessions "sextant", the service is "fathom".
	sextant := filepath.Join(t.TempDir(), "sextant")
	os.MkdirAll(sextant, 0o755)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/statusz" {
			w.Write([]byte(`{"services":[]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"alpha","symbol_count":42,"languages":["go"],"path":"` + repo + `"},{"service":"fathom","symbol_count":7,"languages":["go"],"path":"` + sextant + `"}]}}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	// A HOME ledger the report must NOT read: nine sessions for alpha.
	var homeRows strings.Builder
	for i := 0; i < 9; i++ {
		homeRows.WriteString(`{"session_id":"h-` + string(rune('a'+i)) + `","repo":"alpha"}` + "\n")
	}
	os.MkdirAll(filepath.Join(home, ".fathom"), 0o755)
	os.WriteFile(filepath.Join(home, ".fathom", "session-ledger.jsonl"), []byte(homeRows.String()), 0o644)
	// The named ledger: four rows (the Stop hook writes one per turn) in
	// two sessions for alpha.
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	os.WriteFile(ledger, []byte(`{"session_id":"s-1","repo":"alpha"}
{"session_id":"s-1","repo":"alpha"}
{"session_id":"s-1","repo":"Alpha"}
{"session_id":"s-2","repo":"alpha"}
{"session_id":"s-3","repo":"sextant"}
{"session_id":"s-3","repo":"sextant"}
`), 0o644)
	t.Setenv("FATHOM_SESSION_LEDGER", ledger)
	t.Setenv("FATHOM_CALL_LOG", filepath.Join(t.TempDir(), "missing", "calls.jsonl"))

	var stdout, stderr bytes.Buffer
	if code := cmdFleetReport([]string{"--url", srv.URL, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v", err)
	}
	row := rep["services"].([]any)[0].(map[string]any)
	if row["session_count"] != float64(2) {
		t.Errorf("session_count = %v, want 2 distinct sessions from the named ledger (not 4 rows, not HOME's 9)", row["session_count"])
	}
	if f := rep["services"].([]any)[1].(map[string]any); f["service"] != "fathom" || f["session_count"] != float64(1) {
		t.Errorf("fathom = %v, want its one session, labelled by its root's directory name (never a false 0)", f)
	}
	if note, _ := rep["call_log_note"].(string); note == "" {
		t.Error("an unread call log must be named")
	}
	for _, k := range []string{"calls_total", "calls_agent"} {
		if v, ok := row[k]; ok {
			t.Errorf("%s = %v on a call log that was not read; an unmeasured column carries no zero", k, v)
		}
	}
	md := renderFleetReportMD(fleetReport{CallLogNote: "call log not read", Services: []fleetServiceRow{{Service: "alpha"}}})
	if !strings.Contains(md, "| — |") || strings.Contains(md, "| 0 (0) |") {
		t.Errorf("the markdown traffic cell must be a dash when the log was not read:\n%s", md)
	}
}

// EP-23: a repo with no recorded intent audit reports "not measured", never
// a 0 that a reader would take for "clean" (law 3).
func TestFleetReportUnprovenColumnHonestNA(t *testing.T) {
	root := "/repo/a"
	rows := []intent.LedgerRow{
		{Kind: intent.KindDebt, RepoRoot: root, At: "2026-08-01T00:00:00Z", ByKind: map[string]int{"U2": 3}},
		{Kind: intent.KindDebt, RepoRoot: root, At: "2026-08-20T00:00:00Z", ByKind: map[string]int{"U2": 4, "U12": 6}},
		{Kind: intent.KindClock, RepoRoot: root, At: "2026-08-25T00:00:00Z"},
	}

	n, at, ok := fleetUnprovenColumn(rows, root)
	if !ok || n != 10 || at != "2026-08-20T00:00:00Z" {
		t.Fatalf("newest debt row wins and every kind counts: n=%d at=%q ok=%v", n, at, ok)
	}

	if _, _, ok := fleetUnprovenColumn(rows, "/repo/never-audited"); ok {
		t.Fatal("a repo with no recorded audit must be absent, not zero")
	}
	if _, _, ok := fleetUnprovenColumn(nil, root); ok {
		t.Fatal("an empty ledger must be absent, not zero")
	}

	// The rendered cell says so in words.
	md := renderFleetReportMD(fleetReport{
		SchemaVersion: fleetReportSchemaVersion,
		Services:      []fleetServiceRow{{Service: "never-audited", Freshness: "fresh"}},
	})
	if !strings.Contains(md, "n/a: no audit recorded") {
		t.Fatalf("the unmeasured cell must name itself:\n%s", md)
	}
	if !strings.Contains(md, "| unproven |") {
		t.Fatalf("the column must be in the header:\n%s", md)
	}
}

// TestFleetWiringNotes_DetectsUserScopeWiring pins DG-14/O15: a project
// settings.json with zero (or no) fathom references does not mean fathom
// has no hooks here — `fathom init`'s own user-scope dedup (O42)
// deliberately wires hooks at ~/.claude/settings.json instead of
// per-project once they're already present there. fleetWiringNotes must
// check that scope before reporting an absence, and the reported note must
// carry the "hook reference" substring realizationGrade and observatory's
// wiring-remedy check both key off — otherwise a real user-scope find
// still grades as a wiring gap and gets a bogus reconcile remedy.
func TestFleetWiringNotes_DetectsUserScopeWiring(t *testing.T) {
	t.Run("project settings.json present but empty of fathom refs falls back to user scope", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644); err != nil {
			t.Fatal(err)
		}

		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}

		hooks, _ := fleetWiringNotes(repo)
		if !strings.Contains(hooks, "user scope") {
			t.Fatalf("hooks note must name the user scope: %q", hooks)
		}
		if !strings.Contains(hooks, "hook reference") {
			t.Fatalf("hooks note must carry the substring realizationGrade/observatory key off: %q", hooks)
		}
	})

	t.Run("no project settings.json at all also falls back to user scope", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644); err != nil {
			t.Fatal(err)
		}

		repo := t.TempDir() // no .claude/settings.json here at all

		hooks, _ := fleetWiringNotes(repo)
		if !strings.Contains(hooks, "user scope") || !strings.Contains(hooks, "hook reference") {
			t.Fatalf("hooks note must report the user-scope find: %q", hooks)
		}
	})

	t.Run("neither scope wired reports the honest absence, unchanged", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home) // no ~/.claude/settings.json in this HOME

		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}

		hooks, _ := fleetWiringNotes(repo)
		if hooks != "settings.json present, no fathom hooks" {
			t.Fatalf("got %q, want the unchanged honest-absence message", hooks)
		}
	})

	t.Run("realizationGrade treats a user-scope find as wired, not a gap", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"command":"fathom edit-hook"}]}]}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}

		hooks, stanza := fleetWiringNotes(repo)
		row := obsServiceRow{fleetServiceRow: fleetServiceRow{Service: "svc", HooksNote: hooks, StanzaNote: stanza}}
		row.StanzaNote = "present"
		rep := realizationGrade(row, realizationWindow{HasPlans: false})
		for _, d := range rep.Dims {
			if d.Name != "wired" {
				continue
			}
			if d.Met == nil || !*d.Met {
				t.Fatalf("wired dim = %+v, want MET given a genuine user-scope find (%q)", d, hooks)
			}
			return
		}
		t.Fatal("no 'wired' dimension in the report")
	})
}

// TestUserScopeFathomHookCount_PermissionsOnlyIsNotWired is the finding [5]
// regression test: a ~/.claude/settings.json that only registers the
// fathom MCP server and/or mcp__fathom__* permissions -- no hooks key at
// all -- must never be counted as wired hooks. Before this, the raw
// strings.Count(data, "fathom") substring scan over the WHOLE file matched
// these unrelated mentions, so a user with fathom's MCP tools allowed but
// NO hooks configured anywhere was reported "wired at user scope" --
// realizationGrade's hooksOK check keys off the literal "hook reference"
// substring, so this flipped the 'wired' ladder dimension to a false MET.
func TestUserScopeFathomHookCount_PermissionsOnlyIsNotWired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A very ordinary settings.json: the fathom MCP server registered and
	// its tools allowed, but genuinely zero hooks configured anywhere.
	settings := `{
		"permissions": {"allow": ["mcp__fathom__ingest", "mcp__fathom__find_symbol"]},
		"mcpServers": {"fathom": {"command": "fathom", "args": ["mcp"]}}
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	if n, ok := userScopeFathomHookCount(); !ok || n != 0 {
		t.Fatalf("userScopeFathomHookCount() = (%d, %v), want (0, true) for a permissions/mcpServers-only settings.json with no hooks key", n, ok)
	}

	repo := t.TempDir() // no project .claude/settings.json either
	hooks, _ := fleetWiringNotes(repo)
	if strings.Contains(hooks, "wired at user scope") {
		t.Fatalf("fleetWiringNotes reported user-scope wiring from a permissions/mcpServers-only settings.json: %q", hooks)
	}
}
