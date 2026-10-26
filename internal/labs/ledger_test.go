package labs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/internal/verify"
	labsschema "github.com/geoffmilleraz/fathom/schema/labs"
)

func resultFixture(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "schema", "labs", "testdata", "valid.cue"))
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileString(labsschema.Source + "\n" + strings.Replace(string(b), "package labs", "", 1)).LookupPath(cue.ParsePath("result"))
	var row map[string]any
	if err := v.Decode(&row); err != nil {
		t.Fatal(err)
	}
	row["evaluation"].(map[string]any)["verdict"] = "pass"
	return row
}

func TestAppendLabResultLedgerOracle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docs", "bench", "labs-ledger.jsonl")
	row := resultFixture(t)
	if err := AppendLabResult(path, row); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(before), `{"run_id":"find-symbol-v1","task_id":"find-definition","arm":"replay","trials":3,"evaluation":`) {
		t.Fatalf("ledger keys not first: %s", before)
	}
	var got map[string]any
	if err := json.Unmarshal(before, &got); err != nil {
		t.Fatal(err)
	}
	if len(got["stages"].(map[string]any)) != 5 {
		t.Fatal("funnel stages lost")
	}
	md := `- [x] Recorded {id: LAB-TEST, oracle: ledger, path: docs/bench/labs-ledger.jsonl, run_id: find-symbol-v1, task: find-definition, arm: replay, criterion: firing, min_trials: 3}`
	for _, tc := range []struct {
		name, md string
		want     verify.CriterionStatus
	}{
		{"exact", md, verify.Pass},
		{"other-run", strings.Replace(md, "run_id: find-symbol-v1", "run_id: another", 1), verify.Fail},
		{"other-task", strings.Replace(md, "task: find-definition", "task: another", 1), verify.Fail},
		{"other-arm", strings.Replace(md, "arm: replay", "arm: another", 1), verify.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := verify.ParseMarkdownPlan("fixture", []byte(tc.md))
			report := verify.Report{Tasks: []verify.TaskVerdict{{TaskID: "LAB-TEST"}}}
			verify.OverlayLocal(&report, plan, verify.DirFiles(dir))
			if len(report.Tasks) != 1 || report.Tasks[0].Status != tc.want {
				t.Fatalf("ledger oracle: %+v", report)
			}
		})
	}
	if err := AppendLabResult(path, row); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate accepted: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("duplicate append changed ledger")
	}
	// Each component of the key is independent.
	for _, field := range []string{"run_id", "task_id", "arm"} {
		m := resultFixture(t)
		m[field] = "other-" + field
		if err := AppendLabResult(path, m); err != nil {
			t.Fatalf("distinct %s: %v", field, err)
		}
	}
}

func TestAppendLabResultSchemaRefusals(t *testing.T) {
	for _, field := range []string{"run_id", "task_id", "arm", "trials", "evaluation", "stages", "prereg_sha"} {
		t.Run(field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ledger.jsonl")
			m := resultFixture(t)
			delete(m, field)
			if err := AppendLabResult(path, m); err == nil {
				t.Fatal("incomplete result accepted")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("invalid row created ledger: %v", err)
			}
		})
	}
	for _, verdict := range []string{"confirmed", "refuted", "win"} {
		t.Run(verdict, func(t *testing.T) {
			m := resultFixture(t)
			m["evaluation"].(map[string]any)["verdict"] = verdict
			if err := AppendLabResult(filepath.Join(t.TempDir(), "ledger.jsonl"), m); err == nil {
				t.Fatal("non-schema verdict accepted")
			}
		})
	}
	for _, verdict := range []string{"pass", "fail", "indeterminate"} {
		t.Run(verdict, func(t *testing.T) {
			m := resultFixture(t)
			m["evaluation"].(map[string]any)["verdict"] = verdict
			m["instrument_defect"] = "fixture defect"
			m["surface_unused"] = true
			path := filepath.Join(t.TempDir(), "ledger.jsonl")
			if err := AppendLabResult(path, m); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			last := -1
			for _, key := range []string{"run_id", "task_id", "arm", "trials", "evaluation", "surface_unused", "instrument_defect", "attributes"} {
				i := strings.Index(string(data), `"`+key+`":`)
				if i <= last {
					t.Fatalf("key order at %s: %s", key, data)
				}
				last = i
			}
		})
	}
}

func TestAppendLabResultExistingLedger(t *testing.T) {
	for _, existing := range []string{"{bad\n", "{}\n", "null\n", `{ "run_id":"old", "task_id":"task", "arm":"replay" }` + "\n"} {
		t.Run(existing, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ledger.jsonl")
			if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
				t.Fatal(err)
			}
			if err := AppendLabResult(path, resultFixture(t)); err == nil {
				t.Fatal("corrupt existing ledger accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != existing {
				t.Fatal("corrupt ledger changed")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	m := resultFixture(t)
	m["run_id"] = "old"
	m["attributes"].(map[string]any)["large"] = strings.Repeat("x", 128*1024)
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := AppendLabResult(path, resultFixture(t)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two separate JSONL rows, got %d", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal("joined or invalid row")
		}
	}
}

func TestAppendLabResultConcurrentDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	m := resultFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errs <- AppendLabResult(path, m) })
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !strings.Contains(err.Error(), "duplicate") {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal(fmt.Sprintf("want exactly one successful append, got %d", successes))
	}
}
