package assess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func noRedact(s string) (string, int) { return s, 0 }

func sampleUnit() UnitEvidence {
	catches := 0
	return UnitEvidence{
		WindowStart: "2026-09-25T10:00:00Z",
		WindowEnd:   "2026-09-25T11:00:00Z",
		Commits: []Commit{
			{SHA: "abcdef1234567", Subject: "feat: add widget", Patch: "diff --git a/w.go b/w.go\n+func Widget() {}\n"},
		},
		PlanDeltas:    []PlanDelta{{Plan: "docs/plans/p.md", Kind: DeltaFlipped, Row: "- [x] widget exists {id: W-1}"}},
		Gates:         []GateResult{{Plan: "docs/plans/p.md", Origin: "cli", Claimed: 1, Verified: 1}},
		VerifyCatches: &catches,
	}
}

// Every unit judgment is a TYPED question (noul or choice) — never a
// free-form prompt, and never a score (the port's score answer carries no
// spread, so its confidence would always read 1.0 and band as proceed).
func TestUnitQuestionsAreTypedAndVersioned(t *testing.T) {
	b := BuildUnit(sampleUnit(), DefaultCaps(), noRedact)
	if err := decision.Validate(b.Request); err != nil {
		t.Fatalf("built request is not a valid decision.Request: %v", err)
	}
	ids := map[string]decision.Kind{}
	for _, q := range b.Request.Questions {
		ids[q.ID] = q.Kind
		if q.Kind == decision.KindScore {
			t.Errorf("question %s is a score — a score answer has no confidence spread; use a choice over levels", q.ID)
		}
	}
	for id, want := range map[string]decision.Kind{"coherent": decision.KindPredicate, "claims_backed": decision.KindPredicate, "tests_accompany": decision.KindPredicate, "quality": decision.KindChoice} {
		if ids[id] != want {
			t.Errorf("question %s kind = %q, want %q", id, ids[id], want)
		}
	}
	if b.Digest == "" || b.Digest != UnitQuestionSetDigest() {
		t.Errorf("digest = %q, want the unit question-set digest %q", b.Digest, UnitQuestionSetDigest())
	}
	if UnitQuestionSetDigest() == InterventionQuestionSetDigest() {
		t.Error("the two lanes share a question-set digest; readings from different sets would pool")
	}

	// claims_backed is asked only when the unit flipped a row — asking it of a
	// unit with no flip invites a confident answer about nothing.
	ev := sampleUnit()
	ev.PlanDeltas = nil
	b2 := BuildUnit(ev, DefaultCaps(), noRedact)
	for _, q := range b2.Request.Questions {
		if q.ID == "claims_backed" {
			t.Fatal("claims_backed asked of a unit that flipped no plan row")
		}
	}
	if !strings.Contains(strings.Join(b2.Skipped, "\n"), "claims_backed") {
		t.Errorf("the unasked question must be named, got skipped=%v", b2.Skipped)
	}
}

// The state carries the work products and states its own boundary: no
// transcript is included, and every cut at a cap is named inside the state
// AND in Truncations, marking the run partial.
func TestRenderUnitStateCarriesProductsAndNamesCuts(t *testing.T) {
	b := BuildUnit(sampleUnit(), DefaultCaps(), noRedact)
	for _, want := range []string{"func Widget()", "flipped to done", "W-1", "claimed 1, verified 1, failing 0", "no transcript"} {
		if !strings.Contains(b.Request.State, want) {
			t.Errorf("state missing %q:\n%s", want, b.Request.State)
		}
	}
	if b.Partial || len(b.Truncations) != 0 {
		t.Errorf("small unit marked partial: %v", b.Truncations)
	}

	big := sampleUnit()
	big.Commits = append(big.Commits, Commit{SHA: "1111111222222", Subject: "huge", Patch: strings.Repeat("+x\n", 40000)})
	big.Commits = append(big.Commits, Commit{SHA: "3333333444444", Subject: "after", Patch: "+y\n"})
	caps := DefaultCaps()
	caps.MaxStateBytes = 4096
	bb := BuildUnit(big, caps, noRedact)
	if len(bb.Request.State) > caps.MaxStateBytes+512 {
		t.Errorf("state %d bytes exceeds the cap %d by more than the cut notices", len(bb.Request.State), caps.MaxStateBytes)
	}
	if !bb.Partial || len(bb.Truncations) == 0 {
		t.Fatal("an over-cap unit must be marked partial with its cuts named")
	}
	if !strings.Contains(bb.Request.State, "cut at the state cap") {
		t.Errorf("the cut must be named inside the state the judge reads:\n%s", bb.Request.State[len(bb.Request.State)-400:])
	}
}

// DLP runs before egress: the redactor sees the whole state and a redaction
// is reported.
func TestBuildRedactsBeforeEgress(t *testing.T) {
	ev := sampleUnit()
	ev.Commits[0].Patch += "+key := \"SECRET-SHAPED\"\n"
	b := BuildUnit(ev, DefaultCaps(), func(s string) (string, int) {
		return strings.ReplaceAll(s, "SECRET-SHAPED", "[REDACTED]"), strings.Count(s, "SECRET-SHAPED")
	})
	if strings.Contains(b.Request.State, "SECRET-SHAPED") || b.Redactions != 1 {
		t.Fatalf("redaction not applied/counted: redactions=%d", b.Redactions)
	}
}

