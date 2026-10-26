package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerQRV3RefusesNewArtifactFields(t *testing.T) {
	args, cases, dir := ownerQRSetup(t)
	in, err := loadQualifiedInputs(context.Background(), filepath.Join(dir, "trial.json"), filepath.Join(dir, "prior-spend.json"), args[len(args)-1], args[:len(args)-6])
	if err != nil {
		t.Fatal(err)
	}
	id := cases[0].CaseID
	journal, artifacts := filepath.Join(dir, "journal"), filepath.Join(dir, "artifacts")
	calls := 0
	var out, diag bytes.Buffer
	if code := qualifiedRunCapture(context.Background(), in, in.cases[id], in.wires[id], qualifiedPreparedDigest(in, id, in.wires[id]), journal, artifacts, false, &out, &diag, ownerQRDeps(t, cases, nil, &calls)); code != 0 {
		t.Fatalf("initial capture=%d: %s", code, diag.String())
	}
	var r reservation
	b, err := os.ReadFile(filepath.Join(journal, "reserve-000001.json"))
	if err != nil || json.Unmarshal(b, &r) != nil {
		t.Fatal("reservation unavailable")
	}
	var d qualifiedDecision
	b, err = os.ReadFile(filepath.Join(artifacts, id+".reason.json"))
	if err != nil || json.Unmarshal(b, &d) != nil {
		t.Fatal("reason unavailable")
	}
	root, err := os.OpenRoot(artifacts)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := qualifiedArtifactCheck(root, in, id, r, d, 2, 2); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		suffix, key string
		value       any
	}{{"started", "protocol", ""}, {"started", "protocol", nil}, {"receipt", "protocol", ""}, {"receipt", "protocol", nil}, {"reason", "examined_fact_ids", nil}} {
		t.Run(tc.suffix+"/"+tc.key+"/"+string(jsonBytes(tc.value)), func(t *testing.T) {
			path := filepath.Join(artifacts, id+"."+tc.suffix+".json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Error(err)
				}
			})
			ownerFloorMutate(t, path, func(m map[string]any) { m[tc.key] = tc.value })
			if err := qualifiedArtifactCheck(root, in, id, r, d, 2, 2); err == nil {
				t.Fatal("protocol 3 accepted a new artifact field")
			}
		})
	}
}

// These tests exercise the selected native workflow, not semantic accuracy.
// Load the immutable floor once; every capture still replays its entire history.
func ownerQRV4Inputs(t *testing.T) ([]string, []ownerQRCase, string, qualifiedInputs) {
	t.Helper()
	args, cases, dir := ownerQRSetup(t)
	ownerFloorMutate(t, filepath.Join(dir, "trial.json"), func(m map[string]any) {
		m["version"] = "prose-lab.qualified-review-study/2"
		m["review_protocol"] = "prose-lab.support-review/4"
	})
	in, err := loadQualifiedInputs(context.Background(), filepath.Join(dir, "trial.json"), filepath.Join(dir, "prior-spend.json"), args[len(args)-1], args[:len(args)-6])
	if err != nil {
		t.Fatal(err)
	}
	return args, cases, dir, in
}

