package runreport

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
)

// Render produces a deterministic Markdown run-report from in.
//
// The document is structured as:
//
//	# Run report
//	## Summary
//	## Agent activity          (journal entries)
//	## Turn timeline           (llm_call / turn telemetry events)
//	## Flags                   (injection + compaction events)
//	## Cost
//
// Each section degrades gracefully when its inputs are absent: the section
// heading is still emitted but the body states "not captured" so a reader
// can distinguish "this data was not recorded" from "the section was omitted".
//
// Render is a pure function — given the same Input it returns the same bytes.
// It never returns a non-nil error for missing data; errors are reserved for
// internal rendering bugs that produce an invalid state (currently none).
func Render(in Input) ([]byte, error) {
	var b strings.Builder

	writeHeader(&b, in)
	writeSummary(&b, in)
	writeAgentActivity(&b, in)
	writeTurnTimeline(&b, in)
	writeFlags(&b, in)
	writeCost(&b, in)

	return []byte(b.String()), nil
}

// ─── header ──────────────────────────────────────────────────────────────────

func writeHeader(b *strings.Builder, in Input) {
	b.WriteString("# Run report\n\n")

	runID := in.RunID
	if runID == "" {
		runID = "(no trace ID)"
	}
	fmt.Fprintf(b, "**Run ID:** %s\n\n", runID)

	startStr := formatTime(in.StartedAt)
	finishStr := formatTime(in.FinishedAt)
	fmt.Fprintf(b, "**Started:** %s  \n", startStr)
	fmt.Fprintf(b, "**Finished:** %s", finishStr)
	if !in.StartedAt.IsZero() && !in.FinishedAt.IsZero() {
		dur := in.FinishedAt.Sub(in.StartedAt)
		fmt.Fprintf(b, "  (%s)", formatDuration(dur))
	}
	b.WriteString("\n\n")
}

// ─── summary ─────────────────────────────────────────────────────────────────

func writeSummary(b *strings.Builder, in Input) {
	b.WriteString("## Summary\n\n")

	model := in.Model
	if model == "" {
		model = "(not recorded)"
	}
	stopCause := in.StopCause
	if stopCause == "" {
		stopCause = "(not recorded)"
	}

	fmt.Fprintf(b, "| Field | Value |\n")
	fmt.Fprintf(b, "|---|---|\n")
	fmt.Fprintf(b, "| Model | %s |\n", model)
	fmt.Fprintf(b, "| Stop cause | %s |\n", stopCause)
	fmt.Fprintf(b, "| Turns completed | %d |\n", in.Turns)
	fmt.Fprintf(b, "| Tool calls dispatched | %d |\n", in.ToolCalls)
	b.WriteString("\n")
}

// ─── agent activity ───────────────────────────────────────────────────────────

func writeAgentActivity(b *strings.Builder, in Input) {
	b.WriteString("## Agent activity\n\n")

	if in.Journal == nil {
		b.WriteString("_not captured_\n\n")
		return
	}

	entries := in.Journal.Entries()
	if len(entries) == 0 {
		b.WriteString("_no journal entries recorded_\n\n")
		return
	}

	for _, e := range entries {
		ts := formatTime(e.At)
		fmt.Fprintf(b, "- `%s` **%s** — %s\n", ts, e.Step, e.Note)
	}
	b.WriteString("\n")
}

// ─── turn timeline ────────────────────────────────────────────────────────────

