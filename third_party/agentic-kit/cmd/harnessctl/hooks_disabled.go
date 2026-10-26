//go:build !harness_maintenance

package main

import (
	"fmt"
	"io"
)

func hooksUsage() string { return "harnessctl hooks: unsupported in this build.\n" }
func runHooks(args []string, out, errOut io.Writer) int {
	operation := "evaluate"
	if len(args) > 0 && args[0] == "run" {
		operation = "run"
	}
	if operation == "run" {
		if errOut == nil {
			return 2
		}
		msg := "harnessctl spawn check blocked\n"
		n, e := io.WriteString(errOut, msg)
		if e != nil || n != len(msg) {
			return 2
		}
		return 2
	}
	if out == nil {
		return 2
	}
	body := `{"version":"harnessctl.spawn-report/1","status":"hold","code":"unsupported","requiredDecision":"","codes":[],"class":"","classSource":"","advisorySelected":false,"advisoryEvaluated":false,"adviceMatches":0,"noActionAuthority":true,"coverage":"selected-project-spawn-events-only","installation":"unobserved","stateWritesAttempted":0,"stateWritesCommitted":0,"nativeStampAttempts":0,"nativeStampInvoked":0,"nativeStampCompleted":0,"evaluatorAttempts":0,"evaluatorInvoked":0,"evaluatorCompleted":0}` + "\n"
	n, e := fmt.Fprint(out, body)
	if e != nil || n != len(body) {
		return 2
	}
	return 2
}
