package console

// wiring.go — OC-40 (vision 35 §3 "Wiring: doctor checks, drift, and
// receipts"; design §3): one repository's wiring as three instruments side
// by side — doctor's repository-side checks, the drift between what fathom
// recorded and what is on disk (each thing that differs counted once: the
// preview plan's apply and BLOCKED items, the drift beyond the plan, and
// the doctor rows no item accounts for), and the receipts every apply
// leaves — beside the existing preview → apply → undo, which this
// projection never performs. It only reads.
//
// A repository whose root is not on this machine has no doctor rows, no
// plan and no receipts to read: every figure is then absent with the
// reason, never "0 failing".

import (
	"fmt"
	"sort"
)

// WiringCheck is one doctor row.
type WiringCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass | warn | fail | skipped
	Reason string `json:"reason"`
	Remedy string `json:"remedy,omitempty"`
	// Group is "wiring" (what fathom writes into the repository) or
	// "configuration" (the repository's .fathom files and user scope).
	Group string `json:"group"`
}

// WiringPlanItem is one preview-plan item (wiring.Item's fields the page
// reads).
type WiringPlanItem struct {
	File    string `json:"file"`
	Target  string `json:"target"`
	Kind    string `json:"kind,omitempty"` // region | key (wiring.Item.Kind)
	Verdict string `json:"verdict"`        // apply | skip | blocked
	Reason  string `json:"reason"`
	Cause   string `json:"cause,omitempty"`
}

// WiringReceipt is one receipt (wiring.Receipt's summary fields).
type WiringReceipt struct {
	ID     string `json:"id"`
	Time   string `json:"time"`
	Fathom string `json:"fathom_version,omitempty"`
	UndoOf string `json:"undo_of,omitempty"`
	Files  int    `json:"files"`
}

// WiringFacts is what the adapter read for one repository.
type WiringFacts struct {
	Service string
	Root    string
	// RootNote, when set, is why the root cannot be read here (no local
	// root, not a directory on this machine); everything else is ignored.
	RootNote string
	// Harness is the harness doctor's rows were read for ("claude",
	// "codex", …): the preview plan is Claude-shaped, so only under Claude
	// do its hook items itemize doctor's hooks_installed.
	Harness string
	Checks  []WiringCheck
	// BeyondPlan is the drift doctor's wiring_drift row sees that no
	// preview plan can: owned MCP entries and skill files changed since
	// fathom recorded them. With it, wiring_drift is itemized — its hook
	// and stanza drift are the plan's BLOCKED items, the rest these.
	BeyondPlan []WiringDrift
	// Plan is the preview's plan (read-only: PlanWiring); PlanErr why it
	// could not be computed.
	Plan       []WiringPlanItem
	PlanDigest string
	PlanErr    string
	// Receipts newest first, the total and why they could not be read.
	Receipts      []WiringReceipt
	ReceiptsTotal int
	ReceiptsErr   string
	Source        string
}

