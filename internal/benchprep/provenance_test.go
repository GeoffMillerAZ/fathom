package benchprep

import (
	"strings"
	"testing"
	"time"
)

func TestCheckPreregProvenance_Table(t *testing.T) {
	const sha = "aaaa"
	committed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	row := func(task, stamp, first string) ProvenanceRow {
		return ProvenanceRow{RunID: "run-1", TaskID: task, Arm: "A2", PreregSHA256: stamp, FirstCellAt: first}
	}
	after := "2026-10-01T12:30:00Z"
	for _, tc := range []struct {
		name        string
		committedAt time.Time
		rows        []ProvenanceRow
		want        []string // refusal names, in order; nil means the run passes
	}{
		{"a commit before the first cell passes", committed, []ProvenanceRow{row("T1", sha, after), row("T2", sha, "2026-10-01T13:00:00Z")}, nil},
		{"a commit at the first cell passes", committed, []ProvenanceRow{row("T1", sha, "2026-10-01T12:00:00Z")}, nil},
		{"a commit in another zone at the first cell passes", committed.In(time.FixedZone("x", 2*3600)), []ProvenanceRow{row("T1", sha, "2026-10-01T12:00:00Z")}, nil},
		{"a commit after the first cell is refused", committed, []ProvenanceRow{row("T1", sha, after), row("T2", sha, "2026-10-01T11:59:59Z")}, []string{"committed-after-first-cell"}},
		{"a sha mismatch is refused", committed, []ProvenanceRow{row("T1", sha, after), row("T2", "bbbb", after)}, []string{"prereg-sha256-mismatch"}},
		{"a missing sha is refused", committed, []ProvenanceRow{row("T1", sha, after), row("T2", "", after)}, []string{"missing-prereg-sha256"}},
		{"a missing first_cell_at is refused", committed, []ProvenanceRow{row("T1", sha, after), row("T2", sha, "")}, []string{"missing-first-cell-at"}},
		{"an unreadable first_cell_at is refused", committed, []ProvenanceRow{row("T1", sha, "yesterday")}, []string{"unreadable-first-cell-at"}},
		{"zero rows for the run is refused", committed, []ProvenanceRow{{RunID: "run-2", TaskID: "T1", Arm: "A2", PreregSHA256: sha, FirstCellAt: after}}, []string{"no-rows"}},
		{"an unknown commit time is refused", time.Time{}, []ProvenanceRow{row("T1", sha, after)}, []string{"commit-time-unknown"}},
		{"every cause is named at once", committed, []ProvenanceRow{row("T1", "bbbb", "2026-10-01T11:00:00Z"), row("T2", "", "")}, []string{"committed-after-first-cell", "missing-first-cell-at", "missing-prereg-sha256", "prereg-sha256-mismatch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckPreregProvenance("run-1", tc.committedAt, sha, tc.rows)
			var names []string
			for _, r := range got.Refusals {
				name, _, _ := strings.Cut(r, ":")
				names = append(names, name)
			}
			if strings.Join(names, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("refusals = %q, want names %q", got.Refusals, tc.want)
			}
			if got.Refused() != (len(tc.want) > 0) {
				t.Errorf("Refused() = %v with refusals %q", got.Refused(), got.Refusals)
			}
		})
	}
}

func TestCheckPreregProvenance_NamesBothHashesAndTheRows(t *testing.T) {
	committed := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	got := CheckPreregProvenance("run-1", committed, "aaaa", []ProvenanceRow{
		{RunID: "run-1", TaskID: "T2", Arm: "A2", PreregSHA256: "bbbb", FirstCellAt: "2026-10-01T13:00:00Z"},
		{RunID: "run-1", TaskID: "T1", Arm: "A1", PreregSHA256: "aaaa", FirstCellAt: "2026-10-01T12:30:00+02:00"},
	})
	if len(got.Refusals) != 1 {
		t.Fatalf("want one refusal, got %q", got.Refusals)
	}
	for _, want := range []string{"aaaa", "bbbb", "T2/A2"} {
		if !strings.Contains(got.Refusals[0], want) {
			t.Errorf("mismatch refusal should name %q: %s", want, got.Refusals[0])
		}
	}
	if got.Rows != 2 {
		t.Errorf("Rows = %d, want 2", got.Rows)
	}
	// 12:30+02:00 is 10:30Z, the earliest of the two.
	if want := time.Date(2026, 10, 1, 10, 30, 0, 0, time.UTC); !got.FirstCellAt.Equal(want) {
		t.Errorf("FirstCellAt = %v, want the minimum over the rows %v", got.FirstCellAt, want)
	}
	if !got.CommittedAt.Equal(committed) || got.PreregSHA256 != "aaaa" || got.RunID != "run-1" {
		t.Errorf("reading header wrong: %+v", got)
	}
}
