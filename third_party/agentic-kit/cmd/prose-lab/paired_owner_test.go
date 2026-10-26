package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const ownerPairedScopePin = "f8d86a5366e2317f2a5dd6bc90fae074452a5d3c2176dccf9b1e589043ac6a49"

func ownerPairedSetup(t *testing.T) ([]string, []ownerReviewCase, string) {
	t.Helper()
	_, cases := ownerReviewSetup(t, mustReadDocumentTest(t, "testdata/document-scopes/v2-support-review.json"))
	var scope struct {
		Extensions []struct {
			ID        string   `json:"id"`
			Assertion string   `json:"assertion"`
			Expected  string   `json:"expected"`
			FactIDs   []string `json:"fact_ids"`
		} `json:"extensions"`
	}
	raw := mustReadDocumentTest(t, "testdata/document-scopes/v2-paired-support-review.json")
	if hashBytes(raw) != ownerPairedScopePin || json.Unmarshal(raw, &scope) != nil || len(scope.Extensions) != 3 {
		t.Fatal("owner-reviewed scope changed")
	}
	for _, c := range scope.Extensions {
		cases = append(cases, ownerReviewCase{c.ID, c.Assertion, c.Expected, c.FactIDs})
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	ids := []string{}
	for _, c := range cases {
		ids = append(ids, "baseline--"+c.ID, "candidate--"+c.ID)
	}
	if len(cases) != 70 || len(ids) != 140 {
		t.Fatal("owner denominator changed")
	}
	study := map[string]any{
		"version": "prose-lab.paired-review-study/1", "id": "owner-paired-study", "dataset_kind": "synthetic",
		"budget_micro_usd": 1000000, "prior_direct_micro_usd": 424414, "direct_stage_budget_micro_usd": 10000000,
		"review":                 lane{Provider: "anthropic", Model: "pair-owner-model", PricingSource: "synthetic-fixture", MaxInputTokens: 1000000, MaxOutputTokens: 256, InputRate: 1000, OutputRate: 1000, ReserveInputRate: 1000, ReserveOutputRate: 1000},
		"variants":               []map[string]string{{"id": "baseline", "protocol": "prose-lab.support-review/1"}, {"id": "candidate", "protocol": "prose-lab.support-review/2"}},
		"expected_execution_ids": ids,
	}
	root := t.TempDir()
	file := filepath.Join(root, "study.json")
	if err := os.WriteFile(file, jsonBytes(study), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--study", file, "--corpus", "testdata/document-corpus-v2", "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope, "--base-review-scope", "testdata/document-scopes/v2-support-review.json", "--review-scope", "testdata/document-scopes/v2-paired-support-review.json", "--review-scope-digest", ownerPairedScopePin, "--regression-source", "../../docs/evidence/prose-headless/2026-10-25-long-pilot/regressions.json"}
	return args, cases, root
}

func ownerPairedRun(t *testing.T, deps reviewDeps, want int, args ...string) ([]byte, map[string]any) {
	t.Helper()
	var out, diag bytes.Buffer
	code := runReviewPairWith(context.Background(), args, &out, &diag, deps)
	if code != want {
		t.Fatalf("%s exit%d want%d: %s", args[0], code, want, diag.String())
	}
	if bytes.Contains(out.Bytes(), []byte("PRIVATE_PAIRED_REASON")) && !strings.Contains(strings.Join(args, " "), "--include-content") {
		t.Fatal("private reason in default output")
	}
	var got map[string]any
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"independent_acceptance", "publication_authority", "model_site_adoption"} {
			if got[key] != false {
				t.Fatalf("authority field %s = %v", key, got[key])
			}
		}
	}
	return append([]byte(nil), out.Bytes()...), got
}

func ownerPairedPrepare(t *testing.T, base []string, id, variant string) ([]byte, string) {
	t.Helper()
	a := append(append([]string{"review-pair-prepare"}, base...), "--case", id, "--variant", variant)
	raw, p := ownerPairedRun(t, reviewDeps{}, 0, a...)
	return raw, p["prepared_digest"].(string)
}

