package review

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

const tracePlanMD = `# Plan

- [x] The thing, honoring ADR-7 and I12 {id: T-1, symbol: DoThing, package: internal/thing, reachable_from: TestDoThing}
- [ ] The other thing {id: T-2, symbol: Other, package: internal/thing, accept: "an operator sees the other thing"}
- [x] Certified {id: T-3, symbol: Certified, package: internal/thing, certified_by: TestCertified}
- [-] Dropped {id: T-4, disposition: refused, reason: "ADR-9 said no"}
`

func traceFixture(t *testing.T) TraceInput {
	t.Helper()
	plan := verify.ParseMarkdownPlan("svc", []byte(tracePlanMD))
	rep := verify.Report{Service: "svc", Tasks: []verify.TaskVerdict{
		{TaskID: "T-1", Status: verify.Pass, Label: "verified", Claimed: true, Criteria: []verify.Criterion{
			{Name: "symbol_present", Status: verify.Pass, Detail: "DoThing at internal/thing/a.go:3"},
			{Name: "test_reaches", Status: verify.Pass, Detail: "1 test(s) statically reach it, e.g. TestDoThing"},
		}},
		{TaskID: "T-2", Status: verify.Fail, Label: "claimed-but-absent", Criteria: []verify.Criterion{
			{Name: "symbol_present", Status: verify.Fail, Detail: "Other not found"},
			{Name: "test_reaches", Status: verify.NA, Detail: "symbol absent"},
		}},
		{TaskID: "T-3", Status: verify.Pass, Label: "verified", Claimed: true, Criteria: []verify.Criterion{
			{Name: "symbol_present", Status: verify.Pass, Detail: "Certified at internal/thing/c.go:9"},
			{Name: "certified_by", Status: verify.Pass, Detail: "TestCertified live"},
		}},
	}}
	return TraceInput{
		Sources: []TraceSource{{Path: "docs/plans/p.md", Plan: plan}},
		Reports: map[string]verify.Report{"docs/plans/p.md": rep},
		Certificates: []TraceCertificate{
			{TargetKind: "symbol", TargetID: "internal/thing.Certified", Test: "TestCertified", State: "live", RunDigest: "sha256:abc"},
			{TargetKind: "symbol", TargetID: "internal/thing.DoThing", Test: "TestDoThing", State: "flaky"},
		},
		Commit:    "deadbeef",
		Decisions: map[string]bool{"ADR-7": true, "ADR-9": true},
	}
}

func rowByID(t *testing.T, m TraceMatrix, id string) TraceRow {
	t.Helper()
	for _, r := range m.Rows {
		if r.Requirement.ID == id {
			return r
		}
	}
	t.Fatalf("no row %s in matrix", id)
	return TraceRow{}
}

// TestTraceMatrixLiveStates (CX-14): every cell carries its live state and
// the reading that proved it, and identical input renders identical bytes.
func TestTraceMatrixLiveStates(t *testing.T) {
	in := traceFixture(t)
	m := BuildTraceMatrix(in)

	t1 := rowByID(t, m, "T-1")
	if t1.Status.State != CellProven || !strings.Contains(t1.Status.Reading, "verified") {
		t.Errorf("T-1 status = %+v", t1.Status)
	}
	if t1.Code.State != CellPresent || !strings.Contains(t1.Code.Reading, "internal/thing/a.go:3") {
		t.Errorf("T-1 code = %+v", t1.Code)
	}
	if t1.Test.State != CellReaches || !strings.Contains(t1.Test.Reading, "TestDoThing") {
		t.Errorf("T-1 test = %+v", t1.Test)
	}
	if t1.Certificate.State != "flaky" {
		t.Errorf("T-1 certificate = %+v, want flaky (a non-live certificate is named)", t1.Certificate)
	}
	if t1.AttestedRun.State != CellNone {
		t.Errorf("T-1 attested run = %+v, want none (no digest)", t1.AttestedRun)
	}
	if t1.Decision.State != CellUnproven || !strings.Contains(t1.Decision.Reading, "not found: I12") {
		t.Errorf("T-1 decision = %+v, want I12 named missing", t1.Decision)
	}

	t2 := rowByID(t, m, "T-2")
	if t2.Status.State != CellUnproven || t2.Code.State != CellAbsent || t2.Test.State != CellNone {
		t.Errorf("T-2 = %+v", t2)
	}
	if t2.Decision.State != CellNone {
		t.Errorf("T-2 decision = %+v", t2.Decision)
	}

	t3 := rowByID(t, m, "T-3")
	if t3.Test.State != CellReaches || !strings.Contains(t3.Test.Reading, "certified_by: TestCertified") {
		t.Errorf("T-3 test = %+v", t3.Test)
	}
	if t3.Certificate.State != "live" {
		t.Errorf("T-3 certificate = %+v", t3.Certificate)
	}
	if t3.AttestedRun.State != CellAttested || t3.AttestedRun.Reading != "sha256:abc @ deadbeef" {
		t.Errorf("T-3 attested run = %+v", t3.AttestedRun)
	}

	t4 := rowByID(t, m, "T-4")
	if t4.Status.State != CellSkipped || t4.Decision.State != CellCited {
		t.Errorf("T-4 = %+v", t4)
	}

	for _, r := range m.Rows {
		for _, c := range []TraceCell{r.Status, r.Code, r.Test, r.Certificate, r.AttestedRun, r.Decision} {
			if c.State == "" || c.Reading == "" {
				t.Errorf("%s: a cell without state or reading: %+v", r.Requirement.ID, c)
			}
		}
	}

	// Determinism: shuffled certificates, identical bytes.
	in2 := traceFixture(t)
	in2.Certificates[0], in2.Certificates[1] = in2.Certificates[1], in2.Certificates[0]
	p1 := TracePack{Standard: "s", Commit: "deadbeef", Service: "svc", Matrix: m}
	p2 := TracePack{Standard: "s", Commit: "deadbeef", Service: "svc", Matrix: BuildTraceMatrix(in2)}
	if !bytes.Equal(RenderTraceJSON(p1), RenderTraceJSON(p2)) || !bytes.Equal(RenderTraceMarkdown(p1), RenderTraceMarkdown(p2)) {
		t.Error("identical evidence rendered different bytes")
	}
}

