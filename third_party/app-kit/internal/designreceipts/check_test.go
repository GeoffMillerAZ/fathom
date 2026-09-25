package designreceipts

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

var testGateNames = []string{
	"reproducibility", "structure", "visual", "interaction", "lifecycle",
	"security", "data", "performance", "integration",
}

type testBundle struct {
	contract Contract
	receipt  Receipt
	content  map[string][]byte
}

func testAsset(id, text string) (Asset, []byte) {
	body := []byte(text)
	return Asset{ID: id, SHA256: Digest(body), Bytes: len(body)}, body
}

func newTestBundle() testBundle {
	content := map[string][]byte{}
	add := func(id, text string) Asset {
		a, body := testAsset(id, text)
		content[id] = body
		return a
	}
	source := add("source", "source bytes")
	output := add("output", "output bytes")
	c := Contract{SchemaVersion: 1, Subject: Subject{ID: "synthetic", Profile: "static-web",
		Variant: "candidate", SourceArchive: "source@" + source.SHA256,
		SourceRevision: source.SHA256, ArtifactSHA256: output.SHA256}, Mode: "prospective",
		Inputs: Inputs{Source: []Asset{source}, Transformer: []Asset{add("transformer", "transformer bytes")},
			Config: []Asset{}, Patch: []Asset{}, Runtime: []Asset{}, Output: []Asset{output},
			Fixture:   []Asset{add("fixture", "fixture bytes")},
			Toolchain: []Asset{add("toolchain", "toolchain bytes")},
			Baseline:  []Asset{add("baseline", "baseline bytes")}}, Gates: map[string]Gate{}}
	r := Receipt{SchemaVersion: 1, Results: map[string]Result{}}
	for _, name := range testGateNames {
		evidence := add("evidence-"+name, "evidence for "+name)
		c.Gates[name] = Gate{Applicable: true, Scope: "scope " + name,
			Slots: []Slot{{ID: evidence.ID, Role: "report"}}}
		r.Results[name] = Result{Status: "passed", Evidence: []Evidence{{Asset: evidence, Role: "report"}}}
	}
	return testBundle{contract: c, receipt: r, content: content}
}

func sealTestBundle(t *testing.T, b testBundle) ([]byte, []byte, Expected) {
	t.Helper()
	contractBody, err := EncodeContract(b.contract)
	if err != nil {
		t.Fatalf("encode contract: %v", err)
	}
	b.receipt.ContractSHA256 = Digest(contractBody)
	b.receipt.SourceRevision = b.contract.Subject.SourceRevision
	b.receipt.ArtifactSHA256 = b.contract.Subject.ArtifactSHA256
	receiptBody, err := EncodeReceipt(b.receipt)
	if err != nil {
		t.Fatalf("encode receipt: %v", err)
	}
	return contractBody, receiptBody, Expected{ContractSHA256: Digest(contractBody),
		SourceRevision: b.contract.Subject.SourceRevision, ArtifactSHA256: b.contract.Subject.ArtifactSHA256}
}

func checkTestBundle(t *testing.T, b testBundle) Report {
	t.Helper()
	contractBody, receiptBody, expected := sealTestBundle(t, b)
	report, err := Check(contractBody, receiptBody, expected, b.content)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return report
}

func requireRejected(t *testing.T, contractBody, receiptBody []byte, expected Expected, content map[string][]byte) {
	t.Helper()
	if _, err := Check(contractBody, receiptBody, expected, content); err == nil {
		t.Fatal("invalid receipt bundle accepted")
	}
}

func TestCheckMatchesCompleteContractButNeverVerifiesIt(t *testing.T) {
	b := newTestBundle()
	report := checkTestBundle(t, b)
	if report.Integrity != "matched" || report.Verification != "not-verified" ||
		len(report.ClaimedStatuses) != len(testGateNames) || len(report.IncompleteGates) != 0 {
		t.Fatalf("unexpected all-passed report: %#v", report)
	}

	for _, name := range testGateNames {
		result := b.receipt.Results[name]
		for _, evidence := range result.Evidence {
			delete(b.content, evidence.ID)
		}
		b.receipt.Results[name] = Result{Status: "not-run", Reason: "not run", Evidence: []Evidence{}}
	}
	report = checkTestBundle(t, b)
	if report.Verification != "not-verified" || len(report.IncompleteGates) != len(testGateNames) {
		t.Fatalf("zero-pass report overstated: %#v", report)
	}
}

