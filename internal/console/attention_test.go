package console

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

// attentionFixture is a fleet with one problem of every kind, listed in the
// reverse of the queue's order so the sort, not the input, decides.
func attentionFixture() Projects {
	return Projects{
		Now:    time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
		Ledger: SourceRead{Name: SourceVerifyLedger, State: StatePresent},
		Store:  SourceRead{Name: SourceStore, State: StatePresent},
		Rows: []ProjectFacts{
			{
				Service: "notes", Path: "/src/notes", Freshness: "fresh", OpenObservations: intp(7),
				Claim: Present(ClaimCounts{Total: 3, Fresh: 3}, "store"), Certs: Present(CertCounts{}, "store"),
				Guide: Present(GuideCoverage{SectionsTotal: 4}, "store"),
			},
			{
				Service: "guides", Path: "/src/guides", Freshness: "fresh", HasGuide: true,
				Guide: Present(GuideCoverage{SectionsTotal: 72, BudgetStopped: true, RanAt: "2026-09-20T10:00:00Z", Uncompiled: []string{"internal/CLAUDE.md"}}, "store"),
				Certs: Present(CertCounts{Declared: 45, Live: 0, Gaps: 45}, "store"), CertRemedy: "fathom certify gaps --service guides",
			},
			{
				Service: "stale", Path: "/src/stale", Freshness: "stale: indexed aaaaaaaaaaaa vs HEAD bbbbbbbbbbbb",
				IndexedCommit: "aaaaaaaaaaaa", RepoHead: "bbbbbbbbbbbb", RefreshRemedy: "fathom post-commit-hook --dir /src/stale --service stale --timeout 2m",
			},
			{
				Service: "remote", Freshness: "unknown: no recorded corpus path",
			},
			{
				Service: "fathom", Path: "/src/fathom", Freshness: "fresh", HasPlans: true,
				Runs: []VerifyRun{
					{Plan: "docs/plans/receipts.md", At: "2026-09-19T08:00:00Z", Commit: "954a3296aaaa", Total: 45, Claimed: 45, Verified: 42, Failing: 3},
					{Plan: "docs/plans/harness-port.md", At: "2026-09-14T08:00:00Z", Total: 34, Claimed: 34, Verified: 30},
					{Plan: "docs/plans/verbs.md", At: "2026-09-14T08:00:00Z", Total: 16, Claimed: 16, Verified: 16},
				},
			},
		},
	}
}

