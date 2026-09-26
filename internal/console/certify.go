package console

// certify.go — the certifications page's projection (OC-24, design §3
// "Certifications"): the targets the certifications layer considers
// (declared targets, invariant rows, incident records, ports) by the state
// of their best certificate, per kind, with the gap list and its remedies.
// It reads `fathom certify gaps`'s own report (certify.Gaps) per service;
// fathom asserts liveness, never adequacy (ADR-043 decision 3), so the
// matrix counts states and never scores them.

import (
	"fmt"
	"sort"
	"strings"
)

// The certifications layer's shared metrics (OC-9): the fleet's targets
// with no live certificate and with one, each CertMatrix's own value.
const (
	MetricCertGaps MetricID = "certify.gaps"
	MetricCertLive MetricID = "certify.live"
)

// CertFact is one certificate on a target.
type CertFact struct {
	Test     string `json:"test"`
	Suite    string `json:"suite,omitempty"`
	Strength string `json:"strength,omitempty"`
	State    string `json:"state"`
	Tier     string `json:"tier,omitempty"`
}

// CertTargetFact is one target as `certify gaps` reported it.
type CertTargetFact struct {
	Kind         string     `json:"kind"`
	ID           string     `json:"id"`
	Gap          bool       `json:"gap"`
	Remedy       string     `json:"remedy,omitempty"`
	Certificates []CertFact `json:"certificates"`
	SourceFile   string     `json:"source_file,omitempty"`
	SourceLine   int        `json:"source_line,omitempty"`
}

// CertServiceFacts is one service's `certify gaps` report, or why there is
// none.
type CertServiceFacts struct {
	Service     string           `json:"service"`
	Targets     []CertTargetFact `json:"targets"`
	Declared    int              `json:"declared"`
	Live        int              `json:"live"`
	Gaps        int              `json:"gaps"`
	JoinedCases int              `json:"joined_cases"`
	Cases       int              `json:"cases"`
	// Skip names an empty target universe ("no manifests found — nothing
	// declared"): not a pass. Err names a report that could not be made.
	Skip string `json:"skip,omitempty"`
	Err  string `json:"error,omitempty"`
}

// The certificate states a target can stand in, best first (certify.State
// plus "none" for a target no certificate names). Only live satisfies.
var CertStates = []string{"live", "flaky", "broken", "unproven", "unreachable", "none"}

// CertMatrixCell is one kind × state count.
type CertMatrixCell struct {
	Count int `json:"count"`
}

// CertMatrixRow is one target kind across the states.
type CertMatrixRow struct {
	Kind  string           `json:"kind"`
	Total int              `json:"total"`
	Cells []CertMatrixCell `json:"cells"`
}

// CertServiceRow is one service's line.
type CertServiceRow struct {
	Service  string     `json:"service"`
	Declared Value[int] `json:"declared"`
	Live     Value[int] `json:"live"`
	Gaps     Value[int] `json:"gaps"`
	// JoinRate is joined test cases over cases, as a percentage; absent
	// when the service has recorded no test cases.
	JoinRate Value[int] `json:"join_rate_percent"`
	Href     string     `json:"href"`
}

