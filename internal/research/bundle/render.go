package bundle

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders b as the human-readable .md twin of the bundle
// JSON — same data, no more and no less (the bundle command writes one JSON
// file and one Markdown rendering of that exact JSON, never two different
// views).
func RenderMarkdown(b Bundle) string {
	var s strings.Builder

	fmt.Fprintf(&s, "# fathom research bundle (schema %s)\n\n", b.SchemaVersion)
	fmt.Fprintf(&s, "This file and its `.json` twin carry the exact same data — nothing more.\n"+
		"It was generated locally and has NOT been transmitted anywhere; sharing it is a manual choice you make.\n\n")
	fmt.Fprintf(&s, "- Generated: %s\n- Window: %s -> %s\n- fathom %s @ %s%s (%s/%s)\n\n",
		b.GeneratedAt, b.WindowSince, b.WindowUntil,
		b.FathomVersion, shortOrUnknown(b.CommitSHA), dirtyTag(b.CommitDirty), b.GOOS, b.GOARCH)

	fmt.Fprintf(&s, "## Traffic\n\nTotal calls: %d\n\n", b.TotalCalls)
	if len(b.SurfaceTraffic) > 0 {
		s.WriteString("| surface | calls |\n|---|---|\n")
		for _, r := range b.SurfaceTraffic {
			surface := r.Surface
			if surface == "" {
				surface = "(bare)"
			}
			fmt.Fprintf(&s, "| %s | %d |\n", surface, r.Calls)
		}
		s.WriteString("\n")
	}
	if len(b.ToolCalls) > 0 {
		s.WriteString("| tool | calls |\n|---|---|\n")
		for _, r := range b.ToolCalls {
			fmt.Fprintf(&s, "| %s | %d |\n", r.Tool, r.Calls)
		}
		s.WriteString("\n")
	}
	if b.LatencyP50MS != nil {
		fmt.Fprintf(&s, "Latency: p50=%.1fms", *b.LatencyP50MS)
		if b.LatencyP95MS != nil {
			fmt.Fprintf(&s, " p95=%.1fms", *b.LatencyP95MS)
		}
		s.WriteString("\n\n")
	}

	fmt.Fprintf(&s, "## Injection coverage\n\nseen=%d answered=%d obligations_served=%d",
		b.InjectionSeen, b.InjectionAnswered, b.ObligationsServed)
	if b.InjectionCoverageRate != nil {
		fmt.Fprintf(&s, " rate=%.0f%%", *b.InjectionCoverageRate*100)
	}
	s.WriteString("\n\n")

	fmt.Fprintf(&s, "## Verify gate\n\nclaimed=%d verified=%d failing=%d instrumented_calls=%d "+
		"predates_instrumentation=%d review_active_findings=%d\n\n",
		b.VerifyClaimed, b.VerifyVerified, b.VerifyFailing,
		b.VerifyInstrumentedCalls, b.VerifyPredatesInstrumentation, b.ReviewActiveFindings)

	fmt.Fprintf(&s, "## Staleness\n\nstaleness_report_calls=%d\n\n", b.StalenessReportCalls)

	s.WriteString("## Observations\n\n")
	if b.TotalObservations != nil {
		fmt.Fprintf(&s, "total=%d", *b.TotalObservations)
		if b.OpenObservations != nil {
			fmt.Fprintf(&s, " open=%d", *b.OpenObservations)
		}
		s.WriteString("\n\n")
	} else {
		s.WriteString("no observations file observed in this window\n\n")
	}

	if len(b.ServiceActivity) > 0 {
		s.WriteString("## Service activity (salted hashes — never plaintext names)\n\n")
		s.WriteString("| service_hash | calls |\n|---|---|\n")
		for _, r := range b.ServiceActivity {
			fmt.Fprintf(&s, "| %s | %d |\n", r.ServiceHash, r.Calls)
		}
		s.WriteString("\n")
	}

	if len(b.BenchArms) > 0 {
		s.WriteString("## Bench arms\n\n")
		s.WriteString("| arm | tasks | success | success_rate | mean_tokens | mean_wall_ms |\n|---|---|---|---|---|---|\n")
		for _, a := range b.BenchArms {
			fmt.Fprintf(&s, "| %s | %d | %d | %.0f%% | %.0f | %.0f |\n",
				a.Arm, a.TaskCount, a.SuccessCount, a.SuccessRate*100, a.MeanTokens, a.MeanWallMS)
		}
		s.WriteString("\n")
	}

	return s.String()
}

func shortOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func dirtyTag(dirty bool) string {
	if dirty {
		return " (dirty)"
	}
	return ""
}