func TestOwnerQRV4CompleteNativeReplayAndPrivacy(t *testing.T) {
	args, cases, dir, in := ownerQRV4Inputs(t)
	journal, artifacts := filepath.Join(dir, "journal"), filepath.Join(dir, "artifacts")
	calls := 0
	deps := ownerQRDeps(t, cases, func(i int, answer map[string]any) {
		c := in.cases[cases[i].CaseID]
		ids := make([]string, 0, len(c.Facts))
		for _, f := range c.Facts {
			ids = append(ids, f.ID)
		}
		answer["examined_fact_ids"] = ids
		if c.Expected != "supported" {
			answer["premise_fact_ids"] = []string{}
		}
	}, &calls)
	for _, id := range in.ordered {
		var out, diag bytes.Buffer
		if code := qualifiedRunCapture(context.Background(), in, in.cases[id], in.wires[id], qualifiedPreparedDigest(in, id, in.wires[id]), journal, artifacts, false, &out, &diag, deps); code != 0 {
			t.Fatalf("capture %s=%d: %s", id, code, diag.String())
		}
		if bytes.Contains(out.Bytes(), []byte("OWNER_PRIVATE_QR_REASON")) || bytes.Contains(out.Bytes(), []byte(id)) {
			t.Fatal("capture content leak")
		}
	}
	if calls != 85 {
		t.Fatalf("native attempts %d", calls)
	}
	reportArgs := append(append([]string(nil), args...), "--journal", journal, "--artifacts", artifacts)
	r := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-report", reportArgs...)
	if r["protocol"] != "prose-lab.support-review/4" || r["qualified"] != true || r["matched"] != float64(85) || r["inconclusive"] != float64(1) || r["missing"] != float64(0) || r["charged_micro_usd"] != float64(170) {
		t.Fatalf("cohort/accounting %v", r)
	}
	for _, raw := range r["rows"].([]any) {
		row := raw.(map[string]any)
		for _, key := range []string{"decision", "case_id", "assertion", "facts", "reason", "expected"} {
			if _, ok := row[key]; ok {
				t.Fatalf("private row field %s", key)
			}
		}
	}
	full := ownerQRRun(t, reviewDeps{}, 0, "review-qualified-report", append(reportArgs, "--include-content")...)
	for _, raw := range full["rows"].([]any) {
		decision := raw.(map[string]any)["decision"].(map[string]any)
		if len(decision["examined_fact_ids"].([]any)) == 0 {
			t.Fatal("full report dropped examined sources")
		}
	}
	// A protocol marker cannot be removed without invalidating replay.
	ownerFloorMutate(t, filepath.Join(artifacts, cases[0].CaseID+".started.json"), func(m map[string]any) { delete(m, "protocol") })
	ownerQRRun(t, reviewDeps{}, 2, "review-qualified-report", reportArgs...)
}

func TestOwnerQRV4TerminalStatesRetainAllPositions(t *testing.T) {
	for _, mode := range []string{"false-support", "invalid", "pre-dispatch"} {
		t.Run(mode, func(t *testing.T) {
			args, cases, dir, in := ownerQRV4Inputs(t)
			journal, artifacts := filepath.Join(dir, "journal"), filepath.Join(dir, "artifacts")
			calls := 0
			deps := ownerQRDeps(t, cases, func(i int, answer map[string]any) {
				answer["examined_fact_ids"] = cases[i].Cited
				if mode == "false-support" {
					answer["status"] = "supported"
				} else {
					answer["examined_fact_ids"] = []string{}
				}
			}, &calls)
			wantCapture, wantReport, counter := 2, 2, "invalid_count"
			if mode == "false-support" {
				wantCapture, wantReport, counter = 1, 1, "false_support"
			}
			if mode == "pre-dispatch" {
				deps.apiKey = ""
				t.Setenv("ANTHROPIC_API_KEY", "")
				counter = "failed"
			}
			capture := func(id string) int {
				var out, diag bytes.Buffer
				return qualifiedRunCapture(context.Background(), in, in.cases[id], in.wires[id], qualifiedPreparedDigest(in, id, in.wires[id]), journal, artifacts, false, &out, &diag, deps)
			}
			if got := capture(cases[0].CaseID); got != wantCapture {
				t.Fatalf("initial capture=%d want%d", got, wantCapture)
			}
			if got := capture(cases[1].CaseID); got != 2 {
				t.Fatalf("continued terminal trial=%d", got)
			}
			wantCalls := 1
			if mode == "pre-dispatch" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d want%d", calls, wantCalls)
			}
			r := ownerQRRun(t, reviewDeps{}, wantReport, "review-qualified-report", append(args, "--journal", journal, "--artifacts", artifacts)...)
			if r["qualified"] != false || r[counter] != float64(1) || r["planned"] != float64(85) || r["missing"] != float64(84) {
				t.Fatalf("denominator %v", r)
			}
			if mode == "pre-dispatch" {
				if _, err := os.Stat(filepath.Join(journal, "reserve-000001.json")); !os.IsNotExist(err) {
					t.Fatal("pre-dispatch failure reserved funds")
				}
			}
		})
	}
}