func ownerPairedCaptureArgs(base []string, root, id, variant, pin string) []string {
	return append(append([]string{"review-pair-capture"}, base...), "--case", id, "--variant", variant, "--prepared-digest", pin, "--journal", filepath.Join(root, "journal"), "--artifacts", filepath.Join(root, "artifacts"), "--live")
}

func ownerPairedReportArgs(base []string, root string) []string {
	return append(append([]string{"review-pair-report"}, base...), "--journal", filepath.Join(root, "journal"), "--artifacts", filepath.Join(root, "artifacts"))
}

func ownerPairedDeps(t *testing.T, cases []ownerReviewCase, variant *string, calls *int, baselineHold, candidateBad, unknownUsage bool) reviewDeps {
	t.Helper()
	byText := map[string]ownerReviewCase{}
	for _, c := range cases {
		byText[c.Assertion] = c
	}
	expectedFacts := map[string]map[string]string{}
	for _, c := range cases {
		doc, _, ok := strings.Cut(c.ID, "--")
		if !ok {
			t.Fatal("owner case identity")
		}
		if expectedFacts[doc] != nil {
			continue
		}
		var data struct {
			Facts []struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			} `json:"facts"`
		}
		if json.Unmarshal(mustReadDocumentTest(t, filepath.Join("testdata/document-corpus-v2", doc+".json")), &data) != nil {
			t.Fatal("owner source facts")
		}
		expectedFacts[doc] = map[string]string{}
		for _, f := range data.Facts {
			expectedFacts[doc][f.ID] = f.Text
		}
	}
	payloads := map[string][]byte{}
	return reviewDeps{apiKey: "offline-owner-fixture", now: func() time.Time { return time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC) }, captureHTTP: func() (*http.Client, *captureTransport) {
		capture := &captureTransport{origin: "https://api.anthropic.com", path: "/v1/messages", inner: ownerReviewTransport(func(req *http.Request) (*http.Response, error) {
			*calls++
			wire, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var body struct {
				Messages []struct{ Content json.RawMessage } `json:"messages"`
			}
			if err := json.Unmarshal(wire, &body); err != nil || len(body.Messages) != 1 {
				return nil, fmt.Errorf("unexpected native messages")
			}
			var text string
			if err := json.Unmarshal(body.Messages[0].Content, &text); err != nil {
				var blocks []struct{ Type, Text string }
				if err := json.Unmarshal(body.Messages[0].Content, &blocks); err != nil || len(blocks) != 1 || blocks[0].Type != "text" {
					return nil, fmt.Errorf("unexpected native content")
				}
				text = blocks[0].Text
			}
			var payload map[string]json.RawMessage
			if json.Unmarshal([]byte(text), &payload) != nil || len(payload) != 4 {
				t.Fatal("payload fields changed or oracle metadata leaked")
			}
			var assertion string
			var cited []string
			if json.Unmarshal(payload["assertion"], &assertion) != nil || json.Unmarshal(payload["cited_fact_ids"], &cited) != nil {
				t.Fatal("missing native assertion/citations")
			}
			oracle, ok := byText[assertion]
			if !ok || !bytes.Equal(jsonBytes(oracle.FactIDs), jsonBytes(cited)) || bytes.Contains(wire, []byte(oracle.ID)) || bytes.Contains(wire, []byte("independent-agent-heuristic-and-owner-source-review")) {
				t.Fatal("native request differs from independent oracle or leaks labels")
			}
			doc, _, _ := strings.Cut(oracle.ID, "--")
			var allFacts map[string]string
			if json.Unmarshal(payload["all_facts"], &allFacts) != nil || !bytes.Equal(jsonBytes(allFacts), jsonBytes(expectedFacts[doc])) {
				t.Fatal("required contradiction context shrank or changed")
			}
			if *variant == "baseline" {
				payloads[oracle.ID] = append([]byte(nil), body.Messages[0].Content...)
			} else if !bytes.Equal(payloads[oracle.ID], body.Messages[0].Content) {
				t.Fatal("candidate changed facts/payload rather than only prompt")
			}
			status := oracle.Expected
			if baselineHold && *variant == "baseline" && strings.HasSuffix(oracle.ID, "lp-rhetorical-scope") {
				status = "unsupported"
			}
			if candidateBad && *variant == "candidate" {
				if status == "supported" {
					status = "unsupported"
				} else {
					status = "supported"
				}
			}
			answer := string(jsonBytes(map[string]string{"status": status, "reason": "PRIVATE_PAIRED_REASON"}))
			native := map[string]any{"id": "msg-pair-owner", "type": "message", "role": "assistant", "model": "pair-owner-model", "content": []map[string]string{{"type": "text", "text": answer}}, "stop_reason": "end_turn", "stop_sequence": nil}
			if !unknownUsage {
				native["usage"] = map[string]int{"input_tokens": 1000, "output_tokens": 1}
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(jsonBytes(native))), Request: req}, nil
		})}
		return &http.Client{Transport: capture}, capture
	}}
}

