package authority_test

import (
	"testing"
	"time"

	ap "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"

	"github.com/geoffmilleraz/agentic-kit/foundations/knowledge/authority"
)

// ── fixed reference time ──────────────────────────────────────────────────────

// epoch is a fixed reference instant used throughout the test suite so that
// all time comparisons are deterministic and independent of wall-clock time.
var epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func future(d time.Duration) time.Time { return epoch.Add(d) }
func past(d time.Duration) time.Time   { return epoch.Add(-d) }

// ── helpers ───────────────────────────────────────────────────────────────────

func fixedClock() func() time.Time {
	return func() time.Time { return epoch }
}

func validHumanAuthor() ap.Author {
	return ap.NewHuman("counsel@example.com")
}

// makeAuthority returns a well-formed Authority with sane defaults.
func makeAuthority(id string, typ authority.AuthorityType, weight authority.Weight, status authority.Status) authority.Authority {
	return authority.Authority{
		ID:               id,
		Citation:         "42 U.S.C. § 1983",
		Type:             typ,
		Jurisdiction:     "US-SDNY",
		Weight:           weight,
		Status:           status,
		EffectiveFrom:    past(365 * 24 * time.Hour),
		EffectiveTo:      future(365 * 24 * time.Hour),
		SourceRecordID:   "rec-001",
		SourceLocatorRef: "loc-001",
		Author:           validHumanAuthor(),
	}
}

// ── enum validation ───────────────────────────────────────────────────────────

func TestAuthorityType_Validate(t *testing.T) {
	valid := []authority.AuthorityType{
		authority.TypeStatute,
		authority.TypeRegulation,
		authority.TypeCase,
		authority.TypeConstitutional,
		authority.TypeContractTerm,
		authority.TypePolicy,
		authority.TypeSecondary,
	}
	for _, typ := range valid {
		t.Run(string(typ), func(t *testing.T) {
			if err := typ.Validate(); err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
		})
	}

	invalid := authority.AuthorityType("ordinance")
	if err := invalid.Validate(); err == nil {
		t.Error("expected error for unknown AuthorityType, got nil")
	}
}

func TestWeight_Validate(t *testing.T) {
	valid := []authority.Weight{
		authority.WeightBinding,
		authority.WeightPersuasive,
		authority.WeightNonAuthoritative,
	}
	for _, w := range valid {
		t.Run(string(w), func(t *testing.T) {
			if err := w.Validate(); err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
		})
	}

	if err := authority.Weight("mandatory").Validate(); err == nil {
		t.Error("expected error for unknown Weight, got nil")
	}
}

func TestStatus_Validate(t *testing.T) {
	valid := []authority.Status{
		authority.StatusGoodLaw,
		authority.StatusQuestioned,
		authority.StatusSuperseded,
		authority.StatusOverruled,
		authority.StatusUnknown,
	}
	for _, s := range valid {
		t.Run(string(s), func(t *testing.T) {
			if err := s.Validate(); err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
		})
	}

	if err := authority.Status("abrogated").Validate(); err == nil {
		t.Error("expected error for unknown Status, got nil")
	}
}

// ── Authority.Validate ────────────────────────────────────────────────────────

func TestAuthorityValidate_Valid(t *testing.T) {
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	if err := a.Validate(); err != nil {
		t.Fatalf("expected valid authority, got: %v", err)
	}
}

