package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	recordfingerprint "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/record-fingerprint"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

func TestDomainCollectors(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		collect      func() (report, error)
		wantScore    bool
	}{
		{"benchmark-cost-unknown", "unknown", func() (report, error) {
			return benchmarkReport(filepath.Join("..", "..", "docs", "evidence", "benchmarks", "2026-09-16", "consumer-receipt.json"))
		}, false},
		{"screen-and-effect-gate", "pass", func() (report, error) { return securityReport(true) }, true},
		{"disabled-effect-gate-negative-control", "fail", func() (report, error) { return securityReport(false) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := tc.collect()
			if err != nil {
				t.Fatal(err)
			}
			if string(r.Assessment.RequiredStatus) != tc.status || (r.Assessment.SoftScore != nil) != tc.wantScore {
				t.Fatalf("status=%s score=%v; want %s, score present=%v", r.Assessment.RequiredStatus, r.Assessment.SoftScore, tc.status, tc.wantScore)
			}
			if tc.wantScore && *r.Assessment.SoftScore != 1 {
				t.Fatal("benign counter did not execute")
			}
			for digest, content := range r.Artifacts {
				if recordfingerprint.HashContent([]byte(content)) != digest {
					t.Fatal("artifact digest mismatch")
				}
			}
			for _, o := range r.Observations {
				for _, ref := range o.ArtifactDigests {
					if _, ok := r.Artifacts[ref]; !ok {
						t.Fatalf("missing artifact %s", ref)
					}
				}
			}
			var roundTrip report
			wire, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(wire, &roundTrip); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := fitness.AssessProfile(roundTrip.Profile, roundTrip.Subject, roundTrip.Observations, observed)
			if err != nil || !reflect.DeepEqual(rebuilt, r.Assessment) {
				t.Fatalf("cannot reconstruct assessment: %v", err)
			}
			validateWire(t, wire)
		})
	}
}

func TestNegativeControlCannotHideBehindBenignScore(t *testing.T) {
	r, err := securityReport(false)
	if err != nil {
		t.Fatal(err)
	}
	facts := map[string]fitness.FindingOutcome{}
	for _, trace := range r.Assessment.Trace {
		facts[trace.CriterionID] = trace.Outcome
	}
	if facts["benign-utility"] != "pass" || facts["independent-authorization"] != "fail" || r.Assessment.SoftScore != nil {
		t.Fatalf("unsafe effects masked: %+v", r.Assessment)
	}
}

func TestDomainCollectorsRepeatDeterministically(t *testing.T) {
	a, err := securityReport(true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := securityReport(true)
	if err != nil {
		t.Fatal(err)
	}
	wa, _ := json.Marshal(a)
	wb, _ := json.Marshal(b)
	if !bytes.Equal(wa, wb) {
		t.Fatal("synthetic collector output drifted")
	}
}

func validateWire(t *testing.T, wire []byte) {
	t.Helper()
	if _, err := exec.LookPath("cue"); err != nil {
		t.Fatal("cue is required for the example wire contract")
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, wire, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cue", "vet", "schema.cue", path, "-d", "#Report")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("CUE wire mismatch: %v\n%s", err, output)
	}
}