func ownerPairedCUE(t *testing.T, definition string, raw []byte, valid bool) {
	t.Helper()
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue unavailable; no schema proof")
	}
	file := filepath.Join(t.TempDir(), "actual.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("cue", "vet", "schema.cue", file, "-d", definition).CombinedOutput()
	if (err == nil) != valid {
		t.Fatalf("CUE %s valid=%v: %v %s", definition, valid, err, output)
	}
}

func TestOwnerPairedCompleteNativeComparison(t *testing.T) {
	base, cases, root := ownerPairedSetup(t)
	variant, calls := "baseline", 0
	deps := ownerPairedDeps(t, cases, &variant, &calls, true, false, false)
	for _, c := range cases {
		for _, v := range []string{"baseline", "candidate"} {
			variant = v
			prepared, pin := ownerPairedPrepare(t, base, c.ID, v)
			if calls == 0 {
				ownerPairedCUE(t, "#PairedPrepared", prepared, true)
			}
			ownerPairedRun(t, deps, 0, ownerPairedCaptureArgs(base, root, c.ID, v, pin)...)
		}
	}
	if calls != 140 {
		t.Fatalf("actual fake-native calls%d", calls)
	}
	raw, report := ownerPairedRun(t, reviewDeps{}, 0, ownerPairedReportArgs(base, root)...)
	if report["qualified"] != true || len(report["rows"].([]any)) != 140 {
		t.Fatal("full paired roster missing or not qualified")
	}
	baseline := report["baseline"].(map[string]any)
	candidate := report["candidate"].(map[string]any)
	total := report["total"].(map[string]any)
	if baseline["false_hold"] != float64(1) || candidate["false_hold"] != float64(0) || candidate["complete"] != float64(70) || total["complete"] != float64(140) || total["charged_micro_usd"] != float64(280) {
		t.Fatalf("independent expected cohort counters differ: %s", raw)
	}
	ownerPairedCUE(t, "#PairedReport", raw, true)
	for _, state := range []string{"missing", "invalid", "inconclusive"} {
		var altered map[string]any
		if json.Unmarshal(raw, &altered) != nil {
			t.Fatal("report clone")
		}
		altered["rows"].([]any)[0].(map[string]any)["state"] = state
		ownerPairedCUE(t, "#PairedReport", jsonBytes(altered), false)
	}
	if bytes.Contains(raw, []byte(`"expected"`)) || bytes.Contains(raw, []byte(`"provenance"`)) || bytes.Contains(raw, []byte(cases[0].Assertion)) {
		t.Fatal("default report disclosed oracle/source content")
	}
	args := append(ownerPairedReportArgs(base, root), "--include-content")
	content, _ := ownerPairedRun(t, reviewDeps{}, 0, args...)
	if !bytes.Contains(content, []byte("PRIVATE_PAIRED_REASON")) || !bytes.Contains(content, []byte(`"expected"`)) {
		t.Fatal("explicit content missing inspectable oracle/reason")
	}
	ownerPairedCUE(t, "#PairedReport", content, true)
}

