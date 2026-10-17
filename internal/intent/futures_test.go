package intent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// intentSchemaSource reads schema/intent/schema.cue, the gated source (never
// the embedded copy, so this test and VetFuture are two checks of one shape).
func intentSchemaSource(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "schema", "intent", name))
	if err != nil {
		t.Fatalf("schema/intent/%s unreadable (%v) — an unread schema must not pass as a satisfied one", name, err)
	}
	return src
}

// futureDef compiles the source schema and returns its #Future definition.
func futureDef(t *testing.T, cctx *cue.Context) cue.Value {
	t.Helper()
	v := cctx.CompileBytes(intentSchemaSource(t, "schema.cue"), cue.Filename("schema/intent/schema.cue"))
	if err := v.Err(); err != nil {
		t.Fatalf("schema/intent/schema.cue does not compile: %v", err)
	}
	def := v.LookupPath(cue.ParsePath("#Future"))
	if !def.Exists() {
		t.Fatal("schema/intent/schema.cue has no #Future definition")
	}
	return def
}

// vetAgainstSource vets one parsed future's JSON form against the source
// #Future, concretely: a missing required field fails, as the gate's does.
func vetAgainstSource(cctx *cue.Context, def cue.Value, f intent.Future) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	v := cctx.CompileBytes(data)
	if err := v.Err(); err != nil {
		return err
	}
	return def.Unify(v).Validate(cue.Concrete(true))
}

const futuresFixture = "# Future directions: fixture\n\n" +
	"## F90. Every field\n\n" +
	"- Confidence: plausible (fixture, 2026-10-17). Horizon:\n" +
	"  later. Value: high, it saves a re-proposal a month. Basis: measured, `docs/measurements/m.md`. Evidence: the\n" +
	"  fixture.\n" +
	"- Motivation: a Horizon: inside prose is not a field.\n" +
	"- Promote when: the owner asks {instrument: k1, op: \">=\", value: 1}.\n" +
	"- Drop if: never.\n\n" +
	"## F91. The two required fields\n\n" +
	"- Confidence: **likely**. Horizon: next.\n\n" +
	"## F92. A Horizon off the list\n\n" +
	"- Confidence: speculative. Horizon: later/someday.\n\n" +
	"## F93. No Confidence\n\n" +
	"- Horizon: someday. Evidence: none.\n\n" +
	"## F94. A measured basis citing nothing\n\n" +
	"- Confidence: likely. Horizon: next. Basis: measured, we looked.\n\n" +
	"## F95. A value without a why\n\n" +
	"- Confidence: likely. Horizon: next. Value: very-high.\n"

// TestParseFutures_OutputVetsAgainstSchemaIntent is IV-5 (ADR-074 decision
// 5): ParseFutures reads a future's parsed form — Confidence, Horizon,
// Value with its why, Basis with its cite, and the conditions — and that
// output vets against schema/intent #Future; an off-list value, a missing
// required field, a measured basis citing nothing and a value with no why
// each fail naming the id. VetFuture (the embedded copy `fathom intent
// audit` uses) agrees with the source schema on every case: one shape,
// two checks.
func TestParseFutures_OutputVetsAgainstSchemaIntent(t *testing.T) {
	fs := intent.ParseFutures("docs/intent/futures/fixture.md", []byte(futuresFixture))
	if len(fs) != 6 {
		t.Fatalf("parsed %d futures, want 6: %+v", len(fs), fs)
	}

	full := fs[0]
	want := intent.Future{
		ID: "F90", Title: "Every field", Confidence: "plausible", Horizon: "later",
		Value: &intent.FutureValue{Level: "high", Why: "it saves a re-proposal a month"},
		Basis: &intent.FutureBasis{Kind: "measured", Cite: "`docs/measurements/m.md`"},
		Conditions: []intent.FutureCondition{
			{Label: "Promote when", Text: "the owner asks ."},
			{Label: "Drop if", Text: "never."},
		},
		File: "docs/intent/futures/fixture.md", Line: 3,
	}
	gotJSON, _ := json.Marshal(full)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) || full.File != want.File || full.Line != want.Line {
		t.Errorf("F90 parsed as\n %s (%s:%d)\nwant\n %s (%s:%d)", gotJSON, full.File, full.Line, wantJSON, want.File, want.Line)
	}
	if fs[1].Confidence != "likely" || fs[1].Horizon != "next" || fs[1].Value != nil || fs[1].Basis != nil {
		t.Errorf("F91 parsed as %+v, want likely/next and no value or basis", fs[1])
	}
	if fs[2].Horizon != "later/someday" {
		t.Errorf("F92 Horizon = %q, want the off-list value kept whole so it fails", fs[2].Horizon)
	}

	cctx := cuecontext.New()
	def := futureDef(t, cctx)
	pass := map[string]bool{"F90": true, "F91": true}
	says := map[string]string{
		"F92": `horizon "later/someday" is off the list`,
		"F93": "confidence missing",
		"F94": `basis.cite "we looked" does not match the declared form`,
		"F95": "value.why",
	}
	for _, f := range fs {
		srcErr := vetAgainstSource(cctx, def, f)
		embErr := intent.VetFuture(f)
		if pass[f.ID] {
			if srcErr != nil || embErr != nil {
				t.Errorf("%s: want a clean vet, got source %v / embedded %v", f.ID, srcErr, embErr)
			}
			continue
		}
		if srcErr == nil {
			t.Errorf("%s: vetted clean against schema/intent, want a failure", f.ID)
		}
		if embErr == nil {
			t.Errorf("%s: VetFuture passed, want a failure (the embedded copy disagrees with the source)", f.ID)
		} else if !strings.HasPrefix(embErr.Error(), f.ID+": ") || !strings.Contains(embErr.Error(), says[f.ID]) {
			t.Errorf("%s: VetFuture error %q does not name the id first and say %q", f.ID, embErr, says[f.ID])
		}
	}
}

