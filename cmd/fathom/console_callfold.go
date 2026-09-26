package main

// console_callfold.go — the call log's all-time aggregates (RT-112,
// docs/design/responsible-tenant.md §14: "the call-log tail keeps
// aggregates plus a 35-day ring (no row per line forever)").
//
// Every all-time figure a console page or verb shows over the call log —
// row counts, per-tool outcomes and costs, per-service totals, per-surface
// traffic, caller classes, the flywheel's call-log check — is a fold over
// the rows in file order. callLogFold is that fold: the console's tail
// adds each line once, as it is read, and keeps only the fold plus the
// rows inside the ring's window; the verbs fold a whole read the same way
// (foldCallLog), so a page and its verb compute every all-time figure with
// one function and can never disagree.
//
// The fold holds one small record per tool, per (tool, surface, origin),
// per service and per surface — bounded by the vocabulary, not by the
// log's length. The one list it keeps is the `fathom review` runs that
// carry per-dimension finding counts (reviewRunsOf's rows: one per review
// run, a handful a day), which the catches page lists individually.

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// callLogFold is the all-time fold of a call log's counted rows.
type callLogFold struct {
	rows, unattributed int
	// lastTS is the greatest ts; agentSurface counts rows whose surface
	// names the agent (the flywheel's call-log check).
	lastTS       string
	agentSurface int
	obligations  int
	briefCalls   int
	briefClaims  int
	tools        map[string]toolFold
	callers      map[callerKey]int
	services     map[string]serviceFold // lower-cased service name
	surfaces     map[string]surfaceFold
	reviews      []console.ReviewRun
}

// toolFold is one tool's calls, outcomes (IQ-1) and response cost (BA-6).
type toolFold struct {
	calls                           int
	notRecorded, didYouMean, misses int
	hits                            int
	totalBytes, truncating, elided  int
}

// callerKey is what a caller class is decided from (console.ClassifyCaller).
type callerKey struct{ tool, surface, origin string }

// serviceFold is one service's traffic: every call, the agent's, the
// latest ts and the obligations served.
type serviceFold struct {
	total, agent, obligations int
	last                      string
}

// surfaceFold is one surface's calls and the ts of its last row in file
// order (the adoption gauge's "last write wins").
type surfaceFold struct {
	calls  int
	lastTS string
}

func newCallLogFold() *callLogFold {
	return &callLogFold{tools: map[string]toolFold{}, callers: map[callerKey]int{},
		services: map[string]serviceFold{}, surfaces: map[string]surfaceFold{}}
}

// foldCallLog folds a whole read's entries, in file order.
func foldCallLog(entries []adoptionCallEntry) *callLogFold {
	f := newCallLogFold()
	for i := range entries {
		f.add(&entries[i])
	}
	return f
}

// add folds one counted row; rows must arrive in file order.
func (f *callLogFold) add(e *adoptionCallEntry) {
	f.rows++
	if strings.TrimSpace(e.Service) == "" {
		f.unattributed++
	}
	if e.TS > f.lastTS {
		f.lastTS = e.TS
	}
	if strings.Contains(e.Surface, "agent") {
		f.agentSurface++
	}
	if e.Tool == "obligations" {
		f.obligations++
	}
	if e.Tool == "brief" {
		f.briefCalls++
		if e.ClaimsAttached > 0 {
			f.briefClaims++
		}
	}
	t := f.tools[e.Tool]
	t.calls++
	switch {
	case e.ResultCount == nil:
		t.notRecorded++
	case e.MatchedVia == "did_you_mean":
		t.didYouMean++
	case *e.ResultCount == 0:
		t.misses++
	default:
		t.hits++
	}
	if e.ResponseBytes > 0 {
		t.totalBytes += e.ResponseBytes
	}
	if e.ElidedCount != nil {
		t.truncating++
		if *e.ElidedCount > 0 {
			t.elided++
		}
	}
	f.tools[e.Tool] = t
	f.callers[callerKey{e.Tool, e.Surface, e.Origin}]++
	if e.Service != "" {
		k := strings.ToLower(e.Service)
		s := f.services[k]
		s.total++
		if e.Surface == "agent" || e.Surface == "companion" {
			s.agent++
		}
		if e.TS > s.last {
			s.last = e.TS
		}
		if e.Tool == "obligations" {
			s.obligations++
		}
		f.services[k] = s
	}
	s := f.surfaces[e.Surface]
	s.calls++
	s.lastTS = e.TS
	f.surfaces[e.Surface] = s
	if e.Tool == "review" && e.VerdictOutcome != nil {
		f.reviews = append(f.reviews, console.ReviewRun{At: e.TS, Service: e.Service, Outcome: e.VerdictOutcome})
	}
}

// unregistered folds the rows whose service r does not list (OC-H2) over
// every row the fold counted, in service order.
func (f *callLogFold) unregistered(r console.Registry) console.Unregistered {
	if !r.Read {
		return console.Unregistered{RegistryNote: r.Note}
	}
	u := console.Unregistered{}
	for _, k := range slices.Sorted(maps.Keys(f.services)) {
		if r.Excludes(k) {
			u.Merge(console.Unregistered{Rows: f.services[k].total, Services: []string{k}})
		}
	}
	return u
}

// clone is a copy a reader owns while the tail keeps folding.
func (f *callLogFold) clone() *callLogFold {
	c := *f
	c.tools, c.callers = maps.Clone(f.tools), maps.Clone(f.callers)
	c.services, c.surfaces = maps.Clone(f.services), maps.Clone(f.surfaces)
	c.reviews = slices.Clip(f.reviews)
	return &c
}

// missModel is buildMissModel's answer: outcomes per tool, most-called
// first. Rows logged before IQ-1 count as NotRecorded — never as hits.
func (f *callLogFold) missModel() []obsMissRow {
	rows := make([]obsMissRow, 0, len(f.tools))
	for tool, t := range f.tools {
		if tool == "" {
			continue
		}
		rows = append(rows, obsMissRow{Tool: tool, Calls: t.calls, Hits: t.hits, Misses: t.misses, DidYouMean: t.didYouMean, NotRecorded: t.notRecorded})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Calls != rows[j].Calls {
			return rows[i].Calls > rows[j].Calls
		}
		return rows[i].Tool < rows[j].Tool
	})
	return rows
}

// toolClasses is toolClassesOf's answer: each tool's dominant caller class.
func (f *callLogFold) toolClasses() map[string]console.CallerClass {
	counts := make([]console.CallerCount, 0, len(f.callers))
	for k, n := range f.callers {
		counts = append(counts, console.CallerCount{Tool: k.tool, Surface: k.surface, Origin: k.origin, Calls: n})
	}
	out := map[string]console.CallerClass{}
	for _, tl := range console.ToolLoadsCounted(counts) {
		out[tl.Tool] = tl.Class
	}
	return out
}

// reviewRuns is reviewRunsOf's answer, in file order.
func (f *callLogFold) reviewRuns() []console.ReviewRun {
	return slices.Clone(f.reviews)
}

// toolCallCounts is every tool's calls (the empty tool included), as the
// adoption gauge counts them.
func (f *callLogFold) toolCallCounts() map[string]int {
	out := make(map[string]int, len(f.tools))
	for tool, t := range f.tools {
		out[tool] = t.calls
	}
	return out
}

// facts is callLogFactsFor over the fold.
func (f *callLogFold) facts(resolved console.ResolvedCallLog, readErr error) console.CallLogFacts {
	out, ok := callLogFactsState(resolved, readErr)
	if ok && f != nil {
		out.Rows, out.Unattributed = f.rows, f.unattributed
	}
	return out
}
