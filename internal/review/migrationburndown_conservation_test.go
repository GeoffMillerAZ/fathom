package review

import (
	"strings"
	"testing"
)

// TestMigrationBurnDownNeedsConservation: every gone: row proven is not
// enough — a repository whose conservation verdict was never established is
// incomplete and says why; the reason is appended, never replacing what else
// is wrong.
func TestMigrationBurnDownNeedsConservation(t *testing.T) {
	m := Migration{Kind: MigrationSymbol, From: "Old", To: "New"}
	proven := []RowVerdict{{ID: "R-1", Verdict: RowProven}}
	not := "not run: pass --conserve-since <ref>"

	got := MigrationBurnDown(m, nil, []RepoRows{
		{Repository: "a", Rows: proven, ConservationUnproven: not},
		{Repository: "b", Rows: proven},
		{Repository: "c", Rows: []RowVerdict{{ID: "R-2", Verdict: RowUnproven}}, ConservationUnproven: not},
		{Repository: "d", Rows: proven, Unexcused: true, Detail: "test deleted", ConservationUnproven: "plane certificates unproven"},
	})
	byRepo := map[string]RepoMigrationBurnDown{}
	for _, r := range got {
		byRepo[r.Repository] = r
	}
	if a := byRepo["a"]; a.Complete || !strings.Contains(a.Reason, "conservation not established") || !strings.Contains(a.Reason, not) {
		t.Errorf("a = %+v, want incomplete naming the unestablished conservation", a)
	}
	if b := byRepo["b"]; !b.Complete || b.Reason != "" {
		t.Errorf("b = %+v, want complete: proven rows and conservation established", b)
	}
	if c := byRepo["c"]; c.Complete || !strings.Contains(c.Reason, "unproven gone: row(s) in c: R-2") || !strings.Contains(c.Reason, not) {
		t.Errorf("c = %+v, want both the unproven row and the unestablished conservation named", c)
	}
	if d := byRepo["d"]; d.Complete || !strings.Contains(d.Reason, "unexcused conservation loss") || !strings.Contains(d.Reason, "plane certificates unproven") {
		t.Errorf("d = %+v, want the loss and the unproven plane both named", d)
	}
}
