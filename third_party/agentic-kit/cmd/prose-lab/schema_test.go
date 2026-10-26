package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCanaryStudyGoCUEParity(t *testing.T) {
	bin, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("cue is required by task validate; absent in this focused environment")
	}
	for _, kind := range []string{"valid", "no-cases", "duplicate-cases", "no-lanes", "unpriced-input", "private-dataset", "bad-budget"} {
		t.Run(kind, func(t *testing.T) {
			s := canaryStudy{Version: "prose-lab.canary/1", ID: "study", DatasetKind: "synthetic", BudgetMicroUSD: 200, Lanes: map[string]lane{"decision": testLane(100)}, ExpectedCaseIDs: []string{"case-a"}}
			switch kind {
			case "no-cases":
				s.ExpectedCaseIDs = []string{}
			case "duplicate-cases":
				s.ExpectedCaseIDs = []string{"case-a", "case-a"}
			case "no-lanes":
				s.Lanes = map[string]lane{}
			case "unpriced-input":
				l := s.Lanes["decision"]
				l.InputRate = 0
				s.Lanes["decision"] = l
			case "private-dataset":
				s.DatasetKind = "private"
			case "bad-budget":
				s.BudgetMicroUSD = 10_000_001
			}
			data := filepath.Join(t.TempDir(), "study.json")
			if err := os.WriteFile(data, jsonBytes(s), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "vet", "schema.cue", data, "-d", "#CanaryStudy", "-c")
			output, err := cmd.CombinedOutput()
			goOK := s.validate() == nil
			cueOK := err == nil
			if goOK != (kind == "valid") || cueOK != goOK {
				t.Fatalf("Go=%v CUE=%v: %s", goOK, cueOK, output)
			}
		})
	}
}