// IR-3: one choice per judged file, cannot_tell always offered, provenance
// shown, the cap on questions named.
func TestInterventionQuestionsOnePerFileWithCannotTell(t *testing.T) {
	items := []Item{
		{Channel: "guard", File: "a.go", Outcome: "committed", Guidance: "never import net/http here", Provenance: "guard note in .fathom/guard.cue at assess time", Diff: "+import \"net/http\"\n"},
		{Channel: "obligations", File: "b.go", Outcome: "modified-uncommitted", Guidance: "wrap errors with %w", Provenance: "rules served now", Diff: "+return err\n"},
	}
	b := BuildInterventions(items, DefaultCaps(), noRedact)
	if err := decision.Validate(b.Request); err != nil {
		t.Fatalf("invalid request: %v", err)
	}
	if len(b.Request.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(b.Request.Questions))
	}
	for _, q := range b.Request.Questions {
		if q.Kind != decision.KindChoice {
			t.Errorf("%s kind %q, want choice", q.ID, q.Kind)
		}
		names := []string{}
		for _, o := range q.Options {
			names = append(names, o.Name)
		}
		if strings.Join(names, ",") != "complied,departed,not_applicable,cannot_tell" {
			t.Errorf("%s options = %v", q.ID, names)
		}
	}
	if b.Subjects["F01"] != "a.go" || b.Subjects["F02"] != "b.go" {
		t.Errorf("subjects = %v", b.Subjects)
	}
	for _, want := range []string{"never import net/http here", "guard note in .fathom/guard.cue", "rules served now", "== F02 obligations: b.go"} {
		if !strings.Contains(b.Request.State, want) {
			t.Errorf("state missing %q", want)
		}
	}

	many := make([]Item, 30)
	for i := range many {
		many[i] = Item{Channel: "guard", File: filepathName(i), Outcome: "committed", Guidance: "g", Provenance: "p", Diff: "+x\n"}
	}
	caps := DefaultCaps()
	bm := BuildInterventions(many, caps, noRedact)
	if len(bm.Request.Questions) != caps.MaxQuestions {
		t.Errorf("questions = %d, want the cap %d", len(bm.Request.Questions), caps.MaxQuestions)
	}
	if len(bm.Skipped) != 30-caps.MaxQuestions {
		t.Errorf("skipped = %d, want %d named", len(bm.Skipped), 30-caps.MaxQuestions)
	}
}

func filepathName(i int) string {
	return filepath.Join("pkg", string(rune('a'+i%26))+strings.Repeat("x", i/26)+".go")
}

// The calibrated-band policy: proceed is the verdict, confirm is leaning,
// unsure is undecided, invalid is named; every verdict carries its band.
func TestVerdictBands(t *testing.T) {
	b := BuildUnit(sampleUnit(), DefaultCaps(), noRedact)
	resp := decision.Response{
		Provider: "mock",
		Answers: map[string]decision.Answer{
			"coherent":        {ID: "coherent", Kind: decision.KindPredicate, Noul: 0.95, Confidence: 0.95},
			"tests_accompany": {ID: "tests_accompany", Kind: decision.KindPredicate, Noul: 0.3, Confidence: 0.7},
			"quality":         {ID: "quality", Kind: decision.KindChoice, Choice: "solid", Probabilities: map[string]float64{"solid": 0.4, "adequate": 0.3, "harmful": 0.1, "incomplete": 0.1, "exemplary": 0.1}, Confidence: 0.25},
		},
		Invalid: map[string]string{"claims_backed": "missing value"},
	}
	vs := Verdicts(b, resp)
	got := map[string]Verdict{}
	for _, v := range vs {
		got[v.QuestionID] = v
	}
	if v := got["coherent"]; v.Band != "proceed" || v.Answer != "true" || v.Subject != "unit" {
		t.Errorf("coherent = %+v", v)
	}
	if v := got["tests_accompany"]; v.Band != "confirm" || v.Answer != "false" {
		t.Errorf("tests_accompany = %+v", v)
	}
	if v := got["quality"]; v.Band != "unsure" || v.Answer != "solid" {
		t.Errorf("quality = %+v", v)
	}
	if v := got["claims_backed"]; v.Invalid != "missing value" || v.Band != "" {
		t.Errorf("claims_backed = %+v", v)
	}
	if len(vs) != len(b.Request.Questions) {
		t.Errorf("verdicts = %d, want one per asked question (%d)", len(vs), len(b.Request.Questions))
	}
	// Deterministic order: question order, never map order.
	for i, q := range b.Request.Questions {
		if vs[i].QuestionID != q.ID {
			t.Errorf("verdict %d = %s, want %s (question order)", i, vs[i].QuestionID, q.ID)
		}
	}
}