func TestOwnerPairedCandidateFailureAndIntegrityPrecedence(t *testing.T) {
	base, cases, root := ownerPairedSetup(t)
	variant, calls := "baseline", 0
	deps := ownerPairedDeps(t, cases, &variant, &calls, false, true, false)
	_, pin := ownerPairedPrepare(t, base, cases[0].ID, variant)
	ownerPairedRun(t, deps, 0, ownerPairedCaptureArgs(base, root, cases[0].ID, variant, pin)...)
	incomplete, report := ownerPairedRun(t, reviewDeps{}, 2, ownerPairedReportArgs(base, root)...)
	ownerPairedCUE(t, "#PairedReport", incomplete, true)
	report["qualified"] = true
	ownerPairedCUE(t, "#PairedReport", jsonBytes(report), false)
	variant = "candidate"
	_, pin = ownerPairedPrepare(t, base, cases[0].ID, variant)
	ownerPairedRun(t, deps, 1, ownerPairedCaptureArgs(base, root, cases[0].ID, variant, pin)...)
	_, failed := ownerPairedRun(t, reviewDeps{}, 1, ownerPairedReportArgs(base, root)...)
	if failed["qualified"] != false || len(failed["rows"].([]any)) != 140 || failed["candidate"].(map[string]any)["false_support"] != float64(1) {
		t.Fatal("known candidate failure or missing denominator lost")
	}
	variant = "baseline"
	_, next := ownerPairedPrepare(t, base, cases[1].ID, variant)
	ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(base, root, cases[1].ID, variant, next)...)
	if calls != 2 {
		t.Fatal("provider called after candidate stop")
	}
	marker := filepath.Join(root, "artifacts", "baseline--"+cases[0].ID+".started.json")
	if err := os.WriteFile(marker, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, invalid := ownerPairedRun(t, reviewDeps{}, 2, ownerPairedReportArgs(base, root)...)
	if invalid["qualified"] != false {
		t.Fatal("tampered evidence remained qualified")
	}
}

func TestOwnerPairedUnknownSpendStopsBothVariants(t *testing.T) {
	base, cases, root := ownerPairedSetup(t)
	variant, calls := "baseline", 0
	deps := ownerPairedDeps(t, cases, &variant, &calls, false, false, true)
	_, pin := ownerPairedPrepare(t, base, cases[0].ID, variant)
	_, held := ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(base, root, cases[0].ID, variant, pin)...)
	if held["case_charged_micro_usd"] != float64(1001) || held["cumulative_charged_micro_usd"] != float64(1001) {
		t.Fatalf("unknown usage did not retain full reservation: %v", held)
	}
	var reserve map[string]any
	var outcome map[string]any
	if json.Unmarshal(mustReadDocumentTest(t, filepath.Join(root, "journal", "reserve-000001.json")), &reserve) != nil || json.Unmarshal(mustReadDocumentTest(t, filepath.Join(root, "journal", "outcome-000001.json")), &outcome) != nil {
		t.Fatal("missing retained uncertain attempt")
	}
	if reserve["reserved_micro_usd"] != float64(1001) || outcome["status"] != "unknown" || outcome["usage_priced_micro_usd"] != nil {
		t.Fatal("uncertain spend regraded as known/free")
	}
	lock := filepath.Join(root, "journal", ".lock")
	if st, err := os.Stat(lock); err != nil || !st.IsDir() {
		t.Fatal("uncertain journal lock lost", err)
	}
	raw, report := ownerPairedRun(t, reviewDeps{}, 2, ownerPairedReportArgs(base, root)...)
	if len(raw) != 0 || report != nil {
		t.Fatal("locked journal yielded an apparently audited report")
	}
	variant = "candidate"
	_, pin = ownerPairedPrepare(t, base, cases[0].ID, variant)
	ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(base, root, cases[0].ID, variant, pin)...)
	if calls != 1 {
		t.Fatal("candidate dispatched after uncertain baseline spend")
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("refusal removed uncertain lock", err)
	}
}

