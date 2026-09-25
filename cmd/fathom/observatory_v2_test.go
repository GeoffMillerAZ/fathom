package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMissModel pins IQ-3: per-tool outcomes fold with pre-IQ-1 rows as
// not-recorded, never hits.
func TestMissModel(t *testing.T) {
	one, zero := 1, 0
	entries := []adoptionCallEntry{
		{Tool: "find_symbol", ResultCount: &one, MatchedVia: "hit"},
		{Tool: "find_symbol", ResultCount: &zero, MatchedVia: "miss"},
		{Tool: "find_symbol", ResultCount: &zero, MatchedVia: "did_you_mean"},
		{Tool: "find_symbol"}, // pre-IQ-1 row
		{Tool: "outline", ResultCount: &one, MatchedVia: "hit"},
	}
	rows := buildMissModel(entries)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	fs := rows[0] // find_symbol has more calls → first
	if fs.Tool != "find_symbol" || fs.Calls != 4 || fs.Hits != 1 || fs.Misses != 1 || fs.DidYouMean != 1 || fs.NotRecorded != 1 {
		t.Errorf("find_symbol row = %+v", fs)
	}
}

// TestWiringGrade pins IQ-15: four signals fold to a letter with the one
// command that raises it.
func TestWiringGrade(t *testing.T) {
	full := obsServiceRow{}
	full.Service = "svc"
	full.Path = "/tmp/svc"
	full.HooksNote = "3 fathom hook reference(s) in project settings"
	full.StanzaNote = "present"
	full.Freshness = "fresh"
	full.CallsTotal = 5
	if g, raise := wiringGrade(full); g != "A" || raise != "" {
		t.Errorf("full wiring = %q/%q, want A with no raise", g, raise)
	}

	unwired := obsServiceRow{}
	unwired.Service = "svc"
	unwired.Path = "/tmp/svc"
	unwired.HooksNote = "no project settings.json (user-scope or plugin wiring not visible from here)"
	unwired.StanzaNote = "no CLAUDE.md"
	unwired.Freshness = "fresh"
	unwired.CallsTotal = 5
	g, raise := wiringGrade(unwired)
	if g != "C" {
		t.Errorf("grade = %q, want C (2 of 4 signals)", g)
	}
	if !strings.Contains(raise, "reconcile") {
		t.Errorf("raise = %q, want the reconcile command", raise)
	}

	stale := full
	stale.Freshness = "stale: indexed a vs HEAD b"
	if _, raise := wiringGrade(stale); !strings.Contains(raise, "fathom post-commit-hook --dir /tmp/svc --service svc") || strings.Contains(raise, "fathom ingest") {
		t.Errorf("stale raise = %q, want the daemon-routed refresh (a shell ingest writes a store the daemon never reads)", raise)
	}
}

// TestFleetSparklines pins IQ-8: tiles carry per-service sparklines, the
// grid orders by activity, the top-tools chart renders, grades chip on.
func TestFleetSparklines(t *testing.T) {
	m := syntheticModel()
	m.Services[0].TrafficDays = []int{0, 1, 5, 2}
	m.Services[0].Grade = "B"
	quiet := obsServiceRow{}
	quiet.Service = "aardvark" // alphabetically first, least active — must render AFTER alpha
	quiet.CallsTotal = 1
	quiet.Grade = "F"
	m.Services = append(m.Services, quiet)
	m.Misses = []obsMissRow{{Tool: "find_symbol", Calls: 9, Hits: 8, Misses: 1}}

	body := renderObsFleet(m)
	if !strings.Contains(body, "14-day traffic") {
		t.Error("active tile must carry its traffic sparkline")
	}
	if !strings.Contains(body, "which tools carry the load") || !strings.Contains(body, "find_symbol") {
		t.Error("top-tools chart missing")
	}
	if !strings.Contains(body, `>B<`) || !strings.Contains(body, `>F<`) {
		t.Error("grade chips missing")
	}
	if alphaAt, quietAt := strings.Index(body, ">alpha<"), strings.Index(body, ">aardvark<"); alphaAt < 0 || quietAt < 0 {
		t.Error("both tiles must render")
	}
}

// TestImpactFourQuestions pins IQ-7.
func TestImpactFourQuestions(t *testing.T) {
	m := syntheticModel()
	catches := 3
	med := 4
	m.Impact = obsImpact{
		LedgerSessions:     10,
		VerifyCatchesTotal: &catches,
		VerifyCatchesNote:  "verify ran in 2 of 10 ledger sessions",
		DiscoveryNote:      "instrumented in 10 of 10 ledger sessions",
		PilotState:         "idle",
		PilotDetail:        "no OFF-arm sessions recorded",
		FirstEditMedian:    &med,
		FirstEditNote:      "median across 6 sessions with a recorded first edit",
		StoriesNote:        "no story rows quoted here",
		ObligationCalls:    42,
	}
	m.Misses = []obsMissRow{{Tool: "find_symbol", Calls: 5, Hits: 4, NotRecorded: 1}}
	body := renderObsImpact(m)
	for _, want := range []string{"Q1", "Q2", "Q3", "Q4", "law 12", "misses are the roadmap", "2 of 10", "per-tool outcomes", "not-recorded", "law 14"} {
		if !strings.Contains(body, want) {
			t.Errorf("impact page missing %q", want)
		}
	}
}

