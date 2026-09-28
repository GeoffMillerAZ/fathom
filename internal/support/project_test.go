package support

import (
	"testing"
	"time"
)

func fixedClock(ts string) func() time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	return func() time.Time { return t }
}

// TestProjectRegressedFromLedger proves SM-9: Project derives each cell via
// ComputeState, then overrides to `regressed` exactly when the ledger shows
// the capability's target was met once and the current cell no longer
// meets it — never when the cell is currently proven, and never for a cell
// with no reading history to have regressed from.
func TestProjectRegressedFromLedger(t *testing.T) {
	in := ProjectInput{
		Entries: []EntryInput{
			{
				Kind: KindLanguage, ID: "typescript", Name: "TypeScript", Registered: true, Wave: 1, Engine: "tags",
				Cells: []CellInput{
					// Regressed: the ledger shows L4 was met on 09-20; the
					// live reading now only reaches L1.
					{Capability: "L4", ApplicableToKind: true, Registered: true, TargetDeclared: true, Target: CapabilityTarget{Level: L4}, LevelMet: L1, Grade: GradeGood, ReadingPath: "r.json"},
					// Not regressed: currently proven-excellent, whatever
					// noise the ledger carries.
					{Capability: "L1", ApplicableToKind: true, Registered: true, TargetDeclared: true, Target: CapabilityTarget{Level: L1}, LevelMet: L1, Grade: GradeExcellent, ReadingPath: "r.json"},
					// Not regressed: tolerated with no history at all.
					{Capability: "L2", ApplicableToKind: true, Registered: true, TargetDeclared: true, Target: CapabilityTarget{Level: L4}},
				},
			},
			{
				Kind: KindClient, ID: "cursor", Name: "Cursor", Registered: true,
				Cells: []CellInput{
					{Capability: "mcp-tools", ApplicableToKind: true, Registered: true, TargetDeclared: true, Target: CapabilityTarget{ProvenOnly: true}},
				},
			},
		},
		Ledger: []LedgerRow{
			{Kind: KindLanguage, ID: "typescript", Capability: "L4", LevelMet: L4, Grade: GradeGood, ReadingDate: "2026-09-20", Commit: "aaa"},
			{Kind: KindLanguage, ID: "typescript", Capability: "L1", LevelMet: L1, Grade: GradeExcellent, ReadingDate: "2026-09-20", Commit: "aaa"},
			// Noise for an entry/capability that never appears in Entries.
			{Kind: KindClient, ID: "codex", Capability: "mcp-tools", Grade: GradeExcellent, ReadingDate: "2026-09-20", Commit: "aaa"},
		},
		Commit: "86d4c5e2",
	}

	m := Project(in, fixedClock("2026-09-27T12:00:00Z"))

	if m.Schema != "fathom.support/v1" {
		t.Errorf("schema = %q", m.Schema)
	}
	if m.GeneratedAt != "2026-09-27T12:00:00Z" {
		t.Errorf("generated_at = %q, want the injected clock's stamp", m.GeneratedAt)
	}
	if m.Commit != "86d4c5e2" {
		t.Errorf("commit = %q", m.Commit)
	}
	if len(m.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(m.Entries))
	}
	// Sorted by (kind, id): client < language.
	if m.Entries[0].Kind != KindClient || m.Entries[1].Kind != KindLanguage {
		t.Fatalf("entries not sorted by kind: got %s, %s", m.Entries[0].Kind, m.Entries[1].Kind)
	}

	ts := m.Entries[1]
	if ts.ID != "typescript" {
		t.Fatalf("entries[1].ID = %q, want typescript", ts.ID)
	}
	// Cells sorted by Level rank: L1, L2, L4.
	wantCapOrder := []string{"L1", "L2", "L4"}
	for i, c := range ts.Cells {
		if c.Capability != wantCapOrder[i] {
			t.Fatalf("cells[%d].Capability = %s, want %s (L1,L2,L4 order)", i, c.Capability, wantCapOrder[i])
		}
	}

	byCap := map[string]Cell{}
	for _, c := range ts.Cells {
		byCap[c.Capability] = c
	}

	if got := byCap["L4"].State; got != StateRegressed {
		t.Fatalf("L4 state = %q, want %q (ledger proved it once, now only L1)", got, StateRegressed)
	}
	if got := byCap["L1"].State; got != StateProvenExcellent {
		t.Fatalf("L1 state = %q, want %q (currently proven — never overridden)", got, StateProvenExcellent)
	}
	if got := byCap["L2"].State; got != StateTolerated {
		t.Fatalf("L2 state = %q, want %q (no ledger history for L2 at all)", got, StateTolerated)
	}

	cursor := m.Entries[0]
	if cursor.ID != "cursor" || cursor.Cells[0].State != StateTolerated {
		t.Fatalf("cursor cell = %+v, want tolerated (its own ledger entry is for a different id)", cursor.Cells[0])
	}
}