// TestIntentSchema_Negatives asserts schema/intent/testdata/invalid.cue's
// cases (commented there so live `cue vet` stays green) and that valid.cue
// unifies.
func TestIntentSchema_Negatives(t *testing.T) {
	cctx := cuecontext.New()
	schema := string(intentSchemaSource(t, "schema.cue"))
	valid := strings.Replace(string(intentSchemaSource(t, filepath.Join("testdata", "valid.cue"))), "package intent", "", 1)
	if v := cctx.CompileString(schema + "\n" + valid); v.Err() != nil || v.Validate(cue.Concrete(true)) != nil {
		t.Fatalf("valid.cue does not unify with the schema: %v / %v", v.Err(), v.Validate(cue.Concrete(true)))
	}
	invalid := string(intentSchemaSource(t, filepath.Join("testdata", "invalid.cue")))
	cases := regexp.MustCompile(`(?m)^// (bad\d+: .*)$`).FindAllStringSubmatch(invalid, -1)
	if len(cases) < 7 {
		t.Fatalf("invalid.cue carries %d cases, want 7", len(cases))
	}
	for _, c := range cases {
		v := cctx.CompileString(schema + "\n" + c[1])
		if v.Err() == nil && v.Validate(cue.Concrete(true)) == nil {
			t.Errorf("vetted clean, want a schema failure: %s", c[1])
		}
	}
}

// TestRepoFuturesVetAgainstSchemaIntent is IV-5b's gate (ADR-074 decision
// 5): every future under docs/intent/futures/ vets against schema/intent
// #Future, so an off-list value or a missing Confidence or Horizon fails
// `task validate` naming the id. Baseline 2026-10-17: 8 off-list Horizons
// and 2 futures with neither field before ADR-074's repair, 0 after. Every
// "## F<n>." heading must parse to a future, so a parser that stops seeing
// records cannot pass by seeing none.
func TestRepoFuturesVetAgainstSchemaIntent(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "intent", "futures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("docs/intent/futures unreadable (%v) — an unread future must not pass as a clean one", err)
	}
	cctx := cuecontext.New()
	def := futureDef(t, cctx)
	heading := regexp.MustCompile(`(?m)^##\s+F\d+\.`)
	var bad []string
	parsed, headings := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		rel := "docs/intent/futures/" + name
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s unreadable (%v) — an unread future must not pass as a clean one", rel, err)
		}
		headings += len(heading.FindAll(body, -1))
		for _, f := range intent.ParseFutures(rel, body) {
			parsed++
			if err := vetAgainstSource(cctx, def, f); err != nil {
				bad = append(bad, f.ID+" ("+rel+":"+itoa(f.Line)+"): "+strings.ReplaceAll(err.Error(), "\n", "; "))
			}
		}
	}
	if parsed == 0 || parsed != headings {
		t.Fatalf("parsed %d futures from %d \"## F<n>.\" headings — the parser lost records", parsed, headings)
	}
	if len(bad) > 0 {
		t.Fatalf("%d of %d futures fail schema/intent #Future (set Confidence likely|plausible|speculative and Horizon next|later|someday, docs/intent/futures.md's key):\n%s",
			len(bad), parsed, strings.Join(bad, "\n"))
	}
}

// TestRunAudit_NamesAFutureWhoseFieldsFail is IV-5's product half: Load
// reads every future into the corpus, and `fathom intent audit` names a
// future whose fields fail schema/intent #Future (U16), with the remedy.
func TestRunAudit_NamesAFutureWhoseFieldsFail(t *testing.T) {
	c, err := intent.Load(repoRoot(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Futures) < 80 {
		t.Fatalf("Load read %d futures, want the repository's 80 or more", len(c.Futures))
	}
	for _, u := range intent.RunAudit(intent.AuditInput{Corpus: c}).Unproven {
		if u.Kind == "U16" {
			t.Errorf("the repaired corpus carries a U16: %+v", u)
		}
	}

	c.Futures = append(c.Futures, intent.ParseFutures("docs/intent/futures/fixture.md", []byte(futuresFixture))...)
	got := map[string]intent.Claim{}
	for _, u := range intent.RunAudit(intent.AuditInput{Corpus: c}).Unproven {
		if u.Kind == "U16" {
			got[u.Ref] = u
		}
	}
	for _, id := range []string{"F92", "F93", "F94", "F95"} {
		u, ok := got[id]
		if !ok {
			t.Errorf("audit does not name %s, whose fields fail", id)
			continue
		}
		if u.Remedy == "" || !strings.Contains(u.Evidence, "fixture.md:") {
			t.Errorf("%s: claim lacks a remedy or a file:line: %+v", id, u)
		}
	}
	if _, ok := got["F90"]; ok {
		t.Error("audit names F90, whose fields are sound")
	}
}