func TestOwnerPairedStudyCUEAndRuntimeParity(t *testing.T) {
	base, cases, _ := ownerPairedSetup(t)
	var study map[string]any
	if json.Unmarshal(mustReadDocumentTest(t, base[1]), &study) != nil {
		t.Fatal("study")
	}
	ownerPairedCUE(t, "#PairedStudy", jsonBytes(study), true)
	for _, tc := range []string{"provider", "output"} {
		raw := jsonBytes(study)
		var altered map[string]any
		_ = json.Unmarshal(raw, &altered)
		review := altered["review"].(map[string]any)
		if tc == "provider" {
			review["provider"] = "openrouter-jev"
			review["output_rate"] = float64(0)
			review["reserve_output_rate"] = float64(0)
		} else {
			review["max_output_tokens"] = float64(8193)
		}
		raw = jsonBytes(altered)
		ownerPairedCUE(t, "#PairedStudy", raw, false)
		file := filepath.Join(t.TempDir(), "bad-study.json")
		if err := os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
		args := append([]string{}, base...)
		args[1] = file
		args = append(append([]string{"review-pair-prepare"}, args...), "--case", cases[0].ID, "--variant", "baseline")
		out, _ := ownerPairedRun(t, reviewDeps{}, 2, args...)
		if len(out) != 0 {
			t.Fatal("bad study emitted a preparation")
		}
	}
}

type ownerPairedBrokenWriter struct{}

