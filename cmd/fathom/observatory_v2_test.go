package main

import (
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
	if _, raise := wiringGrade(stale); !strings.Contains(raise, "ingest") {
		t.Errorf("stale raise = %q, want the ingest command", raise)
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
	h := landingHandler("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"the four questions", "Q1", "Q2", "Q3", "Q4", "law 12", "/observatory/story"} {
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
	for _, want := range []string{"## Q1", "## Q2", "## Q3", "## Q4", "## Not measured", "law 12", "law 14"} {
		if !strings.Contains(md, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}
