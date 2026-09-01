// Package topology renders a hierarchical status tree for ops mission-control —
// a global → regional → cluster → workload rollup where every node carries a
// name, a status badge (healthy / degraded / failed / unknown), and an optional
// metric, with each level collapsible via a no-JS <details>/<summary>
// disclosure (the same pattern appkit/ui/dashboard uses for its mobile nav). It
// is the at-a-glance fleet view the kit otherwise lacks — built for the Bridge
// and Muster control planes.
//
// Output is deterministic, server-rendered HTML themed through the design
// tokens (D7): status maps to kit role colors via the callout Badge, layout
// uses only role utilities (text-ink, border-line, bg-surface-raised) so the
// tree re-themes light/dark with the token CSS. Pure Go, no Node (D2). All
// caller text is escaped (html.EscapeString); drill-through hrefs are caller
// HTML attributes, never JS — the consuming app wires the loopback navigation,
// keeping the tree within the Loopback Invariant (D5).
//
// Files: topology.go (status roles + helpers), tree.go (Node, Options, Render).
package topology

import "html"

// Status is a node's health, mapped to a kit token role for its badge.
type Status string

const (
	// Healthy: operating nominally (positive role, green).
	Healthy Status = "healthy"
	// Degraded: partially impaired but serving (caution role, amber).
	Degraded Status = "degraded"
	// Failed: down or in error (danger role, red).
	Failed Status = "failed"
	// Unknown: no signal / not yet reported. Renders as a neutral pill rather
	// than a colored status, so an absence of data never reads as "healthy".
	Unknown Status = "unknown"
)

// statusRole maps a Status to the token role passed to callout.Badge. An
// unrecognized status is treated as Unknown so output is always well-formed and
// never silently green.
func statusRole(s Status) string {
	switch s {
	case Healthy:
		return "positive"
	case Degraded:
		return "caution"
	case Failed:
		return "danger"
	default: // Unknown and any unrecognized value
		// "ink" is not a soft status role, so callout.Badge renders the neutral
		// pill — the honest rendering for "no signal".
		return "ink"
	}
}

// statusLabel is the human-readable text shown inside the status badge.
func statusLabel(s Status) string {
	switch s {
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

func esc(s string) string { return html.EscapeString(s) }
