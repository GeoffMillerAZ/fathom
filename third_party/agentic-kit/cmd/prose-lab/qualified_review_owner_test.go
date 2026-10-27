package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type ownerQRCase struct {
	CaseID   string   `json:"case_id"`
	Expected string   `json:"expected"`
	Cited    []string `json:"cited_fact_ids"`
}

func ownerQRSetup(t *testing.T) ([]string, []ownerQRCase, string) {
	t.Helper()
	dir, floorArgs := ownerFloorFiles(t)
	var fixture struct {
		Cases []ownerQRCase `json:"cases"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "fixtures/review-floor-v1.json"))
	if err != nil || json.Unmarshal(b, &fixture) != nil {
		t.Fatal("reviewed fixture unavailable")
	}
	byID := map[string]ownerQRCase{}
	for _, c := range fixture.Cases {
		byID[c.CaseID] = c
	}
	b, err = os.ReadFile(filepath.Join(dir, "scope.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scope struct {
		IDs []string `json:"expected_case_ids"`
	}
	if json.Unmarshal(b, &scope) != nil || len(scope.IDs) != 85 {
		t.Fatal("owner roster changed")
	}
	cases := make([]ownerQRCase, 0, 85)
	for _, id := range scope.IDs {
		cases = append(cases, byID[id])
	}
	trial := map[string]any{
		"version": "prose-lab.qualified-review-study/1", "id": "owner-qualified-study", "dataset_kind": "synthetic",
		"budget_micro_usd": 1000000, "prior_direct_micro_usd": 778690, "direct_stage_budget_micro_usd": 10000000,
		"review":            lane{Provider: "anthropic", Model: "owner-qualified-model", PricingSource: "synthetic-test-only", MaxInputTokens: 1000000, MaxOutputTokens: 8192, InputRate: 1000, OutputRate: 1000, ReserveInputRate: 1000, ReserveOutputRate: 1000},
		"expected_case_ids": scope.IDs,
	}
	p := filepath.Join(dir, "trial.json")
	if err = os.WriteFile(p, jsonBytes(trial), 0600); err != nil {
		t.Fatal(err)
	}
	prior := map[string]any{"version": "prose-lab.prior-spend/1", "stage_id": "owner-synthetic-stage", "charged_micro_usd": 778690, "uncertain_reserved_micro_usd": 0, "source_receipts": []any{map[string]any{"id": "synthetic-prior-record", "sha256": strings.Repeat("a", 64), "charged_micro_usd": 778690, "uncertain_reserved_micro_usd": 0}}}
	priorBytes := jsonBytes(prior)
	priorPath := filepath.Join(dir, "prior-spend.json")
	if err = os.WriteFile(priorPath, priorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string(nil), floorArgs[1:]...), "--trial", p, "--prior-spend", priorPath, "--prior-spend-digest", ownerOracleHash(priorBytes))
	return args, cases, dir
}

func ownerQRRun(t *testing.T, deps reviewDeps, want int, command string, args ...string) map[string]any {
	t.Helper()
	var out, diag bytes.Buffer
	code := runQualifiedReviewWith(context.Background(), append([]string{command}, args...), &out, &diag, deps)
	if code != want {
		t.Fatalf("%s exit%d want%d: %s", command, code, want, diag.String())
	}
	if bytes.Contains(out.Bytes(), []byte("OWNER_PRIVATE_QR_REASON")) && !strings.Contains(strings.Join(args, " "), "--include-content") {
		t.Fatal("private rationale in default output")
	}
	if out.Len() == 0 {
		return nil
	}
	m := ownerOracleJSON(t, out.Bytes())
	for _, key := range []string{"source_authenticated", "identity_authenticated", "independent_acceptance", "publication_authority", "ui_entry", "model_site_adoption", "cross_trial_spend_verified", "provider_spend_limit_enforced"} {
		if m[key] != false {
			t.Fatalf("authority %s=%v", key, m[key])
		}
	}
	return m
}

type ownerQRTransport func(*http.Request) (*http.Response, error)

func (f ownerQRTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func ownerQRDeps(t *testing.T, cases []ownerQRCase, alter func(int, map[string]any), calls *int) reviewDeps {
	t.Helper()
	return reviewDeps{apiKey: "offline-owner-placeholder", now: func() time.Time { return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) }, captureHTTP: func() (*http.Client, *captureTransport) {
		inner := ownerQRTransport(func(req *http.Request) (*http.Response, error) {
			wire, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			// Expected labels, reviewer bases, roles and opaque logical IDs are not
			// necessary to judge the assertion and must never reach the provider.
			for _, forbidden := range []string{"historical_expected", "old_case_ids", "reviewer_ref", "uncertainty-control", "OWNER_PRIVATE_QR_REASON"} {
				if bytes.Contains(wire, []byte(forbidden)) {
					t.Fatalf("provider payload leaked %s", forbidden)
				}
			}
			for _, c := range cases {
				if bytes.Contains(wire, []byte(c.CaseID)) {
					t.Fatalf("provider payload leaked control ID %s", c.CaseID)
				}
			}
			i := *calls
			if i >= len(cases) {
				t.Fatal("extra native attempt")
			}
			*calls++
			c := cases[i]
			answer := map[string]any{"status": c.Expected, "reason": "OWNER_PRIVATE_QR_REASON: synthetic transport result, not semantic evidence.", "premise_fact_ids": c.Cited, "counter_fact_ids": []string{}}
			if alter != nil {
				alter(i, answer)
			}
			response := map[string]any{"id": "msg-owner-fixture", "type": "message", "role": "assistant", "model": "owner-qualified-model", "content": []any{map[string]any{"type": "text", "text": string(jsonBytes(answer))}}, "stop_reason": "end_turn", "stop_sequence": nil, "usage": map[string]int{"input_tokens": 11, "output_tokens": 19}}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(jsonBytes(response))), Request: req}, nil
		})
		capture := &captureTransport{inner: inner, origin: "https://api.anthropic.com", path: "/v1/messages"}
		return &http.Client{Transport: capture}, capture
	}}
}

func TestOwnerQRCompleteNativeReplayAndInconclusive(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	journal := filepath.Join(dir, "journal")
	artifacts := filepath.Join(dir, "artifacts")
	calls := 0
	deps := ownerQRDeps(t, cases, nil, &calls)
	for _, c := range cases {
		prep := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-prepare", append(append([]string(nil), args...), "--case", c.CaseID)...)
		capArgs := append(append([]string(nil), args...), "--case", c.CaseID, "--prepared-digest", prep["prepared_digest"].(string), "--journal", journal, "--artifacts", artifacts, "--live")
		ownerQRRun(t, deps, 0, "review-qualified-capture", capArgs...)
	}
	if calls != 85 {
		t.Fatalf("native calls=%d", calls)
	}
	report := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-report", append(append([]string(nil), args...), "--journal", journal, "--artifacts", artifacts)...)
	if report["qualified"] != true || report["planned"] != float64(85) || report["completed"] != float64(85) || report["matched"] != float64(85) || report["inconclusive"] != float64(1) || report["invalid_count"] != float64(0) {
		t.Fatalf("complete result %v", report)
	}
	matrix := report["confusion_matrix"].(map[string]any)
	for label, want := range map[string]float64{"supported": 32, "unsupported": 52, "inconclusive": 1} {
		if matrix[label].(map[string]any)[label] != want {
			t.Fatalf("confusion %s=%v", label, matrix[label])
		}
	}
}

func TestOwnerQRCriticalMismatchStopsAndRetainsDenominator(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	if cases[0].Expected != "unsupported" {
		t.Fatal("planted first negative changed")
	}
	journal := filepath.Join(dir, "journal")
	artifacts := filepath.Join(dir, "artifacts")
	calls := 0
	deps := ownerQRDeps(t, cases, func(_ int, m map[string]any) { m["status"] = "supported" }, &calls)
	prep := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-prepare", append(append([]string(nil), args...), "--case", cases[0].CaseID)...)
	ownerQRRun(t, deps, 1, "review-qualified-capture", append(append([]string(nil), args...), "--case", cases[0].CaseID, "--prepared-digest", prep["prepared_digest"].(string), "--journal", journal, "--artifacts", artifacts, "--live")...)
	prep = ownerQRRun(t, reviewDeps{}, 0, "review-qualified-prepare", append(append([]string(nil), args...), "--case", cases[1].CaseID)...)
	ownerQRRun(t, deps, 2, "review-qualified-capture", append(append([]string(nil), args...), "--case", cases[1].CaseID, "--prepared-digest", prep["prepared_digest"].(string), "--journal", journal, "--artifacts", artifacts, "--live")...)
	if calls != 1 {
		t.Fatalf("advanced after critical mismatch: %d", calls)
	}
	r := ownerQRRun(t, reviewDeps{}, 1, "review-qualified-report", append(append([]string(nil), args...), "--journal", journal, "--artifacts", artifacts)...)
	if r["qualified"] != false || r["false_support"] != float64(1) || r["mismatched"] != float64(1) || r["missing"] != float64(84) {
		t.Fatalf("retained result %v", r)
	}
}

func TestOwnerQRPublicEntrypointAndNonEffects(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	code, b, diag := ownerOracleRun(context.Background(), append(append([]string{"review-qualified-prepare"}, args...), "--case", cases[0].CaseID))
	if code != 0 || len(diag) != 0 || ownerOracleJSON(t, b)["planned"] != float64(85) {
		t.Fatalf("routed prepare=%d %s", code, diag)
	}
	for _, cmd := range []string{"review-qualified-prepare", "review-qualified-capture", "review-qualified-report"} {
		code, b, diag = ownerOracleRun(context.Background(), []string{cmd, "--help"})
		if code != 0 || len(diag) != 0 || !bytes.Contains(b, []byte("--trial")) {
			t.Fatalf("help %s", cmd)
		}
	}
	for _, flag := range []string{"--live", "--journal", "--artifacts", "--prepared-digest"} {
		a := append(append([]string(nil), args...), "--case", cases[0].CaseID)
		if flag == "--live" {
			a = append(a, "--live=false")
		} else {
			a = append(a, flag, "")
		}
		ownerQRRun(t, reviewDeps{}, 2, "review-qualified-prepare", a...)
	}
	if _, err := os.Stat(filepath.Join(dir, "journal")); !os.IsNotExist(err) {
		t.Fatal("prepare created private state")
	}
}

func TestOwnerQRNewTrialCannotResetPinnedPrior(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	ownerFloorMutate(t, filepath.Join(dir, "trial.json"), func(m map[string]any) { m["prior_direct_micro_usd"] = 0 })
	ownerQRRun(t, reviewDeps{}, 2, "review-qualified-prepare", append(args, "--case", cases[0].CaseID)...)
}

func TestOwnerQRIdenticalRelocatedInputsPreserveIdentity(t *testing.T) {
	left, cases, _ := ownerQRSetup(t)
	right, _, _ := ownerQRSetup(t)
	a := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-prepare", append(left, "--case", cases[0].CaseID)...)
	b := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-prepare", append(right, "--case", cases[0].CaseID)...)
	for _, key := range []string{"study_digest", "native_request_digest", "prepared_digest"} {
		if a[key] != b[key] {
			t.Fatalf("relocated byte-identical inputs changed %s", key)
		}
	}
}

func TestOwnerQRPriorReceiptPinAndRequiredZeroLiterals(t *testing.T) {
	for _, field := range []string{"uncertain_reserved_micro_usd", "charged_micro_usd"} {
		for _, mode := range []string{"omit", "null", "changed"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				args, cases, dir := ownerQRSetup(t)
				ownerFloorMutate(t, filepath.Join(dir, "prior-spend.json"), func(m map[string]any) {
					switch mode {
					case "omit":
						delete(m, field)
					case "null":
						m[field] = nil
					case "changed":
						m[field] = 123
					}
				})
				// Even an explicitly reviewed new raw pin cannot excuse missing
				// required zero literals or a sum that contradicts its source rows.
				b, err := os.ReadFile(filepath.Join(dir, "prior-spend.json"))
				if err != nil {
					t.Fatal(err)
				}
				for i := range args {
					if args[i] == "--prior-spend-digest" {
						args[i+1] = ownerOracleHash(b)
					}
				}
				ownerQRRun(t, reviewDeps{}, 2, "review-qualified-prepare", append(args, "--case", cases[0].CaseID)...)
			})
		}
	}
}
