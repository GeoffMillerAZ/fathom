// ruleeffectiveness_test.go — GL-11 (docs/plans/governance-learning.md,
// design doc §5's class table): proves each of the four classes and the
// "not yet classifiable" (none) case from hand-built event slices, plus
// the "a rule with zero events still gets a verdict" contract that
// distinguishes this from a plain fold over observed rule IDs.
package semantic

import (
	"reflect"
	"testing"
	"time"
)

func TestRuleEffectivenessClasses(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	servedEvents := func(ruleID string, n int, at time.Time) []RuleOutcomeEvent {
		out := make([]RuleOutcomeEvent, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, RuleOutcomeEvent{RuleID: ruleID, Event: RuleEventServed, At: at})
		}
		return out
	}
	kindEvents := func(ruleID, kind string, n int, at time.Time) []RuleOutcomeEvent {
		out := make([]RuleOutcomeEvent, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, RuleOutcomeEvent{RuleID: ruleID, Event: kind, At: at})
		}
		return out
	}

	recent := now.Add(-24 * time.Hour) // well inside the 30-day window

	var events []RuleOutcomeEvent

	// "ignored": served >= 20, still_served/served >= 0.5, never blocked.
	events = append(events, servedEvents("ignored-rule", 20, recent)...)
	events = append(events, kindEvents("ignored-rule", RuleEventStillServed, 10, recent)...)

	// "effective": served >= 20, top_changed/served >= 0.9.
	events = append(events, servedEvents("effective-rule", 20, recent)...)
	events = append(events, kindEvents("effective-rule", RuleEventTopChanged, 18, recent)...)

	// "never_triggered": zero served events in the 90-day window (this rule
	// is only known because it is listed in ruleIDs — it never appears in
	// events at all).

	// "contested": too few served events (5 < 20) to judge the ratio
	// classes, but refuted >= 3, and served recently enough that
	// never_triggered does not apply either.
	events = append(events, servedEvents("contested-rule", 5, recent)...)
	events = append(events, kindEvents("contested-rule", RuleEventRefuted, 3, recent)...)

	// "none": some signal, but not enough to classify either way (n < 20,
	// zero refutes, served recently so not never-triggered).
	events = append(events, servedEvents("unclassified-rule", 5, recent)...)

	ruleIDs := []string{
		"ignored-rule", "effective-rule", "never-triggered-rule",
		"contested-rule", "unclassified-rule",
	}

	got := RuleEffectiveness(ruleIDs, events, now)
	if len(got) != len(ruleIDs) {
		t.Fatalf("want %d rows (one per known rule id), got %d: %+v", len(ruleIDs), len(got), got)
	}

	byID := make(map[string]RuleEffectivenessResult, len(got))
	for _, r := range got {
		byID[r.RuleID] = r
	}

	cases := []struct {
		id     string
		class  RuleEffectivenessClass
		remedy string
	}{
		{"ignored-rule", RuleClassIgnored, "move to a hook (deny) or rewrite with the violating example"},
		{"effective-rule", RuleClassEffective, "keep"},
		{"never-triggered-rule", RuleClassNeverTriggered, "prune or re-scope"},
		{"contested-rule", RuleClassContested, "human review"},
		{"unclassified-rule", RuleClassNone, ""},
	}
	for _, tc := range cases {
		r, ok := byID[tc.id]
		if !ok {
			t.Fatalf("missing result row for %q", tc.id)
		}
		if r.Class != tc.class {
			t.Errorf("%s: class = %q, want %q (row: %+v)", tc.id, r.Class, tc.class, r)
		}
		if r.Remedy != tc.remedy {
			t.Errorf("%s: remedy = %q, want %q", tc.id, r.Remedy, tc.remedy)
		}
	}

	// Output is sorted by RuleID (determinism contract, internal/AGENTS.md).
	for i := 1; i < len(got); i++ {
		if got[i-1].RuleID > got[i].RuleID {
			t.Errorf("output not sorted by RuleID: %q before %q", got[i-1].RuleID, got[i].RuleID)
		}
	}
}

// TestRuleEffectiveness_NeverTriggeredWindowIs90Days proves the table's own
// stated exception: never_triggered reads a 90-day window, not the 30-day
// window every other class uses — a rule served 60 days ago (outside the
// 30-day window, inside the 90-day one) must NOT read as never triggered.
func TestRuleEffectiveness_NeverTriggeredWindowIs90Days(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	sixtyDaysAgo := now.Add(-60 * 24 * time.Hour)
	events := []RuleOutcomeEvent{
		{RuleID: "old-served", Event: RuleEventServed, At: sixtyDaysAgo},
	}
	got := RuleEffectiveness([]string{"old-served"}, events, now)
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d", len(got))
	}
	if got[0].Class == RuleClassNeverTriggered {
		t.Errorf("a rule served 60 days ago must not read as never_triggered (90-day window): %+v", got[0])
	}
	if got[0].Served90 != 1 {
		t.Errorf("Served90 = %d, want 1", got[0].Served90)
	}
	if got[0].Served30 != 0 {
		t.Errorf("Served30 = %d, want 0 (60 days ago is outside the 30-day window)", got[0].Served30)
	}
}

