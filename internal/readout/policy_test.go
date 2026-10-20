package readout

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/decide"
)

// TestPolicyTableIsDeterministicAndRawOnEditedFiles: the policy table of
// design §12, rules 1 to 5 in order, over counts and flags only. Each rule's
// case answers as the design says; a failure is rule 1 whatever else holds;
// the same inputs give the same choice across runs and record-map orders; a
// whole-file read of an edited file is raw even with records and a promoted
// rule; an unpromoted rule never gets lean; rules 1 and 4 and long reads of
// unedited files are marked contested, as data only; every choice says
// decided_by table (JG-1's token).
func TestPolicyTableIsDeterministicAndRawOnEditedFiles(t *testing.T) {
	builtin := BuiltinPolicy()
	cases := []struct {
		name      string
		f         Features
		p         Policy
		mode      string
		rule      int
		contested bool
	}{
		{"failure naming an edited file is enriched",
			Features{Event: EventFailure, Class: StreamDiagnostic, Bytes: 900, Lines: 12, Records: map[string]int{RecordCompileError: 3}, EditedOverlap: 1, Recovery: -1},
			builtin, ChoiceEnriched, 1, true},
		{"failure naming a failing test is enriched",
			Features{Event: EventFailure, Class: StreamDiagnostic, Records: map[string]int{"tests_to_run": 2}, FailingOverlap: 1, Recovery: -1},
			builtin, ChoiceEnriched, 1, true},
		{"failure naming neither is raw",
			Features{Event: EventFailure, Class: StreamDiagnostic, Records: map[string]int{RecordCompileError: 4}, Recovery: -1},
			builtin, ChoiceRaw, 1, true},
		{"failure comes first: protected lines and a promoted rule over its floor do not move it",
			Features{Event: EventFailure, Class: StreamDiagnostic, Bytes: 90000, Lines: 2000, Protected: 5, Promoted: true, RuleFloor: 100, Recovery: -1},
			builtin, ChoiceRaw, 1, true},
		{"success with protected lines in a diagnostic stream is raw plus enrichment",
			Features{Event: EventSuccess, Class: StreamDiagnostic, Bytes: 4000, Lines: 40, Protected: 2, Records: map[string]int{RecordCompileError: 2}, EditedOverlap: 1, Recovery: -1},
			builtin, ChoiceEnriched, 2, false},
		{"success with protected lines and no overlap is raw",
			Features{Event: EventSuccess, Class: StreamDiagnostic, Protected: 2, Records: map[string]int{RecordCompileError: 2}, Recovery: -1},
			builtin, ChoiceRaw, 2, false},
		{"overlap with no record enriches nothing",
			Features{Event: EventFailure, Class: StreamDiagnostic, EditedOverlap: 1, Recovery: -1},
			builtin, ChoiceRaw, 1, true},
		{"success over the floor with the rule promoted is lean",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 5000, Lines: 80, Promoted: true, RuleFloor: 1000, Recovery: -1},
			builtin, ChoiceLean, 3, false},
		{"success at the floor is raw",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 1000, Lines: 80, Promoted: true, RuleFloor: 1000, Recovery: -1},
			builtin, ChoiceRaw, 5, false},
		{"a promoted rule recovering over the demotion bar is raw",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 5000, Promoted: true, RuleFloor: 1000, Recovery: 0.2},
			builtin, ChoiceRaw, 5, false},
		{"a fitted floor above the rule's own holds lean back",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 5000, Promoted: true, RuleFloor: 1000, Recovery: -1},
			Policy{LeanFloorBytes: 8000, LongReadLines: 200}, ChoiceRaw, 5, false},
		{"a whole-file read of an edited file is raw even with records and a promoted rule",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 60000, Lines: 1500, Records: map[string]int{"outline": 4, RecordCompileError: 1}, EditedOverlap: 1, WholeRead: true, ReadEdited: true, Promoted: true, RuleFloor: 100, Recovery: -1},
			builtin, ChoiceRaw, 4, true},
		{"a long whole read of an unedited file is contested",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 9000, Lines: 300, WholeRead: true, Recovery: -1},
			builtin, ChoiceRaw, 5, true},
		{"a short whole read of an unedited file is not",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 900, Lines: 100, WholeRead: true, Recovery: -1},
			builtin, ChoiceRaw, 5, false},
		{"a promoted rule leans a long read only past the long-read cut, contested",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 9000, Lines: 300, WholeRead: true, Promoted: true, RuleFloor: 100, Recovery: -1},
			builtin, ChoiceLean, 3, true},
		{"a promoted rule keeps a short read raw",
			Features{Event: EventSuccess, Class: StreamContent, Bytes: 9000, Lines: 150, WholeRead: true, Promoted: true, RuleFloor: 100, Recovery: -1},
			builtin, ChoiceRaw, 5, false},
		{"otherwise raw",
			Features{Event: EventSuccess, Class: StreamDiagnostic, Bytes: 300, Lines: 4, Recovery: -1},
			builtin, ChoiceRaw, 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Choose(tc.f, tc.p)
			if got.Mode != tc.mode || got.Rule != tc.rule || got.Contested != tc.contested {
				t.Fatalf("Choose = %+v, want mode %s rule %d contested %v", got, tc.mode, tc.rule, tc.contested)
			}
			if got.DecidedBy != decide.DecidedTable {
				t.Fatalf("decided_by %q, want JG-1's table token %q", got.DecidedBy, decide.DecidedTable)
			}
			sub := tc.f
			sub.SubAgent = !sub.SubAgent
			if again := Choose(sub, tc.p); again != got {
				t.Fatalf("the sub-agent flag moved the table's answer: %+v vs %+v", again, got)
			}
		})
	}

	t.Run("the same inputs give the same choice across runs and map orders", func(t *testing.T) {
		kinds := []string{RecordCompileError, "tests_to_run", "outline", "frame", "summary"}
		var first Choice
		for run := 0; run < 64; run++ {
			recs := map[string]int{}
			for i := range kinds {
				k := kinds[(i+run)%len(kinds)]
				recs[k] = len(k)
			}
			f := Features{Event: EventSuccess, Class: StreamDiagnostic, Bytes: 4000, Lines: 40, Protected: 1, Records: recs, EditedOverlap: 1, Recovery: -1}
			got := Choose(f, BuiltinPolicy())
			if run == 0 {
				first = got
				continue
			}
			if got != first {
				t.Fatalf("run %d: %+v, want %+v", run, got, first)
			}
		}
	})

	t.Run("an unpromoted rule never gets lean", func(t *testing.T) {
		for _, ev := range []string{EventSuccess, EventFailure} {
			for _, cls := range []Stream{StreamContent, StreamDiagnostic} {
				for _, bytes := range []int{0, 1, 999, 1001, 1 << 20} {
					for _, lines := range []int{0, 199, 201, 10000} {
						for _, whole := range []bool{false, true} {
							for _, rec := range []float64{-1, 0, 0.5} {
								f := Features{Event: ev, Class: cls, Bytes: bytes, Lines: lines, WholeRead: whole, Recovery: rec, RuleFloor: 1}
								for _, p := range []Policy{BuiltinPolicy(), {LeanFloorBytes: 0, LongReadLines: 1}} {
									if c := Choose(f, p); c.Mode == ChoiceLean {
										t.Fatalf("Choose(%+v, %+v) = lean for an unpromoted rule", f, p)
									}
								}
							}
						}
					}
				}
			}
		}
	})

	t.Run("a zero policy reads as the built-in one", func(t *testing.T) {
		f := Features{Event: EventSuccess, Class: StreamContent, Bytes: 9000, Lines: 300, WholeRead: true, Recovery: -1}
		if a, b := Choose(f, Policy{}), Choose(f, BuiltinPolicy()); a != b {
			t.Fatalf("zero policy %+v, built-in %+v", a, b)
		}
	})
}

// BenchmarkChoose is the table's own cost on the hook path.
func BenchmarkChoose(b *testing.B) {
	f := Features{Event: EventFailure, Class: StreamDiagnostic, Bytes: 4000, Lines: 40, Records: map[string]int{RecordCompileError: 3, "tests_to_run": 2}, EditedOverlap: 1, Recovery: -1}
	p := BuiltinPolicy()
	for i := 0; i < b.N; i++ {
		_ = Choose(f, p)
	}
}
