package scenariobench

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func verdictFixture(t *testing.T) (Corpus, []CellResult, []DryRunTaskVerdict) {
	t.Helper()
	data, err := os.ReadFile("testdata/dryrun/verdicts.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Task   string
		Null   []bool
		Oracle []bool
		Wrong  []bool `json:"wrong-patch"`
		Reason string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	corpus := Corpus{Arms: map[string]Arm{"control": {}}}
	var cells []CellResult
	var want []DryRunTaskVerdict
	for _, c := range cases {
		corpus.Tasks = append(corpus.Tasks, Task{ID: c.Task})
		want = append(want, DryRunTaskVerdict{TaskID: c.Task, Keep: c.Reason == "", Reason: c.Reason})
		for name, results := range map[string][]bool{"null": c.Null, "oracle": c.Oracle, "wrong-patch": c.Wrong} {
			for trial, pass := range results {
				cells = append(cells, CellResult{TaskID: c.Task, Arm: "control", Trial: trial + 1, HarnessVersion: name + "-1", Success: pass, Oracle: OracleResult{Passed: pass}, Sessions: []SessionResult{{}}})
			}
		}
	}
	return corpus, cells, want
}

func TestDryRunVerdict(t *testing.T) {
	corpus, cells, want := verdictFixture(t)
	// Explicit expected order: no dependence on corpus, directory or map ordering.
	want = []DryRunTaskVerdict{want[4], want[0], want[1], want[2], want[3]}
	got, err := DryRunVerdict(corpus, cells)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verdict = %+v, want %+v", got, want)
	}
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	again, err := DryRunVerdict(corpus, cells)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("nondeterministic: %+v %v", again, err)
	}
}

func TestDryRunVerdictRejectsIncompleteOrVoidCells(t *testing.T) {
	for _, kind := range []string{"missing", "duplicate", "void", "lost", "extra trial", "unmeasurable"} {
		t.Run(kind, func(t *testing.T) {
			corpus, cells, _ := verdictFixture(t)
			switch kind {
			case "missing":
				cells = cells[1:]
			case "duplicate":
				cells = append(cells, cells[0])
			case "void":
				cells[0].InstrumentDefect = "broken grader"
			case "lost":
				cells[0].Sessions[0].InstrumentError = "missing server"
			case "extra trial":
				cells[0].Trial = 4
			case "unmeasurable":
				cells[0].Oracle.Calibration = &CalibrationSummary{NotMeasurable: "broken tests"}
			}
			if _, err := DryRunVerdict(corpus, cells); err == nil {
				t.Fatal("invalid evidence earned a verdict")
			}
		})
	}
}

func TestDryRunVerdictFlakyWithinHarnessAndArm(t *testing.T) {
	corpus, cells, _ := verdictFixture(t)
	corpus.Tasks = corpus.Tasks[:1]
	corpus.Arms["other"] = Arm{}
	var selected []CellResult
	for _, c := range cells {
		if c.TaskID == "kept" {
			selected = append(selected, c)
			c.Arm = "other"
			if strings.HasPrefix(c.HarnessVersion, "null-") {
				c.Success = true
			}
			selected = append(selected, c)
		}
	}
	verdict, err := DryRunVerdict(corpus, selected)
	if err != nil || len(verdict) != 1 || verdict[0].Reason != "null passed" {
		t.Fatalf("arm differences are not flakiness: %+v %v", verdict, err)
	}
}

func TestDryRunVerdictSkipsDroppedTasks(t *testing.T) {
	corpus, cells, _ := verdictFixture(t)
	corpus.Tasks = append(corpus.Tasks, Task{ID: "already-dropped", Disposition: "dropped"})
	got, err := DryRunVerdict(corpus, cells)
	if err != nil || len(got) != 5 {
		t.Fatalf("dropped task requires nonexistent cells: %+v %v", got, err)
	}
}
