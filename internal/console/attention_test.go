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
		gate.Evidence.Href != "/projects/fathom?tab=plans" || gate.Area != "Gate" || gate.AsOf != "2026-09-19T08:00:00Z" {
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
	if c := a.Items[4]; c.Count != 45 || c.Area != "Proof" || c.Evidence.Href != "/projects/guides?tab=proof" || c.Remedy != "fathom certify gaps --service guides" {
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

func kindStrings(ks []AttentionKind) []string {
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = string(k)
	}
	return out
}

func TestNewestRunsAt_RegisteredRootOnly(t *testing.T) {
	runs, malformed := ParseVerifyLedger([]byte(`{"at":"2026-09-14T00:00:00Z","repo_root":"/src/f","plan":"docs/plans/a.md","claimed":2,"verified":1}
{"at":"2026-09-20T00:00:00Z","repo_root":"/src/f","plan":"/src/f/docs/plans/a.md","claimed":2,"verified":2}
{"at":"2026-09-25T00:00:00Z","repo_root":"/tmp/clone","plan":"docs/plans/a.md","claimed":2,"failing":2}
{"at":"2026-09-25T00:00:00Z","repo_root":"/src/f","plan":"docs/plans/archived.md","claimed":1,"failing":1}
not json
`))
	if malformed != 1 || len(runs) != 4 {
		t.Fatalf("parsed %d runs, %d malformed", len(runs), malformed)
	}
	got := NewestRunsAt(runs, "/src/f", func(r string) bool { return r == "/src/f" }, func(p string) bool { return p != "docs/plans/archived.md" })
	if len(got) != 1 || got[0].Plan != "docs/plans/a.md" || got[0].Verified != 2 || got[0].At != "2026-09-20T00:00:00Z" {
		t.Errorf("newest runs = %+v; want the registered root's newest run of a.md, the clone's and the archived plan's left out", got)
	}
	if p := SumPlanProof([]VerifyRun{{Total: 10, Claimed: 8, Verified: 5, Failing: 1}}); p.Unproven != 2 || p.Unclaimed != 2 {
		t.Errorf("proof = %+v", p)
	}
}
