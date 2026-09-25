//go:build workflowprobe && (server || !darwin || !appkit_contained)

package main

import "context"

func workflowNativeMain(_ context.Context, h *host, m *workflowMeter, r *workflowReceipt) {
	r.Failure = "native workflow requires prepared macOS HTTP-only build"
	finishWorkflowHost(h, m, r)
	writeWorkflowReceipt(*r)
}