// TestLandingFourQuestions pins IQ-6.
func TestLandingFourQuestions(t *testing.T) {
	hermeticObsEnv(t)
	h := newUnreachableConsoleHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"the four questions", "Q1", "Q2", "Q3", "Q4", "law 12", `href="/sessions"`} {
		if !strings.Contains(body, want) {
			t.Errorf("landing missing %q", want)
		}
	}
}

// TestObservatorySessionPage pins IQ-4: the replay renders a timeline from
// beats, the ledger verdict card, and honest absences for unknown ids.
func TestObservatorySessionPage(t *testing.T) {
	dir := hermeticObsEnv(t)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(dir, "beats.jsonl"))
	beats := strings.Join([]string{
		`{"channel":"session-hook","kind":"emitted","at":"2026-08-08T10:00:00Z","meta":{"session":"s1","service":"svc"}}`,
		`{"channel":"edit-hook","kind":"seen","at":"2026-08-08T10:05:00Z","meta":{"session":"s1"}}`,
		`{"channel":"edit-hook","kind":"seen","at":"2026-08-08T10:06:00Z","meta":{"session":"OTHER"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "beats.jsonl"), []byte(beats), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := `{"schema_version":2,"session_id":"s1","arm":"on","repo":"svc","at":"2026-08-08T10:10:00Z","tokens_note":"","turn_count":9,"first_edit_turn":4}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}

	st := buildSessionStory("s1")
	if len(st.Beats) != 2 {
		t.Fatalf("beats = %d, want 2 (the OTHER session's beat excluded)", len(st.Beats))
	}
	if st.Ledger == nil || st.Ledger.FirstEditTurn == nil || *st.Ledger.FirstEditTurn != 4 {
		t.Fatalf("ledger row not resolved: %+v", st.Ledger)
	}
	body := renderSessionStory(st)
	for _, want := range []string{"session s1", "law 14", "timeline", "first edit at turn"} {
		if !strings.Contains(body, want) {
			t.Errorf("session page missing %q", want)
		}
	}

	h := observatorySessionPage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/session", nil))
	if rec.Code != 400 {
		t.Errorf("missing id = %d, want 400", rec.Code)
	}
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/session?id=s1", nil))
	if rec.Code != 200 {
		t.Errorf("GET session s1 = %d", rec.Code)
	}
}

