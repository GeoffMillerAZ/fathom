package main

import (
	"fmt"
	"io"
)

// research.go — the research flywheel (docs/design/research-flywheel.md):
// fathom self-collects the local evidence that proves (or disproves) it is
// working well, aggregates it into an honest report on demand, and can
// produce a sanitized, review-first bundle a user may choose to share.
// CLI/local-file only, no new MCP tool (docs/vision/11-dial-to-eleven.md
// doctrine 4: consolidate, don't grow).
//
// cmdResearch implements the "research" subcommand, dispatching to its two
// verbs exactly like cmdPlans dispatches status/next/archive/restore.
func cmdResearch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: research requires a subcommand (report, bundle)")
		return 1
	}
	switch args[0] {
	case "report":
		return cmdResearchReport(args[1:], stdout, stderr)
	case "bundle":
		return cmdResearchBundle(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown research subcommand %q (want report, bundle)\n", args[0])
		return 1
	}
}

// researchRunner runs `fathom research` (registered below; EW-45).
type researchRunner struct{}

func (researchRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdResearch(args, stdout, stderr)
}

func init() {
	// SR-16: frozen (ADR-046 decision 6), not deleted
	declareVerb(verb{Name: "research", Group: "govern", Order: 40, Hidden: true, Run: researchRunner{},
		Summary: "The research flywheel: report (aggregate local evidence) | bundle (sanitized, shareable summary)"})
}
