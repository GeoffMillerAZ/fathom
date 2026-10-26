//go:build !command_classifier

package main

const classifierBuildEnabled = false
const classifierCapabilityLine = "Concern classifier: unavailable (build with -tags command_classifier).\n"

func validClassifierControls(s scope, p policy) bool { return !selectedClassifierControls(s, p) }
func classifySelected(_ classifierPolicy, _ [][]byte, r *classifierReceipt) string {
	if r != nil {
		r.Status = "incomplete"
		r.Code = "execution_incomplete"
	}
	return "incomplete"
}