// TestAttentionQueue_OrdersByProofBlockingFirst pins OC-17: the queue lists
// gate failures, then unproven claims, then stale or unknown indexes, then
// certificate gaps, then uncompiled governing guides, then open
// observations — and every item names its area, its evidence link and the
// exact command that fixes it. A source that was not read makes the queue a
// lower bound, never a shorter one.
func TestAttentionQueue_OrdersByProofBlockingFirst(t *testing.T) {
	a := AttentionQueue(attentionFixture())
	var kinds []AttentionKind
	for i, it := range a.Items {
		kinds = append(kinds, it.Kind)
		if it.Rank != i+1 {
			t.Errorf("item %d has rank %d", i, it.Rank)
		}
		if it.Area == "" || it.Title == "" || it.Detail == "" || it.Remedy == "" || it.Evidence.Href == "" || it.Evidence.Label == "" {
			t.Errorf("item %d lacks its area, title, detail, evidence or remedy: %+v", i, it)
		}
		if !strings.HasPrefix(it.Evidence.Href, "/") {
			t.Errorf("item %d's evidence is not a console route: %q", i, it.Evidence.Href)
		}
	}
	want := []AttentionKind{AttentionGate, AttentionUnproven, AttentionIndex, AttentionIndex, AttentionCertificates, AttentionGuides, AttentionObservations}
	if strings.Join(kindStrings(kinds), ",") != strings.Join(kindStrings(want), ",") {
		t.Fatalf("queue order = %v, want %v", kinds, want)
	}
	gate := a.Items[0]
	if gate.Service != "fathom" || gate.Subject != "docs/plans/receipts.md" || gate.Count != 3 ||
		gate.Remedy != "cd /src/fathom && fathom verify --service fathom --plan docs/plans/receipts.md" ||
		gate.Evidence.Href != "/plans?svc=fathom" || gate.Area != "Gate" || gate.AsOf != "2026-09-19T08:00:00Z" {
		t.Errorf("gate item = %+v", gate)
	}
	if !strings.Contains(gate.Detail, "954a3296") || !strings.Contains(gate.Detail, "exits 1") {
		t.Errorf("a gate item names the run it read and what the gate does: %q", gate.Detail)
	}
	unproven := a.Items[1]
	if unproven.Subject != "docs/plans/harness-port.md" || unproven.Count != 4 || !strings.Contains(unproven.Remedy, "fathom verify --strict --service fathom --plan docs/plans/harness-port.md") {
		t.Errorf("unproven item = %+v", unproven)
	}
	// A stale index outranks one whose freshness nobody can determine, and
	// the unknown one is listed — unknown is never read as fresh.
	if a.Items[2].Service != "stale" || a.Items[2].Remedy != "fathom post-commit-hook --dir /src/stale --service stale --timeout 2m" || a.Items[3].Service != "remote" || !strings.Contains(a.Items[3].Title, "cannot be determined") {
		t.Errorf("index items = %+v / %+v", a.Items[2], a.Items[3])
	}
	if c := a.Items[4]; c.Count != 45 || c.Area != "Proof" || c.Evidence.Href != "/certifications?svc=guides" || c.Remedy != "fathom certify gaps --service guides" {
		t.Errorf("certificate item = %+v", c)
	}
	if g := a.Items[5]; !strings.Contains(g.Title, "internal/CLAUDE.md") || !strings.Contains(g.Detail, "the last obligations compile (2026-09-20, 72 sections) did not compile them — it stopped at its budget") || g.Remedy != "fathom compile --service guides --layer obligations" {
		t.Errorf("guide item = %+v", g)
	}
	if o := a.Items[6]; o.Count != 7 || o.Service != "notes" || !strings.Contains(o.Remedy, "/src/notes/.fathom/observations.md") {
		t.Errorf("observation item = %+v", o)
	}
	if a.State != StatePresent || a.ByKind[AttentionIndex] != 2 || a.Projects != 5 {
		t.Errorf("queue state %s, by kind %v, projects %d", a.State, a.ByKind, a.Projects)
	}
	// verbs.md is fully proven: no item.
	for _, it := range a.Items {
		if strings.Contains(it.Subject, "verbs.md") {
			t.Errorf("a fully proven plan is not a problem: %+v", it)
		}
	}

	// An unreadable store and an unreadable observations file: the queue
	// is a named lower bound, and the unreadable file is itself an item.
	p := attentionFixture()
	p.Store = SourceRead{Name: SourceStore, State: StateError, Reason: "database is locked"}
	p.Rows[0].OpenObservations, p.Rows[0].ObservationsErr = nil, "operation not permitted"
	b := AttentionQueue(p)
	if b.State != StatePartial || !strings.Contains(b.Reason, "database is locked") || !strings.Contains(b.Reason, "lower bound") {
		t.Errorf("an unread store must make the queue partial: %s %q", b.State, b.Reason)
	}
	last := b.Items[len(b.Items)-1]
	if last.Kind != AttentionObservations || !strings.Contains(last.Title, "cannot be read") {
		t.Errorf("an unreadable observations file must be named: %+v", last)
	}

	// Several plans with unproven rows in one project are one item, the
	// largest named first, with the command that lists every one.
	many := attentionFixture()
	many.Rows = []ProjectFacts{{Service: "pe", Path: "/src/pe", Freshness: "fresh", HasPlans: true, Runs: []VerifyRun{
		{Plan: "docs/plans/a.md", At: "2026-09-14T00:00:00Z", Claimed: 10, Verified: 8},
		{Plan: "docs/plans/b.md", At: "2026-09-20T00:00:00Z", Claimed: 50, Verified: 7},
		{Plan: "docs/plans/c.md", At: "2026-09-20T00:00:00Z", Claimed: 5, Verified: 4, Failing: 1},
	}}}
	agg := AttentionQueue(many)
	if len(agg.Items) != 2 || agg.Items[0].Kind != AttentionGate || agg.Items[1].Kind != AttentionUnproven {
		t.Fatalf("items = %+v", agg.Items)
	}
	u := agg.Items[1]
	if u.Count != 45 || u.Subject != "" || !strings.Contains(u.Title, "45 rows across 2 plans in pe") || !strings.Contains(u.Detail, "b.md 43, a.md 2") ||
		!strings.Contains(u.Detail, "2026-09-14 to 2026-09-20") || u.Remedy != "cd /src/pe && fathom plans status --service pe" {
		t.Errorf("aggregated unproven item = %+v", u)
	}

	// Nothing wrong and everything read: a measured empty queue.
	clean := Projects{Ledger: SourceRead{Name: SourceVerifyLedger, State: StatePresent}, Store: SourceRead{Name: SourceStore, State: StatePresent},
		Rows: []ProjectFacts{{Service: "ok", Freshness: "fresh", Claim: Present(ClaimCounts{}, "s"), Certs: Present(CertCounts{}, "s"), Guide: Present(GuideCoverage{}, "s")}}}
	if z := AttentionQueue(clean); z.State != StateZero || len(z.Items) != 0 {
		t.Errorf("a clean fleet = %s with %d items", z.State, len(z.Items))
	}
	// No registry: absent, with the reason — never an empty all-clear.
	if n := AttentionQueue(Projects{RegistryNote: "connection refused"}); n.State != StateAbsent || !strings.Contains(n.Reason, "connection refused") {
		t.Errorf("no registry = %s %q", n.State, n.Reason)
	}
	// The JSON carries every item with its rank, kind and remedy.
	raw, err := json.Marshal(a)
	if err != nil || !strings.Contains(string(raw), `"kind":"gate"`) || !strings.Contains(string(raw), `"by_kind":{`) {
		t.Errorf("json = %s, %v", raw, err)
	}
}