// TestTraceRequirementsIncludeAccept (CX-18): an accept: criterion and a
// foreign spec task each appear as matrix requirements; the accept one
// never inherits the row's proof (SI-8: never gating by itself).
func TestTraceRequirementsIncludeAccept(t *testing.T) {
	native := verify.ParseMarkdownPlan("svc", []byte(tracePlanMD))
	foreign := verify.Plan{Service: "svc", Tasks: []verify.Task{{ID: "T010", Title: "Foreign task", Expect: verify.Expect{Symbol: "F"}}}}
	reqs := traceRequirements([]TraceSource{
		{Path: "specs/001/tasks.md", Dialect: "spec-kit", Plan: foreign},
		{Path: "docs/plans/p.md", Plan: native},
	})
	var ids []string
	kinds := map[string]string{}
	for _, r := range reqs {
		ids = append(ids, r.ID)
		kinds[r.ID] = r.Kind
	}
	want := "T-1,T-2,T-2/accept,T-3,T-4,T010"
	if strings.Join(ids, ",") != want {
		t.Fatalf("requirements = %v, want %s", ids, want)
	}
	if kinds["T-2/accept"] != ReqAccept || kinds["T010"] != ReqSpecTask || kinds["T-1"] != ReqPlanRow {
		t.Errorf("kinds = %v", kinds)
	}
	for _, r := range reqs {
		if r.ID == "T-2/accept" && r.Text != "an operator sees the other thing" {
			t.Errorf("accept text = %q", r.Text)
		}
	}

	in := traceFixture(t)
	in.Sources = append(in.Sources, TraceSource{Path: "specs/001/tasks.md", Dialect: "spec-kit", Plan: foreign})
	m := BuildTraceMatrix(in)
	acc := rowByID(t, m, "T-2/accept")
	if acc.Status.State != CellNotGating {
		t.Errorf("accept row status = %+v, want not_gating", acc.Status)
	}
	if f := rowByID(t, m, "T010"); f.Requirement.Kind != ReqSpecTask || f.Status.State != CellUnverified {
		t.Errorf("foreign row = %+v", f)
	}
}

func TestStandardMappingApply(t *testing.T) {
	src := []byte(`standard: "example"
clauses: [
	{id: "C-1", requirements: ["T-1", "T-99"]},
	{id: "C-2", patterns: ["^T-[23]"]},
	{id: "C-3"},
]
`)
	m, err := ParseStandardMapping("example.cue", src)
	if err != nil {
		t.Fatal(err)
	}
	cov := ApplyMapping(m, BuildTraceMatrix(traceFixture(t)))
	if len(cov) != 3 {
		t.Fatalf("coverage = %+v", cov)
	}
	if strings.Join(cov[0].Requirements, ",") != "T-1" || strings.Join(cov[0].Missing, ",") != "T-99" {
		t.Errorf("C-1 = %+v", cov[0])
	}
	if strings.Join(cov[1].Requirements, ",") != "T-2,T-2/accept,T-3" {
		t.Errorf("C-2 = %+v", cov[1])
	}
	if len(cov[2].Requirements) != 0 {
		t.Errorf("C-3 = %+v", cov[2])
	}
	if _, err := ParseStandardMapping("bad.cue", []byte(`standard: "x"
clauses: [{id: "C", patterns: ["("]}]`)); err == nil {
		t.Error("an invalid pattern should fail to parse")
	}
}

func TestDiffTracePacksNamesCells(t *testing.T) {
	m := BuildTraceMatrix(traceFixture(t))
	p := TracePack{Standard: "s", Commit: "c", Service: "svc", Matrix: m}
	a := map[string][]byte{"matrix.json": RenderTraceJSON(p)}
	m2 := BuildTraceMatrix(traceFixture(t))
	m2.Rows[0].Code.State = CellAbsent
	b := map[string][]byte{"matrix.json": RenderTraceJSON(TracePack{Standard: "s", Commit: "c", Service: "svc", Matrix: m2}), "extra": nil}
	d := DiffTracePacks(a, b)
	if len(d) != 2 || d[0].File != "extra" || d[1].File != "matrix.json" {
		t.Fatalf("diff = %+v", d)
	}
	if len(d[1].Cells) != 1 || !strings.HasSuffix(d[1].Cells[0], ".code") {
		t.Errorf("cells = %v", d[1].Cells)
	}
	if DiffTracePacks(a, a) != nil {
		t.Error("identical packs differ")
	}
}
