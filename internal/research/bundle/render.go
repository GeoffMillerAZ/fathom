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

	s.WriteString("## Call traffic\n\n")
	if b.CallTraffic == nil {
		s.WriteString("not instrumented: the MCP call log was absent/unreadable over this window\n\n")
	} else {
		ct := b.CallTraffic
		fmt.Fprintf(&s, "Total calls: %d\n\n", ct.TotalCalls)
		if len(ct.SurfaceTraffic) > 0 {
			s.WriteString("| surface | calls |\n|---|---|\n")
			for _, r := range ct.SurfaceTraffic {
				surface := r.Surface
				if surface == "" {
					surface = "(bare)"
				}
				fmt.Fprintf(&s, "| %s | %d |\n", surface, r.Calls)
			}
			s.WriteString("\n")
		}
		if len(ct.ToolCalls) > 0 {
			s.WriteString("| tool | calls |\n|---|---|\n")
			for _, r := range ct.ToolCalls {
				fmt.Fprintf(&s, "| %s | %d |\n", r.Tool, r.Calls)
			}
			s.WriteString("\n")
		}
		if ct.LatencyP50MS != nil {
			fmt.Fprintf(&s, "Latency: p50=%.1fms", *ct.LatencyP50MS)
			if ct.LatencyP95MS != nil {
				fmt.Fprintf(&s, " p95=%.1fms", *ct.LatencyP95MS)
			}
			s.WriteString("\n\n")
		}
		fmt.Fprintf(&s, "staleness_report calls: %d\n\n", ct.StalenessReportCalls)
	}

	s.WriteString("## Injection coverage\n\n")
	if b.Injection == nil {
		s.WriteString("not instrumented: the hooks beat log was absent/unreadable over this window\n\n")
	} else {
		inj := b.Injection
		fmt.Fprintf(&s, "seen=%d answered=%d obligations_served=%d", inj.Seen, inj.Answered, inj.ObligationsServed)
		if inj.CoverageRate != nil {
			fmt.Fprintf(&s, " rate=%.0f%%", *inj.CoverageRate*100)
		}
		s.WriteString("\n\n")
	}

	s.WriteString("## Verify gate\n\n")
	if b.VerifyGate == nil {
		s.WriteString("not instrumented: the MCP call log was absent/unreadable over this window\n\n")
	} else {
		vg := b.VerifyGate
		fmt.Fprintf(&s, "claimed=%d verified=%d failing=%d instrumented_calls=%d "+
			"predates_instrumentation=%d review_active_findings=%d\n\n",
			vg.Claimed, vg.Verified, vg.Failing, vg.InstrumentedCalls, vg.PredatesInstrumentation, vg.ReviewActiveFindings)
	}

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

	// Opt-in like the bench arms: absent from the JSON means absent here,
	// so a bundle built without --include-retros renders byte-identical to
	// one built before the group existed (design §5).
	if b.Retros != nil {
		r := b.Retros
		s.WriteString("## Retro corpus\n\n")
		fmt.Fprintf(&s, "retros=%d unreadable=%d off_arm_retros=%d guard_fires=%d obligation_injections=%d "+
			"silent_on_clean=%d unjoinable=%d custodian_remedied=%d custodian_unremedied=%d\n\n",
			r.Retros, r.Unreadable, r.OffArmRetros, r.GuardFires, r.ObligationInjections,
			r.SilentOnClean, r.Unjoinable, r.CustodianRemedied, r.CustodianUnremedied)
		if len(r.Outcomes) > 0 {
			s.WriteString("| channel | outcome | firings |\n|---|---|---|\n")
			for _, o := range r.Outcomes {
				fmt.Fprintf(&s, "| %s | %s | %d |\n", o.Channel, o.Outcome, o.Firings)
			}
			s.WriteString("\n")
		}
		s.WriteString("Outcome-shaped counts, never causality: the arm carries causality (law 12).\n\n")
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
