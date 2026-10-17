package ask

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestReadAgreementIsReadWithTheLabelAsTruth pins E2's reading to E1's:
// on predicate items, agreement by band, the baseline's agreement, the lift
// and the bar read exactly what Read reads with the label as the truth.
func TestReadAgreementIsReadWithTheLabelAsTruth(t *testing.T) {
	type labelled struct {
		p            float64
		label, major bool
	}
	in := []labelled{
		{0.95, true, true}, {0.97, true, true}, {0.92, false, true}, {0.99, true, true},
		{0.05, false, true}, {0.3, true, true}, {0.6, false, true}, {0.91, true, true},
		{0.93, true, true}, {0.94, true, true}, {0.96, true, true}, {0.98, true, true},
	}
	var items []Item
	var agree []AgreementItem
	for _, it := range in {
		items = append(items, Item{P: it.p, Truth: it.label, Baseline: it.major})
		agree = append(agree, AgreementItem{P: it.p, Agrees: (it.p >= 0.5) == it.label, BaselineAgrees: it.major == it.label})
	}
	r, a := Read(items), ReadAgreement(agree)
	if a.N != r.N || a.Agreement != r.Accuracy || a.BaselineAgreement != r.BaselineAccuracy || a.Lift != r.Lift ||
		a.Qualifies != r.Qualifies || a.QualifyingBand != r.QualifyingBand {
		t.Fatalf("agreement %+v differs from Read %+v", a, r)
	}
	for i := range Bands {
		rb, ab := r.Bands[i], a.Bands[i]
		if rb.Band != ab.Band || rb.N != ab.N || rb.Share != ab.Share || rb.Accuracy != ab.Agreement ||
			rb.WilsonLow != ab.WilsonLow || rb.WilsonHigh != ab.WilsonHigh || rb.BaselineAccuracy != ab.BaselineAgreement || rb.Lift != ab.Lift {
			t.Errorf("band %s: agreement %+v, Read %+v", rb.Band, ab, rb)
		}
	}
}

// TestReadAgreementBarNeedsAMajorityBandThatBeatsTheBaseline: a choice's
// band is its chosen option's probability, and the bar is EXP-M1's.
func TestReadAgreementBarNeedsAMajorityBandThatBeatsTheBaseline(t *testing.T) {
	var items []AgreementItem
	for i := 0; i < 10; i++ {
		items = append(items, AgreementItem{P: 0.95, Agrees: true, BaselineAgrees: i < 5})
	}
	items = append(items, AgreementItem{P: 0.4, Agrees: false, BaselineAgrees: true})
	r := ReadAgreement(items)
	if !r.Qualifies || r.QualifyingBand != "0.9 and above" {
		t.Fatalf("10 of 11 agreeing at 0.95 over a 0.5 baseline should qualify, got %+v", r)
	}
	for i := range items[:10] {
		items[i].BaselineAgrees = true
	}
	if r := ReadAgreement(items); r.Qualifies {
		t.Errorf("a band that only ties the baseline must not qualify: %+v", r)
	}
	if r := ReadAgreement(nil); r.N != 0 || r.Qualifies || len(r.Bands) != len(Bands) {
		t.Errorf("no items: %+v", r)
	}
}

func TestMajorityBreaksTiesByTheOptionOrder(t *testing.T) {
	order := []string{"code_bug", "environment", "timing", "outdated_expectation", "other"}
	cases := []struct {
		labels []string
		want   string
	}{
		{[]string{"timing", "code_bug", "timing"}, "timing"},
		{[]string{"timing", "code_bug"}, "code_bug"},
		{[]string{"zeta", "alpha"}, "alpha"},
		{[]string{"zeta", "other"}, "other"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := Majority(c.labels, order); got != c.want {
			t.Errorf("Majority(%v) = %q, want %q", c.labels, got, c.want)
		}
	}
	if got := Majority([]string{"false", "true"}, []string{"true", "false"}); got != "true" {
		t.Errorf("a predicate tie goes to true first, got %q", got)
	}
}

// TestStreamDrawsSamplesOrderAndGoesOn: a Stream named like a template
// shuffles a sorted pool into exactly Sample's order, and the generator
// goes on from there for a later draw from the same stream.
func TestStreamDrawsSamplesOrderAndGoesOn(t *testing.T) {
	tmpl, _ := TemplateByID("wrap")
	pool := poolOf("a", 23)
	got := Sample(tmpl, pool, []Quota{{Repo: "a", N: len(pool)}}, SampleSeed)
	s := NewStream(SampleSeed, "wrap", "a")
	mine := append([]Computed(nil), pool...)
	s.Shuffle(len(mine), func(i, j int) { mine[i], mine[j] = mine[j], mine[i] })
	if !reflect.DeepEqual(keys(mine), keys(got)) {
		t.Fatalf("Stream's shuffle differs from Sample's draw:\n%v\n%v", keys(mine), keys(got))
	}
	g := splitmix64(streamSeed(SampleSeed, "wrap", "a"))
	for i := 0; i < len(pool)-1; i++ {
		g.next()
	}
	if want, next := g.next(), s.Next(); next != want {
		t.Errorf("after the shuffle the stream reads %#x, want the generator's next %#x", next, want)
	}
	if n := NewStream(SampleSeed, "wrap", "a").Intn(7); n < 0 || n >= 7 {
		t.Errorf("Intn(7) = %d", n)
	}
	if NewStream(SampleSeed, "e2-caller", "fathom").Next() == NewStream(SampleSeed, "e2-failure", "fathom").Next() {
		t.Error("two named streams must not share a generator")
	}
}

// TestFuncSourceFramesLikeCompute: parsing alone finds the declaration
// Compute finds and frames it identically, func keyword to closing brace
// with no doc comment, on every hand-labelled fixture.
func TestFuncSourceFramesLikeCompute(t *testing.T) {
	dir := filepath.Join("testdata", "facts")
	fixtures := fixtureSpans(t, dir)
	spans := make([]Span, len(fixtures))
	for i, f := range fixtures {
		spans[i] = f.span
	}
	computed, err := Compute(context.Background(), dir, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range spans {
		src, err := os.ReadFile(filepath.Join(dir, s.File))
		if err != nil {
			t.Fatal(err)
		}
		line, text, ok := FuncSource(s.File, src, s)
		if !ok {
			t.Errorf("%s %s: FuncSource found nothing", s.File, s.Name)
			continue
		}
		if c := computed[i]; c.Excluded == "" && (line != c.Line || text != c.Source) {
			t.Errorf("%s %s: FuncSource line %d %q, Compute line %d %q", s.File, s.Name, line, text, c.Line, c.Source)
		}
	}
	if _, _, ok := FuncSource("x.go", []byte("package x\nfunc F() {}\n"), Span{Name: "G", Start: 2}); ok {
		t.Error("a name that matches nothing must not be found")
	}
	if _, _, ok := FuncSource("x.go", []byte("package x\nfunc F( {\n"), Span{Start: 2}); ok {
		t.Error("a file that does not parse must not be found")
	}
}
