package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func floorTestCase(id, expected, role string, oldIDs []string) floorCase {
	c := floorCase{CaseID: id, DocumentID: "fixture", Assertion: "The fixture has a recorded state.", CitedFactIDs: []string{"f-1"}, Facts: []floorFact{{ID: "f-1", Text: "The fixture has a recorded state.", OccurredAt: "2026-10-26T00:00:00Z", Kind: "observed", Required: true}}, Expected: expected, Role: role, OldCaseIDs: oldIDs, Basis: "Synthetic source declaration for structural validation only.", Reviews: []floorReview{{ReviewerRef: "reader-a", Status: "determinate", Expected: expected, PremiseFactIDs: []string{"f-1"}, CounterFactIDs: []string{}, Basis: "Synthetic source declaration for structural validation only."}, {ReviewerRef: "reader-b", Status: "determinate", Expected: expected, PremiseFactIDs: []string{"f-1"}, CounterFactIDs: []string{}, Basis: "Synthetic source declaration for structural validation only."}}}
	floorTestPin(&c, oracleCriterionID, oracleCriterionPin)
	return c
}
func floorTestPin(c *floorCase, id, pin string) {
	for i := range c.Reviews {
		c.Reviews[i].CriterionID = id
		c.Reviews[i].CriterionSHA256 = pin
		c.Reviews[i].CaseDigest = c.digest(id, pin)
	}
}
func floorTestData() (oraclePacket, map[string]oracleCase, floorScope, map[string]floorCase, floorReceipt) {
	p := oraclePacket{PacketDigest: strings.Repeat("a", 64)}
	old := map[string]oracleCase{}
	future := map[string]floorCase{}
	r := floorReceipt{Version: "prose-lab.oracle-floor-review/1", PacketDigest: p.PacketDigest, CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, AssessmentsDigest: strings.Repeat("b", 64), AssessmentReportDigest: strings.Repeat("c", 64), FutureScopeDigest: strings.Repeat("d", 64), Reviewers: []floorReviewer{{Ref: "reader-a", Kind: "agent", Role: "independent-source-reviewer", Exposure: "Historical labels were withheld for this synthetic structural test.", AssessmentsDigest: strings.Repeat("e", 64), AssessmentReportDigest: strings.Repeat("f", 64)}, {Ref: "reader-b", Kind: "human", Role: "independent-source-reviewer", Exposure: "Historical labels were withheld for this synthetic structural test.", AssessmentsDigest: strings.Repeat("1", 64), AssessmentReportDigest: strings.Repeat("2", 64)}}, OldCaseDispositions: []floorDisposition{}, Contrasts: []floorContrast{}, RemainingAmbiguities: []floorAmbiguity{}, Verdict: "ready-for-new-study-design"}
	for i := 0; i < 70; i++ {
		id := fmt.Sprintf("old-%02d", i)
		if i == 3 {
			id = "operational-runbook--p-03"
		}
		expected := "supported"
		if i == 0 {
			expected = "unsupported"
		}
		c := floorTestCase(id, expected, "ordinary", []string{id})
		future[id] = c
		o := oracleCase{CaseID: id, DocumentID: c.DocumentID, Assertion: c.Assertion, CitedFactIDs: c.CitedFactIDs, HistoricalExpected: expected, ContextFacts: []oracleFact{{ID: "f-1", Text: c.Facts[0].Text, OccurredAt: c.Facts[0].OccurredAt, Kind: "observed", Required: true}}}
		old[id] = o
		r.OldCaseDispositions = append(r.OldCaseDispositions, floorDisposition{CaseID: id, CaseDigest: hashBytes(jsonBytes(o)), Disposition: "retain", ProspectiveIDs: []string{id}, ChangedFields: []string{}, Basis: "Synthetic source declaration for structural validation only.", ReviewerRefs: []string{"reader-a", "reader-b"}})
	}
	for i := 1; i <= 7; i++ {
		a := fmt.Sprintf("contrast-%d-supported", i)
		b := fmt.Sprintf("contrast-%d-unsupported", i)
		future[a] = floorTestCase(a, "supported", "modal-control", []string{})
		future[b] = floorTestCase(b, "unsupported", "modal-control", []string{})
		if i == 7 {
			c := future[b]
			c.Role = "uncertainty-control"
			c.OldCaseIDs = []string{"operational-runbook--p-03"}
			c.Assertion = "The fixture state remains unresolved."
			floorTestPin(&c, oracleCriterionID, oracleCriterionPin)
			future[b] = c
			d := &r.OldCaseDispositions[3]
			d.Disposition = "uncertainty"
			d.ProspectiveIDs = []string{"operational-runbook--p-03", b}
			d.ChangedFields = []string{"assertion", "label"}
		}
		r.Contrasts = append(r.Contrasts, floorContrast{ID: fmt.Sprintf("OQ-C%02d", i), Cases: []floorCaseRef{{a, future[a].digest(oracleCriterionID, oracleCriterionPin)}, {b, future[b].digest(oracleCriterionID, oracleCriterionPin)}}, ReviewerRefs: []string{"reader-a", "reader-b"}})
	}
	ids := []string{}
	for id := range future {
		ids = append(ids, id)
	}
	s := floorScope{Version: "prose-lab.oracle-floor-scope/1", ScopeID: "future", CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin, ExpectedCaseIDs: ids, Fixtures: []floorFixturePin{{ID: "fixture", SHA256: strings.Repeat("3", 64)}}}
	return p, old, s, future, r
}
func floorTestV2(s *floorScope, future map[string]floorCase, r *floorReceipt) {
	s.CriterionID = floorCriterionV2ID
	s.CriterionSHA256 = floorCriterionV2Pin
	for id, c := range future {
		floorTestPin(&c, floorCriterionV2ID, floorCriterionV2Pin)
		future[id] = c
	}
	for i := range r.OldCaseDispositions {
		d := &r.OldCaseDispositions[i]
		if d.Disposition == "retain" {
			d.Disposition = "replace"
		}
		d.ChangedFields = append(d.ChangedFields, "criterion")
	}
	for i := range r.Contrasts {
		for j := range r.Contrasts[i].Cases {
			ref := &r.Contrasts[i].Cases[j]
			ref.CaseDigest = future[ref.CaseID].digest(floorCriterionV2ID, floorCriterionV2Pin)
		}
	}
}
func TestFloorCriterionV2BindingsAndInconclusive(t *testing.T) {
	if got := hashBytes(floorCriterionV2); got != floorCriterionV2Pin {
		t.Fatalf("embedded criterion drift: %s", got)
	}
	for _, tc := range []struct {
		name      string
		mutate    func(*floorScope, map[string]floorCase, *floorReceipt)
		wantReady bool
		wantError bool
	}{
		{"v2-ready", func(*floorScope, map[string]floorCase, *floorReceipt) {}, true, false},
		{"stale-scope-pin", func(s *floorScope, _ map[string]floorCase, _ *floorReceipt) { s.CriterionSHA256 = oracleCriterionPin }, false, true},
		{"stale-review-pin", func(_ *floorScope, f map[string]floorCase, _ *floorReceipt) {
			c := f["old-00"]
			c.Reviews[0].CriterionSHA256 = oracleCriterionPin
			f["old-00"] = c
		}, false, true},
		{"stale-case-pin", func(_ *floorScope, f map[string]floorCase, _ *floorReceipt) {
			c := f["old-00"]
			c.Assertion = "Changed without re-review."
			f["old-00"] = c
		}, false, true},
		{"missing-criterion-disposition", func(_ *floorScope, _ map[string]floorCase, r *floorReceipt) {
			r.OldCaseDispositions[0].ChangedFields = []string{}
		}, false, true},
		{"old-negative-inconclusive", func(_ *floorScope, f map[string]floorCase, _ *floorReceipt) {
			c := f["old-00"]
			c.Expected = "inconclusive"
			for i := range c.Reviews {
				c.Reviews[i].Expected = "inconclusive"
			}
			floorTestPin(&c, floorCriterionV2ID, floorCriterionV2Pin)
			f["old-00"] = c
		}, false, true},
		{"determinate-inconclusive", func(s *floorScope, f map[string]floorCase, _ *floorReceipt) {
			c := floorTestCase("new-inconclusive", "inconclusive", "ordinary", []string{})
			floorTestPin(&c, floorCriterionV2ID, floorCriterionV2Pin)
			f[c.CaseID] = c
			s.ExpectedCaseIDs = append(s.ExpectedCaseIDs, c.CaseID)
		}, true, false},
		{"unresolved-inconclusive-held", func(s *floorScope, f map[string]floorCase, r *floorReceipt) {
			c := floorTestCase("new-inconclusive", "inconclusive", "ordinary", []string{})
			floorTestPin(&c, floorCriterionV2ID, floorCriterionV2Pin)
			c.Reviews[0].Status = "unresolved"
			f[c.CaseID] = c
			s.ExpectedCaseIDs = append(s.ExpectedCaseIDs, c.CaseID)
			r.Verdict = "held"
		}, false, false},
		{"unresolved-inconclusive-ready-refused", func(s *floorScope, f map[string]floorCase, _ *floorReceipt) {
			c := floorTestCase("new-inconclusive", "inconclusive", "ordinary", []string{})
			floorTestPin(&c, floorCriterionV2ID, floorCriterionV2Pin)
			c.Reviews[0].Status = "unresolved"
			f[c.CaseID] = c
			s.ExpectedCaseIDs = append(s.ExpectedCaseIDs, c.CaseID)
		}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, o, s, f, r := floorTestData()
			floorTestV2(&s, f, &r)
			tc.mutate(&s, f, &r)
			ready, e := floorValidate(p, o, s, f, r)
			if (e != nil) != tc.wantError || ready != tc.wantReady {
				t.Fatalf("ready=%v error=%v; want ready=%v error=%v", ready, e, tc.wantReady, tc.wantError)
			}
		})
	}
}

