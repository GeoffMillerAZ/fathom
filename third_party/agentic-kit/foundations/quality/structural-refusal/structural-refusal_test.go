package structuralrefusal_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	structuralrefusal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/structural-refusal"
)

var (
	cited    = structuralrefusal.Evidence{Kind: "record", ID: "acme-r03", Snapshot: "sha256:9f2c"}
	noID     = structuralrefusal.Evidence{Kind: "record", ID: "  "}
	noKind   = structuralrefusal.Evidence{Kind: "", ID: "acme-r03"}
	emptyRef = structuralrefusal.Evidence{}
)

func TestVerdictVocabulary(t *testing.T) {
	for _, v := range structuralrefusal.Verdicts {
		if !v.Valid() {
			t.Errorf("%q is in Verdicts but not Valid", v)
		}
	}
	for _, v := range []structuralrefusal.Verdict{"", "MEETS", "meets ", "maybe"} {
		if v.Valid() {
			t.Errorf("%q validated — two spellings would be two buckets", v)
		}
	}
	if !cited.Citable() || noID.Citable() || noKind.Citable() || emptyRef.Citable() {
		t.Error("Citable disagrees with 'a non-blank kind and id'")
	}
}

// TestRefuse is the rule in the value, case by case.
func TestRefuse(t *testing.T) {
	E := func(es ...structuralrefusal.Evidence) []structuralrefusal.Evidence { return es }
	tests := []struct {
		name     string
		v        structuralrefusal.Verdict
		evidence []structuralrefusal.Evidence
		want     structuralrefusal.Verdict
	}{
		{"meets with evidence stands", structuralrefusal.VerdictMeets, E(cited), structuralrefusal.VerdictMeets},
		{"meets with nothing is unknown", structuralrefusal.VerdictMeets, nil, structuralrefusal.VerdictUnknown},
		{"meets with an empty list is unknown", structuralrefusal.VerdictMeets, E(), structuralrefusal.VerdictUnknown},
		{"meets with only uncitable refs is unknown", structuralrefusal.VerdictMeets, E(noID, noKind, emptyRef), structuralrefusal.VerdictUnknown},
		{"meets with one citable among junk stands", structuralrefusal.VerdictMeets, E(emptyRef, cited), structuralrefusal.VerdictMeets},
		{"does-not-meet needs no evidence", structuralrefusal.VerdictDoesNotMeet, nil, structuralrefusal.VerdictDoesNotMeet},
		{"unknown is never promoted", structuralrefusal.VerdictUnknown, E(cited), structuralrefusal.VerdictUnknown},
		{"an invented verdict is unknown", structuralrefusal.Verdict("MEETS"), E(cited), structuralrefusal.VerdictUnknown},
		{"an empty verdict is unknown", structuralrefusal.Verdict(""), E(cited), structuralrefusal.VerdictUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := structuralrefusal.Refuse(tt.v, tt.evidence); got != tt.want {
				t.Fatalf("Refuse(%q, %d refs) = %q, want %q", tt.v, len(tt.evidence), got, tt.want)
			}
		})
	}
}

// TestNormalizeRefusesInTheValue: the verdict itself changes, and the reason
// travels with it.
func TestNormalizeRefusesInTheValue(t *testing.T) {
	j := structuralrefusal.New(" led a team ", structuralrefusal.VerdictMeets, nil, " ")
	if j.Verdict != structuralrefusal.VerdictUnknown {
		t.Fatalf("Verdict = %q, want unknown", j.Verdict)
	}
	if j.Refused != `refused "meets": no citable evidence` {
		t.Errorf("Refused = %q", j.Refused)
	}
	if j.Subject != "led a team" || j.Citation != "" {
		t.Errorf("Normalize did not trim: %+v", j)
	}
}

func TestNormalizeDiscardsUncitableBeforeCounting(t *testing.T) {
	j := structuralrefusal.Normalize(structuralrefusal.Judgment{
		Subject:  "x",
		Verdict:  structuralrefusal.VerdictMeets,
		Evidence: []structuralrefusal.Evidence{emptyRef, noID, noKind},
	})
	if j.Verdict != structuralrefusal.VerdictUnknown {
		t.Fatalf("evidence: [{}] satisfied 'has evidence' — Verdict = %q", j.Verdict)
	}
	if !strings.HasSuffix(j.Refused, "(3 unresolvable reference(s) discarded)") {
		t.Errorf("Refused = %q, want the discard named", j.Refused)
	}
	if j.Evidence != nil {
		t.Errorf("Evidence = %v, want nil after discarding everything", j.Evidence)
	}
}

