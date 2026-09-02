package harness

import (
	"fmt"
	"strings"
	"time"
)

// CapabilityRow is one row of a harness's generated capability table.
type CapabilityRow struct {
	Capability Capability `json:"capability"`
	Vehicle    Vehicle    `json:"vehicle"`
	Status     string     `json:"status"`
	Note       string     `json:"note,omitempty"`
}

// CapabilityRows computes the honest per-capability table for one
// descriptor (vision 25 §2.2 rule 5): the vehicle the descriptor declares,
// a status column that says "unverified on this machine" on EVERY row of
// an unverified descriptor, "hooks: not wired" on a hook vehicle whose
// codec has not landed, "demoted to tier D" past the demotion clock, and
// "verified" only when a real session verified the descriptor. An absent
// vehicle prints as absent. now drives the demotion clock.
func CapabilityRows(desc Descriptor, now time.Time) []CapabilityRow {
	demoted, _ := DemotionDue(desc, now)
	rows := make([]CapabilityRow, 0, len(Capabilities))
	for _, cap := range Capabilities {
		v, ok := desc.Vehicles[cap]
		if !ok {
			v = VehicleAbsent
		}
		if demoted && isHookVehicle(v) {
			v = VehicleAbsent
		}
		rows = append(rows, CapabilityRow{
			Capability: cap,
			Vehicle:    v,
			Status:     rowStatus(desc, v, demoted),
			Note:       desc.Notes[cap],
		})
	}
	return rows
}

func isHookVehicle(v Vehicle) bool {
	return v == VehicleHookInject || v == VehicleHookBlock || v == VehicleHookSilent
}

func rowStatus(desc Descriptor, v Vehicle, demoted bool) string {
	var parts []string
	switch {
	case demoted:
		parts = append(parts, "demoted to tier D (unverified past the 60-day clock)")
	case desc.Unverified():
		parts = append(parts, "unverified on this machine")
	default:
		parts = append(parts, "verified")
	}
	if isHookVehicle(v) && !desc.HooksWired() {
		parts = append(parts, "hooks: not wired (wave 2)")
	}
	return strings.Join(parts, "; ")
}

// RenderCapabilityTable renders CapabilityRows as a Markdown table for the
// descriptor id — the ONE source docs/adoption/<id>.md's capability
// section is pinned to. An unknown id renders a one-line table saying so
// rather than an empty string (a doc must never pin silence).
func RenderCapabilityTable(id string, now time.Time) string {
	desc, ok := Lookup(id)
	if !ok {
		return fmt.Sprintf("| Capability | Vehicle | Status |\n|---|---|---|\n| (unknown harness %q) | — | not in the registry: `fathom harness list` |\n", id)
	}
	var sb strings.Builder
	sb.WriteString("| Capability | Vehicle | Status |\n|---|---|---|\n")
	for _, r := range CapabilityRows(desc, now) {
		status := r.Status
		if r.Note != "" {
			status += " — " + r.Note
		}
		fmt.Fprintf(&sb, "| `%s` | `%s` | %s |\n", r.Capability, r.Vehicle, status)
	}
	return sb.String()
}
