package scenariobench

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewWrongPatchHarness(t *testing.T) {
	corpus, _, _ := dryFixture(t)
	h := NewWrongPatchHarness()
	if h.Name() != "wrong-patch" || h.Version() != "wrong-patch-1" {
		t.Fatal("wrong identity")
	}
	result := runDryFixture(t, corpus, h)
	if len(result.Lost) != 0 || len(result.Cells) != 2 {
		t.Fatalf("not measured: %+v", result)
	}
	for _, cell := range result.Cells {
		if cell.Success || cell.Oracle.Passed || CellLost(cell.Sessions, h) != "" {
			t.Fatalf("wrong patch did not fail: %+v", cell)
		}
		if len(cell.Sessions) != 1 || cell.Sessions[0].Turns != 0 || cell.Readings.TotalCostUSD != 0 {
			t.Fatal("not zero-turn and free")
		}
		if cell.TaskID == "F1" {
			if cell.Oracle.Localization.HitAt1 || cell.Oracle.Localization.HitAt5 {
				t.Fatal("non-gold files hit gold")
			}
			if got := ParseAnswerFiles(cell.Sessions[0].FinalAnswer); !reflect.DeepEqual(got, []string{"go.mod", "marker.txt", "thing_test.go"}) {
				t.Fatalf("not deterministic first three non-gold files: %v", got)
			}
		} else {
			if cell.Oracle.FailToPassOK || !cell.Oracle.PassToPassOK || cell.Oracle.Calibration == nil || cell.Oracle.Calibration.NotMeasurable != "" {
				t.Fatalf("hidden tests not a measured FAIL: %+v", cell.Oracle)
			}
			if b, err := os.ReadFile(filepath.Join(cell.WorktreePath, "thing_fix_test.go")); err != nil || len(b) == 0 {
				t.Fatal("test patch not applied")
			}
			if !reflect.DeepEqual(cell.Sessions[0].FilesEdited, []string{"thing_fix_test.go"}) {
				t.Fatal("wrong patch must edit only tests")
			}
		}
	}
}
