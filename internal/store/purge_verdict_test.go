package store

import (
	"strings"
	"testing"
)

// TestPurgeVerdictTable pins purgeVerdict — the ONE place a purge outcome
// is computed — against every V1 regression: a skipped probe, a probe read
// error, raw residue, a survivor, and a wired-but-failed derived store must
// each land on their named non-success state; only "SQL zero + probe ran
// clean" is verified.
func TestPurgeVerdictTable(t *testing.T) {
	cases := []struct {
		name      string
		survivors []LocationCount
		derived   []DerivedOutcome
		probe     RawProbe
		want      PurgeVerdict
		wantIn    string
	}{
		{
			name:   "sql zero and probe clean is the only verified state",
			probe:  RawProbe{Ran: true, Files: []string{"f.sqlite"}, Needles: []string{"secrets/token.go"}},
			want:   PurgeVerified,
			wantIn: "raw-bytes probe",
		},
		{
			name:   "probe skipped (empty needle set) is bytes-unchecked, never verified",
			probe:  RawProbe{Ran: false, Reason: "nothing to probe: every resolved path is still live under another service"},
			want:   PurgeBytesUnchecked,
			wantIn: "NOT VERIFIED",
		},
		{
			name:   "probe read error is bytes-unchecked, never the default verdict",
			probe:  RawProbe{Ran: false, Reason: "could not read f.sqlite: permission denied"},
			want:   PurgeBytesUnchecked,
			wantIn: "permission denied",
		},
		{
			name:      "a survivor fails and names its table",
			survivors: []LocationCount{{Location: "knowledge_cards", Rows: 1}},
			probe:     RawProbe{Ran: true, Files: []string{"f.sqlite"}, Needles: []string{"x"}},
			want:      PurgeFailed,
			wantIn:    "knowledge_cards",
		},
		{
			name:   "raw residue fails and names the offset",
			probe:  RawProbe{Ran: true, Files: []string{"f.sqlite"}, Needles: []string{"x"}, Hits: []RawProbeHit{{File: "f.sqlite", Needle: "x", Offset: 42}}},
			want:   PurgeFailed,
			wantIn: "42",
		},
		{
			name:    "a wired derived store that errored fails the verdict",
			derived: []DerivedOutcome{{Location: LocVectorIndex, Failed: true, Reason: "index unavailable"}},
			probe:   RawProbe{Ran: true, Files: []string{"f.sqlite"}, Needles: []string{"x"}},
			want:    PurgeFailed,
			wantIn:  "vector_index",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, note := purgeVerdict(tc.survivors, tc.derived, tc.probe)
			if got != tc.want {
				t.Fatalf("verdict = %s, want %s (note: %s)", got, tc.want, note)
			}
			if !strings.Contains(note, tc.wantIn) {
				t.Errorf("note %q does not contain %q", note, tc.wantIn)
			}
		})
	}
}
