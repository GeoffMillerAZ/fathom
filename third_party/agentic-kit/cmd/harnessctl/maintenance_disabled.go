//go:build !harness_maintenance

package main

import (
	"fmt"
	"io"
)

func maintenanceUsage() string { return "harnessctl maintenance: unsupported in this build.\n" }
func runMaintenance(args []string, out io.Writer) int {
	operation := "unknown"
	if len(args) > 0 {
		switch args[0] {
		case "init", "prepare", "enter", "check", "end", "recover":
			operation = args[0]
		}
	}
	// This build has no maintenance receiver. Parse no paths, open no files and
	// invoke no native process, even when the caller supplied plausible flags.
	body := fmt.Sprintf("{\"version\":\"harnessctl.maintenance-report/1\",\"status\":\"hold\",\"operation\":%q,\"code\":\"unsupported\",\"noHostActionAuthority\":true,\"installation\":\"unobserved\",\"coverage\":\"selected-project-only\",\"stateWritesAttempted\":0,\"stateWritesCommitted\":0,\"nativeStampAttempts\":0,\"nativeStampInvoked\":0,\"nativeStampCompleted\":0,\"nativeApprovalAttempts\":0,\"nativeApprovalInvoked\":0,\"nativeApprovalCompleted\":0,\"nativeSignatureVerified\":false,\"storedSignatureVerified\":false}\n", operation)
	n, e := io.WriteString(out, body)
	if e != nil || n != len(body) {
		return 2
	}
	return 2
}
