package scenariobench

import (
	"strings"
	"testing"
)

func TestDryRunVerdictRejectsEmptySelection(t *testing.T) {
	corpus := Corpus{Arms: map[string]Arm{"control": {}}, Tasks: []Task{{ID: "dropped", Disposition: "dropped"}}}
	verdicts, err := DryRunVerdict(corpus, nil)
	if err == nil || !strings.Contains(err.Error(), "selected tasks") || verdicts != nil {
		t.Fatalf("empty selection earned verdicts: %+v %v", verdicts, err)
	}
}