// TestAttentionQueue_StaleUnrunAndUnreadNeverReadClean: the queue says
// what the gate says of HEAD. A failure recorded at an older commit is
// history — never a present-tense gate failure — and is listed as proof
// HEAD lacks, with its commit and date; plans no run ever checked are
// listed, never skipped; an intent corpus that could not be read makes the
// certificates a named lower bound and the queue partial; and an item
// whose count nobody knows carries its state and no count at all.
func TestAttentionQueue_StaleUnrunAndUnreadNeverReadClean(t *testing.T) {
	head := "3d58a0db490552d76bd15876f6b12cbbacd492c2"
	clean := func(svc string) ProjectFacts {
		return ProjectFacts{Service: svc, Path: "/src/" + svc, Freshness: "fresh", RepoHead: head,
			Claim: Present(ClaimCounts{}, "s"), Certs: Present(CertCounts{}, "s"), Guide: Present(GuideCoverage{}, "s")}
	}
	f := clean("fathom")
	f.HasPlans, f.PlanFiles = true, []string{"docs/plans/new.md", "docs/plans/receipts.md", "docs/plans/toolident.md"}
	f.Runs = []VerifyRun{
		{Plan: "docs/plans/receipts.md", At: "2026-09-26T04:13:00Z", Commit: "3d58a0db", Total: 45, Claimed: 45, Verified: 45},
		{Plan: "docs/plans/toolident.md", At: "2026-09-19T21:00:33Z", Commit: "d8770983", Total: 10, Claimed: 8, Verified: 7, Failing: 1,
			StaleReason: "recorded at d8770983; HEAD is 3d58a0db"},
	}
	cloud := clean("cloud")
	cloud.HasPlans = true
	for i := 0; i < 11; i++ {
		cloud.PlanFiles = append(cloud.PlanFiles, "docs/plans/p"+string(rune('a'+i))+".md")
	}
	partial := clean("partial")
	partial.Certs = Partial(CertCounts{Declared: 3, Gaps: 3}, "intent corpus not fully read (docs/intent/invariants.md: operation not permitted); a lower bound")
	blind := clean("blind")
	blind.Certs = Errored[CertCounts]("intent corpus unreadable (docs/intent/README.md: operation not permitted)")
	p := Projects{Now: time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC),
		Ledger: SourceRead{Name: SourceVerifyLedger, State: StatePresent}, Store: SourceRead{Name: SourceStore, State: StatePresent},
		Rows: []ProjectFacts{f, cloud, partial, blind}}
	a := AttentionQueue(p)
	byTitle := map[string]AttentionItem{}
	for _, it := range a.Items {
		byTitle[it.Title] = it
		if it.Kind == AttentionGate {
			t.Errorf("a failure recorded at an older commit is not a gate failure at HEAD: %+v", it)
		}
		if it.State == "" || !it.State.Valid() {
			t.Errorf("item without a truth state: %+v", it)
		}
	}
	var stale, unrunF, unrunCloud, certsPartial, certsError *AttentionItem
	for i := range a.Items {
		it := &a.Items[i]
		switch {
		case it.Service == "fathom" && it.State == StateStale:
			stale = it
		case it.Service == "fathom" && strings.Contains(it.Title, "no recorded verify run"):
			unrunF = it
		case it.Service == "cloud":
			unrunCloud = it
		case it.Service == "partial" && it.Kind == AttentionCertificates:
			certsPartial = it
		case it.Service == "blind" && it.Kind == AttentionCertificates:
			certsError = it
		}
	}
	if stale == nil || stale.Kind != AttentionUnproven || stale.Subject != "docs/plans/toolident.md" || stale.AsOf != "2026-09-19T21:00:33Z" ||
		!strings.Contains(stale.Title, "before HEAD") || !strings.Contains(stale.Detail, "d8770983") || !strings.Contains(stale.Detail, "failed 1") ||
		strings.Contains(stale.Title, " fails ") || !strings.Contains(stale.Remedy, "fathom verify --service fathom --plan docs/plans/toolident.md") {
		t.Errorf("stale item = %+v; want the older run named as history, in the past tense, with the command that verifies HEAD", stale)
	}
	if unrunF == nil || unrunF.Subject != "docs/plans/new.md" || unrunF.Count != 1 {
		t.Errorf("a plan no run ever checked = %+v", unrunF)
	}
	if unrunCloud == nil || unrunCloud.Kind != AttentionUnproven || unrunCloud.Count != 11 || !strings.Contains(unrunCloud.Title, "11 plans in cloud") ||
		unrunCloud.Remedy != "cd /src/cloud && fathom plans status --service cloud" {
		t.Errorf("a project whose plans were never verified = %+v", unrunCloud)
	}
	if certsPartial == nil || certsPartial.State != StatePartial || certsPartial.Count != 3 || !strings.Contains(certsPartial.Title, "at least 3") || !strings.Contains(certsPartial.Detail, "invariants.md") {
		t.Errorf("a partly read corpus = %+v", certsPartial)
	}
	if certsError == nil || certsError.State != StateError || certsError.Count != 0 {
		t.Errorf("an unreadable corpus = %+v", certsError)
	}
	// An unknown count is no count at all: the JSON carries the state and
	// the reason, never a measured-looking 0.
	raw, _ := json.Marshal(certsError)
	if strings.Contains(string(raw), `"count"`) || !strings.Contains(string(raw), `"state":"error"`) {
		t.Errorf("an unknown count serialized as %s", raw)
	}
	if a.State != StatePartial || !strings.Contains(a.Reason, "blind") || !strings.Contains(a.Reason, "partial") {
		t.Errorf("queue = %s %q; unread intent corpora make it a lower bound", a.State, a.Reason)
	}

	// Plans no run ever checked are never "nothing needs attention" (the
	// Fleet page calls the same project unknown).
	only := Projects{Ledger: p.Ledger, Store: p.Store, Rows: []ProjectFacts{cloud}}
	if z := AttentionQueue(only); z.State != StatePresent || len(z.Items) != 1 {
		t.Errorf("plans with no recorded run = %s with %d items", z.State, len(z.Items))
	}
	// No ledger at all: the same (nothing is recorded); an unreadable one is
	// a lower bound, not a list of "never verified".
	only.Ledger = SourceRead{Name: SourceVerifyLedger, State: StateAbsent, Reason: "no verify ledger"}
	if z := AttentionQueue(only); z.State != StatePresent || len(z.Items) != 1 {
		t.Errorf("no ledger = %s with %d items", z.State, len(z.Items))
	}
	only.Ledger = SourceRead{Name: SourceVerifyLedger, State: StateError, Reason: "operation not permitted"}
	if z := AttentionQueue(only); z.State != StatePartial || len(z.Items) != 0 {
		t.Errorf("an unreadable ledger = %s with %+v", z.State, z.Items)
	}
}

