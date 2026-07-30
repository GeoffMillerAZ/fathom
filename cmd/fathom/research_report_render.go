package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// renderResearchReportText writes r's human-readable table form.
func renderResearchReportText(w io.Writer, r ResearchReport) {
	fmt.Fprintln(w, "fathom research report")
	fmt.Fprintf(w, "window: %s -> %s (generated %s)\n", r.Since, r.Until, r.GeneratedAt)
	if r.Service != "" {
		fmt.Fprintf(w, "service: %s\n", r.Service)
	}
	fmt.Fprintln(w)

	if !r.CallLogRead {
		fmt.Fprintf(w, "call log: %s (not found — 0 calls observed, not measured)\n", r.CallLogPath)
	} else {
		fmt.Fprintf(w, "call log: %s (%d calls in window)\n", r.CallLogPath, r.TotalCallsInWindow)
	}
	if r.SkippedMalformedLines > 0 {
		fmt.Fprintf(w, "  ⚠ skipped %d malformed line(s)\n", r.SkippedMalformedLines)
	}

	fmt.Fprintln(w, "\nsurface traffic:")
	if len(r.SurfaceTraffic) == 0 {
		fmt.Fprintln(w, "  (no calls observed)")
	} else {
		for _, s := range r.SurfaceTraffic {
			surface := s.Surface
			if surface == "" {
				surface = "(bare)"
			}
			fmt.Fprintf(w, "  %-12s calls=%d\n", surface, s.Calls)
		}
	}

	fmt.Fprintln(w, "\ntop tools:")
	if len(r.TopTools) == 0 {
		fmt.Fprintln(w, "  (no calls observed)")
	} else {
		for _, t := range r.TopTools {
			fmt.Fprintf(w, "  %-24s %d\n", t.Tool, t.Calls)
		}
	}

	fmt.Fprintln(w, "\nlatency:")
	if r.LatencyNote != "" {
		fmt.Fprintf(w, "  %s\n", r.LatencyNote)
	} else {
		fmt.Fprintf(w, "  p50=%.1fms p95=%.1fms\n", *r.LatencyP50MS, *r.LatencyP95MS)
	}

	fmt.Fprintln(w, "\ninjection coverage (obligations, per-user):")
	fmt.Fprintf(w, "  seen=%d answered=%d obligations_served=%d", r.InjectionSeen, r.InjectionAnswered, r.ObligationsServed)
	if r.InjectionCoverageRate != nil {
		fmt.Fprintf(w, " rate=%.0f%%", *r.InjectionCoverageRate*100)
	} else {
		fmt.Fprint(w, " rate=n/a (no edits seen)")
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  note: %s\n", r.InjectionScopeNote)
	fmt.Fprintf(w, "  obligations evicted: %s\n", r.ObligationsEvictedNote)

	fmt.Fprintln(w, "\nguarded-edit coverage (pre|post, docs/design/guarded-edits.md):")
	fmt.Fprintf(w, "  pre: seen=%d served=%d   post: seen=%d served=%d   dedup_suppressed=%d\n",
		r.GuardPreSeen, r.GuardPreServed, r.GuardPostSeen, r.GuardPostServed, r.GuardDedupSuppressed)
	fmt.Fprintf(w, "  %s\n", r.GuardCoverageNote)

	fmt.Fprintln(w, "\nplatform-module serving (docs/design/platform-modules.md):")
	if len(r.ModuleRulesServed) == 0 {
		fmt.Fprintf(w, "  %s\n", r.ModuleServingNote)
	} else {
		modules := make([]string, 0, len(r.ModuleRulesServed))
		for m := range r.ModuleRulesServed {
			modules = append(modules, m)
		}
		sort.Strings(modules)
		for _, m := range modules {
			fmt.Fprintf(w, "  %-24s %d\n", m, r.ModuleRulesServed[m])
		}
	}

	fmt.Fprintln(w, "\nverify gate:")
	fmt.Fprintf(w, "  claimed=%d verified=%d failing=%d review_active_findings=%d\n", r.VerifyClaimed, r.VerifyVerified, r.VerifyFailing, r.ReviewActiveFindings)
	fmt.Fprintf(w, "  %s\n", r.VerifyNote)
	fmt.Fprintf(w, "  %s\n", r.ReviewActiveFindingsNote)

	fmt.Fprintln(w, "\nstaleness:")
	if r.StalenessNote != "" {
		fmt.Fprintf(w, "  %s\n", r.StalenessNote)
	} else {
		fmt.Fprintf(w, "  staleness_report calls in window: %d\n", r.StalenessReportCalls)
	}
	fmt.Fprintf(w, "  re-anchor events: %s\n", r.ReAnchorNote)

	fmt.Fprintln(w, "\nwhat fathom SPENT:")
	if r.PayloadCostNote != "" && len(r.PayloadCostByChannel) == 0 {
		fmt.Fprintf(w, "  %s\n", r.PayloadCostNote)
	} else {
		channels := make([]string, 0, len(r.PayloadCostByChannel))
		for c := range r.PayloadCostByChannel {
			channels = append(channels, c)
		}
		sort.Strings(channels)
		for _, c := range channels {
			s := r.PayloadCostByChannel[c]
			fmt.Fprintf(w, "  %-16s beats=%-6d bytes=%-8d tokens_estimate=%d\n", c, s.Beats, s.TotalBytes, s.TotalTokensEstimate)
		}
		if r.PayloadCostPerPayloadP50Estimate != nil && r.PayloadCostPerPayloadP95Estimate != nil {
			fmt.Fprintf(w, "  per-payload tokens_estimate: p50=%.0f p95=%.0f\n", *r.PayloadCostPerPayloadP50Estimate, *r.PayloadCostPerPayloadP95Estimate)
		}
		fmt.Fprintf(w, "  %s\n", r.PayloadCostNote)
	}

	fmt.Fprintln(w, "\ndiscovery share (fathom index tools vs grep/glob/read/shell-search):")
	fmt.Fprintf(w, "  agent-surface fathom calls in window: %d\n", r.AgentSurfaceCalls)
	fmt.Fprintf(w, "  fathom index-tool calls: %d   discovery actions observed: %d\n", r.DiscoveryIndexToolCalls, r.DiscoveryActionsTotal)
	if len(r.DiscoveryActionsByKind) > 0 {
		kinds := make([]string, 0, len(r.DiscoveryActionsByKind))
		for k := range r.DiscoveryActionsByKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			label := k
			if k == "read" {
				label = "read (dual-use — reading a file you already located is not necessarily discovery)"
			}
			fmt.Fprintf(w, "    %-60s %d\n", label, r.DiscoveryActionsByKind[k])
		}
	}
	if r.DiscoverySharePercent != nil {
		fmt.Fprintf(w, "  fathom share: %.0f%% (CORRELATIONAL — not causal until the pre-registered A/B runs)\n", *r.DiscoverySharePercent)
	} else if r.DiscoveryShareDenominatorNote != "" {
		fmt.Fprintf(w, "  %s\n", r.DiscoveryShareDenominatorNote)
	}
	fmt.Fprintf(w, "  %s\n", r.DiscoveryShareNote)

	fmt.Fprintln(w, "\nservice-identity cache (latency/load, not a token signal):")
	fmt.Fprintf(w, "  hits=%d misses=%d", r.IdentCacheHits, r.IdentCacheMisses)
	if r.IdentCacheHitRate != nil {
		fmt.Fprintf(w, " hit_rate=%.0f%%", *r.IdentCacheHitRate*100)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", r.IdentCacheNote)

	fmt.Fprintln(w, "\nobservations:")
	if !r.ObservationsRead {
		fmt.Fprintf(w, "  no %s yet\n", r.ObservationsPath)
	} else {
		fmt.Fprintf(w, "  total=%d open=%d (%s)\n", *r.TotalObservations, *r.OpenObservations, r.ObservationsPath)
		if len(r.ObservationsByTool) > 0 {
			tools := make([]string, 0, len(r.ObservationsByTool))
			for t := range r.ObservationsByTool {
				tools = append(tools, t)
			}
			sort.Strings(tools)
			for _, t := range tools {
				fmt.Fprintf(w, "    %-24s %d\n", t, r.ObservationsByTool[t])
			}
		}
	}
	fmt.Fprintf(w, "  note: %s\n", r.ObservationsWindowNote)

	fmt.Fprintln(w, "\ncontext-bench:")
	if len(r.BenchArms) == 0 {
		fmt.Fprintf(w, "  %s\n", r.BenchNote)
	} else {
		fmt.Fprintf(w, "  %s\n", r.BenchNote)
		for _, a := range r.BenchArms {
			fmt.Fprintf(w, "  %-10s tasks=%d success=%d (%.0f%%) mean_tokens=%.0f mean_wall_ms=%.0f\n",
				a.Arm, a.TaskCount, a.SuccessCount, a.SuccessRate*100, a.MeanTokens, a.MeanWallMS)
		}
	}
}

// renderResearchReportMarkdown renders r as a standalone Markdown document
// (the same content renderResearchReportText prints to stdout, framed as a
// committable/inspectable artifact).
func renderResearchReportMarkdown(r ResearchReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fathom research report\n\n")
	fmt.Fprintf(&b, "Window: `%s` -> `%s` (generated `%s`)", r.Since, r.Until, r.GeneratedAt)
	if r.Service != "" {
		fmt.Fprintf(&b, " — service `%s`", r.Service)
	}
	b.WriteString("\n\n")
	b.WriteString("This file is local evidence only — it is not transmitted anywhere. " +
		"Run `fathom research bundle` to produce a sanitized, shareable summary.\n\n")

	renderResearchReportText(&b, r)
	return b.String()
}

// writeResearchReport writes report.md and report.json to outDir
// (overwriting any prior run — the flywheel's report is a snapshot, not an
// accumulating log).
func writeResearchReport(outDir string, r ResearchReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	md := renderResearchReportMarkdown(r)
	if err := os.WriteFile(filepath.Join(outDir, "report.md"), []byte(md), 0o644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "report.json"), data, 0o644)
}
