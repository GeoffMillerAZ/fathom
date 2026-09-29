package console

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/support"
)

// TestSupportHeatmapShapeAndLabelPerState pins SM-21 (design
// docs/design/support-matrix.md §5 panel 1): every computed state renders as
// its own shape AND its own label (colour is only the third channel, never
// the only one), a column with no cell is named as such rather than drawn as
// the lowest state, and each cell carries the evidence a hover or click
// opens — the reading and the plan row with its verdict.
func TestSupportHeatmapShapeAndLabelPerState(t *testing.T) {
	states := []support.State{
		support.StateProvenExcellent, support.StateProvenGood, support.StateBelowTarget,
		support.StateTolerated, support.StateDeclared, support.StateRegressed,
		support.StateRefused, support.StateDeferred, support.StateNA,
	}
	shapes, labels := map[string]support.State{}, map[string]support.State{}
	for _, s := range states {
		m := SupportStateMark(s)
		if m.Shape == "" || m.Label == "" || m.Role == "" {
			t.Fatalf("state %s: mark %+v lacks a shape, label or role", s, m)
		}
		if prev, dup := shapes[m.Shape]; dup {
			t.Errorf("states %s and %s share the shape %q: shape alone must tell them apart", prev, s, m.Shape)
		}
		if prev, dup := labels[m.Label]; dup {
			t.Errorf("states %s and %s share the label %q", prev, s, m.Label)
		}
		shapes[m.Shape], labels[m.Label] = s, s
	}
	if none := SupportStateMark(""); none.Label == "" || shapes[none.Shape] != "" {
		t.Errorf("a missing cell must be named and drawn unlike every state, got %+v", none)
	}

	// Entries in Project's order: by kind (client < language), then id.
	m := support.Matrix{Schema: "fathom.support/v1", Entries: []support.Entry{
		{Kind: support.KindClient, ID: "codex", Name: "Codex", Registered: true, Cells: []support.Cell{
			{Capability: "hooks", State: support.StateTolerated},
		}},
		{Kind: support.KindClient, ID: "cursor", Name: "Cursor", Cells: []support.Cell{
			{Capability: "mcp", State: support.StateDeclared, Target: true, PlanRow: "SM-13", Verdict: support.VerdictProven},
		}},
		{Kind: support.KindLanguage, ID: "python", Name: "Python", Registered: true, Wave: 1, Cells: []support.Cell{
			{Capability: "L1", State: support.StateProvenExcellent, Target: true, LevelMet: "L1", Reading: "docs/bench/readings/lang/python.json", PlanRow: "LB-Q-python", Verdict: support.VerdictOpen},
			{Capability: "L2", State: support.StateBelowTarget, Target: true, PlanRow: "LB-Q-python", Verdict: support.VerdictOpen},
		}},
	}}
	groups := SupportHeatmap(m)
	if len(groups) != 2 || groups[0].Kind != support.KindClient || groups[1].Kind != support.KindLanguage {
		t.Fatalf("groups = %+v, want client then language (the matrix's kind order)", groups)
	}
	lang := groups[1]
	if strings.Join(lang.Columns, ",") != "L1,L2,L3,L4" {
		t.Errorf("language columns = %v, want the four levels", lang.Columns)
	}
	py := lang.Rows[0]
	if len(py.Cells) != 4 {
		t.Fatalf("python cells = %d, want one per column", len(py.Cells))
	}
	c := py.Cells[0]
	if c.Mark != SupportStateMark(support.StateProvenExcellent) || c.Reading != "docs/bench/readings/lang/python.json" || c.PlanRow != "LB-Q-python" || c.Verdict != support.VerdictOpen {
		t.Errorf("L1 cell = %+v, want the excellent mark with its reading and plan row verdict", c)
	}
	if !strings.Contains(c.Title, "python.json") || !strings.Contains(c.Title, "LB-Q-python") || !strings.Contains(c.Title, "open") {
		t.Errorf("L1 hover %q must name the reading and the plan row verdict", c.Title)
	}
	if py.Cells[1].Mark.Label != "below target" || !strings.Contains(py.Cells[1].Title, "no reading") {
		t.Errorf("L2 cell = %+v, want below target naming that no reading exists", py.Cells[1])
	}
	if py.Cells[2].State != "" || py.Cells[2].Mark != SupportStateMark("") {
		t.Errorf("L3 has no cell; got %+v", py.Cells[2])
	}
	clients := groups[0]
	if strings.Join(clients.Columns, ",") != "hooks,mcp" {
		t.Errorf("client columns = %v, want the union of capabilities in first-seen order", clients.Columns)
	}
	if len(clients.Rows) != 2 || clients.Rows[1].Cells[1].Mark.Label != "declared" || clients.Rows[1].Cells[0].State != "" {
		t.Errorf("client rows = %+v", clients.Rows)
	}
	if got := clients.Counts[support.StateTolerated]; got != 1 {
		t.Errorf("client tolerated count = %d, want 1", got)
	}
}

