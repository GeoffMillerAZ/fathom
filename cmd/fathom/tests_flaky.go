package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gatereceipt"
)

func cmdTestsFlaky(args []string, stdout, stderr io.Writer) int {
	return runTestsFlaky(args, time.Now, stdout, stderr)
}

func runTestsFlaky(args []string, now func() time.Time, stdout, stderr io.Writer) int {
	fs := newFlagSet("tests flaky", stderr)
	jsonOut := fs.Bool("json", false, "emit JSON (lost_ns is recorded gate duration in nanoseconds)")
	since := fs.Duration("since", 0, "include runs ending within this duration; exclude undated runs (default: all history)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 || *since < 0 {
		fmt.Fprintln(stderr, "usage: fathom tests flaky [--json] [--since <duration>]")
		return 1
	}
	options, _, err := gateReceiptOptions(context.Background(), ".", "validate")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	cutoff := time.Time{}
	if *since > 0 {
		cutoff = now().Add(-*since)
	}
	report, err := gatereceipt.FlakyTests([]gatereceipt.FlakyDir{
		{Path: options.Dir, Role: gatereceipt.FlakyReceipts},
		{Path: filepath.Join(options.Root, "workspace/logs"), Role: gatereceipt.FlakyGateLogs},
		{Path: filepath.Join(options.Root, testRunSavedDir), Role: gatereceipt.FlakySavedRuns},
	}, cutoff)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Same-tree fail-then-pass tests: %d; unknown identity: %d run(s)\n", len(report.Tests), report.UnknownIdentity)
	for _, test := range report.Tests {
		fmt.Fprintf(stdout, "%s %s: %s gate time lost (%d failing run(s))\n", test.Package, test.Test, test.Lost, test.FailedRuns)
	}
	return 0
}
