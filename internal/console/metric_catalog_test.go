package console

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestMetricCatalog_OneMetricPerQuestion pins OC-H3 (vision 35: one
// projection per metric): every shared metric answers its own question, and
// the ids integration kept side by side for one question are gone —
// certificates.live beside certify.live, claims.fresh beside
// claims.current, plans.verified beside plans.proven.
func TestMetricCatalog_OneMetricPerQuestion(t *testing.T) {
	ids := map[MetricID]bool{}
	questions := map[string]MetricID{}
	for _, m := range MetricCatalog {
		if m.ID == "" || strings.TrimSpace(m.Question) == "" {
			t.Errorf("catalog entry %+v has no id or question", m)
		}
		if ids[m.ID] {
			t.Errorf("metric %s is catalogued twice", m.ID)
		}
		ids[m.ID] = true
		q := strings.ToLower(strings.TrimSpace(m.Question))
		if other, dup := questions[q]; dup {
			t.Errorf("metrics %s and %s answer one question (%q): one of them must go", other, m.ID, m.Question)
		}
		questions[q] = m.ID
	}
	for _, retired := range []MetricID{"plans.verified", "claims.fresh", "certificates.live"} {
		if ids[retired] {
			t.Errorf("%s is still a metric; its question has one metric already", retired)
		}
	}
	for _, id := range []MetricID{MetricPlansProven, MetricPlansUnproven, MetricClaims, MetricCertLive, MetricCertGaps} {
		if !ids[id] {
			t.Errorf("%s is not catalogued", id)
		}
	}
	if _, ok := ProjectMetrics(Projects{})[MetricID("claims.fresh")]; ok {
		t.Error("ProjectMetrics still projects claims.fresh")
	}
}

// TestClaimsCurrent_ProjectsAndKnowledgeAgree: the Now and Fleet pages read
// claims per project; the Layers and Packs pages read the knowledge fold.
// Both are one projection over the same per-service statuses, so they say
// the same thing, byte for byte — including an unreadable project, which
// makes it a lower bound naming the project.
func TestClaimsCurrent_ProjectsAndKnowledgeAgree(t *testing.T) {
	claims := func(st ...core.ClaimStatus) []core.SemanticClaim {
		var out []core.SemanticClaim
		for _, s := range st {
			out = append(out, core.SemanticClaim{Status: s})
		}
		return out
	}
	k := knowledge{Services: []serviceKnowledge{
		{Service: "a", Claims: claims(core.ClaimFresh, core.ClaimFresh, core.ClaimStale, core.ClaimContradicted, core.ClaimSuperseded, core.ClaimRefuted)},
		{Service: "b", Err: "boom"},
		{Service: "c"},
	}}
	p := Projects{Store: SourceRead{State: StatePresent}, Rows: []ProjectFacts{
		{Service: "a", Claim: Present(ClaimCounts{Total: 6, Fresh: 2, Stale: 1, Contradicted: 1, Superseded: 1, Refuted: 1, Other: 2}, "s")},
		{Service: "b", Claim: Errored[ClaimCounts]("boom")},
		{Service: "c", Claim: Present(ClaimCounts{}, "s")},
	}}
	fromProjects := ProjectMetrics(p)[MetricClaims]
	fromKnowledge := currentClaims(k)
	if !reflect.DeepEqual(fromProjects, fromKnowledge) {
		t.Errorf("claims.current differs by page:\n projects  %+v\n knowledge %+v", fromProjects, fromKnowledge)
	}
	if fromProjects.V != 4 || fromProjects.State != StatePartial || !strings.Contains(fromProjects.Reason, "b") {
		t.Errorf("claims.current = %+v, want 4 (fresh, stale, contradicted) as a lower bound naming b", fromProjects)
	}
	q := p
	q.Store = SourceRead{State: StateAbsent, Reason: "no store recorded for this daemon"}
	if v := ProjectMetrics(q)[MetricClaims]; v.State != StateAbsent || v.Reason != "no store recorded for this daemon" {
		t.Errorf("claims with no store = %+v", v)
	}
}

// TestCertifyTotals_ProjectsAndMatrixAgree: the Now and Fleet pages read
// each project's certify status; the Certifications page reads each
// service's certify gaps report. One projection over the same per-service
// facts: the same live and gap totals, byte for byte, on every page — and a
// service whose targets could not all be read makes every total an error
// naming it, never a smaller count (I38).
func TestCertifyTotals_ProjectsAndMatrixAgree(t *testing.T) {
	p := Projects{Store: SourceRead{State: StatePresent}, Rows: []ProjectFacts{
		{Service: "a", Certs: Present(CertCounts{Declared: 4, Live: 1, Gaps: 3}, "s")},
		{Service: "b", Certs: Present(CertCounts{}, "s")},
		{Service: "c", Certs: Present(CertCounts{Declared: 2, Live: 2}, "s")},
	}}
	facts := []CertServiceFacts{
		{Service: "a", Declared: 4, Live: 1, Gaps: 3, Targets: []CertTargetFact{}},
		{Service: "c", Declared: 2, Live: 2, Targets: []CertTargetFact{}},
	}
	m := CertMatrix(facts, "", nil)
	pm := ProjectMetrics(p)
	if !reflect.DeepEqual(pm[MetricCertLive], m.Live) || !reflect.DeepEqual(pm[MetricCertGaps], m.GapCount) {
		t.Errorf("certify totals differ by page:\n projects live %+v gaps %+v\n matrix   live %+v gaps %+v", pm[MetricCertLive], pm[MetricCertGaps], m.Live, m.GapCount)
	}
	if m.Live.V != 3 || m.GapCount.V != 3 {
		t.Errorf("live %+v gaps %+v, want 3 and 3", m.Live, m.GapCount)
	}
	// A partly unreadable project: no total, the project named.
	p.Rows[0].Certs = Partial(CertCounts{Declared: 4, Live: 1, Gaps: 3, Unreadable: []string{"docs/intent/invariants.md"}}, "targets read in part")
	pm = ProjectMetrics(p)
	facts[0].Err = "certify could not read docs/intent/invariants.md — its targets would be a lower bound"
	m = CertMatrix(facts, "", nil)
	if !reflect.DeepEqual(pm[MetricCertLive], m.Live) || pm[MetricCertLive].State != StateError || !strings.Contains(pm[MetricCertLive].Reason, "a (") {
		t.Errorf("an unreadable project: projects %+v, matrix %+v", pm[MetricCertLive], m.Live)
	}
}