// TestSupportDialectsEmbeddedTotalAgainstCap pins SM-22 (§5 panel 2): one
// row per dialect directory with its engine, grammar pin and licence, level
// and reading date; the embedded blob size is counted once per blob in a
// running total held against the 40 MiB cap; a dialect whose engine embeds
// no grammar has no size (absent, never 0 bytes).
func TestSupportDialectsEmbeddedTotalAgainstCap(t *testing.T) {
	in := []SupportDialectInput{
		{ID: "typescript", Name: "TypeScript", Engine: "tags", Blob: "typescript", GrammarRepo: "tree-sitter/tree-sitter-typescript", GrammarTag: "v0.23.2", Licence: "MIT", BlobBytes: 3 << 20, InLock: true, LevelMet: "L3", ReadingDate: "2026-09-20"},
		{ID: "python", Name: "Python", Engine: "tags", Blob: "python", GrammarRepo: "tree-sitter/tree-sitter-python", GrammarTag: "v0.25.0", Licence: "MIT", BlobBytes: 1 << 20, InLock: true},
		{ID: "tsx", Name: "TSX", Engine: "tags", Blob: "typescript", GrammarRepo: "tree-sitter/tree-sitter-typescript", GrammarTag: "v0.23.2", Licence: "MIT", BlobBytes: 3 << 20, InLock: true},
		{ID: "markdown", Name: "Markdown", Engine: "doc"},
		{ID: "ghost", Name: "Ghost", Engine: "tags", Blob: "ghost"},
	}
	v := SupportDialects(in, SupportEmbedCapBytes)
	ids := []string{}
	for _, r := range v.Rows {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "ghost,markdown,python,tsx,typescript" {
		t.Fatalf("rows = %v, want sorted by id", ids)
	}
	if v.Cap != 40<<20 || v.Total != 4<<20 {
		t.Fatalf("total = %d of cap %d, want 4 MiB (a shared blob counts once) of 40 MiB", v.Total, v.Cap)
	}
	byID := map[string]SupportDialectRow{}
	for _, r := range v.Rows {
		byID[r.ID] = r
	}
	if r := byID["markdown"]; r.Embedded.State != StateAbsent || r.Embedded.Reason == "" {
		t.Errorf("markdown embeds no grammar: size = %+v, want absent with a reason", r.Embedded)
	}
	if r := byID["ghost"]; r.Embedded.State != StateAbsent || !strings.Contains(r.Embedded.Reason, "grammars.lock.json") {
		t.Errorf("a blob missing from the lock must say so: %+v", r.Embedded)
	}
	if r := byID["python"]; r.Embedded.V != 1<<20 || r.RunningTotal != 1<<20 {
		t.Errorf("python = %+v, want 1 MiB and a running total of 1 MiB", r)
	}
	if r := byID["tsx"]; r.RunningTotal != 4<<20 || !r.SharedBlob {
		t.Errorf("tsx = %+v, want running total 4 MiB and its blob counted here", r)
	}
	if r := byID["typescript"]; r.RunningTotal != 4<<20 || !r.SharedBlob || r.Embedded.V != 3<<20 {
		t.Errorf("typescript = %+v, want the shared blob not counted twice", r)
	}
	if r := byID["typescript"]; r.Pin != "tree-sitter/tree-sitter-typescript@v0.23.2" || r.LevelMet != "L3" || r.ReadingDate != "2026-09-20" {
		t.Errorf("typescript pin/level = %+v", r)
	}
	if v.Share < 0.099 || v.Share > 0.101 || v.Headroom != 36<<20 {
		t.Errorf("share = %v headroom = %d, want 10%% and 36 MiB", v.Share, v.Headroom)
	}
	if v.NearCap {
		t.Error("10% of the cap is not near it")
	}
	if near := SupportDialects([]SupportDialectInput{{ID: "big", Blob: "big", InLock: true, BlobBytes: 37 << 20}}, SupportEmbedCapBytes); !near.NearCap {
		t.Error("37 of 40 MiB must raise the packs trigger")
	}
}

// TestSupportBurnUpAbsentNeverZero pins SM-23 (§5 panel 3): entries at
// target per kind per day from the readings ledger; a day without a reading
// of that kind is absent (never a 0), and milestones come from the queue.
func TestSupportBurnUpAbsentNeverZero(t *testing.T) {
	in := SupportBurnUpInput{
		Requirements: []SupportRequirement{
			{Kind: support.KindLanguage, ID: "python", Capability: "L1"},
			{Kind: support.KindLanguage, ID: "python", Capability: "L2"},
			{Kind: support.KindLanguage, ID: "ruby", Capability: "L1"},
			{Kind: support.KindClient, ID: "codex", Capability: "hooks", Level: support.L2},
		},
		Ledger: []support.LedgerRow{
			{Kind: support.KindLanguage, ID: "python", Capability: "L1", Grade: support.GradeGood, ReadingDate: "2026-09-01"},
			{Kind: support.KindLanguage, ID: "ruby", Capability: "L1", Grade: support.GradeExcellent, ReadingDate: "2026-09-01"},
			{Kind: support.KindLanguage, ID: "python", Capability: "L2", Grade: support.GradeGood, ReadingDate: "2026-09-03"},
			{Kind: support.KindClient, ID: "codex", Capability: "hooks", LevelMet: support.L1, Grade: support.GradeGood, ReadingDate: "2026-09-02"},
			{Kind: support.KindLanguage, ID: "ruby", Capability: "L1", ReadingDate: "2026-09-04"}, // ruby regressed: no grade
		},
		Milestones: []SupportMilestone{{Kind: support.KindLanguage, Label: "after wave 1", Count: 2}},
	}
	v := SupportBurnUp(in)
	if v.From != "2026-09-01" || v.To != "2026-09-04" {
		t.Fatalf("window %s..%s", v.From, v.To)
	}
	if len(v.Series) != 2 || v.Series[0].Kind != support.KindClient || v.Series[1].Kind != support.KindLanguage {
		t.Fatalf("series = %+v", v.Series)
	}
	lang := v.Series[1]
	if lang.Entries != 2 || len(lang.Days) != 4 {
		t.Fatalf("language series = %+v", lang)
	}
	want := []struct {
		state State
		v     int
	}{{StatePresent, 1}, {StateAbsent, 0}, {StatePresent, 2}, {StatePresent, 1}}
	for i, w := range want {
		d := lang.Days[i]
		if d.AtTarget.State != w.state || (w.state == StatePresent && d.AtTarget.V != w.v) {
			t.Errorf("language day %s = %+v, want %s %d", d.Date, d.AtTarget, w.state, w.v)
		}
	}
	if d := lang.Days[1]; d.AtTarget.Reason == "" {
		t.Error("an absent day must name why")
	}
	if lang.Measured != 3 || lang.Absent != 1 {
		t.Errorf("measured/absent = %d/%d", lang.Measured, lang.Absent)
	}
	client := v.Series[0]
	if d := client.Days[1]; d.AtTarget.State != StateZero && !(d.AtTarget.State == StatePresent && d.AtTarget.V == 0) {
		t.Errorf("codex read L1 against an L2 target: day 2 = %+v, want a measured 0", d.AtTarget)
	}
	if client.Days[0].AtTarget.State != StateAbsent {
		t.Errorf("no client reading on day 1: %+v, want absent", client.Days[0].AtTarget)
	}
	if len(v.Milestones) != 1 || v.Milestones[0].Count != 2 {
		t.Errorf("milestones = %+v", v.Milestones)
	}
	if empty := SupportBurnUp(SupportBurnUpInput{Requirements: in.Requirements}); len(empty.Series) != 0 || empty.Note == "" {
		t.Errorf("an empty ledger draws no series and says why: %+v", empty)
	}
}

// TestSupportNextEligibleByWave pins SM-24 (§5 panel 4): the next eligible
// target rows — own plan row still open, every need claimed — grouped by
// wave, the language queue's rule (docs/design/language-breadth.md §13.0 step
// 2, G2-6 since 2026-09-29): a deferred need blocks like an open one, and a
// refused need puts the item under "needs re-planning" instead of waiting, as
// does an item refused itself (its row open, its -rest sibling refused).
func TestSupportNextEligibleByWave(t *testing.T) {
	rows := map[string]SupportRowState{
		"LB-F1": SupportRowClaimed, "LB-F2": SupportRowDispositioned, "LB-F3": SupportRowOpen, "LB-F4": SupportRowRefused,
		"LB-Q-go": SupportRowOpen, "LB-Q-ruby": SupportRowOpen, "LB-Q-php": SupportRowClaimed, "LB-Q-astro": SupportRowOpen,
		"LB-Q-lua": SupportRowOpen, "LB-Q-zig": SupportRowRefused, "SM-13": SupportRowOpen,
		"LB-Q-swift": SupportRowOpen, // its -rest sibling is refused (§14.2): Refused
	}
	items := []SupportNextItem{
		{Kind: support.KindLanguage, ID: "ruby", Name: "Ruby", Wave: 2, PlanRow: "LB-Q-ruby", Needs: []string{"LB-F1", "LB-F2"}},
		{Kind: support.KindLanguage, ID: "lua", Name: "Lua", Wave: 2, PlanRow: "LB-Q-lua", Needs: []string{"LB-F1", "LB-F4"}},
		{Kind: support.KindLanguage, ID: "go", Name: "Go", Wave: 1, PlanRow: "LB-Q-go", Needs: []string{"language-breadth.md#LB-F1"}},
		{Kind: support.KindLanguage, ID: "php", Name: "PHP", Wave: 1, PlanRow: "LB-Q-php", Needs: []string{"LB-F1"}},
		{Kind: support.KindLanguage, ID: "zig", Name: "Zig", Wave: 1, PlanRow: "LB-Q-zig", Needs: []string{"LB-F1"}},
		{Kind: support.KindLanguage, ID: "swift", Name: "Swift", Wave: 2, PlanRow: "LB-Q-swift", Needs: []string{"LB-F1"}, Refused: true},
		{Kind: support.KindLanguage, ID: "astro", Name: "Astro", Wave: 1, PlanRow: "LB-Q-astro", Needs: []string{"LB-F1", "LB-F3"}},
		{Kind: support.KindClient, ID: "cursor", Name: "Cursor", Wave: 1, PlanRow: "support-matrix.md#SM-13", Needs: []string{"LB-F9"}},
	}
	v := SupportNext(items, rows)
	if len(v.Waves) != 2 || v.Waves[0].Wave != 1 || v.Waves[1].Wave != 2 {
		t.Fatalf("waves = %+v", v.Waves)
	}
	if len(v.Waves[0].Items) != 1 || v.Waves[0].Items[0].ID != "go" {
		t.Errorf("wave 1 eligible = %+v, want only go (php and zig done, astro needs open LB-F3, cursor needs an unknown row)", v.Waves[0].Items)
	}
	if len(v.Waves[1].Items) != 0 {
		t.Errorf("wave 2 eligible = %+v, want none: ruby's LB-F2 is deferred (G2-6)", v.Waves[1].Items)
	}
	if len(v.Waves[1].Replan) != 2 || v.Waves[1].Replan[0].ID != "lua" || v.Waves[1].Replan[1].ID != "swift" {
		t.Errorf("wave 2 re-planning = %+v, want lua (LB-F4 is refused) and swift (refused itself, its row open)", v.Waves[1].Replan)
	}
	if v.Eligible != 1 || v.Blocked != 3 || v.Replan != 2 || v.Done != 2 {
		t.Errorf("eligible/blocked/replan/done = %d/%d/%d/%d, want 1/3/2/2", v.Eligible, v.Blocked, v.Replan, v.Done)
	}
	if v.Waves[0].Blocked != 2 || v.Waves[1].Blocked != 1 {
		t.Errorf("blocked per wave = %d/%d, want 2/1", v.Waves[0].Blocked, v.Waves[1].Blocked)
	}
}