func (ownerPairedBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestOwnerPairedHelpAndOutputFailures(t *testing.T) {
	for _, cmd := range []string{"review-pair-prepare", "review-pair-capture", "review-pair-report"} {
		for _, flag := range []string{"--help", "-h", "help"} {
			var out, diag bytes.Buffer
			if code := runLab(context.Background(), []string{cmd, flag}, &out, &diag); code != 0 || !strings.Contains(out.String(), "Usage:") {
				t.Fatalf("help failed %s %s", cmd, flag)
			}
		}
	}
	base, cases, _ := ownerPairedSetup(t)
	args := append(append([]string{"review-pair-prepare"}, base...), "--case", cases[0].ID, "--variant", "baseline")
	var diag bytes.Buffer
	if code := runReviewPairWith(context.Background(), args, ownerPairedBrokenWriter{}, &diag, reviewDeps{}); code != 2 {
		t.Fatal("broken writer accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if code := runReviewPairWith(ctx, args, &out, &diag, reviewDeps{}); code != 2 || out.Len() != 0 {
		t.Fatal("canceled prepare emitted result")
	}
	out.Reset()
	diag.Reset()
	args = []string{"review-pair-prepare", "--PRIVATE_PAIRED_FLAG", "--help"}
	if code := runLab(context.Background(), args, &out, &diag); code != 2 || strings.Contains(out.String()+diag.String(), "PRIVATE_PAIRED_FLAG") {
		t.Fatal("unknown flag masked by help or leaked")
	}
}

func TestOwnerPairedRejectsLatencyOverflow(t *testing.T) {
	base, cases, root := ownerPairedSetup(t)
	variant, calls := "baseline", 0
	deps := ownerPairedDeps(t, cases, &variant, &calls, false, false, false)
	for _, v := range []string{"baseline", "candidate"} {
		variant = v
		_, pin := ownerPairedPrepare(t, base, cases[0].ID, v)
		ownerPairedRun(t, deps, 0, ownerPairedCaptureArgs(base, root, cases[0].ID, v, pin)...)
	}
	for _, name := range []string{"outcome-000001.json", "outcome-000002.json"} {
		file := filepath.Join(root, "journal", name)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(mustReadDocumentTest(t, file), &fields); err != nil {
			t.Fatal(err)
		}
		fields["latency_nanos"] = json.RawMessage("9223372036854775807")
		if err := os.WriteFile(file, jsonBytes(fields), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := ownerPairedRun(t, reviewDeps{}, 2, ownerPairedReportArgs(base, root)...)
	if len(raw) != 0 {
		t.Fatal("overflowing latency produced an apparently audited JSON report")
	}
}

func TestOwnerPairedPreparationDisplaysDecisionInputs(t *testing.T) {
	base, cases, _ := ownerPairedSetup(t)
	var common string
	for _, v := range []string{"baseline", "candidate"} {
		args := append(append([]string{"review-pair-prepare"}, base...), "--case", cases[0].ID, "--variant", v)
		raw, p := ownerPairedRun(t, reviewDeps{}, 0, args...)
		if p["requested_review_model"] != "pair-owner-model" || p["budget_micro_usd"] != float64(1000000) || p["prior_direct_micro_usd"] != float64(424414) || p["direct_stage_budget_micro_usd"] != float64(10000000) || p["review_scope_digest"] != ownerPairedScopePin || p["base_review_scope_digest"] != ownerReviewPin || p["goal_scope_digest"] != frozenV2GoalScope {
			t.Fatal("preparation hides or misstates review inputs")
		}
		want := "prose-lab.support-review/1"
		if v == "candidate" {
			want = "prose-lab.support-review/2"
		}
		if p["review_protocol"] != want {
			t.Fatal("selected protocol misidentified")
		}
		if common != "" && p["study_digest"] != common {
			t.Fatal("selected variant split common journal binding")
		}
		common = p["study_digest"].(string)
		ownerPairedCUE(t, "#PairedPrepared", raw, true)
	}
}

func TestOwnerPairedSelectionAndStudyPinsBeforeDispatch(t *testing.T) {
	base, cases, root := ownerPairedSetup(t)
	variant, calls := "baseline", 0
	deps := ownerPairedDeps(t, cases, &variant, &calls, false, false, false)
	_, baselinePin := ownerPairedPrepare(t, base, cases[0].ID, "baseline")
	// A preparation for one variant cannot authorize the other.
	ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(base, root, cases[0].ID, "candidate", baselinePin)...)
	if calls != 0 {
		t.Fatal("wrong variant called provider")
	}
	ownerPairedRun(t, deps, 0, ownerPairedCaptureArgs(base, root, cases[0].ID, "baseline", baselinePin)...)
	// A valid preparation cannot skip the next execution in the shared ledger.
	_, skipped := ownerPairedPrepare(t, base, cases[1].ID, "baseline")
	ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(base, root, cases[1].ID, "baseline", skipped)...)
	if calls != 1 {
		t.Fatal("out-of-order call dispatched")
	}
	// Changing declared accounting changes the common binding, not the old journal.
	var study map[string]any
	if json.Unmarshal(mustReadDocumentTest(t, base[1]), &study) != nil {
		t.Fatal("study")
	}
	study["prior_direct_micro_usd"] = float64(424415)
	other := filepath.Join(t.TempDir(), "other.json")
	if os.WriteFile(other, jsonBytes(study), 0600) != nil {
		t.Fatal("changed study")
	}
	changed := append([]string{}, base...)
	changed[1] = other
	variant = "candidate"
	_, pin := ownerPairedPrepare(t, changed, cases[0].ID, variant)
	ownerPairedRun(t, deps, 2, ownerPairedCaptureArgs(changed, root, cases[0].ID, variant, pin)...)
	if calls != 1 {
		t.Fatal("changed study reused journal")
	}
	// Explicitly empty forbidden flags still count as supplied flags.
	for _, flags := range [][]string{{"--live=false"}, {"--journal="}, {"--prepared-digest="}} {
		a := append(append([]string{"review-pair-prepare"}, base...), "--case", cases[0].ID, "--variant", "baseline")
		a = append(a, flags...)
		raw, _ := ownerPairedRun(t, reviewDeps{}, 2, a...)
		if len(raw) != 0 {
			t.Fatal("forbidden prepare flag emitted result")
		}
	}
}