func TestAuthorityValidate_EmptyID(t *testing.T) {
	a := makeAuthority("", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	if err := a.Validate(); err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestAuthorityValidate_EmptyCitation(t *testing.T) {
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	a.Citation = ""
	if err := a.Validate(); err == nil {
		t.Error("expected error for empty Citation")
	}
}

func TestAuthorityValidate_BadEnums(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*authority.Authority)
	}{
		{"bad-type", func(a *authority.Authority) { a.Type = "ordinance" }},
		{"bad-weight", func(a *authority.Authority) { a.Weight = "mandatory" }},
		{"bad-status", func(a *authority.Authority) { a.Status = "abrogated" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
			tc.mut(&a)
			if err := a.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

func TestAuthorityValidate_EffectiveWindowInverted(t *testing.T) {
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	// EffectiveTo before EffectiveFrom — invalid
	a.EffectiveFrom = future(24 * time.Hour)
	a.EffectiveTo = past(24 * time.Hour)
	if err := a.Validate(); err == nil {
		t.Error("expected error when EffectiveFrom > EffectiveTo")
	}
}

func TestAuthorityValidate_OpenWindow(t *testing.T) {
	// Zero EffectiveTo means open-ended (still valid).
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	a.EffectiveTo = time.Time{}
	if err := a.Validate(); err != nil {
		t.Fatalf("open EffectiveTo should be valid, got: %v", err)
	}
}

func TestAuthorityValidate_ZeroEffectiveFrom(t *testing.T) {
	// Both zero — open window; valid.
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	a.EffectiveFrom = time.Time{}
	a.EffectiveTo = time.Time{}
	if err := a.Validate(); err != nil {
		t.Fatalf("both zero EffectiveFrom/To should be valid, got: %v", err)
	}
}

func TestAuthorityValidate_BadAuthor(t *testing.T) {
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	// An invalid Author (unknown kind).
	a.Author = ap.Author{Kind: "robot"}
	if err := a.Validate(); err == nil {
		t.Error("expected error for invalid Author")
	}
}

func TestAuthorityValidate_ZeroAuthor(t *testing.T) {
	// Zero Author (empty kind) is allowed — optional provenance.
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	a.Author = ap.Author{}
	if err := a.Validate(); err != nil {
		t.Fatalf("zero Author should be valid (optional field), got: %v", err)
	}
}

// ── Treatment validation ──────────────────────────────────────────────────────

func TestTreatmentKind_Validate(t *testing.T) {
	valid := []authority.TreatmentKind{
		authority.KindFollows,
		authority.KindDistinguishes,
		authority.KindCriticizes,
		authority.KindOverrules,
		authority.KindReaffirms,
		authority.KindQuestions,
	}
	for _, k := range valid {
		t.Run(string(k), func(t *testing.T) {
			if err := k.Validate(); err != nil {
				t.Fatalf("expected valid TreatmentKind, got: %v", err)
			}
		})
	}

	if err := authority.TreatmentKind("cites").Validate(); err == nil {
		t.Error("expected error for unknown TreatmentKind, got nil")
	}
}

func TestTreatment_Validate(t *testing.T) {
	tr := authority.Treatment{FromID: "a1", ToID: "b1", Kind: authority.KindFollows}
	if err := tr.Validate(); err != nil {
		t.Fatalf("expected valid Treatment, got: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*authority.Treatment)
	}{
		{"empty-FromID", func(t *authority.Treatment) { t.FromID = "" }},
		{"empty-ToID", func(t *authority.Treatment) { t.ToID = "" }},
		{"bad-kind", func(t *authority.Treatment) { t.Kind = "cites" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr2 := authority.Treatment{FromID: "a1", ToID: "b1", Kind: authority.KindFollows}
			tc.mut(&tr2)
			if err := tr2.Validate(); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// ── Graph.Add / Treat / InForce ───────────────────────────────────────────────

func TestGraph_AddAndInForce(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	if err := g.Add(a); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// epoch is within past(365d) → future(365d) window.
	if !g.InForce("auth-001", epoch) {
		t.Error("expected InForce=true at epoch")
	}

	// Far future — beyond EffectiveTo.
	if g.InForce("auth-001", future(400*24*time.Hour)) {
		t.Error("expected InForce=false beyond EffectiveTo")
	}

	// Past — before EffectiveFrom.
	if g.InForce("auth-001", past(400*24*time.Hour)) {
		t.Error("expected InForce=false before EffectiveFrom")
	}
}

func TestGraph_InForce_OpenWindow(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	a.EffectiveFrom = time.Time{} // open start
	a.EffectiveTo = time.Time{}   // open end
	if err := g.Add(a); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	// Open window is always in force.
	if !g.InForce("auth-001", epoch) {
		t.Error("expected InForce=true for open window")
	}
	if !g.InForce("auth-001", future(1000*24*time.Hour)) {
		t.Error("expected InForce=true far future for open window")
	}
}

func TestGraph_InForce_UnknownID(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	// InForce on an unknown ID must return false, not panic.
	if g.InForce("no-such", epoch) {
		t.Error("expected InForce=false for unknown ID")
	}
}

func TestGraph_Treat_BothEndpointsNeedNotPreExist(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	// Treat should succeed even if both authorities are not in the graph yet.
	tr := authority.Treatment{FromID: "x", ToID: "y", Kind: authority.KindFollows}
	if err := g.Treat(tr); err != nil {
		t.Fatalf("Treat with unknown endpoints should succeed, got: %v", err)
	}
}

// ── EffectiveStatus: no inbound treatments ────────────────────────────────────

func TestEffectiveStatus_NoTreatments(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(a)

	st, reasons := g.EffectiveStatus("auth-001")
	if st != authority.StatusGoodLaw {
		t.Errorf("expected StatusGoodLaw, got %q", st)
	}
	if len(reasons) != 0 {
		t.Errorf("expected no reasons, got %v", reasons)
	}
}

func TestEffectiveStatus_UnknownID(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	st, _ := g.EffectiveStatus("no-such")
	if st != authority.StatusUnknown {
		t.Errorf("expected StatusUnknown for unknown ID, got %q", st)
	}
}

// ── EffectiveStatus: binding overrule → overruled ────────────────────────────

func TestEffectiveStatus_BindingOverrule(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("old-case", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	newer := makeAuthority("new-case", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(newer)

	// newer overrules target (From=newer → To=target).
	_ = g.Treat(authority.Treatment{FromID: "new-case", ToID: "old-case", Kind: authority.KindOverrules})

	st, reasons := g.EffectiveStatus("old-case")
	if st != authority.StatusOverruled {
		t.Errorf("expected StatusOverruled, got %q", st)
	}
	if len(reasons) == 0 {
		t.Error("expected at least one reason for overruled status")
	}
}

// ── EffectiveStatus: persuasive overrule does NOT override ───────────────────

func TestEffectiveStatus_PersuasiveOverruleNoEffect(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("old-case", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	sibling := makeAuthority("sibling-case", authority.TypeCase, authority.WeightPersuasive, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(sibling)

	// A persuasive-weight authority cannot overrule (only binding can).
	_ = g.Treat(authority.Treatment{FromID: "sibling-case", ToID: "old-case", Kind: authority.KindOverrules})

	st, _ := g.EffectiveStatus("old-case")
	if st == authority.StatusOverruled {
		t.Error("persuasive overrule must not change status to overruled")
	}
}

// ── EffectiveStatus: questioned/criticizes from binding → questioned ──────────

func TestEffectiveStatus_BindingQuestioned(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindQuestions})

	st, reasons := g.EffectiveStatus("target")
	if st != authority.StatusQuestioned {
		t.Errorf("expected StatusQuestioned from binding questions treatment, got %q", st)
	}
	if len(reasons) == 0 {
		t.Error("expected reasons for questioned downgrade")
	}
}

func TestEffectiveStatus_BindingCriticizes(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindCriticizes})

	st, _ := g.EffectiveStatus("target")
	if st != authority.StatusQuestioned {
		t.Errorf("expected StatusQuestioned from binding criticizes, got %q", st)
	}
}

// ── EffectiveStatus: treatments only downgrade, never upgrade ────────────────

func TestEffectiveStatus_NoUpgrade(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	// target is already questioned.
	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusQuestioned)
	reaffirmer := makeAuthority("reaffirmer", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(reaffirmer)

	// A reaffirms treatment must not upgrade questioned → good_law.
	_ = g.Treat(authority.Treatment{FromID: "reaffirmer", ToID: "target", Kind: authority.KindReaffirms})

	st, _ := g.EffectiveStatus("target")
	// Reaffirms from binding does not upgrade — status stays questioned or
	// potentially lower; it must never return good_law.
	if st == authority.StatusGoodLaw {
		t.Error("treatment must never upgrade status — reaffirms cannot lift questioned to good_law")
	}
}

// ── EffectiveStatus: determinism ─────────────────────────────────────────────

func TestEffectiveStatus_Deterministic(t *testing.T) {
	// Build a graph with multiple inbound treatments and check that EffectiveStatus
	// returns the same result across multiple calls.
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("t", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	for i := 0; i < 5; i++ {
		id := "q" + string(rune('0'+i))
		q := makeAuthority(id, authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
		_ = g.Add(q)
		_ = g.Treat(authority.Treatment{FromID: id, ToID: "t", Kind: authority.KindQuestions})
	}

	first, firstReasons := g.EffectiveStatus("t")
	for i := 0; i < 10; i++ {
		st, reasons := g.EffectiveStatus("t")
		if st != first {
			t.Errorf("call %d: status changed across calls: %q vs %q", i, st, first)
		}
		if len(reasons) != len(firstReasons) {
			t.Errorf("call %d: reasons length changed: %d vs %d", i, len(reasons), len(firstReasons))
		}
	}
}

// ── Graph.Authorities (defensive copy) ───────────────────────────────────────

func TestGraph_Authorities_DefensiveCopy(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	a := makeAuthority("auth-001", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(a)

	list := g.Authorities()
	if len(list) != 1 {
		t.Fatalf("expected 1 authority, got %d", len(list))
	}

	// Mutate the returned slice — must not affect the graph.
	list[0].Citation = "mutated"
	list2 := g.Authorities()
	if list2[0].Citation == "mutated" {
		t.Error("Authorities() returned a live reference — mutations affect stored state")
	}
}

// ── Graph.Treatments (defensive copy) ────────────────────────────────────────

func TestGraph_Treatments_DefensiveCopy(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	_ = g.Treat(authority.Treatment{FromID: "a", ToID: "b", Kind: authority.KindFollows})

	ts := g.Treatments()
	if len(ts) != 1 {
		t.Fatalf("expected 1 treatment, got %d", len(ts))
	}

	ts[0].FromID = "mutated"
	ts2 := g.Treatments()
	if ts2[0].FromID == "mutated" {
		t.Error("Treatments() returned a live reference — mutations affect stored state")
	}
}

// ── Justification validation ──────────────────────────────────────────────────

func TestJustification_Valid(t *testing.T) {
	j := authority.Justification{
		ConclusionID:  "concl-001",
		EvidenceRefs:  []string{"ev-001", "ev-002"},
		AuthorityRefs: []string{"auth-001"},
	}
	if err := j.Validate(); err != nil {
		t.Fatalf("expected valid Justification, got: %v", err)
	}
}

func TestJustification_EmptyConclusionID(t *testing.T) {
	j := authority.Justification{
		ConclusionID:  "",
		EvidenceRefs:  []string{"ev-001"},
		AuthorityRefs: []string{"auth-001"},
	}
	if err := j.Validate(); err == nil {
		t.Error("expected error for empty ConclusionID")
	}
}

func TestJustification_EmptyEvidenceRefs(t *testing.T) {
	j := authority.Justification{
		ConclusionID:  "concl-001",
		EvidenceRefs:  nil,
		AuthorityRefs: []string{"auth-001"},
	}
	if err := j.Validate(); err == nil {
		t.Error("expected error for nil/empty EvidenceRefs")
	}
}

func TestJustification_EmptyAuthorityRefs(t *testing.T) {
	j := authority.Justification{
		ConclusionID:  "concl-001",
		EvidenceRefs:  []string{"ev-001"},
		AuthorityRefs: nil,
	}
	if err := j.Validate(); err == nil {
		t.Error("expected error for nil/empty AuthorityRefs")
	}
}

func TestJustification_EvidenceRefsDefensiveCopy(t *testing.T) {
	original := []string{"ev-001", "ev-002"}
	j := authority.Justification{
		ConclusionID:  "concl-001",
		EvidenceRefs:  original,
		AuthorityRefs: []string{"auth-001"},
	}
	// Mutate original after constructing; Validate should still see the original values.
	// (Validate itself is pure; the Graph stores Justification by value, so this
	// tests that callers copying Justification get a safe copy via Clone().)
	clone := j.Clone()
	original[0] = "mutated"
	if clone.EvidenceRefs[0] == "mutated" {
		t.Error("Clone() did not deep-copy EvidenceRefs")
	}
}

// ── Add invalid authority ─────────────────────────────────────────────────────

func TestGraph_Add_InvalidAuthority(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	bad := makeAuthority("", authority.TypeStatute, authority.WeightBinding, authority.StatusGoodLaw)
	if err := g.Add(bad); err == nil {
		t.Error("expected error when adding invalid Authority")
	}
}

// ── DEFECT 1: downgrade from non-good-law starting status ────────────────────
//
// A binding questions/criticizes treatment must downgrade an authority whose
// stored status is StatusUnknown to StatusQuestioned (unknown sits below
// questioned in the severity order and should be downgraded, not silently
// skipped). Regression for the `if derived == StatusGoodLaw` guard that missed
// non-good-law starting statuses.

func TestEffectiveStatus_BindingQuestions_FromUnknown(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	// target has never been verified — status is unknown.
	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusUnknown)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindQuestions})

	st, reasons := g.EffectiveStatus("target")
	if st != authority.StatusQuestioned {
		t.Errorf("binding questions on unknown-status authority: expected StatusQuestioned, got %q", st)
	}
	if len(reasons) == 0 {
		t.Error("expected at least one reason when downgrading unknown → questioned")
	}
}

func TestEffectiveStatus_BindingCriticizes_FromUnknown(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusUnknown)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindCriticizes})

	st, _ := g.EffectiveStatus("target")
	if st != authority.StatusQuestioned {
		t.Errorf("binding criticizes on unknown-status authority: expected StatusQuestioned, got %q", st)
	}
}

// A binding questions/criticizes treatment on an authority already at
// StatusQuestioned must keep it at StatusQuestioned (same severity — no
// downgrade, no upgrade), and must still record a reason for each treating
// authority.

func TestEffectiveStatus_BindingQuestions_FromQuestioned_RecordsReason(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusQuestioned)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindQuestions})

	st, reasons := g.EffectiveStatus("target")
	if st != authority.StatusQuestioned {
		t.Errorf("already-questioned target: expected StatusQuestioned, got %q", st)
	}
	if len(reasons) == 0 {
		t.Error("expected a reason even when status does not change (binding authority questioned it)")
	}
}

// A binding questions/criticizes treatment on an authority at StatusSuperseded
// must NOT downgrade it (superseded is already worse than questioned in the
// severity order), but must still record a reason.

func TestEffectiveStatus_BindingQuestions_FromSuperseded_NoDowngrade_WithReason(t *testing.T) {
	g := authority.NewGraph(fixedClock())
	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusSuperseded)
	critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(critic)

	_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindQuestions})

	st, reasons := g.EffectiveStatus("target")
	if st != authority.StatusSuperseded {
		t.Errorf("superseded target questioned by binding authority: status must stay superseded, got %q", st)
	}
	if len(reasons) == 0 {
		t.Error("expected a reason recorded even though no severity downgrade occurred")
	}
}