func TestNormalizeKeepsCitableAndTrims(t *testing.T) {
	j := structuralrefusal.Normalize(structuralrefusal.Judgment{
		Subject:  "x",
		Verdict:  structuralrefusal.VerdictMeets,
		Evidence: []structuralrefusal.Evidence{{Kind: " record ", ID: " acme-r03 ", Snapshot: " s "}, emptyRef},
		Refused:  "stale note from an earlier refusal",
	})
	if j.Verdict != structuralrefusal.VerdictMeets {
		t.Fatalf("an evidenced meets was refused: %+v", j)
	}
	want := []structuralrefusal.Evidence{{Kind: "record", ID: "acme-r03", Snapshot: "s"}}
	if !reflect.DeepEqual(j.Evidence, want) {
		t.Errorf("Evidence = %v, want %v", j.Evidence, want)
	}
	if j.Refused != "" {
		t.Errorf("a standing verdict carried a refusal note: %q", j.Refused)
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	once := structuralrefusal.New("x", structuralrefusal.VerdictMeets, []structuralrefusal.Evidence{emptyRef}, "")
	twice := structuralrefusal.Normalize(once)
	if !reflect.DeepEqual(once, twice) {
		t.Fatalf("Normalize is not idempotent:\n%+v\n%+v", once, twice)
	}
	if twice.Refused == "" {
		t.Error("the refusal reason was erased on the second pass")
	}
}

func TestNormalizeInvalidVerdictNamesTheVocabulary(t *testing.T) {
	j := structuralrefusal.New("x", "MEETS", []structuralrefusal.Evidence{cited}, "")
	if j.Verdict != structuralrefusal.VerdictUnknown {
		t.Fatalf("Verdict = %q, want unknown", j.Verdict)
	}
	if j.Refused != `refused verdict "MEETS": not one of meets | does-not-meet | unknown` {
		t.Errorf("Refused = %q", j.Refused)
	}
}

func TestDoesNotMeetNeedsNoEvidence(t *testing.T) {
	j := structuralrefusal.New("x", structuralrefusal.VerdictDoesNotMeet, nil, "")
	if j.Verdict != structuralrefusal.VerdictDoesNotMeet || j.Refused != "" {
		t.Errorf("an absence was refused: %+v", j)
	}
}

// TestUnknownRateNormalizesOnRead: a judgment that reached the store as a
// struct literal — the loader's path — is counted by the same rule as one
// built through New.
func TestUnknownRateNormalizesOnRead(t *testing.T) {
	js := []structuralrefusal.Judgment{
		{Subject: "a", Verdict: structuralrefusal.VerdictMeets},                                                // never normalized: counts as unknown
		{Subject: "b", Verdict: structuralrefusal.VerdictMeets, Evidence: []structuralrefusal.Evidence{cited}}, // stands
		{Subject: "c", Verdict: structuralrefusal.VerdictDoesNotMeet},                                          // stands
		{Subject: "d", Verdict: "maybe"},                                                                       // unknown
	}
	r := structuralrefusal.UnknownRate(js)
	if r.Unknown != 2 || r.Total != 4 {
		t.Fatalf("UnknownRate = %+v, want 2 of 4", r)
	}
	if f, ok := r.Fraction(); !ok || f != 0.5 {
		t.Errorf("Fraction() = %v, %v", f, ok)
	}
	if js[0].Verdict != structuralrefusal.VerdictMeets {
		t.Error("UnknownRate mutated its input")
	}
}

func TestRateRenderRespectsTheFloor(t *testing.T) {
	cases := []struct {
		r     structuralrefusal.Rate
		floor int
		want  string
	}{
		{structuralrefusal.Rate{}, 10, "no judgments"},
		{structuralrefusal.Rate{Unknown: 1, Total: 3}, 10, "not enough judgments yet (3, floor 10)"},
		{structuralrefusal.Rate{Unknown: 1, Total: 3}, 3, "1 of 3 unknown (33%)"},
		{structuralrefusal.Rate{Unknown: 0, Total: 12}, 10, "0 of 12 unknown (0%)"},
	}
	for _, c := range cases {
		if got := c.r.Render(c.floor); got != c.want {
			t.Errorf("%+v.Render(%d) = %q, want %q", c.r, c.floor, got, c.want)
		}
	}
	if _, ok := (structuralrefusal.Rate{}).Fraction(); ok {
		t.Error("Fraction over nothing reported a value")
	}
}

// TestJSONMirrorsSchema pins the wire shape: a refused judgment carries
// refused and no evidence key; an evidenced one carries evidence and no
// refused key.
func TestJSONMirrorsSchema(t *testing.T) {
	keys := func(j structuralrefusal.Judgment) map[string]any {
		b, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	refused := keys(structuralrefusal.New("x", structuralrefusal.VerdictMeets, []structuralrefusal.Evidence{emptyRef}, ""))
	if _, has := refused["evidence"]; has {
		t.Errorf("a refused judgment serialised an empty evidence list: %v", refused)
	}
	if refused["refused"] == nil || refused["verdict"] != "unknown" {
		t.Errorf("refused judgment = %v", refused)
	}
	stands := keys(structuralrefusal.New("x", structuralrefusal.VerdictMeets, []structuralrefusal.Evidence{cited}, "span"))
	if _, has := stands["refused"]; has {
		t.Errorf("a standing judgment serialised a refusal: %v", stands)
	}
	if stands["evidence"] == nil || stands["citation"] != "span" {
		t.Errorf("standing judgment = %v", stands)
	}
}