// TestRuleEffectiveness_EventForUnlistedRuleIsNotDropped proves an event
// for a rule id the caller never listed still surfaces a result row
// (folded in as a newly-seen rule) rather than being silently discarded —
// an event is signal that already happened; it must never vanish because
// of a caller's incomplete rule-id list.
func TestRuleEffectiveness_EventForUnlistedRuleIsNotDropped(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	events := []RuleOutcomeEvent{
		{RuleID: "unlisted-rule", Event: RuleEventServed, At: now.Add(-time.Hour)},
	}
	got := RuleEffectiveness(nil, events, now)
	if len(got) != 1 || got[0].RuleID != "unlisted-rule" {
		t.Fatalf("want 1 row for unlisted-rule, got %+v", got)
	}
}

// TestCanonicalRuleEvent (JF-2): the two names the tracker wrote before JF-1
// are read under the names that say what they measured, and every other name,
// current or unknown, passes through unchanged.
func TestCanonicalRuleEvent(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"complied", RuleEventTopChanged},
		{"violated", RuleEventStillServed},
		{RuleEventTopChanged, RuleEventTopChanged},
		{RuleEventStillServed, RuleEventStillServed},
		{RuleEventServed, RuleEventServed},
		{RuleEventBlocked, RuleEventBlocked},
		{RuleEventRefuted, RuleEventRefuted},
		{RuleEventCorroborated, RuleEventCorroborated},
		{"", ""},
		{"Violated", "Violated"}, // the log's names are exact: no case folding
		{"something_new", "something_new"},
	} {
		if got := CanonicalRuleEvent(tc.in); got != tc.want {
			t.Errorf("CanonicalRuleEvent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRuleEffectivenessReadsOldEventNames (JF-2): a ledger written before the
// rename holds complied and violated rows. They are counted, never dropped,
// and a rule classifies exactly as it does from the renamed rows; a ledger
// that spans the rename (old rows, then new ones) folds into one count.
func TestRuleEffectivenessReadsOldEventNames(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)
	rows := func(still, top string) []RuleOutcomeEvent {
		var out []RuleOutcomeEvent
		add := func(id, event string, n int) {
			for i := 0; i < n; i++ {
				out = append(out, RuleOutcomeEvent{RuleID: id, Event: event, At: recent})
			}
		}
		add("ignored-rule", RuleEventServed, 20)
		add("ignored-rule", still, 10)
		add("effective-rule", RuleEventServed, 20)
		add("effective-rule", top, 18)
		return out
	}
	ids := []string{"ignored-rule", "effective-rule"}

	oldRows := RuleEffectiveness(ids, rows("violated", "complied"), now)
	newRows := RuleEffectiveness(ids, rows(RuleEventStillServed, RuleEventTopChanged), now)
	if !reflect.DeepEqual(oldRows, newRows) {
		t.Fatalf("old-name rows read differently from their renamed twins:\n old %+v\n new %+v", oldRows, newRows)
	}
	byID := map[string]RuleEffectivenessResult{}
	for _, r := range oldRows {
		byID[r.RuleID] = r
	}
	if got := byID["ignored-rule"].Class; got != RuleClassIgnored {
		t.Errorf("ignored-rule class = %q from violated rows, want %q", got, RuleClassIgnored)
	}
	if got := byID["effective-rule"].Class; got != RuleClassEffective {
		t.Errorf("effective-rule class = %q from complied rows, want %q", got, RuleClassEffective)
	}

	// One ledger across the rename: 10 old rows and 5 new ones are 15.
	mixed := rows("violated", "complied")
	for i := 0; i < 5; i++ {
		mixed = append(mixed, RuleOutcomeEvent{RuleID: "ignored-rule", Event: RuleEventStillServed, At: recent})
	}
	for _, r := range RuleEffectiveness(ids, mixed, now) {
		if r.RuleID == "ignored-rule" && (r.Class != RuleClassIgnored || r.Violated30 != 15) {
			t.Errorf("a ledger spanning the rename lost rows: ignored-rule = %+v, want 15 repeat-edit rows", r)
		}
	}
}