func writeTurnTimeline(b *strings.Builder, in Input) {
	b.WriteString("## Turn timeline\n\n")

	// Extract turn events: events where AttrStep == "turn".
	var turns []event.Event
	for _, ev := range in.Events {
		if ev.Attributes[event.AttrStep] == event.StepTurn {
			turns = append(turns, ev)
		}
	}

	if len(turns) == 0 {
		if len(in.Events) == 0 {
			b.WriteString("_not captured_\n\n")
		} else {
			b.WriteString("_no turn events recorded_\n\n")
		}
		return
	}

	// Sort by turn index (AttrTurn), then by timestamp as tiebreak.
	sort.SliceStable(turns, func(i, j int) bool {
		ti := turns[i].Attributes[event.AttrTurn]
		tj := turns[j].Attributes[event.AttrTurn]
		if ti != tj {
			return ti < tj
		}
		return turns[i].Timestamp.Before(turns[j].Timestamp)
	})

	fmt.Fprintf(b, "| Turn | Kind | Stop reason | Tools | Duration ms | Trace span |\n")
	fmt.Fprintf(b, "|---|---|---|---|---|---|\n")
	for _, ev := range turns {
		turn := ev.Attributes[event.AttrTurn]
		stopReason := ev.Attributes[event.AttrStopReason]
		if stopReason == "" {
			stopReason = ev.Attributes[event.AttrError]
			if stopReason == "" {
				stopReason = "-"
			}
		}
		tools := ev.Attributes[event.AttrToolNames]
		if tools == "" {
			tools = "-"
		}
		durMS := ev.Attributes[event.AttrDurationMS]
		if durMS == "" {
			durMS = "-"
		}
		spanID := ev.Attributes[event.AttrSpanID]
		if spanID == "" {
			spanID = "-"
		}
		kind := string(ev.Kind)
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n",
			turn, kind, stopReason, tools, durMS, spanID)
	}
	b.WriteString("\n")
}

// ─── flags ────────────────────────────────────────────────────────────────────

func writeFlags(b *strings.Builder, in Input) {
	b.WriteString("## Flags\n\n")

	var injections, compactions []event.Event
	for _, ev := range in.Events {
		switch ev.Attributes[event.AttrStep] {
		case event.StepInjection:
			injections = append(injections, ev)
		case "compaction":
			compactions = append(compactions, ev)
		}
	}
	// Also pick up compaction journal entries as a fallback: harness journals
	// "compaction-fired" as a step name. We surface these from Events only
	// (the journal section already shows them as activity).

	if len(injections) == 0 && len(compactions) == 0 {
		if len(in.Events) == 0 {
			b.WriteString("_not captured_\n\n")
		} else {
			b.WriteString("_no flags raised_\n\n")
		}
		return
	}

	if len(injections) > 0 {
		b.WriteString("### Injection detections\n\n")
		fmt.Fprintf(b, "| Turn | Action | Severity | Rules | Tool use ID |\n")
		fmt.Fprintf(b, "|---|---|---|---|---|\n")
		for _, ev := range injections {
			turn := ev.Attributes[event.AttrTurn]
			action := ev.Attributes[event.AttrAction]
			severity := ev.Attributes[event.AttrSeverity]
			rules := ev.Attributes[event.AttrRuleIDs]
			toolID := ev.Attributes[event.AttrToolUseID]
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", turn, action, severity, rules, toolID)
		}
		b.WriteString("\n")
	}

	if len(compactions) > 0 {
		b.WriteString("### Context compactions\n\n")
		for _, ev := range compactions {
			turn := ev.Attributes[event.AttrTurn]
			dropped := ev.Attributes["dropped"]
			fmt.Fprintf(b, "- Turn %s: %s messages dropped\n", turn, dropped)
		}
		b.WriteString("\n")
	}
}

// ─── cost ─────────────────────────────────────────────────────────────────────

func writeCost(b *strings.Builder, in Input) {
	b.WriteString("## Cost\n\n")
	if strings.TrimSpace(in.CostReport) == "" {
		b.WriteString("_not captured_\n\n")
		return
	}
	b.WriteString("```\n")
	b.WriteString(strings.TrimRight(in.CostReport, "\n"))
	b.WriteString("\n```\n\n")
}

// ─── formatting helpers ───────────────────────────────────────────────────────

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "(unknown)"
	}
	return t.UTC().Format(time.RFC3339)
}

// formatDuration renders a duration in a human-readable form: hours + minutes
// if ≥ 1 minute, otherwise seconds with one decimal.
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m < 60 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}