// WiringDrift is one thing on disk that differs from what fathom recorded
// or would write, with where it was seen.
type WiringDrift struct {
	Source string `json:"source"` // "doctor" or "plan"
	What   string `json:"what"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
	// Doctor names the failing doctor row that summarizes this thing (its
	// "6 drift(s): …" or its missing hooks): that row is not counted again.
	Doctor string `json:"doctor,omitempty"`
}

// WiringViewModel is the wiring tab's projection.
type WiringViewModel struct {
	Service string `json:"service"`
	Root    string `json:"root,omitempty"`
	// Checks by group, in doctor's own order.
	Checks []WiringCheck `json:"checks"`
	Pass   Value[int]    `json:"checks_pass"`
	Warn   Value[int]    `json:"checks_warn"`
	Fail   Value[int]    `json:"checks_fail"`
	Skip   Value[int]    `json:"checks_skipped"`
	// Drift and its count.
	Drift      []WiringDrift `json:"drift"`
	DriftCount Value[int]    `json:"drift_count"`
	// Plan counts by verdict.
	PlanApply   Value[int] `json:"plan_apply"`
	PlanCurrent Value[int] `json:"plan_current"`
	PlanBlocked Value[int] `json:"plan_blocked"`
	PlanDigest  string     `json:"plan_digest,omitempty"`
	// Receipts and their total.
	Receipts      []WiringReceipt `json:"receipts"`
	ReceiptsTotal Value[int]      `json:"receipts_total"`
}

// wiringCheckGroups names doctor's repository-side rows by group; a row not
// listed here is "configuration".
var wiringCheckGroups = map[string]string{
	"hooks_installed": "wiring", "lefthook_gates": "wiring", "harness_hooks": "wiring",
	"stanza_present": "wiring", "skills_installed": "wiring", "wiring_drift": "wiring",
}

// WiringView projects one repository's doctor rows, drift and receipts.
func WiringView(f WiringFacts) WiringViewModel {
	m := WiringViewModel{Service: f.Service, Root: f.Root, Checks: []WiringCheck{}, Drift: []WiringDrift{}, Receipts: []WiringReceipt{}}
	if f.RootNote != "" {
		a := Absent[int](f.RootNote)
		m.Pass, m.Warn, m.Fail, m.Skip, m.DriftCount = a, a, a, a, a
		m.PlanApply, m.PlanCurrent, m.PlanBlocked, m.ReceiptsTotal = a, a, a, a
		return m
	}
	src := f.Source
	counts := map[string]int{}
	var failing []WiringCheck // doctor's failing or warning wiring rows
	for _, c := range f.Checks {
		c.Group = wiringCheckGroups[c.Name]
		if c.Group == "" {
			c.Group = "configuration"
		}
		counts[c.Status]++
		m.Checks = append(m.Checks, c)
		if c.Group == "wiring" && (c.Status == "fail" || c.Status == "warn") {
			failing = append(failing, c)
		}
	}
	// Wiring rows first, each group in doctor's own order.
	sort.SliceStable(m.Checks, func(i, j int) bool {
		return m.Checks[i].Group == "wiring" && m.Checks[j].Group != "wiring"
	})
	denom := fmt.Sprintf("of %d repository-side doctor checks", len(f.Checks))
	if len(f.Checks) == 0 {
		a := Absent[int]("doctor returned no repository-side rows for " + f.Root)
		m.Pass, m.Warn, m.Fail, m.Skip = a, a, a, a
	} else {
		m.Pass = Measured(counts["pass"], src).WithDenominator(denom)
		m.Warn = Measured(counts["warn"], src).WithDenominator(denom)
		m.Fail = Measured(counts["fail"], src).WithDenominator(denom)
		m.Skip = Measured(counts["skipped"], src).WithDenominator(denom)
	}

	failingRow := map[string]WiringCheck{}
	for _, c := range failing {
		failingRow[c.Name] = c
	}
	var planDrift, beyond []WiringDrift
	itemized := map[string]bool{} // failing doctor rows the plan (or beyond-plan drift) itemizes
	if f.PlanErr != "" {
		e := Errored[int]("the wiring preview could not be computed: " + f.PlanErr)
		m.PlanApply, m.PlanCurrent, m.PlanBlocked = e, e, e
	} else {
		apply, skip, blocked := 0, 0, 0
		for _, it := range f.Plan {
			var d WiringDrift
			switch it.Verdict {
			case "apply":
				apply++
				d = WiringDrift{Source: "plan", What: it.File + " · " + it.Target, Detail: "Apply would write this: " + it.Reason, Remedy: "Preview changes, then Apply (or: fathom fleet reconcile --apply)"}
			case "blocked":
				blocked++
				d = WiringDrift{Source: "plan", What: it.File + " · " + it.Target, Detail: "BLOCKED (" + it.Cause + "): " + it.Reason, Remedy: "resolve by hand — fathom never merges a drifted region (law 15)"}
			default:
				skip++
				continue
			}
			if row := summarizingRow(it, f.Harness); row != "" {
				if _, ok := failingRow[row]; ok {
					d.Doctor = row
					itemized[row] = true
				}
			}
			planDrift = append(planDrift, d)
		}
		// wiring_drift's MCP and skill drift: things no plan names.
		if c, ok := failingRow["wiring_drift"]; ok {
			for _, d := range f.BeyondPlan {
				d.Source, d.Doctor = "doctor", "wiring_drift"
				if d.Remedy == "" {
					d.Remedy = c.Remedy
				}
				beyond = append(beyond, d)
				itemized["wiring_drift"] = true
			}
		}
		pd := fmt.Sprintf("of %d items the preview plan names", len(f.Plan))
		m.PlanApply = Measured(apply, "wiring.PlanWiring (the preview's engine, read-only)").WithDenominator(pd)
		m.PlanCurrent = Measured(skip, "wiring.PlanWiring (the preview's engine, read-only)").WithDenominator(pd)
		m.PlanBlocked = Measured(blocked, "wiring.PlanWiring (the preview's engine, read-only)").WithDenominator(pd)
		m.PlanDigest = f.PlanDigest
	}
	// A failing row no item accounts for is a thing of its own; one the plan
	// itemizes is named on its items and never counted again on top of them.
	for _, c := range failing {
		if !itemized[c.Name] {
			m.Drift = append(m.Drift, WiringDrift{Source: "doctor", What: c.Name, Detail: c.Reason, Remedy: c.Remedy})
		}
	}
	m.Drift = append(append(m.Drift, beyond...), planDrift...)
	switch {
	case f.PlanErr != "" && len(m.Drift) == 0:
		m.DriftCount = Errored[int]("drift is unknown: the preview plan could not be computed (" + f.PlanErr + ")")
	case f.PlanErr != "":
		m.DriftCount = Partial(len(m.Drift), "the preview plan could not be computed ("+f.PlanErr+"); doctor's rows only — a lower bound")
	default:
		m.DriftCount = Measured(len(m.Drift), src).WithDenominator("things on disk that differ, each counted once: the preview plan's apply and BLOCKED items, owned MCP entries and skills changed since fathom recorded them, and doctor's failing or warning wiring rows no item accounts for (a row that summarizes items is named on them, never added to them)")
	}

	switch {
	case f.ReceiptsErr != "":
		m.ReceiptsTotal = Errored[int]("the receipts could not be read: " + f.ReceiptsErr)
	default:
		m.Receipts = append(m.Receipts, f.Receipts...)
		d := "receipts fathom recorded for " + f.Root + " (every apply and undo)"
		if f.ReceiptsTotal > len(f.Receipts) {
			d += fmt.Sprintf("; the newest %d listed", len(f.Receipts))
		}
		m.ReceiptsTotal = Measured(f.ReceiptsTotal, "wiring.LoadReceipts").WithDenominator(d)
	}
	return m
}

// summarizingRow is the doctor row whose failure a plan item is one thing
// of: the BLOCKED drift wiring_drift counts, the hooks hooks_installed
// finds missing (under Claude, whose settings the plan writes), the stanza
// regions stanza_present checks. "" when no row summarizes the item.
func summarizingRow(it WiringPlanItem, harness string) string {
	switch {
	case it.Verdict == "blocked" && it.Cause == "drift":
		return "wiring_drift"
	case it.Kind == "key" && harness == "claude":
		return "hooks_installed"
	case it.Kind == "region":
		return "stanza_present"
	}
	return ""
}