// TestObservatoryStoryPage pins IQ-5: the week folds only trailing-7d rows
// and links session replays; absence is named.
func TestObservatoryStoryPage(t *testing.T) {
	dir := hermeticObsEnv(t)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	rows := []string{
		`{"schema_version":2,"session_id":"recent","arm":"on","at":"2026-08-07T10:00:00Z","tokens_note":"","verify_catches":2,"first_edit_turn":3}`,
		`{"schema_version":2,"session_id":"ancient","arm":"on","at":"2026-07-01T10:00:00Z","tokens_note":"","verify_catches":9}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := buildWeekStory(now)
	if ws.Sessions != 1 || ws.Catches != 2 {
		t.Fatalf("week = %+v, want 1 session / 2 catches (ancient row excluded)", ws)
	}
	body := renderWeekStory(ws, observatoryModel{})
	for _, want := range []string{"the week", "law 14", "recent", "1 of 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("story page missing %q", want)
		}
	}

	h := observatoryStoryPage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/story", nil))
	if rec.Code != 200 {
		t.Errorf("GET /observatory/story = %d", rec.Code)
	}
}

// TestInitAutoNarrate pins IQ-10: init arms a bounded window; an active
// window is left untouched.
func TestInitAutoNarrate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(dir, "narrate.json"))

	var out strings.Builder
	armAutoNarrate(&out)
	st, active := loadNarrateState(time.Now())
	if !active {
		t.Fatal("init must arm the narrate window")
	}
	if st.SessionsRemaining == nil || *st.SessionsRemaining != autoNarrateSessions {
		t.Fatalf("window = %+v, want %d sessions", st, autoNarrateSessions)
	}
	if !strings.Contains(out.String(), "self-expiring") {
		t.Error("arming must be loudly announced")
	}

	out.Reset()
	armAutoNarrate(&out)
	if !strings.Contains(out.String(), "already armed") {
		t.Error("an active window must be left untouched, and said so")
	}
}

// TestBriefFourQuestions pins IQ-11.
func TestBriefFourQuestions(t *testing.T) {
	m := observatoryModel{GeneratedAt: "2026-08-08T12:00:00Z", FathomVersion: "test"}
	m.Impact.LedgerSessions = 3
	m.Impact.VerifyCatchesNote = "verify never ran in any ledger session — unknown, not zero"
	m.Impact.DiscoveryNote = "discovery instrument absent"
	m.Impact.FirstEditNote = "no sessions carry first_edit_turn yet"
	m.Impact.PilotState = "idle"
	m.Impact.PilotDetail = "no OFF-arm sessions recorded"
	md := renderBriefMD(m)
	for _, want := range []string{
		"## 1 — How much is fathom being used?",
		"## 2 — What did fathom prevent?",
		"## 3 — Did it save tokens?",
		"## 4 — Where does the agent still flounder?",
		"## 5 — Is the knowledge compounding?",
		"## Not measured", "law 12", "law 14",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}

// TestImpactExplorationFold pins EX-3: the fold computes per-question costs
// and the labeled observational ratio; rows without the ledger are honest.
func TestImpactExplorationFold(t *testing.T) {
	dir := hermeticObsEnv(t)
	rows := []string{
		// raw: 9000 est over 3 calls (3000/q); fathom: 100 over 5 (20/q) → ratio 150.0 → x10 = 1500
		`{"schema_version":2,"session_id":"a","arm":"on","at":"2026-08-08T10:00:00Z","tokens_note":"","raw_discovery_calls":3,"raw_discovery_tokens_est":9000,"fathom_calls":5,"fathom_tokens_est":100,"repeated_read_files":2,"repeated_read_tokens_est":4000,"sidechain_tool_results":6}`,
		`{"schema_version":2,"session_id":"old","arm":"on","at":"2026-08-01T10:00:00Z","tokens_note":""}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := buildImpactModel(time.Now())
	if imp.ExpSessions != 1 {
		t.Fatalf("ExpSessions = %d, want 1", imp.ExpSessions)
	}
	if imp.ExpRatioMedianX10 == nil || *imp.ExpRatioMedianX10 != 1500 {
		t.Fatalf("ExpRatioMedianX10 = %v, want 1500 (150.0x)", imp.ExpRatioMedianX10)
	}
	if !strings.Contains(imp.ExpNote, "OBSERVATIONAL") {
		t.Errorf("the ratio must be labeled observational: %q", imp.ExpNote)
	}
	if imp.ExpRepeatFiles != 2 || imp.ExpRepeatTokens != 4000 || imp.ExpSidechainShare != 6 {
		t.Errorf("duplication fold = %+v", imp)
	}
}

// TestConfigPanel pins EX-7/FR-5: layers with eval state, guard counts,
// binding presence, and named absences with install commands.
func TestConfigPanel(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(".fathom/layers/ui-appkit/layer.cue", "name: \"ui-appkit\"\n")
	mk(".fathom/layers/evaled-pack/layer.cue", "name: \"evaled-pack\"\n")
	mk(".fathom/layers/evaled-pack/eval/results.json", "{}")
	mk(".fathom/guard.cue", "guards: [{scopes: [\"cmd/**\"], note: \"a\"}, {command_scopes: [\"deploy *\"], note: \"b\"}]\n")

	p := buildConfigPanel(root, 42)
	if len(p.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(p.Layers))
	}
	byName := map[string]bool{}
	for _, l := range p.Layers {
		byName[l.Name] = l.Evaled
	}
	if byName["ui-appkit"] || !byName["evaled-pack"] {
		t.Errorf("eval detection wrong: %+v", p.Layers)
	}
	if p.GuardCount != 2 {
		t.Errorf("guards = %d, want 2", p.GuardCount)
	}
	if !strings.Contains(p.BindingNote, "no bindings file") {
		t.Errorf("binding absence must be named: %q", p.BindingNote)
	}
	if p.ObligationServes != 42 {
		t.Errorf("serves = %d", p.ObligationServes)
	}

	empty := buildConfigPanel(t.TempDir(), 0)
	if !strings.Contains(empty.LayersNote, "fathom layers install") {
		t.Errorf("unconfigured repo must see the install command: %q", empty.LayersNote)
	}
}