func kindStrings(ks []AttentionKind) []string {
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = string(k)
	}
	return out
}

// TestPlanRuns_HeadRunFromAnyCloneElseStaleAtRoot: a plan's proof is the
// newest run recorded at the project's HEAD commit — at the registered root
// or in any other clone of the same service (fathom's pre-push gate
// verifies in a detached clone) — and only without one, the registered
// root's newest run, marked stale with the commit it ran at. A clone at
// another commit, or another service at the same commit, is not the
// project's state; an archived plan's last run is history.
func TestPlanRuns_HeadRunFromAnyCloneElseStaleAtRoot(t *testing.T) {
	runs, malformed := ParseVerifyLedger([]byte(`{"at":"2026-09-14T00:00:00Z","repo_root":"/src/f","service":"f","plan":"docs/plans/a.md","commit":"1111111","claimed":2,"verified":1}
{"at":"2026-09-19T21:00:00Z","repo_root":"/src/f","service":"f","plan":"/src/f/docs/plans/a.md","commit":"d8770983","claimed":45,"verified":42,"failing":3}
{"at":"2026-09-26T02:58:00Z","repo_root":"/tmp/pushclone","service":"f","plan":"docs/plans/a.md","commit":"437761a6","claimed":45,"verified":45}
{"at":"2026-09-26T03:00:00Z","repo_root":"/tmp/other-clone","service":"f","plan":"docs/plans/a.md","commit":"99999999","claimed":45,"failing":45}
{"at":"2026-09-26T03:10:00Z","repo_root":"/tmp/bench","service":"f-bench","plan":"docs/plans/a.md","commit":"437761a6","claimed":45,"failing":45}
{"at":"2026-09-19T21:00:00Z","repo_root":"/src/f","service":"f","plan":"docs/plans/b.md","commit":"d8770983","claimed":8,"verified":7,"failing":1}
{"at":"2026-09-25T00:00:00Z","repo_root":"/src/f","service":"f","plan":"docs/plans/archived.md","commit":"437761a6","claimed":1,"failing":1}
not json
`))
	if malformed != 1 || len(runs) != 7 {
		t.Fatalf("parsed %d runs, %d malformed", len(runs), malformed)
	}
	q := ProjectRunsQuery{Service: "f", Root: "/src/f", Head: "437761a6c0ffee00000000000000000000000000",
		SameRoot: func(r string) bool { return r == "/src/f" }, Exists: func(p string) bool { return p != "docs/plans/archived.md" }}
	got := PlanRuns(runs, q)
	if len(got) != 2 {
		t.Fatalf("plan runs = %+v", got)
	}
	a, b := got[0], got[1]
	if a.Plan != "docs/plans/a.md" || a.Commit != "437761a6" || a.Verified != 45 || a.Failing != 0 || a.StaleReason != "" || !a.Current() {
		t.Errorf("a.md = %+v; want the pushclone's run at HEAD, not the root's older failing one, another commit's clone or another service's run", a)
	}
	if b.Plan != "docs/plans/b.md" || b.Failing != 1 || b.Current() || !strings.Contains(b.StaleReason, "d8770983") || !strings.Contains(b.StaleReason, "437761a6") {
		t.Errorf("b.md = %+v; want the root's newest run, stale: it ran at d8770983 and HEAD is 437761a6", b)
	}
	// HEAD unreadable: no run can be confirmed current, and none is.
	q.Head = ""
	for _, r := range PlanRuns(runs, q) {
		if r.Current() || !strings.Contains(r.StaleReason, "HEAD") {
			t.Errorf("with HEAD unknown %s reads current: %+v", r.Plan, r)
		}
	}

	// The fold: failing and unproven are what the gate says at HEAD; a stale
	// run's counts are history, kept apart.
	p := SumPlanProof([]VerifyRun{
		{Plan: "x.md", At: "2026-09-26T00:00:00Z", Total: 10, Claimed: 8, Verified: 5, Failing: 1},
		{Plan: "y.md", At: "2026-09-19T00:00:00Z", Total: 4, Claimed: 4, Verified: 1, Failing: 2, StaleReason: "ran at aaaaaaa; HEAD is bbbbbbb"},
	})
	if p.Plans != 2 || p.Rows != 14 || p.Claimed != 12 || p.Verified != 6 || p.Failing != 1 || p.Unproven != 2 || p.Unclaimed != 2 ||
		p.Stale != 1 || p.StaleFailing != 2 || p.StaleUnproven != 1 || p.StaleOldest != "2026-09-19T00:00:00Z" {
		t.Errorf("proof = %+v", p)
	}
}
