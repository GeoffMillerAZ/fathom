package main

// plans_weighted.go — PE-3 (docs/design/progress-and-economics.md §4, spec
// interop §4): `fathom plans status --weighted` rolls each row's `weight:`
// up by fleet, project, plan and section (a plan's `## ` heading — the wave
// label). Proven weight is claimed rows the gate proves; total weight is
// every row but the dispositioned ([-]) ones, which carry none. The console's
// weighted table (PE-4) renders the same tree.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// weightedScope is one level of the weighted rollup.
type weightedScope struct {
	Scope    string          `json:"scope"` // fleet | project | plan | section
	Name     string          `json:"name"`
	Proven   int             `json:"proven_weight"`
	Total    int             `json:"total_weight"`
	Rows     int             `json:"rows"`
	Children []weightedScope `json:"children,omitempty"`
}

// noSection names rows above a plan's first `## ` heading.
const noSection = "(no section)"

// planSection is one `## ` heading and the line it starts on.
type planSection struct {
	Line  int
	Label string
}

// planSectionsOf lists a plan's `## ` headings in order.
func planSectionsOf(raw []byte) []planSection {
	var out []planSection
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	n := 0
	for sc.Scan() {
		n++
		if l := sc.Text(); strings.HasPrefix(l, "## ") {
			out = append(out, planSection{Line: n, Label: strings.TrimSpace(l[3:])})
		}
	}
	return out
}

// sectionAt is the label of the heading a line falls under.
func sectionAt(secs []planSection, line int) string {
	label := noSection
	for _, s := range secs {
		if s.Line > line {
			break
		}
		label = s.Label
	}
	return label
}

// rowWeight is a row's weight, 1 when unset.
func rowWeight(t verify.Task) int {
	if t.Weight < 1 {
		return 1
	}
	return t.Weight
}

// weightedRollup builds the fleet → project → plan → section tree over one
// service's plans (each row's Section names its wave).
func weightedRollup(service string, plans []string, rowSets [][]planRow) weightedScope {
	return weightedFleet([]weightedScope{weightedProject(service, plans, rowSets)})
}

// weightedFleet sums projects into the fleet.
func weightedFleet(projects []weightedScope) weightedScope {
	fleet := weightedScope{Scope: "fleet", Name: "fleet"}
	for _, p := range projects {
		fleet.Proven, fleet.Total, fleet.Rows = fleet.Proven+p.Proven, fleet.Total+p.Total, fleet.Rows+p.Rows
		fleet.Children = append(fleet.Children, p)
	}
	return fleet
}

// weightedProject is one service's plans, each by section.
func weightedProject(service string, plans []string, rowSets [][]planRow) weightedScope {
	proj := weightedScope{Scope: "project", Name: service}
	for i, pf := range plans {
		plan := weightedScope{Scope: "plan", Name: pf}
		bySec := map[string]*weightedScope{}
		var order []string
		for _, r := range rowSets[i] {
			label := r.Section
			if label == "" {
				label = noSection
			}
			s := bySec[label]
			if s == nil {
				s = &weightedScope{Scope: "section", Name: label}
				bySec[label] = s
				order = append(order, label)
			}
			s.Rows++
			if r.Task.Skipped {
				continue
			}
			w := rowWeight(r.Task)
			s.Total += w
			if r.Task.Claimed && r.Verdict.Status == verify.Pass {
				s.Proven += w
			}
		}
		for _, label := range order {
			s := *bySec[label]
			plan.Proven, plan.Total, plan.Rows = plan.Proven+s.Proven, plan.Total+s.Total, plan.Rows+s.Rows
			plan.Children = append(plan.Children, s)
		}
		proj.Proven, proj.Total, proj.Rows = proj.Proven+plan.Proven, proj.Total+plan.Total, proj.Rows+plan.Rows
		proj.Children = append(proj.Children, plan)
	}
	return proj
}

// weightedShare renders proven/total and the percentage.
func weightedShare(s weightedScope) string {
	if s.Total == 0 {
		return fmt.Sprintf("%d/%d", s.Proven, s.Total)
	}
	return fmt.Sprintf("%d/%d (%d%%)", s.Proven, s.Total, s.Proven*100/s.Total)
}

// writeWeighted prints the rollup as JSON or an indented table.
func writeWeighted(w io.Writer, root weightedScope, jsonOut bool) {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(root)
		return
	}
	fmt.Fprintf(w, "%-60s %6s  %s\n", "scope", "rows", "proven weight / total weight")
	var walk func(s weightedScope, depth int)
	walk = func(s weightedScope, depth int) {
		name := s.Name
		if s.Scope == "plan" {
			name = shortPlanName(name)
		}
		fmt.Fprintf(w, "%-60s %6d  %s\n", strings.Repeat("  ", depth)+s.Scope+" "+name, s.Rows, weightedShare(s))
		for _, c := range s.Children {
			walk(c, depth+1)
		}
	}
	walk(root, 0)
}

// shortPlanName is a plan path from docs/plans on.
func shortPlanName(p string) string {
	if i := strings.Index(p, "docs/plans/"); i >= 0 {
		return p[i:]
	}
	return p
}
