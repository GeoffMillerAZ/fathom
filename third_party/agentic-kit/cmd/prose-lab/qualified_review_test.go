package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func qualifiedTestCase() floorCase {
	return floorCase{CaseID: "internal-only-control-id", Assertion: "A proposed rule is already law", CitedFactIDs: []string{"f-one"}, Facts: []floorFact{{ID: "f-one", Text: "The rule is proposed.", OccurredAt: "2026-01-01T00:00:00Z", Kind: "proposed", Required: true}, {ID: "f-two", Text: "The law is not enacted.", OccurredAt: "2026-01-02T00:00:00Z", Kind: "observed", Required: false}}, Expected: "unsupported", Role: "ordinary", Basis: "private reviewer basis"}
}
func qualifiedTestLane() lane {
	return lane{Provider: "anthropic", Model: "model-x", MaxInputTokens: 100000, MaxOutputTokens: 8192, InputRate: 1, OutputRate: 1, ReserveInputRate: 1, ReserveOutputRate: 1}
}
func TestQualifiedWireExcludesControlAndPreservesTypedFacts(t *testing.T) {
	c := qualifiedTestCase()
	w, e := qualifiedWire(context.Background(), c, qualifiedTestLane())
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{c.CaseID, `"expected"`, c.Role, c.Basis, "old_case_ids", "reviews", "reviewer_ref", "document_id"} {
		if bytes.Contains(w, []byte(secret)) {
			t.Errorf("provider wire leaked %s", secret)
		}
	}
	for _, required := range []string{"cited-entailment/2", "proposed", "occurred_at", "required", "f-two", "not enacted"} {
		if !bytes.Contains(w, []byte(required)) {
			t.Errorf("provider wire omitted %s", required)
		}
	}
}
func TestQualifiedDecisionStrictOriginalAndBasis(t *testing.T) {
	c := qualifiedTestCase()
	good := `{"status":"unsupported","reason":"proposal cannot establish law","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`
	if _, e := qualifiedDecode(good, c); e != nil {
		t.Fatal(e)
	}
	bad := []string{
		`{"status":"unsupported","reason":"x","premise_fact_ids":[]}`,
		`{"status":"unsupported","reason":"x","premise_fact_ids":null,"counter_fact_ids":["f-two"]}`,
		`{"status":"unsupported","status":"supported","reason":"x","premise_fact_ids":["f-one"],"counter_fact_ids":[]}`,
		`{"status":"supported","reason":"x","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`,
		`{"status":"supported","reason":"x","premise_fact_ids":["f-two"],"counter_fact_ids":[]}`,
		`{"status":"unsupported","reason":"x","premise_fact_ids":[],"counter_fact_ids":["f-two","f-two"]}`,
		`{"status":"unsupported","reason":"x","premise_fact_ids":[],"counter_fact_ids":[]}`,
		`{"status":"unsupported","reason":"x","premise_fact_ids":[],"counter_fact_ids":["f-two"],"extra":1}`,
		`{"status":"maybe","reason":"x","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`,
		`{"status":"inconclusive","reason":" ","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`,
	}
	for _, raw := range bad {
		if _, e := qualifiedDecode(raw, c); e == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if _, e := qualifiedDecode(good+string([]byte{0xff}), c); e == nil {
		t.Error("accepted invalid UTF-8")
	}
	composite := `{"status":"unsupported","reason":"composite premise and conflict","premise_fact_ids":["f-one"],"counter_fact_ids":["f-one"]}`
	if _, e := qualifiedDecode(composite, c); e != nil {
		t.Fatalf("rejected source-bound composite overlap: %v", e)
	}
}
func TestQualifiedDecisionCUEParity(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"composite", `{"status":"unsupported","reason":"composite","premise_fact_ids":["f-one"],"counter_fact_ids":["f-one"]}`, true},
		{"blank", `{"status":"unsupported","reason":" ","premise_fact_ids":[],"counter_fact_ids":["f-one"]}`, false},
		{"empty-basis", `{"status":"unsupported","reason":"basis","premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"long-reason", `{"status":"unsupported","reason":"` + strings.Repeat("x", 4097) + `","premise_fact_ids":[],"counter_fact_ids":["f-one"]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "decision.json")
			if e := os.WriteFile(p, []byte(tc.body), 0600); e != nil {
				t.Fatal(e)
			}
			b, e := exec.Command("cue", "vet", "schema.cue", "qualified_review_schema.cue", p, "-d", "#QualifiedReviewDecision").CombinedOutput()
			if (e == nil) != tc.valid {
				t.Fatalf("CUE accepted=%v want=%v: %s", e == nil, tc.valid, b)
			}
			_, goErr := qualifiedDecode(tc.body, qualifiedTestCase())
			if (goErr == nil) != tc.valid {
				t.Fatalf("Go accepted=%v want=%v: %v", goErr == nil, tc.valid, goErr)
			}
		})
	}
}
func TestQualifiedDecisionV4CUEParity(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"citation-gap", `{"status":"unsupported","reason":"cited fact does not establish event","examined_fact_ids":["f-one","f-two"],"premise_fact_ids":[],"counter_fact_ids":[]}`, true},
		{"supported", `{"status":"supported","reason":"cited support","examined_fact_ids":["f-one"],"premise_fact_ids":["f-one"],"counter_fact_ids":[]}`, true},
		{"overlap", `{"status":"unsupported","reason":"mixed fact","examined_fact_ids":["f-one"],"premise_fact_ids":["f-one"],"counter_fact_ids":["f-one"]}`, true},
		{"missing-examined", `{"status":"unsupported","reason":"gap","premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"null-examined", `{"status":"unsupported","reason":"gap","examined_fact_ids":null,"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"empty-examined", `{"status":"unsupported","reason":"gap","examined_fact_ids":[],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"foreign-examined", `{"status":"unsupported","reason":"gap","examined_fact_ids":["f-foreign"],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"duplicate-examined", `{"status":"unsupported","reason":"gap","examined_fact_ids":["f-one","f-one"],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"unexamined-premise", `{"status":"supported","reason":"claim","examined_fact_ids":["f-two"],"premise_fact_ids":["f-one"],"counter_fact_ids":[]}`, false},
		{"uncited-premise", `{"status":"supported","reason":"claim","examined_fact_ids":["f-two"],"premise_fact_ids":["f-two"],"counter_fact_ids":[]}`, false},
		{"unexamined-counter", `{"status":"unsupported","reason":"claim","examined_fact_ids":["f-one"],"premise_fact_ids":[],"counter_fact_ids":["f-two"]}`, false},
		{"no-supported-premise", `{"status":"supported","reason":"claim","examined_fact_ids":["f-one"],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"v3-reply", `{"status":"unsupported","reason":"claim","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`, false},
		{"extra", `{"status":"unsupported","reason":"gap","examined_fact_ids":["f-one"],"premise_fact_ids":[],"counter_fact_ids":[],"extra":1}`, false},
		{"duplicate-key", `{"status":"unsupported","reason":"gap","examined_fact_ids":["f-one"],"examined_fact_ids":["f-two"],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
		{"multibyte-reason", `{"status":"unsupported","reason":"` + strings.Repeat("é", 2049) + `","examined_fact_ids":["f-one"],"premise_fact_ids":[],"counter_fact_ids":[]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "decision.json")
			if e := os.WriteFile(p, []byte(tc.body), 0600); e != nil {
				t.Fatal(e)
			}
			_, cueErr := exec.Command("cue", "vet", "schema.cue", "qualified_review_schema.cue", p, "-d", "#QualifiedReviewDecisionV4").CombinedOutput()
			_, goErr := qualifiedDecodeFor(tc.body, qualifiedTestCase(), qualifiedProtocolV4)
			if tc.name != "foreign-examined" && tc.name != "uncited-premise" && tc.name != "duplicate-key" && (cueErr == nil) != tc.valid {
				t.Fatalf("CUE accepted=%v want=%v: %v", cueErr == nil, tc.valid, cueErr)
			}
			if (goErr == nil) != tc.valid {
				t.Fatalf("Go accepted=%v want=%v: %v", goErr == nil, tc.valid, goErr)
			}
		})
	}
	gap := `{"status":"unsupported","reason":"gap","examined_fact_ids":["f-one"],"premise_fact_ids":[],"counter_fact_ids":[]}`
	if _, e := qualifiedDecode(gap, qualifiedTestCase()); e == nil {
		t.Fatal("v3 accepted v4 gap")
	}
}
func TestQualifiedAccountingCUEParity(t *testing.T) {
	vet := func(t *testing.T, data any, definition string, valid bool) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "accounting.json")
		if err := os.WriteFile(path, jsonBytes(data), 0600); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("cue", "vet", "schema.cue", "qualified_review_schema.cue", path, "-d", definition).CombinedOutput()
		if (err == nil) != valid {
			t.Fatalf("CUE accepted=%v want=%v: %s", err == nil, valid, output)
		}
	}
	source := func(id string, charged int) map[string]any {
		return map[string]any{"id": id, "sha256": strings.Repeat("a", 64), "charged_micro_usd": charged, "uncertain_reserved_micro_usd": 0}
	}
	prior := func(charged int, sources ...any) map[string]any {
		if sources == nil {
			sources = []any{}
		}
		return map[string]any{"version": "prose-lab.prior-spend/1", "stage_id": "stage", "charged_micro_usd": charged, "uncertain_reserved_micro_usd": 0, "source_receipts": sources}
	}
	vet(t, prior(5, source("first", 5)), "#QualifiedPriorSpend", true)
	vet(t, prior(0), "#QualifiedPriorSpend", true)
	vet(t, prior(0, source("first", 5)), "#QualifiedPriorSpend", false)
	uncertainMismatch := prior(0, source("first", 0))
	uncertainMismatch["source_receipts"].([]any)[0].(map[string]any)["uncertain_reserved_micro_usd"] = 1
	vet(t, uncertainMismatch, "#QualifiedPriorSpend", false)
	vet(t, prior(10, source("first", 5), source("first", 5)), "#QualifiedPriorSpend", false)
	vet(t, prior(10_000_000, source("first", 10_000_000)), "#QualifiedPriorSpend", true)
	combined := prior(10_000_000, source("first", 10_000_000))
	combined["uncertain_reserved_micro_usd"] = 1
	combined["source_receipts"].([]any)[0].(map[string]any)["uncertain_reserved_micro_usd"] = 1
	vet(t, combined, "#QualifiedPriorSpend", false)
	_, _, dir := ownerQRSetup(t)
	trialBytes, err := os.ReadFile(filepath.Join(dir, "trial.json"))
	if err != nil {
		t.Fatal(err)
	}
	var trial map[string]any
	if json.Unmarshal(trialBytes, &trial) != nil {
		t.Fatal("invalid owner trial")
	}
	vet(t, trial, "#QualifiedReviewTrial", true)
	trial["prior_direct_micro_usd"] = 9_000_000
	trial["budget_micro_usd"] = 5_000_000
	trial["direct_stage_budget_micro_usd"] = 10_000_000
	vet(t, trial, "#QualifiedReviewTrial", false)
}
func TestQualifiedReplayUsesNativeWireAndRejectsRefusal(t *testing.T) {
	c, l := qualifiedTestCase(), qualifiedTestLane()
	w, e := qualifiedWire(context.Background(), c, l)
	if e != nil {
		t.Fatal(e)
	}
	answer := `{"status":"unsupported","reason":"proposal cannot establish law","premise_fact_ids":[],"counter_fact_ids":["f-two"]}`
	native := func(reason, answer string) []byte {
		return jsonBytes(map[string]any{"id": "msg_01", "type": "message", "role": "assistant", "model": l.Model, "content": []map[string]string{{"type": "text", "text": answer}}, "stop_reason": reason, "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 10}})
	}
	d, e := qualifiedReplay(context.Background(), c, l, w, native("end_turn", answer))
	if e != nil || d.Status != "unsupported" {
		t.Fatalf("valid native replay: %v %+v", e, d)
	}
	if _, e = qualifiedReplay(context.Background(), c, l, w, native("refusal", answer)); e == nil {
		t.Error("accepted refusal")
	}
	if _, e = qualifiedReplay(context.Background(), c, l, append([]byte(nil), w[:len(w)-1]...), native("end_turn", answer)); e == nil {
		t.Error("accepted request drift")
	}
}
func TestQualifiedV4NativeReplayAndVersionSeparation(t *testing.T) {
	c, l := qualifiedTestCase(), qualifiedTestLane()
	w3, err := qualifiedWire(context.Background(), c, l)
	if err != nil {
		t.Fatal(err)
	}
	w4, err := qualifiedWireFor(context.Background(), c, l, qualifiedProtocolV4)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(w3, w4) || !bytes.Contains(w4, []byte("examined_fact_ids")) {
		t.Fatal("protocol wire not separated")
	}
	gap := `{"status":"unsupported","reason":"cited definition does not establish event","examined_fact_ids":["f-one","f-two"],"premise_fact_ids":[],"counter_fact_ids":[]}`
	native := jsonBytes(map[string]any{"id": "msg_01", "type": "message", "role": "assistant", "model": l.Model, "content": []map[string]string{{"type": "text", "text": gap}}, "stop_reason": "end_turn", "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 10}})
	d, err := qualifiedReplayFor(context.Background(), c, l, qualifiedProtocolV4, w4, native)
	if err != nil || d.Status != "unsupported" || len(d.ExaminedFactIDs) != 2 {
		t.Fatalf("v4 replay: %+v %v", d, err)
	}
	if _, err = qualifiedReplay(context.Background(), c, l, w3, native); err == nil {
		t.Fatal("v3 accepted v4 reply")
	}
	if _, err = qualifiedReplayFor(context.Background(), c, l, qualifiedProtocolV4, w3, native); err == nil {
		t.Fatal("v4 accepted v3 wire")
	}
}
func TestQualifiedTrialV2SelectsProtocol(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	path := filepath.Join(dir, "trial.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]any
	if err := json.Unmarshal(original, &base); err != nil {
		t.Fatal(err)
	}
	prepare := func(trial map[string]any) (int, map[string]any) {
		t.Helper()
		if err := os.WriteFile(path, jsonBytes(trial), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diag bytes.Buffer
		code := runQualifiedReview(context.Background(), append(append([]string{"review-qualified-prepare"}, args...), "--case", cases[0].CaseID), &out, &diag)
		var payload map[string]any
		if out.Len() > 0 && json.Unmarshal(out.Bytes(), &payload) != nil {
			t.Fatal("invalid preparation")
		}
		return code, payload
	}
	code, old := prepare(base)
	if code != 0 || old["protocol"] != qualifiedProtocol {
		t.Fatalf("old trial: %d %v", code, old)
	}
	for _, selector := range []any{"", nil, qualifiedProtocolV4} {
		trial := make(map[string]any, len(base)+1)
		for k, v := range base {
			trial[k] = v
		}
		trial["review_protocol"] = selector
		if code, _ := prepare(trial); code != 2 {
			t.Fatalf("v1 accepted selector %v", selector)
		}
	}
	v2 := make(map[string]any, len(base)+1)
	for k, v := range base {
		v2[k] = v
	}
	v2["version"] = qualifiedTrialVersionV2
	for _, selector := range []any{nil, "", qualifiedProtocol, "foreign"} {
		v2["review_protocol"] = selector
		if code, _ := prepare(v2); code != 2 {
			t.Fatalf("v2 accepted selector %v", selector)
		}
	}
	delete(v2, "review_protocol")
	if code, _ := prepare(v2); code != 2 {
		t.Fatal("v2 accepted missing selector")
	}
	v2["review_protocol"] = qualifiedProtocolV4
	code, next := prepare(v2)
	if code != 0 || next["protocol"] != qualifiedProtocolV4 || next["study_digest"] == old["study_digest"] || next["native_request_digest"] == old["native_request_digest"] || next["prepared_digest"] == old["prepared_digest"] {
		t.Fatalf("v2 identity: %d %v", code, next)
	}
	p := filepath.Join(t.TempDir(), "trial.json")
	if err := os.WriteFile(p, jsonBytes(v2), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cue", "vet", "schema.cue", "qualified_review_schema.cue", p, "-d", "#QualifiedReviewTrialV2").CombinedOutput(); err != nil {
		t.Fatalf("CUE v2: %v %s", err, output)
	}
}
func TestQualifiedV3NativeWireGolden(t *testing.T) {
	var golden struct {
		Rows []struct {
			CaseID              string `json:"case_id"`
			NativeRequestDigest string `json:"native_request_digest"`
		} `json:"rows"`
	}
	b, err := os.ReadFile("testdata/qualified-v3-wire-golden.json")
	if err != nil || json.Unmarshal(b, &golden) != nil || len(golden.Rows) != 85 {
		t.Fatal("v3 golden unavailable")
	}
	args, _, dir := ownerQRSetup(t)
	trialPath := filepath.Join(dir, "trial.json")
	b, err = os.ReadFile(trialPath)
	if err != nil {
		t.Fatal(err)
	}
	var trial map[string]any
	if json.Unmarshal(b, &trial) != nil {
		t.Fatal("trial JSON")
	}
	review := trial["review"].(map[string]any)
	review["model"] = "configured-review-model"
	if err := os.WriteFile(trialPath, jsonBytes(trial), 0600); err != nil {
		t.Fatal(err)
	}
	in, err := loadQualifiedInputs(context.Background(), trialPath, filepath.Join(dir, "prior-spend.json"), args[len(args)-1], args[:len(args)-6])
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range golden.Rows {
		if row.CaseID != in.ordered[i] || row.NativeRequestDigest != hashBytes(in.wires[row.CaseID]) {
			t.Fatalf("v3 native wire drift at %d: %s", i+1, row.CaseID)
		}
	}
}
func TestQualifiedHelpKeyFreeAndShortWrite(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runQualifiedReview(context.Background(), []string{"review-qualified-prepare", "--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "--trial") {
		t.Fatalf("help %d %q", code, out.String())
	}
	if code := runQualifiedReview(context.Background(), []string{"review-qualified-prepare", "--help"}, shortQualifiedWriter{}, &errOut); code != 2 {
		t.Fatalf("short write exit %d", code)
	}
}

type shortQualifiedWriter struct{}

func (shortQualifiedWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }

func TestQualifiedPriorSpendBindingAndRefusal(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	prepare := func(a []string) (int, map[string]any) {
		var out, diag bytes.Buffer
		code := runQualifiedReview(context.Background(), append(append([]string{"review-qualified-prepare"}, a...), "--case", cases[0].CaseID), &out, &diag)
		var payload map[string]any
		if out.Len() != 0 && json.Unmarshal(out.Bytes(), &payload) != nil {
			t.Fatalf("bad output: %s", out.String())
		}
		return code, payload
	}
	code, first := prepare(args)
	if code != 0 || first["declared_prior_micro_usd"] != float64(778690) || first["cross_trial_spend_verified"] != false || first["provider_spend_limit_enforced"] != false {
		t.Fatalf("valid prior receipt: %d %v", code, first)
	}
	priorPath := filepath.Join(dir, "prior-spend.json")
	raw, err := os.ReadFile(priorPath)
	if err != nil {
		t.Fatal(err)
	}
	var prior map[string]any
	if json.Unmarshal(raw, &prior) != nil {
		t.Fatal("invalid test receipt")
	}
	sources := prior["source_receipts"].([]any)
	sources[0].(map[string]any)["id"] = "another-reviewed-receipt"
	changed := jsonBytes(prior)
	if err = os.WriteFile(priorPath, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if code, _ = prepare(args); code != 2 {
		t.Fatal("accepted stale prior receipt pin")
	}
	updated := append([]string(nil), args...)
	updated[len(updated)-1] = hashBytes(changed)
	code, second := prepare(updated)
	if code != 0 || second["study_digest"] == first["study_digest"] {
		t.Fatalf("prior receipt not bound: %d %v", code, second)
	}
	badPrior := func(change func(map[string]any)) {
		t.Helper()
		var p map[string]any
		if json.Unmarshal(changed, &p) != nil {
			t.Fatal("invalid test receipt")
		}
		change(p)
		b := jsonBytes(p)
		if err := os.WriteFile(priorPath, b, 0600); err != nil {
			t.Fatal(err)
		}
		a := append([]string(nil), args...)
		a[len(a)-1] = hashBytes(b)
		if code, _ := prepare(a); code != 2 {
			t.Fatalf("accepted invalid prior receipt: %s", b)
		}
	}
	badPrior(func(p map[string]any) { p["charged_micro_usd"] = 0 })
	badPrior(func(p map[string]any) { delete(p, "uncertain_reserved_micro_usd") })
	badPrior(func(p map[string]any) {
		p["source_receipts"] = append(p["source_receipts"].([]any), p["source_receipts"].([]any)[0])
	})
	badPrior(func(p map[string]any) { p["charged_micro_usd"] = 0; p["source_receipts"] = []any{} })
}

func TestQualifiedPreDispatchFailureRetainsRoster(t *testing.T) {
	for _, mode := range []string{"nil-client", "missing-credential", "cancel-before-transport"} {
		t.Run(mode, func(t *testing.T) {
			args, cases, dir := ownerQRSetup(t)
			var out, diag bytes.Buffer
			prepareArgs := append(append([]string{"review-qualified-prepare"}, args...), "--case", cases[0].CaseID)
			if code := runQualifiedReview(context.Background(), prepareArgs, &out, &diag); code != 0 {
				t.Fatalf("prepare=%d %s", code, diag.String())
			}
			var prepared map[string]any
			if json.Unmarshal(out.Bytes(), &prepared) != nil {
				t.Fatal("invalid preparation")
			}
			journalPath, artifactsPath := filepath.Join(dir, "journal"), filepath.Join(dir, "artifacts")
			capArgs := append(append([]string{"review-qualified-capture"}, args...), "--case", cases[0].CaseID, "--prepared-digest", prepared["prepared_digest"].(string), "--journal", journalPath, "--artifacts", artifactsPath, "--live")
			ctx := context.Background()
			deps := reviewDeps{}
			switch mode {
			case "nil-client":
				deps.captureHTTP = func() (*http.Client, *captureTransport) { return nil, nil }
			case "missing-credential":
				t.Setenv("ANTHROPIC_API_KEY", "")
				deps.captureHTTP = func() (*http.Client, *captureTransport) { return studyHTTP("anthropic") }
			case "cancel-before-transport":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				deps.apiKey = "fake-offline-only"
				deps.captureHTTP = func() (*http.Client, *captureTransport) { cancel(); return studyHTTP("anthropic") }
			}
			out.Reset()
			diag.Reset()
			if code := runQualifiedReviewWith(ctx, capArgs, &out, &diag, deps); code != 2 {
				t.Fatalf("pre-dispatch capture=%d %s", code, diag.String())
			}
			out.Reset()
			diag.Reset()
			reportArgs := append(append([]string{"review-qualified-report"}, args...), "--journal", journalPath, "--artifacts", artifactsPath)
			if code := runQualifiedReview(context.Background(), reportArgs, &out, &diag); code != 2 || out.Len() == 0 {
				t.Fatalf("report=%d %s", code, diag.String())
			}
			var report map[string]any
			if json.Unmarshal(out.Bytes(), &report) != nil || report["failed"] != float64(1) || report["missing"] != float64(84) || report["charged_micro_usd"] != float64(0) {
				t.Fatalf("pre-dispatch denominator: %v", report)
			}
			// The exact terminal marker makes even the next lexical case ineligible.
			out.Reset()
			diag.Reset()
			if code := runQualifiedReviewWith(context.Background(), capArgs, &out, &diag, deps); code != 2 {
				t.Fatalf("terminal case resumed: %d", code)
			}
			receiptPath := filepath.Join(artifactsPath, cases[0].CaseID+".receipt.json")
			b, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			var receipt map[string]any
			if json.Unmarshal(b, &receipt) != nil {
				t.Fatal("invalid test receipt")
			}
			receipt["native_request_digest"] = strings.Repeat("0", 64)
			if err := os.WriteFile(receiptPath, jsonBytes(receipt), 0600); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			diag.Reset()
			if code := runQualifiedReview(context.Background(), reportArgs, &out, &diag); code != 2 || out.Len() != 0 {
				t.Fatalf("accepted unbound pre-dispatch artifact: %d %s", code, out.String())
			}
		})
	}
}
