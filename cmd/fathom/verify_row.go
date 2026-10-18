package main

// verify_row.go — `fathom verify --row` (OC-23): one row's verdict, gated
// alone. The console's row evidence carries this verb's JSON as its
// section, built by the same two functions cmdVerify's --json path uses, so
// the drawer and the verb cannot disagree (P10).

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// verifyGateFailures computes the gate over reports: every failing (and,
// with strict, undecidable) claimed row; with requireAll, open rows too.
func verifyGateFailures(reports []verify.Report, requireAll, strict bool) []string {
	var allFail []string
	for _, rep := range reports {
		var gr verify.GateResult
		if requireAll {
			gr = verify.GateRequireAll(rep, strict)
		} else {
			gr = verify.Gate(rep, strict)
		}
		allFail = append(allFail, gr.Failures...)
	}
	return allFail
}

// verifyJSONDoc is `fathom verify --json`'s document.
func verifyJSONDoc(reports []verify.Report, failures []string) map[string]any {
	return map[string]any{
		"gate": map[string]any{
			"pass":     len(failures) == 0,
			"failures": failures,
		},
		"reports": reportsToMaps(reports),
	}
}

// narrowReportsToRow keeps only the row with id in each report (the row's
// own plan; a report without it keeps no task and is dropped), with each
// summary recounted over what is kept.
func narrowReportsToRow(reports []verify.Report, id string) ([]verify.Report, bool) {
	var out []verify.Report
	for _, rep := range reports {
		for _, tv := range rep.Tasks {
			if tv.TaskID != id {
				continue
			}
			n := rep
			n.Tasks = []verify.TaskVerdict{tv}
			n.Summary = verify.Summary{Total: 1}
			switch tv.Status {
			case verify.Pass:
				n.Summary.Pass = 1
			case verify.Fail:
				n.Summary.Fail = 1
			default:
				n.Summary.Unknown = 1
			}
			out = append(out, n)
			break
		}
	}
	return out, len(out) > 0
}

// indexStaleLine is the one aggregate line for a failed gone: precondition
// (O1160): the reason, the first dirty path when there is one, how many
// claimed rows it holds back, and the remedy.
func indexStaleLine(a verify.IndexStale) string {
	line := fmt.Sprintf("index-stale: %s", a.Reason)
	if a.FirstPath != "" {
		line += fmt.Sprintf(" (first dirty path: %s)", a.FirstPath)
	}
	return fmt.Sprintf("%s; %d claimed gone row(s) held back, still unknown (I44); %s", line, a.Rows, a.Remedy)
}