func TestSchemaRequiresNineGatesAndExplicitInputClosure(t *testing.T) {
	b := newTestBundle()
	delete(b.contract.Gates, "integration")
	if _, err := EncodeContract(b.contract); err == nil {
		t.Fatal("missing gate accepted")
	}
	b = newTestBundle()
	delete(b.receipt.Results, "integration")
	requireReceiptEncodeRejected(t, b)
	b = newTestBundle()
	b.contract.Gates["extra"] = b.contract.Gates["visual"]
	if _, err := EncodeContract(b.contract); err == nil {
		t.Fatal("extra gate accepted")
	}

	for _, group := range []string{"source", "transformer", "output", "fixture", "toolchain", "baseline"} {
		t.Run(group, func(t *testing.T) {
			b := newTestBundle()
			switch group {
			case "source":
				b.contract.Inputs.Source = []Asset{}
			case "transformer":
				b.contract.Inputs.Transformer = []Asset{}
			case "output":
				b.contract.Inputs.Output = []Asset{}
			case "fixture":
				b.contract.Inputs.Fixture = []Asset{}
			case "toolchain":
				b.contract.Inputs.Toolchain = []Asset{}
			case "baseline":
				b.contract.Inputs.Baseline = []Asset{}
			}
			if _, err := EncodeContract(b.contract); err == nil {
				t.Fatal("empty required group accepted")
			}
		})
	}
	for name, mutate := range map[string]func(*testBundle){
		"duplicate input": func(b *testBundle) {
			b.contract.Inputs.Source = append(b.contract.Inputs.Source, b.contract.Inputs.Source[0])
		},
		"duplicate slot": func(b *testBundle) {
			g := b.contract.Gates["visual"]
			g.Slots = append(g.Slots, g.Slots[0])
			b.contract.Gates["visual"] = g
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := newTestBundle()
			mutate(&b)
			contractBody, receiptBody, expected := sealTestBundle(t, b)
			requireRejected(t, contractBody, receiptBody, expected, b.content)
		})
	}
	// The valid fixture proves config, patch and runtime may be explicitly empty.
	checkTestBundle(t, newTestBundle())
}

func requireReceiptEncodeRejected(t *testing.T, b testBundle) {
	t.Helper()
	contractBody, err := EncodeContract(b.contract)
	if err != nil {
		t.Fatalf("encode contract: %v", err)
	}
	b.receipt.ContractSHA256 = Digest(contractBody)
	b.receipt.SourceRevision = b.contract.Subject.SourceRevision
	b.receipt.ArtifactSHA256 = b.contract.Subject.ArtifactSHA256
	if _, err := EncodeReceipt(b.receipt); err == nil {
		t.Fatal("invalid receipt encoded")
	}
}

func TestSchemaAndCanonicalJSONRejectInvalidDocuments(t *testing.T) {
	for name, mutate := range map[string]func(*Contract){
		"bad ID":   func(c *Contract) { c.Subject.ID = "Not-ASCII" },
		"bad mode": func(c *Contract) { c.Mode = "certified" },
		"empty scope": func(c *Contract) {
			g := c.Gates["visual"]
			g.Scope = ""
			c.Gates["visual"] = g
		},
		"scope too long": func(c *Contract) {
			g := c.Gates["visual"]
			g.Scope = strings.Repeat("x", 2049)
			c.Gates["visual"] = g
		},
		"asset bytes too large": func(c *Contract) { c.Inputs.Source[0].Bytes = 33554433 },
	} {
		t.Run(name, func(t *testing.T) {
			b := newTestBundle()
			mutate(&b.contract)
			if _, err := EncodeContract(b.contract); err == nil {
				t.Fatal("CUE-invalid contract encoded")
			}
		})
	}
	b := newTestBundle()
	for i := 0; i < 65; i++ {
		a, body := testAsset("config-"+strings.Repeat("x", i+1), "")
		b.contract.Inputs.Config = append(b.contract.Inputs.Config, a)
		b.content[a.ID] = body
	}
	if _, err := EncodeContract(b.contract); err == nil {
		t.Fatal("input group over item bound encoded")
	}
	b = newTestBundle()
	g := b.contract.Gates["visual"]
	for i := 0; i < 64; i++ {
		g.Slots = append(g.Slots, Slot{ID: "slot-" + strings.Repeat("x", i+1), Role: "report"})
	}
	b.contract.Gates["visual"] = g
	if _, err := EncodeContract(b.contract); err == nil {
		t.Fatal("gate over slot bound encoded")
	}

	b = newTestBundle()
	contractBody, receiptBody, expected := sealTestBundle(t, b)
	mutations := map[string][]byte{
		"malformed":     contractBody[:len(contractBody)-1],
		"case alias":    bytes.Replace(contractBody, []byte(`"schema_version"`), []byte(`"SCHEMA_VERSION"`), 1),
		"duplicate key": bytes.Replace(contractBody, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"lossy unicode": bytes.Replace(contractBody, []byte(`"scope":"scope visual"`), []byte(`"scope":"\ud800"`), 1),
	}
	for name, body := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := decode(body, &Contract{}, "#Contract"); err == nil {
				t.Fatal("noncanonical contract decoded")
			}
			stale := expected
			stale.ContractSHA256 = Digest(body)
			requireRejected(t, body, receiptBody, stale, b.content)
		})
	}
	reordered := newTestBundle().contract
	reordered.Gates = map[string]Gate{}
	for i := len(testGateNames) - 1; i >= 0; i-- {
		reordered.Gates[testGateNames[i]] = b.contract.Gates[testGateNames[i]]
	}
	reorderedBody, err := EncodeContract(reordered)
	if err != nil || !bytes.Equal(contractBody, reorderedBody) {
		t.Fatalf("map insertion order changed canonical JSON: %v", err)
	}
	for _, status := range []string{"failed", "not-run", "unknown"} {
		b := newTestBundle()
		r := b.receipt.Results["visual"]
		r.Status, r.Reason, r.Evidence = status, "", []Evidence{}
		b.receipt.Results["visual"] = r
		requireReceiptEncodeRejected(t, b)
	}
}

