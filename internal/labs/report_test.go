package labs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewestLabResultsMissingFile(t *testing.T) {
	rows, err := NewestLabResults(filepath.Join(t.TempDir(), "docs", "bench", "labs-ledger.jsonl"))
	if err != nil || len(rows) != 0 {
		t.Fatalf("missing ledger = %v, %v; want empty and no error", rows, err)
	}
}

func TestNewestLabResultsLastRowWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labs-ledger.jsonl")
	ledger := strings.Join([]string{
		`{"run_id":"r1","task_id":"t1","arm":"replay","lab":"lab-search","evaluation":{"verdict":"fail"}}`,
		`{"run_id":"r1","task_id":"t1","arm":"replay","lab":"lab-find-symbol","evaluation":{"verdict":"pass"}}`,
		``,
		`{"run_id":"r2","task_id":"t2","arm":"plugin","lab":"lab-search","evaluation":{"verdict":"indeterminate"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(ledger), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := NewestLabResults(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []LabResultRow{
		{RunID: "r1", TaskID: "t1", Arm: "replay", Lab: "lab-find-symbol", Verdict: "pass"},
		{RunID: "r2", TaskID: "t2", Arm: "plugin", Lab: "lab-search", Verdict: "indeterminate"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
}

func TestNewestLabResultsNamesMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labs-ledger.jsonl")
	ledger := `{"run_id":"r1","task_id":"t1","arm":"replay","lab":"lab-search","evaluation":{"verdict":"pass"}}` + "\n{torn\n" +
		`{"run_id":"r2","task_id":"t2","arm":"replay","evaluation":{"verdict":"pass"}}` + "\n"
	if err := os.WriteFile(path, []byte(ledger), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := NewestLabResults(path)
	if err == nil || !strings.Contains(err.Error(), "labs-ledger.jsonl:2:") || !strings.Contains(err.Error(), "labs-ledger.jsonl:3: no lab") {
		t.Fatalf("err = %v, want lines 2 and 3 named", err)
	}
	if len(rows) != 1 || rows[0].Lab != "lab-search" {
		t.Fatalf("rows = %+v, want the readable row kept", rows)
	}
}

func writeCase(t *testing.T, dir, lab, name, body string) {
	t.Helper()
	cases := filepath.Join(dir, lab, "cases")
	if err := os.MkdirAll(cases, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cases, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCountLabCasesBySplit(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "lab-search", "a.json", `{"split":"train","input":{}}`)
	writeCase(t, dir, "lab-search", "b.json", `{"split":"train"}`)
	writeCase(t, dir, "lab-search", "c.json", `{"split":"confirm"}`)
	writeCase(t, dir, "lab-search", "notes.md", `not a case`)
	writeCase(t, dir, "lab-blast-radius", "a.json", `{"split":"select"}`)
	counts, err := CountLabCases(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []LabCaseCount{
		{Lab: "lab-blast-radius", Select: 1},
		{Lab: "lab-search", Train: 2, Confirm: 1},
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
}

func TestCountLabCasesMissingDirectory(t *testing.T) {
	counts, err := CountLabCases(filepath.Join(t.TempDir(), "labs.d"))
	if err != nil || len(counts) != 0 {
		t.Fatalf("missing dir = %v, %v; want empty and no error", counts, err)
	}
}

func TestCountLabCasesNamesMalformedCase(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "lab-search", "a.json", `{"split":"train"}`)
	writeCase(t, dir, "lab-search", "b.json", "{\n\"split\": \"train\",\n}")
	writeCase(t, dir, "lab-search", "c.json", `{"split":"holdout"}`)
	counts, err := CountLabCases(dir)
	if err == nil || !strings.Contains(err.Error(), filepath.ToSlash(filepath.Join("lab-search", "cases", "b.json"))+":3:") {
		t.Fatalf("err = %v, want b.json line 3 named", err)
	}
	if !strings.Contains(err.Error(), `c.json:1: unknown split "holdout"`) {
		t.Fatalf("err = %v, want c.json's split named", err)
	}
	if want := []LabCaseCount{{Lab: "lab-search", Train: 1}}; !reflect.DeepEqual(counts, want) {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
}

func TestFixProofNotMeasured(t *testing.T) {
	if FixProofNotMeasured != "not measured (LAB-24 unbuilt)" {
		t.Fatalf("FixProofNotMeasured = %q", FixProofNotMeasured)
	}
}
