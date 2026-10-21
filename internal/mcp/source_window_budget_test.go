package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func syntheticWindowReport(rows int, evidence string) map[string]any {
	tasks := make([]any, rows)
	for i := range tasks {
		tasks[i] = map[string]any{
			"id": fmt.Sprintf("T-%02d", i+1), "title": "synthetic row",
			"status": "pass", "criteria": []any{}, "evidence": evidence,
		}
	}
	return map[string]any{
		"service": "svc", "summary": map[string]any{"total": rows, "pass": rows, "fail": 0, "unknown": 0},
		"metadata": strings.Repeat("m", 6200), "tasks": tasks,
	}
}

func TestFitSourceWindowReportCountsCompleteEscapedEnvelope(t *testing.T) {
	const budget = defaultCharBudget - 256
	out := syntheticWindowReport(31, strings.Repeat("<>&\n\"", 115))
	final, raw, err := fitSourceWindowReport(out, budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > budget {
		t.Fatalf("final report %d > %d bytes", len(raw), budget)
	}
	if final["metadata"] != out["metadata"] || !reflect.DeepEqual(final["summary"], out["summary"]) {
		t.Fatal("fixed report fields changed")
	}
	kept := final["tasks"].([]any)
	if len(kept) == 0 || len(kept) >= 31 {
		t.Fatalf("task prefix = %d of 31", len(kept))
	}
	for i, item := range kept {
		if item.(map[string]any)["id"] != out["tasks"].([]any)[i].(map[string]any)["id"] {
			t.Fatalf("task prefix changed at %d", i)
		}
	}
	el := final["elided"].(*elided)
	if el.Count != 31-len(kept) || el.Of != 31 || el.Ordering != "plan order" || el.HowToGetMore == "" {
		t.Fatalf("elision = %+v", el)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded["tasks"].([]any)) != len(kept) {
		t.Fatalf("saved report disagrees with selected prefix: %v", err)
	}
}

func TestFitSourceWindowReportExactBoundaryAndPassthrough(t *testing.T) {
	out := syntheticWindowReport(2, strings.Repeat("x", 90))
	full, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	got, raw, err := fitSourceWindowReport(out, len(full))
	if err != nil || string(raw) != string(full) || got["elided"] != nil {
		t.Fatalf("exact boundary changed result: size=%d err=%v", len(raw), err)
	}
	got["identity_probe"] = true
	if out["identity_probe"] != true {
		t.Fatal("under-budget report was copied")
	}
	delete(out, "identity_probe")
	cut, raw, err := fitSourceWindowReport(out, len(full)-1)
	if err != nil || len(raw) > len(full)-1 || len(cut["tasks"].([]any)) != 1 {
		t.Fatalf("one byte below boundary did not retain one fitting row: size=%d err=%v", len(raw), err)
	}
}

func TestFitSourceWindowReportCombinesExistingTaskElision(t *testing.T) {
	out := syntheticWindowReport(31, strings.Repeat("e", 560))
	out["summary"] = map[string]any{"total": 35, "pass": 35, "fail": 0, "unknown": 0}
	out["elided"] = &elided{Count: 4, Of: 35, Ordering: "plan order", HowToGetMore: "existing follow-up guidance"}
	final, raw, err := fitSourceWindowReport(out, defaultCharBudget-256)
	if err != nil || len(raw) > defaultCharBudget-256 {
		t.Fatalf("combined result size=%d err=%v", len(raw), err)
	}
	kept := len(final["tasks"].([]any))
	e := final["elided"].(*elided)
	if kept < 1 || kept >= 31 || e.Count != 35-kept || e.Of != 35 || e.Ordering != "plan order" || e.HowToGetMore != "existing follow-up guidance" {
		t.Fatalf("combined elision=%+v kept=%d", e, kept)
	}
	if len(out["tasks"].([]any)) != 31 || out["elided"].(*elided).Count != 4 || out["elided"].(*elided).Of != 35 {
		t.Fatal("source report or prior receipt was mutated")
	}
}

func TestFitSourceWindowReportRefusesMalformedPriorTaskElision(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rows     int
		evidence string
	}{
		{"under budget", 2, "short"},
		{"over budget", 31, strings.Repeat("e", 560)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := syntheticWindowReport(tc.rows, tc.evidence)
			out["elided"] = &elided{Count: 4, Of: 4 + tc.rows - 1, Ordering: "plan order", HowToGetMore: "follow up"}
			if _, _, err := fitSourceWindowReport(out, defaultCharBudget-256); err == nil || !strings.Contains(err.Error(), "invalid task elision receipt") {
				t.Fatalf("inconsistent prior receipt should refuse, err=%v", err)
			}
		})
	}
}

func TestFitSourceWindowReportRefusesImpossibleFirstRow(t *testing.T) {
	out := syntheticWindowReport(2, strings.Repeat("x", 500))
	out["metadata"] = strings.Repeat("m", defaultCharBudget)
	if _, _, err := fitSourceWindowReport(out, defaultCharBudget-256); err == nil || !strings.Contains(err.Error(), "exceeds response budget") {
		t.Fatalf("impossible first row should refuse, err=%v", err)
	}
}