func TestFloorPriorAmbiguousPositive(t *testing.T) {
	for _, source := range []string{"primary", "registered-reviewer"} {
		t.Run(source, func(t *testing.T) {
			p, old, scope, future, receipt := floorTestData()
			prior := map[string]oracleAssessment{"old-01": {Disposition: "ambiguous"}}
			if ready, e := floorValidate(p, old, scope, future, receipt); e != nil || !ready {
				t.Fatalf("counterexample precondition ready=%v err=%v", ready, e)
			}
			sources := []map[string]oracleAssessment{{}, {}}
			if source == "primary" {
				sources[0] = prior
			} else {
				sources[1] = prior
			}
			if e := floorCheckPriorAmbiguity(scope, old, future, receipt, sources...); e == nil {
				t.Fatal("unchanged criterion silently retained an ambiguous old positive")
			}
			c := future["old-01"]
			c.Expected = "unsupported"
			c.Role = "uncertainty-control"
			for i := range c.Reviews {
				c.Reviews[i].Expected = "unsupported"
			}
			floorTestPin(&c, oracleCriterionID, oracleCriterionPin)
			future[c.CaseID] = c
			d := &receipt.OldCaseDispositions[1]
			d.Disposition = "uncertainty"
			d.ChangedFields = []string{"label"}
			if e := floorCheckPriorAmbiguity(scope, old, future, receipt, sources...); e != nil {
				t.Fatalf("uncertainty control rejected: %v", e)
			}
			if ready, e := floorValidate(p, old, scope, future, receipt); e != nil || !ready {
				t.Fatalf("uncertainty floor ready=%v err=%v", ready, e)
			}
			floorTestV2(&scope, future, &receipt)
			d.Disposition = "replace"
			if e := floorCheckPriorAmbiguity(scope, old, future, receipt, sources...); e != nil {
				t.Fatalf("explicit v2 review rejected: %v", e)
			}
			if ready, e := floorValidate(p, old, scope, future, receipt); e != nil || !ready {
				t.Fatalf("v2 floor ready=%v err=%v", ready, e)
			}
		})
	}
}
func TestFloorStructuralRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]oracleCase, map[string]floorCase, *floorReceipt)
	}{
		{"complete", func(_ map[string]oracleCase, _ map[string]floorCase, _ *floorReceipt) {}},
		{"old-negative-omitted", func(_ map[string]oracleCase, _ map[string]floorCase, r *floorReceipt) {
			r.OldCaseDispositions[0] = r.OldCaseDispositions[1]
		}},
		{"old-digest", func(_ map[string]oracleCase, _ map[string]floorCase, r *floorReceipt) {
			r.OldCaseDispositions[0].CaseDigest = strings.Repeat("0", 64)
		}},
		{"missing-contrast-side", func(_ map[string]oracleCase, _ map[string]floorCase, r *floorReceipt) {
			r.Contrasts[0].Cases[1] = r.Contrasts[0].Cases[0]
		}},
		{"forged-reviewer", func(_ map[string]oracleCase, _ map[string]floorCase, r *floorReceipt) {
			r.OldCaseDispositions[0].ReviewerRefs[0] = "stranger"
		}},
		{"uncited-premise", func(_ map[string]oracleCase, f map[string]floorCase, _ *floorReceipt) {
			c := f["old-00"]
			c.Reviews[0].PremiseFactIDs = []string{"f-2"}
			f["old-00"] = c
		}},
		{"disagreement-ready", func(_ map[string]oracleCase, f map[string]floorCase, _ *floorReceipt) {
			c := f["old-00"]
			c.Reviews[0].Status = "unresolved"
			f["old-00"] = c
		}},
		{"criterion-change-claim", func(_ map[string]oracleCase, _ map[string]floorCase, r *floorReceipt) {
			r.OldCaseDispositions[0].Disposition = "replace"
			r.OldCaseDispositions[0].ChangedFields = []string{"criterion"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, o, s, f, r := floorTestData()
			tc.mutate(o, f, &r)
			ready, e := floorValidate(p, o, s, f, r)
			if tc.name == "complete" {
				if e != nil || !ready {
					t.Fatalf("complete ready=%v err=%v", ready, e)
				}
			} else if e == nil {
				t.Fatalf("accepted %s ready=%v", tc.name, ready)
			}
		})
	}
}
func TestFloorHeldAndWireRefusals(t *testing.T) {
	p, o, s, f, r := floorTestData()
	r.Verdict = "held"
	r.RemainingAmbiguities = []floorAmbiguity{{CaseID: "old-00", Basis: "Synthetic unresolved source question."}}
	ready, e := floorValidate(p, o, s, f, r)
	if e != nil || ready {
		t.Fatalf("held ready=%v err=%v", ready, e)
	}
	raw := jsonBytes(r)
	if _, e = floorParseReceipt(raw); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(map[string]any){
		"authority":     func(m map[string]any) { m["provider_qualified"] = true },
		"missing-array": func(m map[string]any) { delete(m, "remaining_ambiguities") },
		"null-array":    func(m map[string]any) { m["remaining_ambiguities"] = nil },
		"unknown-field": func(m map[string]any) { m["secret"] = "unwanted" },
		"overflow":      func(m map[string]any) { m["reviewers"] = make([]any, 9) },
	} {
		t.Run(name, func(t *testing.T) {
			var m map[string]any
			if json.Unmarshal(raw, &m) != nil {
				t.Fatal("decode")
			}
			change(m)
			if _, e := floorParseReceipt(jsonBytes(m)); e == nil {
				t.Fatal("accepted invalid receipt")
			}
		})
	}
	if _, e := floorParseReceipt([]byte(`{"version":"x","version":"y"}`)); e == nil {
		t.Fatal("duplicate keys accepted")
	}
	if _, e := floorParseReceipt(append(raw, 0xff)); e == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	var out, errOut bytes.Buffer
	if code := runReviewFloor(context.Background(), []string{"--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "--future-scope-digest") {
		t.Fatalf("help code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runReviewFloor(context.Background(), []string{}, &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatalf("flags code=%d out=%q", code, out.String())
	}
}
func TestFloorScopeGoCUEParity(t *testing.T) {
	bin, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	_, _, s, _, _ := floorTestData()
	for _, tc := range []struct {
		name   string
		mutate func(*floorScope)
	}{{"valid-v1", func(*floorScope) {}}, {"valid-v2", func(s *floorScope) { s.CriterionID = floorCriterionV2ID; s.CriterionSHA256 = floorCriterionV2Pin }}, {"mismatched-profile", func(s *floorScope) { s.CriterionID = floorCriterionV2ID }}, {"duplicate", func(s *floorScope) { s.ExpectedCaseIDs[1] = s.ExpectedCaseIDs[0] }}, {"criterion", func(s *floorScope) { s.CriterionSHA256 = strings.Repeat("0", 64) }}, {"missing-fixtures", func(s *floorScope) { s.Fixtures = []floorFixturePin{} }}} {
		t.Run(tc.name, func(t *testing.T) {
			x := s
			x.ExpectedCaseIDs = append([]string(nil), s.ExpectedCaseIDs...)
			x.Fixtures = append([]floorFixturePin(nil), s.Fixtures...)
			tc.mutate(&x)
			raw := jsonBytes(x)
			_, goErr := floorParseScope(raw)
			path := filepath.Join(t.TempDir(), "scope.json")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(bin, "vet", "schema.cue", "oracle_floor_schema.cue", path, "-d", "#FloorScope", "-c")
			b, cueErr := cmd.CombinedOutput()
			if (goErr == nil) != (cueErr == nil) {
				t.Fatalf("Go=%v CUE=%v: %s", goErr, cueErr, b)
			}
		})
	}
	raw := jsonBytes(s)
	var m map[string]any
	if e := json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	m["extra"] = "forbidden"
	raw = jsonBytes(m)
	if _, e := floorParseScope(raw); e == nil {
		t.Fatal("Go accepted unknown scope field")
	}
	path := filepath.Join(t.TempDir(), "scope-unknown.json")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if b, e := exec.Command(bin, "vet", "schema.cue", "oracle_floor_schema.cue", path, "-d", "#FloorScope", "-c").CombinedOutput(); e == nil {
		t.Fatalf("CUE accepted unknown scope field: %s", b)
	}
}

func TestFloorReceiptAndFixtureCUEParity(t *testing.T) {
	bin, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	p, old, scopeV1, future, receipt := floorTestData()
	fixture := floorFixture{Version: "prose-lab.oracle-proposed-fixture/1", ID: "fixture", Cases: []floorCase{future["old-00"]}}
	v2Fixture := fixture
	v2Fixture.Cases = append([]floorCase(nil), fixture.Cases...)
	v2Case := v2Fixture.Cases[0]
	v2Case.Reviews = append([]floorReview(nil), v2Case.Reviews...)
	floorTestPin(&v2Case, floorCriterionV2ID, floorCriterionV2Pin)
	v2Fixture.Cases[0] = v2Case
	fixtureMutation := func(base floorFixture, mutate func(*floorCase)) []byte {
		x := base
		x.Cases = append([]floorCase(nil), base.Cases...)
		c := x.Cases[0]
		c.Reviews = append([]floorReview(nil), c.Reviews...)
		mutate(&c)
		x.Cases[0] = c
		return jsonBytes(x)
	}
	unknownField := func(raw []byte, atReview bool) []byte {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			panic("test JSON")
		}
		c := m["cases"].([]any)[0].(map[string]any)
		if atReview {
			c["reviews"].([]any)[0].(map[string]any)["extra"] = "forbidden"
		} else {
			c["extra"] = "forbidden"
		}
		return jsonBytes(m)
	}
	for _, tc := range []struct {
		name string
		def  string
		wire []byte
		goOK bool
	}{
		{"receipt-valid", "#FloorReview", jsonBytes(receipt), true},
		{"receipt-authority", "#FloorReview", func() []byte { x := receipt; x.ProviderQualified = true; return jsonBytes(x) }(), false},
		{"receipt-emoji-exposure", "#FloorReview", func() []byte {
			x := receipt
			x.Reviewers = append([]floorReviewer(nil), receipt.Reviewers...)
			x.Reviewers[0].Exposure = strings.Repeat("🙂", 1500)
			return jsonBytes(x)
		}(), false},
		{"receipt-blank-disposition-basis", "#FloorReview", func() []byte {
			x := receipt
			x.OldCaseDispositions = append([]floorDisposition(nil), receipt.OldCaseDispositions...)
			x.OldCaseDispositions[0].Basis = " \t "
			return jsonBytes(x)
		}(), false},
		{"receipt-blank-ambiguity-basis", "#FloorReview", func() []byte {
			x := receipt
			x.Verdict = "held"
			x.RemainingAmbiguities = []floorAmbiguity{{CaseID: "old-00", Basis: " \t "}}
			return jsonBytes(x)
		}(), false},
		{"fixture-valid", "#FloorFixture", jsonBytes(fixture), true},
		{"fixture-v2-valid", "#FloorFixture", jsonBytes(v2Fixture), true},
		{"fixture-v2-crossed-profile", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) { c.Reviews[0].CriterionSHA256 = oracleCriterionPin }), false},
		{"fixture-v2-unknown-case", "#FloorFixture", unknownField(jsonBytes(v2Fixture), false), false},
		{"fixture-v2-unknown-review", "#FloorFixture", unknownField(jsonBytes(v2Fixture), true), false},
		{"fixture-v2-emoji-byte-overflow", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) {
			c.Assertion = strings.Repeat("🙂", 1500)
			floorTestPin(c, floorCriterionV2ID, floorCriterionV2Pin)
		}), false},
		{"fixture-v2-blank-basis", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) { c.Basis = " \t " }), false},
		{"fixture-v2-emoji-fact", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) {
			c.Facts = append([]floorFact(nil), c.Facts...)
			c.Facts[0].Text = strings.Repeat("🙂", 1500)
			floorTestPin(c, floorCriterionV2ID, floorCriterionV2Pin)
		}), false},
		{"fixture-v2-blank-review-basis", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) { c.Reviews[0].Basis = " \t " }), false},
		{"fixture-v2-supported-no-premise", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) {
			c.Expected = "supported"
			c.Reviews[0].Expected = "supported"
			c.Reviews[0].PremiseFactIDs = []string{}
			c.Reviews[0].CounterFactIDs = []string{"f-1"}
			floorTestPin(c, floorCriterionV2ID, floorCriterionV2Pin)
		}), false},
		{"fixture-v2-definition-no-prefix", "#FloorFixture", fixtureMutation(v2Fixture, func(c *floorCase) {
			c.Facts = append([]floorFact(nil), c.Facts...)
			c.Facts[0].Kind = "definition"
			c.Facts[0].Text = "A fixture rule is defined."
			floorTestPin(c, floorCriterionV2ID, floorCriterionV2Pin)
		}), true},
		{"fixture-v2-stale-review-pin", "#FloorFixture", func() []byte {
			x := v2Fixture
			x.Cases = append([]floorCase(nil), v2Fixture.Cases...)
			c := x.Cases[0]
			c.Reviews = append([]floorReview(nil), c.Reviews...)
			c.Reviews[0].CriterionSHA256 = strings.Repeat("0", 64)
			x.Cases[0] = c
			return jsonBytes(x)
		}(), false},
		{"fixture-uncited", "#FloorFixture", func() []byte {
			x := fixture
			x.Cases = append([]floorCase(nil), fixture.Cases...)
			x.Cases[0].Reviews = append([]floorReview(nil), x.Cases[0].Reviews...)
			x.Cases[0].Reviews[0].PremiseFactIDs = []string{"f-2"}
			return jsonBytes(x)
		}(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if e := os.WriteFile(path, tc.wire, 0600); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(bin, "vet", "schema.cue", "oracle_floor_schema.cue", path, "-d", tc.def, "-c")
			b, cueErr := cmd.CombinedOutput()
			goOK := false
			if tc.def == "#FloorReview" {
				r, e := floorParseReceipt(tc.wire)
				if e == nil {
					_, e = floorValidate(p, old, scopeV1, future, r)
				}
				goOK = e == nil
			} else {
				f, e := floorParseFixture(tc.wire, "fixture")
				scope := floorScope{CriterionID: oracleCriterionID, CriterionSHA256: oracleCriterionPin}
				if strings.HasPrefix(tc.name, "fixture-v2-") {
					scope.CriterionID = floorCriterionV2ID
					scope.CriterionSHA256 = floorCriterionV2Pin
				}
				goOK = e == nil && floorValidateCase(f.Cases[0], scope, old, map[string]floorReviewer{"reader-a": receipt.Reviewers[0], "reader-b": receipt.Reviewers[1]}) == nil
			}
			if goOK != tc.goOK || (cueErr == nil) != goOK {
				t.Fatalf("Go=%v CUE=%v: %s", goOK, cueErr, b)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		raw   []byte
		valid bool
	}{
		{"direct-review-valid-v2", jsonBytes(v2Case.Reviews[0]), true},
		{"direct-review-crossed-pin", func() []byte { r := v2Case.Reviews[0]; r.CriterionSHA256 = oracleCriterionPin; return jsonBytes(r) }(), false},
		{"direct-review-unknown", func() []byte {
			var m map[string]any
			if json.Unmarshal(jsonBytes(v2Case.Reviews[0]), &m) != nil {
				panic("test JSON")
			}
			m["extra"] = "forbidden"
			return jsonBytes(m)
		}(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "review.json")
			if e := os.WriteFile(path, tc.raw, 0600); e != nil {
				t.Fatal(e)
			}
			b, e := exec.Command(bin, "vet", "schema.cue", "oracle_floor_schema.cue", path, "-d", "#FloorCaseReview", "-c").CombinedOutput()
			if (e == nil) != tc.valid {
				t.Fatalf("CUE valid=%v want=%v: %s", e == nil, tc.valid, b)
			}
		})
	}
}