// ── DEFECT 2: multiple binding overrulers — all reasons captured ──────────────
//
// When two or more binding authorities overrule the same target, all of them
// must appear in the returned reasons slice. The old early-return on the first
// overrule silently dropped subsequent overrulers from the trace.

func TestEffectiveStatus_MultipleBindingOverrulers_AllReasonsRecorded(t *testing.T) {
	g := authority.NewGraph(fixedClock())

	target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	newer1 := makeAuthority("newer1", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	newer2 := makeAuthority("newer2", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
	_ = g.Add(target)
	_ = g.Add(newer1)
	_ = g.Add(newer2)

	_ = g.Treat(authority.Treatment{FromID: "newer1", ToID: "target", Kind: authority.KindOverrules})
	_ = g.Treat(authority.Treatment{FromID: "newer2", ToID: "target", Kind: authority.KindOverrules})

	st, reasons := g.EffectiveStatus("target")
	if st != authority.StatusOverruled {
		t.Errorf("expected StatusOverruled, got %q", st)
	}
	// Both overrulers must appear in the reasons trace — no silent truncation.
	if len(reasons) < 2 {
		t.Errorf("expected at least 2 reasons (one per overruling authority), got %d: %v", len(reasons), reasons)
	}
}

// ── DEFECT 1 (all starting statuses × binding questions): exhaustive matrix ───

func TestEffectiveStatus_DowngradeMatrix_BindingQuestions(t *testing.T) {
	type tc struct {
		startStatus authority.Status
		wantStatus  authority.Status
		wantReason  bool // expect at least one reason entry
		desc        string
	}
	cases := []tc{
		{
			startStatus: authority.StatusGoodLaw,
			wantStatus:  authority.StatusQuestioned,
			wantReason:  true,
			desc:        "good_law → questioned",
		},
		{
			startStatus: authority.StatusUnknown,
			wantStatus:  authority.StatusQuestioned,
			wantReason:  true,
			desc:        "unknown → questioned",
		},
		{
			startStatus: authority.StatusQuestioned,
			wantStatus:  authority.StatusQuestioned,
			wantReason:  true,
			desc:        "questioned → questioned (same, reason still recorded)",
		},
		{
			startStatus: authority.StatusSuperseded,
			wantStatus:  authority.StatusSuperseded,
			wantReason:  true,
			desc:        "superseded stays superseded (worse than questioned), reason recorded",
		},
		{
			startStatus: authority.StatusOverruled,
			wantStatus:  authority.StatusOverruled,
			wantReason:  true,
			desc:        "overruled stays overruled (worst), reason recorded",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			g := authority.NewGraph(fixedClock())
			target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, tc.startStatus)
			critic := makeAuthority("critic", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
			_ = g.Add(target)
			_ = g.Add(critic)
			_ = g.Treat(authority.Treatment{FromID: "critic", ToID: "target", Kind: authority.KindQuestions})

			st, reasons := g.EffectiveStatus("target")
			if st != tc.wantStatus {
				t.Errorf("%s: got status %q, want %q", tc.desc, st, tc.wantStatus)
			}
			if tc.wantReason && len(reasons) == 0 {
				t.Errorf("%s: expected at least one reason, got none", tc.desc)
			}
		})
	}
}

// Binding overrule from any starting status always yields StatusOverruled and
// records a reason — overrule is the strongest possible downgrade regardless of
// where the authority started.

func TestEffectiveStatus_DowngradeMatrix_BindingOverrules(t *testing.T) {
	startStatuses := []authority.Status{
		authority.StatusGoodLaw,
		authority.StatusUnknown,
		authority.StatusQuestioned,
		authority.StatusSuperseded,
		authority.StatusOverruled,
	}

	for _, start := range startStatuses {
		t.Run(string(start), func(t *testing.T) {
			g := authority.NewGraph(fixedClock())
			target := makeAuthority("target", authority.TypeCase, authority.WeightBinding, start)
			newer := makeAuthority("newer", authority.TypeCase, authority.WeightBinding, authority.StatusGoodLaw)
			_ = g.Add(target)
			_ = g.Add(newer)
			_ = g.Treat(authority.Treatment{FromID: "newer", ToID: "target", Kind: authority.KindOverrules})

			st, reasons := g.EffectiveStatus("target")
			if st != authority.StatusOverruled {
				t.Errorf("start=%s: binding overrule must always yield StatusOverruled, got %q", start, st)
			}
			if len(reasons) == 0 {
				t.Errorf("start=%s: expected at least one reason for overrule", start)
			}
		})
	}
}
