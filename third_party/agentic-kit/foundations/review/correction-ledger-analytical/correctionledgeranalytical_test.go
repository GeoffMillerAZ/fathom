package correctionledgeranalytical_test

import (
	"testing"
	"time"

	cl "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/correction-ledger"
	cla "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/correction-ledger-analytical"
)

// ---- helpers ----------------------------------------------------------------

func makeTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func correctionEvent(findingID, target string, at time.Time) cl.LedgerEvent {
	return cl.LedgerEvent{
		Kind:       cl.LedgerEventKindCorrectionAdded,
		OccurredAt: at,
		Correction: &cl.Correction{
			FindingID:   findingID,
			Target:      cl.CorrectionTarget{StableTarget: target},
			Original:    "old",
			Corrected:   "new",
			CorrectedBy: "alice",
			CorrectedAt: at,
			Reason:      "test",
			Confidence:  3,
		},
	}
}

func dismissalEvent(findingID string, at time.Time) cl.LedgerEvent {
	return cl.LedgerEvent{
		Kind:       cl.LedgerEventKindDismissalAdded,
		OccurredAt: at,
		Dismissal: &cl.Dismissal{
			FindingID:   findingID,
			DismissedBy: "bob",
			Reason:      "false positive",
			DismissedAt: at,
		},
	}
}

// ---- Project: basic cases ---------------------------------------------------

func TestProject_EmptyInput(t *testing.T) {
	got := cla.Project(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 events for nil input, got %d", len(got))
	}

	got = cla.Project([]cl.LedgerEvent{})
	if len(got) != 0 {
		t.Errorf("expected 0 events for empty input, got %d", len(got))
	}
}

func TestProject_IgnoresNonAnalyticalEvents(t *testing.T) {
	events := []cl.LedgerEvent{
		{
			Kind: cl.LedgerEventKindFindingOpened, OccurredAt: makeTime("2024-01-01T00:00:00Z"),
			FindingOpened: &cl.Finding{
				ID: "f1", State: cl.StateOpen, Description: "test",
				CreatedAt: makeTime("2024-01-01T00:00:00Z"), UpdatedAt: makeTime("2024-01-01T00:00:00Z"),
			},
		},
		{
			Kind: cl.LedgerEventKindTransition, OccurredAt: makeTime("2024-01-01T01:00:00Z"),
			Transition: &cl.TransitionEvent{
				FindingID: "f1", Event: cl.EventClaim,
				FromState: cl.StateOpen, ToState: cl.StateReviewing,
			},
		},
	}
	got := cla.Project(events)
	if len(got) != 0 {
		t.Errorf("expected 0 projected events for non-analytical input, got %d", len(got))
	}
}

func TestProject_SingleCorrection(t *testing.T) {
	at := makeTime("2024-11-01T10:30:00Z")
	events := []cl.LedgerEvent{correctionEvent("finding-001", "report/section/revenue.q4.v3", at)}

	got := cla.Project(events)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}

	ev := got[0]
	if ev.Kind != cla.AnalyticalEventKindCorrection {
		t.Errorf("Kind: want correction, got %q", ev.Kind)
	}
	if ev.Target != "report/section/revenue.q4.v3" {
		t.Errorf("Target: want report/section/revenue.q4.v3, got %q", ev.Target)
	}
	if ev.Version != 1 {
		t.Errorf("Version: want 1, got %d", ev.Version)
	}
	if ev.At.IsZero() {
		t.Error("At must not be zero")
	}
	if ev.ID == "" {
		t.Error("ID must not be empty")
	}
}

func TestProject_SingleDismissal(t *testing.T) {
	at := makeTime("2024-11-02T14:15:00Z")
	events := []cl.LedgerEvent{dismissalEvent("finding-002", at)}

	got := cla.Project(events)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}

	ev := got[0]
	if ev.Kind != cla.AnalyticalEventKindDismissal {
		t.Errorf("Kind: want dismissal, got %q", ev.Kind)
	}
	if ev.Target != "finding-002" {
		t.Errorf("Target: want finding-002, got %q", ev.Target)
	}
	if ev.Version != 1 {
		t.Errorf("Version: want 1, got %d", ev.Version)
	}
}

// ---- Project: versioning ---------------------------------------------------

func TestProject_VersioningPerTarget(t *testing.T) {
	target := "report/section/revenue.q4.v3"
	t1 := makeTime("2024-11-01T10:00:00Z")
	t2 := makeTime("2024-11-01T11:00:00Z")
	t3 := makeTime("2024-11-01T12:00:00Z")

	events := []cl.LedgerEvent{
		correctionEvent("finding-001", target, t1),
		correctionEvent("finding-002", target, t2),
		correctionEvent("finding-003", target, t3),
	}
	got := cla.Project(events)
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
	for i, ev := range got {
		wantVersion := i + 1
		if ev.Version != wantVersion {
			t.Errorf("event[%d] Version: want %d, got %d", i, wantVersion, ev.Version)
		}
	}
}

func TestProject_VersioningIndependentAcrossTargets(t *testing.T) {
	t1 := makeTime("2024-11-01T10:00:00Z")
	events := []cl.LedgerEvent{
		correctionEvent("finding-001", "target-A", t1),
		correctionEvent("finding-002", "target-B", t1),
		correctionEvent("finding-003", "target-A", t1),
	}
	got := cla.Project(events)
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}

	// Collect versions per target.
	byTarget := make(map[string][]int)
	for _, ev := range got {
		byTarget[ev.Target] = append(byTarget[ev.Target], ev.Version)
	}

	// target-A should have versions 1 and 2.
	aVersions := byTarget["target-A"]
	if len(aVersions) != 2 {
		t.Fatalf("target-A: expected 2 versions, got %v", aVersions)
	}
	// target-B should have version 1.
	bVersions := byTarget["target-B"]
	if len(bVersions) != 1 || bVersions[0] != 1 {
		t.Fatalf("target-B: expected version [1], got %v", bVersions)
	}
}