// CertGap is one target with no live certificate, and the remedy.
type CertGap struct {
	Service string `json:"service"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	State   string `json:"state"`
	Remedy  string `json:"remedy"`
	Source  string `json:"source,omitempty"`
}

// CertMatrixView is the certifications page.
type CertMatrixView struct {
	// Scope is the one service the matrix covers, or "" for the fleet.
	Scope    string           `json:"scope"`
	States   []string         `json:"states"`
	Rows     []CertMatrixRow  `json:"rows"`
	Services []CertServiceRow `json:"services"`
	Gaps     []CertGap        `json:"gaps"`
	Declared Value[int]       `json:"declared"`
	Live     Value[int]       `json:"live"`
	GapCount Value[int]       `json:"gap_count"`
}

// BestCertState is a target's standing: its best certificate's state, or
// "none" when no certificate names it.
func BestCertState(t CertTargetFact) string {
	best := len(CertStates) - 1
	for _, c := range t.Certificates {
		for i, s := range CertStates {
			if s == c.State && i < best {
				best = i
			}
		}
	}
	return CertStates[best]
}

// CertMatrix projects the per-service gap reports into the targets ×
// certificate-states matrix, the per-service lines and the gap list
// (OC-24). A service whose report could not be made is named, and the
// fleet totals then are errors rather than smaller numbers (I38); a service
// that declares nothing is listed with its skip reason and adds nothing.
func CertMatrix(services []CertServiceFacts, scope string, serviceHref func(string) string) CertMatrixView {
	if serviceHref == nil {
		serviceHref = func(s string) string { return "/certifications?svc=" + s }
	}
	v := CertMatrixView{Scope: scope, States: CertStates, Rows: []CertMatrixRow{}, Gaps: []CertGap{}}
	byKind := map[string]*CertMatrixRow{}
	var declared, live, gaps int
	var failed []string
	for _, s := range services {
		row := CertServiceRow{Service: s.Service, Href: serviceHref(s.Service)}
		const source = "fathom certify gaps"
		switch {
		case s.Err != "":
			failed = append(failed, s.Service+" ("+s.Err+")")
			row.Declared = Errored[int]("certify gaps failed: " + s.Err).WithSource(source)
			row.Live, row.Gaps, row.JoinRate = row.Declared, row.Declared, row.Declared
		case s.Skip != "" && len(s.Targets) == 0:
			row.Declared = Absent[int](s.Skip).WithSource(source)
			row.Live, row.Gaps = row.Declared, row.Declared
			row.JoinRate = joinRate(s)
		default:
			row.Declared = Measured(s.Declared, source).WithDenominator("targets considered")
			row.Live = Measured(s.Live, source).WithDenominator(fmt.Sprintf("of %d targets", s.Declared))
			row.Gaps = Measured(s.Gaps, source).WithDenominator(fmt.Sprintf("of %d targets", s.Declared))
			row.JoinRate = joinRate(s)
			declared += s.Declared
			live += s.Live
			gaps += s.Gaps
		}
		v.Services = append(v.Services, row)
		for _, t := range s.Targets {
			mr := byKind[t.Kind]
			if mr == nil {
				mr = &CertMatrixRow{Kind: t.Kind, Cells: make([]CertMatrixCell, len(CertStates))}
				byKind[t.Kind] = mr
			}
			state := BestCertState(t)
			for i, st := range CertStates {
				if st == state {
					mr.Cells[i].Count++
				}
			}
			mr.Total++
			if t.Gap {
				g := CertGap{Service: s.Service, Kind: t.Kind, ID: t.ID, State: state, Remedy: t.Remedy}
				if t.SourceFile != "" {
					g.Source = t.SourceFile
					if t.SourceLine > 0 {
						g.Source += fmt.Sprintf(":%d", t.SourceLine)
					}
				}
				v.Gaps = append(v.Gaps, g)
			}
		}
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		v.Rows = append(v.Rows, *byKind[k])
	}
	sort.SliceStable(v.Gaps, func(i, j int) bool {
		a, b := v.Gaps[i], v.Gaps[j]
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.ID < b.ID
	})
	v.Declared, v.Live, v.GapCount = CertifyTotals(services)
	if scope != "" && len(services) == 1 && services[0].Skip != "" && v.Declared.State == StateAbsent {
		// A repository's own view names its own skip.
		reason := services[0].Service + ": " + services[0].Skip
		v.Declared, v.Live, v.GapCount = Absent[int](reason).WithSource(certifySource), Absent[int](reason).WithSource(certifySource), Absent[int](reason).WithSource(certifySource)
	}
	return v
}

// certifySource names what the certification totals are read from.
const certifySource = "fathom certify gaps, per service"

// CertifyTotals is the fleet's certification totals over per-service
// `certify gaps` reports — the targets declared, those with a live
// certificate (certify.live) and the gaps (certify.gaps) — the one
// projection behind the Now, Fleet and Certifications pages (OC-9, OC-H3).
// A service whose report could not be made, or whose targets could not all
// be read, makes every total an error naming it, never a smaller number
// (I38); a service that declares nothing adds nothing.
func CertifyTotals(services []CertServiceFacts) (declared, live, gaps Value[int]) {
	var d, l, g, repos int
	var failed []string
	for _, s := range services {
		switch {
		case s.Err != "":
			failed = append(failed, s.Service+" ("+s.Err+")")
		case s.Declared > 0:
			d += s.Declared
			l += s.Live
			g += s.Gaps
			repos++
		}
	}
	sort.Strings(failed)
	switch {
	case len(failed) > 0:
		reason := "certify gaps failed for " + joinFew(failed) + " — no total is shown, since one without them would read as fewer gaps"
		e := Errored[int](reason).WithSource(certifySource)
		return e, e, e
	case d == 0:
		a := Absent[int]("no service declares a certification target").WithSource(certifySource)
		return a, a, a
	}
	in := countNoun(repos, "repo")
	return Measured(d, certifySource).WithDenominator("targets declared in " + in),
		Measured(l, certifySource).WithDenominator(fmt.Sprintf("of %d targets in %s", d, in)),
		Measured(g, certifySource).WithDenominator(fmt.Sprintf("of %d targets in %s", d, in))
}

// CertUnreadErr is a service's certify report that could not read every
// target source, as every page names it: its counts would be a lower bound.
func CertUnreadErr(unread []string) string {
	return "certify could not read " + strings.Join(unread, "; ") + " — its targets would be a lower bound"
}

// CertFactsFromCounts is a project's certify status (the project set's
// per-project read) as the totals read it — the same facts the
// Certifications page builds from the same report.
func CertFactsFromCounts(service string, c Value[CertCounts]) (CertServiceFacts, bool) {
	f := CertServiceFacts{Service: service, Targets: []CertTargetFact{}}
	switch {
	case c.V.Err != "":
		f.Err = c.V.Err
	case len(c.V.Unreadable) > 0:
		f.Err = CertUnreadErr(c.V.Unreadable)
	case c.State == StateError:
		f.Err = c.Reason
	case !c.State.HasValue():
		return f, false // not read (no local root, no store): adds nothing
	default:
		f.Declared, f.Live, f.Gaps, f.Cases, f.JoinedCases = c.V.Declared, c.V.Live, c.V.Gaps, c.V.Cases, c.V.JoinedCases
	}
	return f, true
}

func joinRate(s CertServiceFacts) Value[int] {
	if s.Cases == 0 {
		return Absent[int]("no test cases recorded for this service — run `task tests:attest` or `fathom tests ingest`").WithSource("the store's test runs")
	}
	return Measured(s.JoinedCases*100/s.Cases, "the store's test runs").WithDenominator(fmt.Sprintf("%d of %d cases joined to a symbol", s.JoinedCases, s.Cases))
}
