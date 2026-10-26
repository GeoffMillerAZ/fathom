package scenariobench

import (
	"fmt"
	"strings"
	"testing"
)

func TestDryRunVerdictRejectsControlTurnsAndCost(t *testing.T) {
	for _, version := range []string{"null-1", "oracle-1", "wrong-patch-1"} {
		for _, kind := range []string{"total turns", "session turns", "total cost", "session cost"} {
			t.Run(version+"/"+kind, func(t *testing.T) {
				corpus, cells, _ := verdictFixture(t)
				var target *CellResult
				for i := range cells {
					if cells[i].HarnessVersion == version {
						target = &cells[i]
						break
					}
				}
				switch kind {
				case "total turns":
					target.Readings.TotalTurns = 1
				case "session turns":
					target.Sessions = append(target.Sessions, SessionResult{Turns: 1})
				case "total cost":
					target.Readings.TotalCostUSD = 0.01
				case "session cost":
					target.Sessions = append(target.Sessions, SessionResult{CostUSD: 0.01})
				}
				verdicts, err := DryRunVerdict(corpus, cells)
				want := fmt.Sprintf("%s/%s/%s/t%d: void:", target.TaskID, target.Arm, version, target.Trial)
				reason := "turns"
				if strings.Contains(kind, "cost") {
					reason = "cost"
				}
				if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), reason) || verdicts != nil {
					t.Fatalf("untrusted control not voided with identity and reason: %+v %v", verdicts, err)
				}
			})
		}
	}
}