// ---- Project: determinism --------------------------------------------------

func TestProject_Deterministic(t *testing.T) {
	t1 := makeTime("2024-11-01T10:00:00Z")
	t2 := makeTime("2024-11-01T11:00:00Z")
	events := []cl.LedgerEvent{
		correctionEvent("finding-001", "report/costs.v1", t1),
		dismissalEvent("finding-002", t2),
		correctionEvent("finding-003", "report/costs.v1", t2),
	}

	first := cla.Project(events)
	second := cla.Project(events)

	if len(first) != len(second) {
		t.Fatalf("non-deterministic: first=%d events, second=%d events", len(first), len(second))
	}
	for i := range first {
		a, b := first[i], second[i]
		if a.ID != b.ID || a.Kind != b.Kind || a.Target != b.Target ||
			a.Version != b.Version || !a.At.Equal(b.At) {
			t.Errorf("event[%d] differs between runs: %+v vs %+v", i, a, b)
		}
	}
}

// ---- Project: ID format ----------------------------------------------------

func TestProject_IDFormat(t *testing.T) {
	at := makeTime("2024-11-01T10:00:00Z")
	events := []cl.LedgerEvent{correctionEvent("finding-001", "target-X", at)}
	got := cla.Project(events)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	// ID format: "<kind>:<FindingID>:<Version>"
	want := "correction:finding-001:1"
	if got[0].ID != want {
		t.Errorf("ID: want %q, got %q", want, got[0].ID)
	}
}

// ---- Project: mixed event stream (from a real Ledger) ----------------------

func TestProject_FromRealLedger(t *testing.T) {
	l := cl.New()

	f1, err := l.OpenFinding("ledger-f1", "revenue figure off")
	if err != nil {
		t.Fatalf("OpenFinding f1: %v", err)
	}
	f2, err := l.OpenFinding("ledger-f2", "methodology claim")
	if err != nil {
		t.Fatalf("OpenFinding f2: %v", err)
	}

	// f1: open → reviewing → corrected
	f1, err = l.ApplyTransition(f1, cl.EventClaim)
	if err != nil {
		t.Fatalf("f1 claim: %v", err)
	}
	err = l.RecordCorrection(cl.Correction{
		FindingID:   "ledger-f1",
		Target:      cl.CorrectionTarget{StableTarget: "report/revenue.q4.v3"},
		Original:    "$4.2M",
		Corrected:   "$4.8M",
		CorrectedBy: "alice",
		Reason:      "wrong draft used",
		Confidence:  4,
	})
	if err != nil {
		t.Fatalf("record correction: %v", err)
	}
	f1, err = l.ApplyTransition(f1, cl.EventCorrect)
	if err != nil {
		t.Fatalf("f1 correct: %v", err)
	}

	// f2: open → reviewing → dismissed
	f2, err = l.ApplyTransition(f2, cl.EventClaim)
	if err != nil {
		t.Fatalf("f2 claim: %v", err)
	}
	if err := l.RecordDismissal(cl.Dismissal{
		FindingID:   "ledger-f2",
		DismissedBy: "bob",
		Reason:      "documented limitation",
	}); err != nil {
		t.Fatalf("RecordDismissal: %v", err)
	}
	_, err = l.ApplyTransition(f2, cl.EventDismiss)
	if err != nil {
		t.Fatalf("f2 dismiss: %v", err)
	}

	projected := cla.Project(l.Events())
	// Expect exactly 2 analytical events: 1 correction + 1 dismissal.
	if len(projected) != 2 {
		t.Fatalf("expected 2 projected events, got %d: %+v", len(projected), projected)
	}

	kinds := make(map[cla.AnalyticalEventKind]int)
	for _, ev := range projected {
		kinds[ev.Kind]++
		if ev.Version < 1 {
			t.Errorf("event %q has Version < 1: %d", ev.ID, ev.Version)
		}
		if ev.Target == "" {
			t.Errorf("event %q has empty Target", ev.ID)
		}
		if ev.At.IsZero() {
			t.Errorf("event %q has zero At", ev.ID)
		}
	}
	if kinds[cla.AnalyticalEventKindCorrection] != 1 {
		t.Errorf("expected 1 correction event, got %d", kinds[cla.AnalyticalEventKindCorrection])
	}
	if kinds[cla.AnalyticalEventKindDismissal] != 1 {
		t.Errorf("expected 1 dismissal event, got %d", kinds[cla.AnalyticalEventKindDismissal])
	}
}

// ---- Project: nil/empty payload events are skipped -------------------------

func TestProject_SkipsNilPayloads(t *testing.T) {
	events := []cl.LedgerEvent{
		{Kind: cl.LedgerEventKindCorrectionAdded, OccurredAt: time.Now(), Correction: nil},
		{Kind: cl.LedgerEventKindDismissalAdded, OccurredAt: time.Now(), Dismissal: nil},
	}
	got := cla.Project(events)
	if len(got) != 0 {
		t.Errorf("expected 0 events for nil payloads, got %d", len(got))
	}
}