// TestImpactExplorationStory pins EX-4: the impact page LEADS with the
// exploration story — per-question raw vs indexed cost, the observed ratio
// labeled OBSERVATIONAL (the arm pilot carries causality), and the
// subagent duplication number.
func TestImpactExplorationStory(t *testing.T) {
	m := syntheticModel()
	ratio := 300
	m.Impact = obsImpact{
		ExpSessions:       2,
		ExpRawCalls:       10,
		ExpRawTokens:      30000,
		ExpFathomCalls:    5,
		ExpFathomTokens:   500,
		ExpRatioMedianX10: &ratio,
		ExpRepeatFiles:    3,
		ExpRepeatTokens:   9000,
		ExpNote:           "OBSERVATIONAL: per-question token cost, raw exploration vs indexed discovery",
		VerifyCatchesNote: "verify never ran in any ledger session — unknown, not zero",
		DiscoveryNote:     "discovery instrument absent",
		FirstEditNote:     "no sessions carry first_edit_turn yet",
		PilotState:        "idle",
		PilotDetail:       "no OFF-arm sessions recorded",
		StoriesNote:       "no story rows quoted here",
	}
	body := renderObsImpact(m)
	for _, want := range []string{"the exploration story", "per question", "30.0×", "re-read", "OBSERVATIONAL"} {
		if !strings.Contains(body, want) {
			t.Errorf("impact page missing %q", want)
		}
	}
}

// TestPlainEnglishLayer pins EX-5: every observatory page opens with a
// plain-English lead and carries a no-JS explainer a reader who has never
// heard of fathom can follow.
func TestPlainEnglishLayer(t *testing.T) {
	m := observatoryModel{GeneratedAt: "2026-08-08T12:00:00Z", FathomVersion: "test"}
	body := obsPage("t", "impact", "<div>x</div>", m)
	if !strings.Contains(body, "what am I looking at?") {
		t.Error("explainer summary missing")
	}
	if !strings.Contains(body, "every number names its instrument") {
		t.Error("impact lead missing")
	}
	if !strings.Contains(body, "<details") {
		t.Error("no-JS explainer must be a details element")
	}
	if home := obsPage("t", "home", "<div>x</div>", m); !strings.Contains(home, "code index") {
		t.Error("home lead must explain fathom as a code index")
	}
}

// TestObservatoryBriefPage pins EX-6: GET /observatory/brief renders the
// management-ready brief as a page (markdown headings become h2 elements)
// and ?format=md downloads the same markdown.
func TestObservatoryBriefPage(t *testing.T) {
	hermeticObsEnv(t)
	h := observatoryBriefPage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/brief", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /observatory/brief = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "download as markdown") {
		t.Error("page must link the markdown download")
	}
	if !strings.Contains(body, "<h2") {
		t.Error("brief headings must render as h2 elements")
	}

	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/brief?format=md", nil))
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/markdown") {
		t.Errorf("Content-Type = %q, want text/markdown", ct)
	}
	if !strings.Contains(rec.Body.String(), "# fathom value brief") {
		t.Error("markdown download must carry the brief")
	}
}

// TestSurfaceSchemaTax pins TD-1 (docs/plans/exploration-ledger.md): the
// standing context cost of the agent surface is MEASURED — tool count and
// serialized schema bytes — and an unreachable surface yields zeros, which
// the renderer names rather than charts (law 3).
func TestSurfaceSchemaTax(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"find_symbol","inputSchema":{"type":"object"}},{"name":"outline","inputSchema":{"type":"object"}}]}}`))
	}))
	defer srv.Close()
	hc := &http.Client{}
	tools, schemaBytes := surfaceSchemaTax(hc, srv.URL)
	if tools != 2 {
		t.Errorf("tools = %d, want 2", tools)
	}
	if schemaBytes <= 0 {
		t.Errorf("schema bytes = %d, want > 0", schemaBytes)
	}
	if tools, schemaBytes := surfaceSchemaTax(hc, "http://127.0.0.1:1"); tools != 0 || schemaBytes != 0 {
		t.Errorf("unreachable surface = %d/%d, want zeros", tools, schemaBytes)
	}
	if v := contextTaxValue(0, 0); !strings.Contains(v, "not measured") {
		t.Errorf("zero tax must be named unmeasured: %q", v)
	}
	if v := contextTaxValue(16, 12600); !strings.Contains(v, "16 tools") || !strings.Contains(v, "KB") {
		t.Errorf("tax line = %q", v)
	}
}

// TestWiringGradeUnknowns pins EX-8 (law 3): a signal invisible from the
// repo walk (user-scope hook wiring) leaves the DENOMINATOR — it must never
// grade as absent.
func TestWiringGradeUnknowns(t *testing.T) {
	row := obsServiceRow{}
	row.Service = "svc"
	row.Path = "/tmp/svc"
	row.HooksNote = "no project settings.json (user-scope or plugin wiring not visible from here)"
	row.StanzaNote = "present"
	row.Freshness = "fresh"
	row.CallsTotal = 9
	g, raise := wiringGrade(row)
	if g != "A" {
		t.Errorf("grade = %q, want A — 3 of 3 VISIBLE signals pass; the invisible one must not count against", g)
	}
	if strings.Contains(raise, "reconcile") {
		t.Errorf("invisible hooks must not trigger the reconcile remedy: %q", raise)
	}
}