func TestCheckRejectsStaleIndependentPinsAndInventories(t *testing.T) {
	b := newTestBundle()
	contractBody, receiptBody, expected := sealTestBundle(t, b)
	stale := Digest([]byte("stale"))
	for name, mutate := range map[string]func(*Expected){
		"contract": func(e *Expected) { e.ContractSHA256 = stale },
		"source":   func(e *Expected) { e.SourceRevision = stale },
		"artifact": func(e *Expected) { e.ArtifactSHA256 = stale },
	} {
		t.Run(name, func(t *testing.T) {
			e := expected
			mutate(&e)
			requireRejected(t, contractBody, receiptBody, e, b.content)
		})
	}
	for name, mutate := range map[string]func(*Receipt){
		"receipt contract": func(r *Receipt) { r.ContractSHA256 = stale },
		"receipt source":   func(r *Receipt) { r.SourceRevision = stale },
		"receipt artifact": func(r *Receipt) { r.ArtifactSHA256 = stale },
	} {
		t.Run(name, func(t *testing.T) {
			copy := b.receipt
			copy.ContractSHA256, copy.SourceRevision, copy.ArtifactSHA256 = expected.ContractSHA256, expected.SourceRevision, expected.ArtifactSHA256
			mutate(&copy)
			body, err := EncodeReceipt(copy)
			if err != nil {
				t.Fatal(err)
			}
			requireRejected(t, contractBody, body, expected, b.content)
		})
	}
	for _, role := range []string{"source", "output"} {
		b = newTestBundle()
		if role == "source" {
			b.contract.Inputs.Source[0].SHA256 = stale
		} else {
			b.contract.Inputs.Output[0].SHA256 = stale
		}
		contractBody, receiptBody, expected = sealTestBundle(t, b)
		requireRejected(t, contractBody, receiptBody, expected, b.content)
	}
}

func TestCheckRejectsEvidenceAndContentSubstitution(t *testing.T) {
	for name, mutate := range map[string]func(*testBundle){
		"wrong role": func(b *testBundle) {
			r := b.receipt.Results["visual"]
			r.Evidence[0].Role = "image"
			b.receipt.Results["visual"] = r
		},
		"pass missing slot": func(b *testBundle) {
			g := b.contract.Gates["visual"]
			g.Slots = append(g.Slots, Slot{ID: "second-slot", Role: "review"})
			b.contract.Gates["visual"] = g
		},
		"duplicate evidence": func(b *testBundle) {
			r := b.receipt.Results["visual"]
			r.Evidence = append(r.Evidence, r.Evidence[0])
			b.receipt.Results["visual"] = r
		},
		"slot shadows input": func(b *testBundle) {
			g := b.contract.Gates["visual"]
			g.Slots[0].ID = b.contract.Inputs.Source[0].ID
			b.contract.Gates["visual"] = g
		},
		"missing content":  func(b *testBundle) { delete(b.content, "source") },
		"tampered content": func(b *testBundle) { b.content["source"] = []byte(strings.Repeat("x", len(b.content["source"]))) },
		"extra content":    func(b *testBundle) { b.content["extra"] = []byte("extra") },
	} {
		t.Run(name, func(t *testing.T) {
			b := newTestBundle()
			mutate(&b)
			contractBody, receiptBody, expected := sealTestBundle(t, b)
			requireRejected(t, contractBody, receiptBody, expected, b.content)
		})
	}
}

