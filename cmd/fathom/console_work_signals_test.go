package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// workRepo is a git repository whose history exercises /work's commit
// reads: c1 proves A-1 and is reworked by c2 a day later; c3 proves A-2
// and is reverted by c4 the next day; c5 touches c1's file after c1's
// 7-day window has closed.
type workRepo struct {
	root                   string
	base                   time.Time
	c1, c2, c3, c4, c5, c6 string
}

func newWorkRepo(t *testing.T) workRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := workRepo{root: t.TempDir(), base: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)}
	write := func(name string, lines ...string) {
		p := filepath.Join(r.root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	head := func() string {
		out, err := exec.Command("git", "-C", r.root, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(at time.Time, msg string) string {
		gitAt(t, r.root, at, "add", "-A")
		gitAt(t, r.root, at, "commit", "-q", "-m", msg)
		return head()
	}
	gitAt(t, r.root, r.base, "init", "-q")
	write("internal/pay/a.go", "l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	write("web/b.go", "b1")
	r.c1 = commit(r.base, "pay: add a")
	write("internal/pay/a.go", "l1", "L2", "L3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	r.c2 = commit(r.base.Add(24*time.Hour), "pay: rework a")
	write("web/c.go", "c1", "c2", "c3", "c4", "c5")
	r.c3 = commit(r.base.Add(48*time.Hour), "web: add c")
	gitAt(t, r.root, r.base.Add(72*time.Hour), "revert", "--no-edit", r.c3)
	r.c4 = head()
	write("internal/pay/a.go", "l1", "L2", "L3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11")
	r.c5 = commit(r.base.Add(10*24*time.Hour), "pay: later change")
	write("web/d.go", "d1")
	r.c6 = commit(r.base.Add(26*24*time.Hour), "web: add d, window still open at now")
	return r
}

// workRepoJoin is the join /work reads for newWorkRepo: A-1 proven at c1,
// A-2 proven at c3 (named abbreviated, as a ledger may) after one
// claimed-unproven reading, A-3 proven at c6, and A-4 at a commit this
// clone does not have.
func workRepoJoin(r workRepo) (console.JoinResult, console.History) {
	at := func(d int) string { return r.base.Add(time.Duration(d) * time.Hour).Format(time.RFC3339) }
	rows := []console.ProvenRow{
		{Service: "svc", Plan: "p.md", ID: "A-1", At: at(1), Commit: r.c1},
		{Service: "svc", Plan: "p.md", ID: "A-2", At: at(49), Commit: r.c3[:12]},
		{Service: "svc", Plan: "p.md", ID: "A-3", At: at(26*24 + 1), Commit: r.c6},
		{Service: "svc", Plan: "p.md", ID: "A-4", At: at(26*24 + 2), Commit: strings.Repeat("d", 40)},
	}
	h := console.History{}
	for _, pr := range rows {
		rh := console.RowHistory{Service: "svc", Plan: "p.md", ID: pr.ID, Readings: []console.RowReading{{At: at(0), State: console.HistOpen}}}
		if pr.ID == "A-2" {
			rh.Readings = append(rh.Readings, console.RowReading{At: at(40), State: console.HistUnproven})
		}
		rh.Readings = append(rh.Readings, console.RowReading{At: pr.At, Commit: pr.Commit, State: console.HistProven, Weight: 1})
		h.Rows = append(h.Rows, rh)
	}
	return console.JoinResult{Outcomes: []console.SessionOutcome{{SessionID: "s1", Rows: rows}}}, h
}

// WI-4w: /work reads rework within 7 days and the later commits that
// revert a proving commit with one read-only git log, beside the
// claimed-unproven retries its proof history carries; a part it could not
// read is named, never a zero.
func TestWorkReadsReworkAndReverts(t *testing.T) {
	r := newWorkRepo(t)
	join, h := workRepoJoin(r)
	now := r.base.Add(28 * 24 * time.Hour)
	facts, note := workCommitFacts(context.Background(), r.root, join, now)

	c1 := facts[r.c1]
	if !c1.ReworkKnown || c1.ReworkLines != 4 {
		t.Fatalf("c1's file was changed again by c2 within 7 days (2 added, 2 deleted); c5 is outside the window: %+v", c1)
	}
	if !c1.RevertsKnown || len(c1.Reverts) != 0 {
		t.Fatalf("nothing reverts c1, read as a measured zero: %+v", c1)
	}
	c3 := facts[r.c3[:12]]
	if !c3.ReworkKnown || c3.ReworkLines != 5 {
		t.Fatalf("the revert deleted c3's five lines within its window: %+v", c3)
	}
	if !c3.RevertsKnown || len(c3.Reverts) != 1 || c3.Reverts[0].Commit != r.c4 || c3.Reverts[0].At == "" {
		t.Fatalf("c4 reverts c3 (the abbreviated spelling the ledger used): %+v", c3)
	}
	if c6 := facts[r.c6]; c6.ReworkKnown || !strings.Contains(c6.ReworkReason, "still open") || !c6.RevertsKnown {
		t.Fatalf("c6's 7-day window is still open: rework unknown and named, reverts read: %+v", c6)
	}
	if _, ok := facts[strings.Repeat("d", 40)]; ok {
		t.Fatal("a commit this clone does not have is not read")
	}
	for _, want := range []string{"3 of 4 proving commits", "not in this clone", "git log", "1 revert"} {
		if !strings.Contains(note, want) {
			t.Errorf("the commits note names %q: %s", want, note)
		}
	}

	changes := console.EffortPerProvenChange(join, h, nil, facts, console.NewZones(nil, nil))
	byCommit := map[string]console.ProvenChange{}
	for _, c := range changes {
		byCommit[c.Commit] = c
	}
	if c := byCommit[r.c1]; !c.Rework.Known() || c.Rework.V != 4 || !c.Reverted.Known() || c.Reverted.V != 0 || c.Retries != 0 {
		t.Fatalf("A-1's change: rework 4, no revert, first attempt: %+v", c)
	}
	if c := byCommit[r.c3[:12]]; c.Rework.V != 4+1 || c.Reverted.V != 1 || c.Retries != 1 {
		t.Fatalf("A-2's change: rework 5, reverted once, one claimed-unproven retry: %+v", c)
	}
	if c := byCommit[r.c6]; c.Rework.Known() || !strings.Contains(c.Rework.Reason, "still open") {
		t.Fatalf("A-3's rework is unknown with its reason: %+v", c.Rework)
	}
	if c := byCommit[strings.Repeat("d", 40)]; c.Rework.Known() || c.Reverted.Known() || c.Reverted.Reason == "" {
		t.Fatalf("A-4's commit was not read: rework and reverts absent, named: %+v", c)
	}

	// A root git cannot read names the failure; nothing is a zero.
	bad, badNote := workCommitFacts(context.Background(), t.TempDir(), join, now)
	if len(bad) != 0 || !strings.Contains(badNote, "could not be read") {
		t.Fatalf("an unreadable repository is named: %v %q", bad, badNote)
	}
}

// workLedgers writes the verify ledger newWorkRepo's rows were proven in
// (FATHOM_VERIFY_LEDGER) and returns the model /work joins it to: one
// session in the repository covering the whole history.
func workLedgers(t *testing.T, r workRepo) observatoryModel {
	t.Helper()
	at := func(h int) string { return r.base.Add(time.Duration(h) * time.Hour).Format(time.RFC3339) }
	n := 0
	line := func(h int, commit string, states map[string]string) string {
		n++
		var rows []string
		for _, id := range []string{"A-1", "A-2", "A-3", "A-4"} {
			rows = append(rows, fmt.Sprintf(`{"id":%q,"state":%q,"weight":1}`, id, states[id]))
		}
		return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":%q,"service":"svc","plan":"docs/plans/p.md","commit":%q,"origin":"cli","total":4,"claimed":1,"verified":1,"digest":"d%d","rows":[%s]}`,
			at(h), r.root, commit, n, strings.Join(rows, ","))
	}
	st := map[string]string{"A-1": "open", "A-2": "open", "A-3": "open", "A-4": "open"}
	step := func(h int, commit, id, state string) string {
		st[id] = state
		cp := map[string]string{}
		for k, v := range st {
			cp[k] = v
		}
		return line(h, commit, cp)
	}
	ledger := strings.Join([]string{
		line(-1, r.c1[:12], st),
		step(1, r.c1[:12], "A-1", "claimed-proven"),
		step(40, r.c2[:12], "A-2", "claimed-unproven"),
		step(49, r.c3[:12], "A-2", "claimed-proven"),
		step(26*24+1, r.c6[:12], "A-3", "claimed-proven"),
		step(26*24+2, "ddddddddddddd", "A-4", "claimed-proven"),
	}, "\n") + "\n"
	lp := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	if err := os.WriteFile(lp, []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", lp)
	sess := fmt.Sprintf(`{"schema_version":2,"session_id":"s1","repo":%q,"at":%q,"duration_ms":%d,"turn_count":40,"tokens_in":50000,"tokens_out":5000}`,
		r.root, at(27*24), int64(28*24*time.Hour/time.Millisecond))
	return observatoryModel{ledger: foldSessionLedger([]byte(sess + "\n"))}
}

// WI-2w: /work reads the CI summary files `[work] ci_summaries` names,
// binds each run to its proving commit and passes the runs to U1; a
// missing or unparseable file is named on the page.
func TestComputeWorkReadsNamedCISummary(t *testing.T) {
	r := newWorkRepo(t)
	m := workLedgers(t, r)
	files := map[string]string{
		"ci/junit.xml": `<?xml version="1.0"?><testsuites time="120"><testsuite name="a" time="120"></testsuite></testsuites>`,
		"ci/runs.json": fmt.Sprintf(`{"workflow_runs":[{"head_sha":%q,"run_started_at":"2026-09-03T10:00:00Z","updated_at":"2026-09-03T10:06:00Z"}]}`, r.c3),
		"ci/bad.xml":   `<testsuites time=`,
		".fathom/config.toml": fmt.Sprintf("[[work.ci_summaries]]\npath = \"ci/junit.xml\"\ncommit = %q\n\n[[work.ci_summaries]]\npath = \"ci/runs.json\"\n\n"+
			"[[work.ci_summaries]]\npath = \"ci/missing.xml\"\ncommit = %q\n\n[[work.ci_summaries]]\npath = \"ci/bad.xml\"\ncommit = %q\n", r.c1, r.c3, r.c3),
	}
	for name, body := range files {
		p := filepath.Join(r.root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	model := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), nil)
	if model.Work.Changes != 4 {
		t.Fatalf("four proven changes joined to the session: %+v (unjoined %d)", model.Work, model.Unjoined)
	}
	read := strings.Join(model.CIRead, "\n")
	for _, want := range []string{"ci/junit.xml: 1 run, 2.0 CI minutes", "ci/runs.json: 1 run, 6.0 CI minutes", "ci/missing.xml: not read", "ci/bad.xml: not parsed"} {
		if !strings.Contains(read, want) {
			t.Errorf("the CI read names %q:\n%s", want, read)
		}
	}
	zones := map[string]console.ZoneCost{}
	for _, z := range model.Work.CostPerZone {
		zones[z.Zone] = z
	}
	if z := zones["internal/pay"]; !z.CIMinutes.Known() || z.CIMinutes.V != 1 || z.CIMinutes.State != console.StatePresent {
		t.Fatalf("c1's 2 minutes split across its two zones, bound through the abbreviated spelling: %+v", z.CIMinutes)
	}
	if z := zones["web"]; z.CIMinutes.State != console.StatePartial || z.CIMinutes.V != 7 {
		t.Fatalf("web: c1's half plus c3's 6 minutes, c6 has no run (a lower bound): %+v", z.CIMinutes)
	}
	out := renderWorkIntelligence(model.Work, model.History, model.Unjoined, model.Commits, model.CIRead)
	if !strings.Contains(out, "ci/missing.xml: not read") {
		t.Error("the cost-per-zone panel names the file it could not read")
	}

	// No [work] table: the absence is named, never a zero.
	if err := os.Remove(filepath.Join(r.root, ".fathom", "config.toml")); err != nil {
		t.Fatal(err)
	}
	none := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), nil)
	if len(none.CIRead) != 1 || !strings.Contains(none.CIRead[0], "no CI summary file named") {
		t.Fatalf("no named file is said: %q", none.CIRead)
	}
}

// fakeWorkIndex answers /work's index reads from maps (the store-backed
// workIndex is storeWorkIndex).
type fakeWorkIndex struct {
	files map[string]string
	rules map[string]string
	refs  map[string]int
	stale []console.StaleDocSpan
}

func (f fakeWorkIndex) SymbolFile(_ context.Context, _, symbol string) (string, bool) {
	p, ok := f.files[symbol]
	return p, ok
}

func (f fakeWorkIndex) RuleTexts(_ context.Context, _ string, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if t, ok := f.rules[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

func (f fakeWorkIndex) RefCount(_ context.Context, _, symbol string) (int, bool) {
	n, ok := f.refs[symbol]
	return n, ok
}

func (f fakeWorkIndex) StaleDocs(_ context.Context, _ string) ([]console.StaleDocSpan, string) {
	return f.stale, fmt.Sprintf("doc-link drift: %d span(s)", len(f.stale))
}

// workActivity writes the call log and rule-outcome log a friction map
// reads for newWorkRepo, and a plan whose A-2 contract anchors web.AddC:
// session sA finds pay.Settle; session sB, after sA ended, re-finds it,
// then runs five searches and lookups ending at web.AddC before its first
// edit; one row carries no session.
func workActivity(t *testing.T, r workRepo, m *observatoryModel, extra ...string) {
	t.Helper()
	ts := func(min int) string {
		return r.base.Add(20*24*time.Hour + time.Duration(min)*time.Minute).Format(time.RFC3339Nano)
	}
	row := func(min int, tool, session string, pointers ...string) string {
		caller := ""
		if session != "" {
			caller = fmt.Sprintf(`,"caller":{"pid":7,"session":%q,"test":false}`, session)
		}
		ps, _ := json.Marshal(pointers)
		return fmt.Sprintf(`{"schema_version":3,"ts":%q,"tool":%q,"service":"svc","latency_ms":1%s,"pointers":%s}`, ts(min), tool, caller, ps)
	}
	lines := []string{
		row(0, "find_symbol", "sA", "internal/pay.Settle", "internal/pay/a.go"),
		row(1, "outline", "sA", "internal/pay/a.go"),
		row(10, "find_symbol", "sB", "internal/pay.Settle", "internal/pay/a.go"),
		row(11, "search", "sB"),
		row(12, "references", "sB", "Charge", "web/b.go"),
		row(13, "search", "sB"),
		row(14, "symbol_context", "sB", "web.AddC", "web/c.go"),
		row(16, "find_symbol", "sB", "web.AddC"),
		row(17, "find_symbol", "", "internal/pay.Settle"),
		`{"schema_version":3,"ts":"` + ts(18) + `","tool":"search","service":"other","latency_ms":1,"caller":{"session":"sB"}}`,
		`{"schema_version":3,"ts":"` + ts(19) + `","tool":"search","service":"svc","origin":"console","latency_ms":1,"caller":{"session":"sB"}}`,
		`not json`,
	}
	lines = append(lines, extra...)
	cl := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
	if err := os.WriteFile(cl, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.callFacts = console.CallLogFacts{Log: console.ResolvedCallLog{Path: cl}}
	edit := fmt.Sprintf(`{"schema_version":1,"ts":%q,"rule_id":"R-web","event":"served","session":"sB","file":%q}`, ts(15), filepath.Join(r.root, "web", "c.go"))
	rl := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	if err := os.WriteFile(rl, []byte(edit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", rl)
	plan := "# p\n\n- [x] A {id: A-1, symbol: Settle, package: internal/pay, reachable_from: TestA}\n" +
		"- [x] B {id: A-2, symbol: AddC, package: web, reachable_from: TestB}\n" +
		"- [x] C {id: A-3, symbol: AddD, package: web, reachable_from: TestC}\n" +
		"- [x] D {id: A-4, symbol: Gone, package: web, reachable_from: TestD}\n"
	if err := os.MkdirAll(filepath.Join(r.root, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.root, "docs", "plans", "p.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

// WI-6w: /work reads search loops and lookups from the call log, the
// edits that end a run from the rule-outcome log, rediscovery against the
// discovery pointers earlier sessions established, claimed-unproven
// anchors from the verify ledger and the plans' contracts, and the
// reverts WI-4w read — and names each source it read.
func TestComputeWorkReadsFrictionSignals(t *testing.T) {
	r := newWorkRepo(t)
	m := workLedgers(t, r)
	workActivity(t, r, &m)
	idx := fakeWorkIndex{files: map[string]string{"web.AddC": "web/c.go"}}
	model := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), idx)
	fm := model.Friction.Map
	find := func(list []console.FrictionHotspot, key string) console.FrictionHotspot {
		for _, h := range list {
			if h.Key == key {
				return h
			}
		}
		t.Fatalf("no hotspot %q in %+v", key, list)
		return console.FrictionHotspot{}
	}
	settle := find(fm.Symbols, "internal/pay.Settle")
	if settle.Counts.Rediscovery != 2 {
		t.Errorf("pay.Settle re-found by sB and by an unsequenced row after sA established it (sA's own second call does not count): %+v", settle.Counts)
	}
	addC := find(fm.Symbols, "web.AddC")
	if addC.Counts.SearchLoops != 1 || addC.Counts.ClaimedUnproven != 1 || len(addC.LoopEnds) != 1 {
		t.Errorf("sB's five lookups end at web.AddC before its edit; A-2's claimed-unproven reading anchors there: %+v", addC)
	}
	c := find(fm.Files, "web/c.go")
	if c.Counts.Reverts != 1 || c.Counts.ClaimedUnproven != 1 || c.Counts.SearchLoops != 1 {
		t.Errorf("web/c.go: c4's revert of c3, A-2's anchor file and the loop's end: %+v", c.Counts)
	}
	if fm.Events == 0 {
		t.Fatal("activity events counted")
	}
	read := strings.Join(model.FrictionRead, "\n")
	for _, want := range []string{"rule outcomes: 1 event", "search and lookup events: 9 from the call log", "8 sequenced by session", "1 row names no session",
		"rediscovery pointers", "claimed-unproven anchors: 1 reading", "1 resolved to a file", "reverts: 1 revert", "repeated-read"} {
		if !strings.Contains(read, want) {
			t.Errorf("the friction sources name %q:\n%s", want, read)
		}
	}
	if strings.Contains(read, "not read by this page") {
		t.Errorf("no signal is left unread:\n%s", read)
	}

	// Without an index the anchor is counted by symbol only, and says so;
	// without a call log the events are named absent.
	m.callFacts = console.CallLogFacts{}
	bare := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), nil)
	bread := strings.Join(bare.FrictionRead, "\n")
	for _, want := range []string{"no call log resolved", "no index to resolve"} {
		if !strings.Contains(bread, want) {
			t.Errorf("the absences are named %q:\n%s", want, bread)
		}
	}
}

// O153: a call-log row naming no session establishes no discovery pointer.
// Its later re-find could be the same session's own, which is not a
// rediscovery, and with no session the two cannot be told apart — so two
// session-less finds of one symbol count no rediscovery, and the sources
// name the pointers left unestablished instead of reading high.
func TestWorkSessionlessCallsEstablishNoPointer(t *testing.T) {
	r := newWorkRepo(t)
	m := workLedgers(t, r)
	ts := func(min int) string {
		return r.base.Add(20*24*time.Hour + time.Duration(min)*time.Minute).Format(time.RFC3339Nano)
	}
	sessionless := func(min int) string {
		return fmt.Sprintf(`{"schema_version":3,"ts":%q,"tool":"find_symbol","service":"svc","latency_ms":1,"pointers":["internal/pay.Refund"]}`, ts(min))
	}
	workActivity(t, r, &m, sessionless(30), sessionless(31))
	idx := fakeWorkIndex{files: map[string]string{"web.AddC": "web/c.go"}}
	model := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), idx)
	for _, h := range model.Friction.Map.Symbols {
		if h.Key == "internal/pay.Refund" && h.Counts.Rediscovery != 0 {
			t.Errorf("two session-less finds of one symbol read as a rediscovery: %+v", h.Counts)
		}
	}
	read := strings.Join(model.FrictionRead, "\n")
	if !strings.Contains(read, "3 from rows naming no session not established") {
		t.Errorf("the sources do not name the session-less pointers left unestablished:\n%s", read)
	}
}

// WI-7w: /work passes the remedy table real inputs — rule texts, the
// loop-end symbols' reference counts and doc-link drift — so every
// pattern can fire, each a proposal with its evidence.
func TestComputeWorkPassesRemedyInputs(t *testing.T) {
	r := newWorkRepo(t)
	m := workLedgers(t, r)
	workActivity(t, r, &m)
	ts := func(min int) string {
		return r.base.Add(21*24*time.Hour + time.Duration(min)*time.Minute).Format(time.RFC3339)
	}
	ev := func(min int, rule, event, session, file string) string {
		return fmt.Sprintf(`{"schema_version":1,"ts":%q,"rule_id":%q,"event":%q,"session":%q,"file":%q}`, ts(min), rule, event, session, file)
	}
	rules := strings.Join([]string{
		ev(0, "R-money", "blocked", "sC", "internal/pay/a.go"),
		ev(1, "R-money", "blocked", "sC", "internal/pay/a.go"),
		ev(2, "R-web", "served", "sD", "web/b.go"),
		ev(3, "R-web", "violated", "sD", "web/b.go"),
	}, "\n") + "\n"
	rl := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	if err := os.WriteFile(rl, []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", rl)
	idx := fakeWorkIndex{
		files: map[string]string{"web.AddC": "web/c.go"},
		rules: map[string]string{"R-money": "Money is integer cents."},
		refs:  map[string]int{"web.AddC": 0},
		stale: []console.StaleDocSpan{{Doc: "docs/web.md", Span: "Adding C", Near: "web/c.go"}},
	}
	model := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), idx)
	fired := map[string]console.Remedy{}
	for _, rem := range model.Friction.Remedies {
		if rem.Status != console.RemedyProposed {
			t.Fatalf("a remedy is a proposal: %+v", rem)
		}
		fired[rem.Pattern] = rem
	}
	for _, p := range []string{console.PatternRepeatedBlocks, console.PatternRediscovery, console.PatternServedIgnored, console.PatternStaleDoc, console.PatternLoopNoRefs} {
		if _, ok := fired[p]; !ok {
			t.Errorf("pattern %q did not fire: %+v", p, model.Friction.Remedies)
		}
	}
	if ev := strings.Join(fired[console.PatternRepeatedBlocks].Evidence, " · "); !strings.Contains(ev, "Money is integer cents.") {
		t.Errorf("the repeated-blocks remedy shows the rule's served text: %s", ev)
	}
	if rem := fired[console.PatternLoopNoRefs]; rem.Target != "web.AddC" {
		t.Errorf("the no-references remedy targets the loop's end: %+v", rem)
	}
	read := strings.Join(model.FrictionRead, "\n")
	for _, want := range []string{"rule texts: 1 of 2", "reference counts: 1 of 1", "doc-link drift: 1 span"} {
		if !strings.Contains(read, want) {
			t.Errorf("the remedy inputs are named %q:\n%s", want, read)
		}
	}

	// Without an index no remedy input is read, and each says so; the
	// rule-outcome patterns still fire from the log alone.
	bare := computeWork(context.Background(), m, serviceRoot{Service: "svc", Root: r.root}, r.base.Add(28*24*time.Hour), nil)
	bread := strings.Join(bare.FrictionRead, "\n")
	for _, want := range []string{"rule texts: not read", "reference counts: not read", "doc-link drift: not read"} {
		if !strings.Contains(bread, want) {
			t.Errorf("an absent index is named %q:\n%s", want, bread)
		}
	}
	for _, rem := range bare.Friction.Remedies {
		if rem.Pattern == console.PatternLoopNoRefs || rem.Pattern == console.PatternStaleDoc {
			t.Errorf("no reference count or drift was read, so %q cannot fire: %+v", rem.Pattern, rem)
		}
	}
}

// storeWorkIndex reads the index the remedy table needs: a function's
// defining file and call sites (a type is not counted), the served text
// of a claim, and names a store with no doc index.
func TestStoreWorkIndexReadsTheIndex(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "svc", SourceRef: "web/c.go:3", SourceKind: core.KindGoFunc, Symbol: "AddC", Line: 3},
		{ServiceID: "svc", SourceRef: "web/c.go:9", SourceKind: core.KindGoFunc, Symbol: "Unused", Line: 9},
		{ServiceID: "svc", SourceRef: "web/t.go:1", SourceKind: core.KindGoType, Symbol: "Cart", Line: 1},
		{ServiceID: "svc", SourceRef: "web/b.go:5", SourceKind: core.KindGoFunc, Symbol: "Checkout", Line: 5, Calls: []string{"web.AddC"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{{ServiceID: "svc", ClaimID: "R-web", Layer: "obligations", Subject: "AGENTS.md#web", Rule: "Carts are immutable."}}, nil); err != nil {
		t.Fatal(err)
	}
	x := storeWorkIndex{st: st}
	if f, ok := x.SymbolFile(ctx, "svc", "web.AddC"); !ok || f != "web/c.go" {
		t.Errorf("web.AddC is defined in web/c.go: %q %v", f, ok)
	}
	if _, ok := x.SymbolFile(ctx, "svc", "internal/pay.AddC"); ok {
		t.Error("a package the symbol is not defined under resolves to nothing")
	}
	if n, ok := x.RefCount(ctx, "svc", "web.AddC"); !ok || n != 1 {
		t.Errorf("Checkout calls AddC once: %d %v", n, ok)
	}
	if n, ok := x.RefCount(ctx, "svc", "web.Unused"); !ok || n != 0 {
		t.Errorf("Unused is a function nothing calls: %d %v", n, ok)
	}
	if _, ok := x.RefCount(ctx, "svc", "web.Cart"); ok {
		t.Error("a type is not counted by call sites")
	}
	texts, err := x.RuleTexts(ctx, "svc", []string{"R-web", "R-gone"})
	if err != nil || len(texts) != 1 || texts["R-web"] != "Carts are immutable." {
		t.Errorf("the served claim text: %v %v", texts, err)
	}
	if spans, note := x.StaleDocs(ctx, "svc"); spans != nil || !strings.Contains(note, "not read") {
		t.Errorf("a store with no doc index is named: %v %q", spans, note)
	}
}

// StaleDocs on a SQLite store: no doc link stored is named as the link
// pass not having run (KR-6); a link whose target moved is a span near
// that target's file.
func TestStoreWorkIndexStaleDocs(t *testing.T) {
	ctx := context.Background()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "work.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	x := storeWorkIndex{st: sq}
	if spans, note := x.StaleDocs(ctx, "svc"); len(spans) != 0 || !strings.Contains(note, "KR-6") {
		t.Fatalf("no doc link stored names the link pass: %v %q", spans, note)
	}
	if err := sq.PutDocUnits(ctx, "svc", "docs/web.md", []store.DocUnit{{UnitID: "u1", Path: "docs/web.md", HeadingPath: "Adding C", Kind: "section", Body: "see web/c.go"}}); err != nil {
		t.Fatal(err)
	}
	if err := sq.SyncDocLinksForUnit(ctx, "svc", "u1", []store.DocLink{{TargetKind: "path", Target: "web/c.go", Line: 3, LinkTier: "path", TargetHashAtLink: "was"}}, 1000); err != nil {
		t.Fatal(err)
	}
	spans, note := x.StaleDocs(ctx, "svc")
	if len(spans) != 1 || spans[0] != (console.StaleDocSpan{Doc: "docs/web.md", Span: "Adding C", Near: "web/c.go"}) || !strings.Contains(note, "1 span possibly stale of 1 open doc link") {
		t.Fatalf("the moved target's span is placed near its file: %+v %q", spans, note)
	}
}