// The ledger round-trips, and its spend rows ride the compile spend meter's
// row type with the spend label as the group key.
func TestLedgerRoundTripAndSpendRuns(t *testing.T) {
	p := filepath.Join(t.TempDir(), "assess-ledger.jsonl")
	in, out := 1200, 80
	usd := 0.0016
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	rows := []Row{
		{At: at.Format(time.RFC3339), RunID: "r1", SpendLabel: LabelUnit, Lane: LaneUnit, Model: "m", TokensIn: &in, TokensOut: &out, CostUSD: &usd, Isolation: IsolationRegistration},
		{At: at.Add(time.Hour).Format(time.RFC3339), RunID: "r2", SpendLabel: LabelInterventions, Lane: LaneInterventions, Model: "jev", TokensIn: &in, TokensOut: &out, CostNote: "cost unknown: no price for model jev", Isolation: IsolationRegistration},
	}
	for _, r := range rows {
		if err := AppendRow(p, r); err != nil {
			t.Fatal(err)
		}
	}
	back, err := ReadRows(p)
	if err != nil || len(back) != 2 {
		t.Fatalf("read back %d rows, err %v", len(back), err)
	}
	if back[0].SchemaVersion != RowSchemaVersion || back[0].SpendLabel != LabelUnit {
		t.Errorf("row 0 = %+v", back[0])
	}
	runs, classes := SpendRuns(back)
	if len(runs) != 2 || len(classes) != 2 || classes[0] != CostPriced || classes[1] != CostUnpriced {
		t.Fatalf("runs = %d classes = %v", len(runs), classes)
	}
	if runs[0].Layer != LabelUnit || runs[0].TokensIn != 1200 || runs[0].CostUSD != usd || !runs[0].RanAt.Equal(at) {
		t.Errorf("run 0 = %+v", runs[0])
	}
	if runs[1].CostUSD != 0 {
		t.Errorf("an unpriced run must contribute no invented cost, got %v", runs[1].CostUSD)
	}
	if _, err := ReadRows(filepath.Join(t.TempDir(), "absent.jsonl")); err != nil {
		t.Errorf("an absent ledger is empty, not an error: %v", err)
	}
	if err := os.WriteFile(p, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadRows(p); len(got) != 0 {
		t.Errorf("malformed line produced rows: %v", got)
	}
}

// Every row lands in exactly one cost class, and only a priced one has a
// cost: a failed call is "failed" whatever it carries (it may still have
// been billed), and a provider that reported nothing is "no_usage".
func TestCostClass(t *testing.T) {
	n, usd := 10, 0.001
	for _, tc := range []struct {
		row  Row
		want string
	}{
		{Row{TokensIn: &n, TokensOut: &n, CostUSD: &usd}, CostPriced},
		{Row{TokensIn: &n, TokensOut: &n}, CostUnpriced},
		{Row{}, CostNoUsage},
		{Row{Error: "timeout"}, CostFailed},
		{Row{Error: "timeout", TokensIn: &n, CostUSD: &usd}, CostFailed},
	} {
		if got := CostClass(tc.row); got != tc.want {
			t.Errorf("CostClass(%+v) = %q, want %q", tc.row, got, tc.want)
		}
	}
}

func TestIsolatedSpendLabel(t *testing.T) {
	for _, l := range []string{LabelUnit, LabelInterventions} {
		if !IsolatedSpendLabel(l) {
			t.Errorf("%q not isolated", l)
		}
	}
	if IsolatedSpendLabel("") {
		t.Error("an unlabelled row is ordinary session spend, not isolated")
	}
	// Any label is isolated: a future lane's label must never leak into
	// arm totals just because this list was not updated.
	if !IsolatedSpendLabel("assess-future-lane") {
		t.Error("an unknown non-empty label must still be isolated (fail toward exclusion)")
	}
}

func TestEstimateTokensIsTheCompileHeuristic(t *testing.T) {
	b := BuildUnit(sampleUnit(), DefaultCaps(), noRedact)
	est := EstimateTokens(b.Request)
	if est <= len(b.Request.State)/4 {
		t.Errorf("estimate %d must cover state bytes/4 (%d) plus the questions", est, len(b.Request.State)/4)
	}
}

// TestQuestionSetDigestsSurviveTheKindRename pins JF-24: the yes/no kind is
// now decision.KindPredicate (agentic-kit DP-12) but no question the provider
// sees changed, so both question-set digests keep the values they were minted
// with (captured before the rename, 2026-10-01).
func TestQuestionSetDigestsSurviveTheKindRename(t *testing.T) {
	if got := UnitQuestionSetDigest(); got != "7f557f9283695d7f" {
		t.Errorf("UnitQuestionSetDigest() = %q, want the pre-rename 7f557f9283695d7f", got)
	}
	if got := InterventionQuestionSetDigest(); got != "7e52a31018bfd234" {
		t.Errorf("InterventionQuestionSetDigest() = %q, want the pre-rename 7e52a31018bfd234", got)
	}
}