func TestNotApplicableMustMatchPolicy(t *testing.T) {
	notApplicable := func(b *testBundle) {
		g := b.contract.Gates["performance"]
		for _, slot := range g.Slots {
			delete(b.content, slot.ID)
		}
		g.Applicable, g.Reason, g.ReviewAssertion, g.Slots = false, "not applicable here", "reviewed exclusion", []Slot{}
		b.contract.Gates["performance"] = g
		b.receipt.Results["performance"] = Result{Status: "not-applicable", Reason: g.Reason, Evidence: []Evidence{}}
	}
	b := newTestBundle()
	notApplicable(&b)
	checkTestBundle(t, b)
	for _, field := range []string{"reason", "review"} {
		b := newTestBundle()
		notApplicable(&b)
		g := b.contract.Gates["performance"]
		if field == "reason" {
			g.Reason = ""
		} else {
			g.ReviewAssertion = ""
		}
		b.contract.Gates["performance"] = g
		if _, err := EncodeContract(b.contract); err == nil {
			t.Fatalf("empty N/A %s accepted", field)
		}
	}
	b = newTestBundle()
	notApplicable(&b)
	r := b.receipt.Results["performance"]
	r.Reason = "different"
	b.receipt.Results["performance"] = r
	contractBody, receiptBody, expected := sealTestBundle(t, b)
	requireRejected(t, contractBody, receiptBody, expected, b.content)
	b = newTestBundle()
	notApplicable(&b)
	r = b.receipt.Results["performance"]
	r.Status, r.Reason = "failed", "failed instead"
	b.receipt.Results["performance"] = r
	contractBody, receiptBody, expected = sealTestBundle(t, b)
	requireRejected(t, contractBody, receiptBody, expected, b.content)
}

func TestSharedEvidenceRequiresMatchingPredeclaration(t *testing.T) {
	share := func(b *testBundle) Evidence {
		e := b.receipt.Results["structure"].Evidence[0]
		old := b.receipt.Results["visual"].Evidence[0]
		delete(b.content, old.ID)
		b.contract.Gates["visual"] = Gate{Applicable: true, Scope: "scope visual", Slots: []Slot{{ID: e.ID, Role: e.Role}}}
		b.receipt.Results["visual"] = Result{Status: "passed", Evidence: []Evidence{e}}
		return e
	}
	b := newTestBundle()
	share(&b)
	checkTestBundle(t, b)
	for name, mutate := range map[string]func(*testBundle, Evidence){
		"unlisted ID": func(b *testBundle, e Evidence) {
			g := b.contract.Gates["visual"]
			g.Slots[0].ID = "other-slot"
			b.contract.Gates["visual"] = g
		},
		"wrong shared role": func(b *testBundle, e Evidence) {
			g := b.contract.Gates["visual"]
			g.Slots[0].Role = "image"
			b.contract.Gates["visual"] = g
			r := b.receipt.Results["visual"]
			r.Evidence[0].Role = "image"
			b.receipt.Results["visual"] = r
		},
		"wrong hash": func(b *testBundle, e Evidence) {
			r := b.receipt.Results["visual"]
			r.Evidence[0].SHA256 = Digest([]byte("other"))
			b.receipt.Results["visual"] = r
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := newTestBundle()
			e := share(&b)
			mutate(&b, e)
			contractBody, receiptBody, expected := sealTestBundle(t, b)
			requireRejected(t, contractBody, receiptBody, expected, b.content)
		})
	}
	b = newTestBundle()
	contractBody, receiptBody, expected := sealTestBundle(t, b)
	expected.ContractSHA256 = Digest([]byte("wrong"))
	report, err := Check(contractBody, receiptBody, expected, b.content)
	if err == nil || !reflect.DeepEqual(report, Report{}) {
		t.Fatalf("error returned nonzero report: %#v, %v", report, err)
	}
}
