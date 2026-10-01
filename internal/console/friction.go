package console

// friction.go — WI-6, WI-7 (docs/design/work-intelligence.md §3 U3): the
// agent friction map per file, symbol and zone, counted from the activity
// record — tool events from the call and hook logs, the governance signal
// source's rule outcomes (GL-10), discovery pointers and the proof
// history — never transcript prose (ADR-041 decision 8). Remedies are
// computed from the design's pattern table and are proposals with their
// evidence: nothing here writes a rule, a doc or an anchor (ADR-051
// decision 8; WI-X4 refuses auto-applied remedies).

import (
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// Activity event kinds: the tool events friction counts.
const (
	ActSearch = "search" // a search or grep-shaped call
	ActLookup = "lookup" // a symbol or reference lookup
	ActEdit   = "edit"
	ActAnswer = "answer" // the turn ended with an answer
	ActRevert = "revert" // a hook-observed edit undone in the same session
)

// SearchLoopMin is U3's search loop: at least 5 consecutive search or
// lookup calls without an edit or an answer.
const SearchLoopMin = 5

// ActivityEvent is one counted tool event. Session is an opaque join key
// and never leaves this package in an output (I58). Note, when set, is
// the evidence line in place of the kind's default (a revert read from
// git names the commits, not a session).
type ActivityEvent struct {
	Session string
	At      string
	Kind    string
	File    string
	Symbol  string
	Note    string
}

// RuleEvent is one GL-10 rule-outcome row (served, still_served, top_changed,
// blocked). ComputeFriction reads Event under the current vocabulary
// (semantic.CanonicalRuleEvent), so a row an older binary wrote as violated
// counts as a still_served one.
type RuleEvent struct {
	Rule    string
	Event   string
	Session string
	File    string
	At      string
}

// DiscoveryPointer is a discovery pointer to a symbol and when it was
// first recorded.
type DiscoveryPointer struct {
	Symbol string
	At     string
}

// UnprovenAnchor is a claimed-unproven row's anchor, from the proof
// history and the plan's contract.
type UnprovenAnchor struct {
	Row    string
	File   string
	Symbol string
}

// FrictionInputs is everything FrictionSignals counts over.
type FrictionInputs struct {
	Events   []ActivityEvent
	Rules    []RuleEvent
	Pointers []DiscoveryPointer
	Unproven []UnprovenAnchor
	Zones    Zones
}

// FrictionCounts are one hotspot's signals.
type FrictionCounts struct {
	Rediscovery     int `json:"rediscovery"`
	Blocked         int `json:"blocked_edits"`
	ClaimedUnproven int `json:"claimed_unproven"`
	Reverts         int `json:"reverts"`
	SearchLoops     int `json:"search_loops"`
}

func (c FrictionCounts) total() int {
	return c.Rediscovery + c.Blocked + c.ClaimedUnproven + c.Reverts + c.SearchLoops
}

// RuleCount is how often one rule fired a signal at a hotspot.
type RuleCount struct {
	Rule string `json:"rule"`
	N    int    `json:"n"`
}

// Hotspot kinds.
const (
	HotspotFile   = "file"
	HotspotSymbol = "symbol"
	HotspotZone   = "zone"
)

// maxFrictionEvidence bounds the evidence lines a hotspot carries.
const maxFrictionEvidence = 8

// FrictionHotspot is one file, symbol or zone's friction.
type FrictionHotspot struct {
	Kind   string         `json:"kind"`
	Key    string         `json:"key"`
	Counts FrictionCounts `json:"counts"`
	Total  int            `json:"total"`
	// Blocks are PreToolUse denies per rule; ServedViolations are the
	// edit-hook repeat edits under a rule the same session was served
	// (still_served rows: the rule still topped the file's served set, which
	// is scope persistence, not proof the edit ignored it; JF-2, O255).
	Blocks           []RuleCount `json:"blocks,omitempty"`
	ServedViolations []RuleCount `json:"served_violations,omitempty"`
	// LoopEnds are the symbols the search loops here ended at.
	LoopEnds []string `json:"loop_ends,omitempty"`
	Evidence []string `json:"evidence"`
}

// FrictionMap is U3.
type FrictionMap struct {
	Files      []FrictionHotspot `json:"files"`
	Symbols    []FrictionHotspot `json:"symbols"`
	Zones      []FrictionHotspot `json:"zones"`
	Events     int               `json:"activity_events"`
	RuleEvents int               `json:"rule_events"`
}

type frictionAcc struct {
	h        FrictionHotspot
	blocks   map[string]int
	served   map[string]int
	loopEnds map[string]bool
}

// FrictionSignals counts U3's signals per file, symbol and zone. Every
// list sorts by total, highest first, then key.
func FrictionSignals(in FrictionInputs) FrictionMap {
	accs := map[string]map[string]*frictionAcc{HotspotFile: {}, HotspotSymbol: {}, HotspotZone: {}}
	get := func(kind, key string) *frictionAcc {
		a := accs[kind][key]
		if a == nil {
			a = &frictionAcc{h: FrictionHotspot{Kind: kind, Key: key}, blocks: map[string]int{}, served: map[string]int{}, loopEnds: map[string]bool{}}
			accs[kind][key] = a
		}
		return a
	}
	// hit applies f to the file, symbol and zone a signal touches.
	hit := func(file, symbol, evidence string, f func(*frictionAcc)) {
		var targets []*frictionAcc
		if file != "" {
			targets = append(targets, get(HotspotFile, file), get(HotspotZone, ZoneOf(file, in.Zones)))
		}
		if symbol != "" {
			targets = append(targets, get(HotspotSymbol, symbol))
		}
		for _, a := range targets {
			f(a)
			if len(a.h.Evidence) < maxFrictionEvidence {
				a.h.Evidence = append(a.h.Evidence, evidence)
			}
		}
	}

	pointer := map[string]string{}
	for _, p := range in.Pointers {
		if at, ok := pointer[p.Symbol]; !ok || p.At < at {
			pointer[p.Symbol] = p.At
		}
	}
	bySession := map[string][]ActivityEvent{}
	for _, e := range in.Events {
		bySession[e.Session] = append(bySession[e.Session], e)
		switch e.Kind {
		case ActSearch, ActLookup:
			if at, ok := pointer[e.Symbol]; ok && e.Symbol != "" && at < e.At {
				hit(e.File, e.Symbol, fmt.Sprintf("%s: %s found by %s after a discovery pointer to it existed (%s)", e.At, e.Symbol, e.Kind, at),
					func(a *frictionAcc) { a.h.Counts.Rediscovery++ })
			}
		case ActRevert:
			note := fmt.Sprintf("%s: an edit undone in the same session", e.At)
			if e.Note != "" {
				note = e.Note
			}
			hit(e.File, e.Symbol, note, func(a *frictionAcc) { a.h.Counts.Reverts++ })
		}
	}
	for _, s := range sortedKeysOf(bySession) {
		evs := bySession[s]
		sort.SliceStable(evs, func(i, j int) bool { return evs[i].At < evs[j].At })
		run := 0
		var last ActivityEvent
		flush := func() {
			if run >= SearchLoopMin {
				end := last
				hit(end.File, end.Symbol, fmt.Sprintf("%s: %d consecutive search or lookup calls without an edit or answer", end.At, run),
					func(a *frictionAcc) {
						a.h.Counts.SearchLoops++
						if end.Symbol != "" {
							a.loopEnds[end.Symbol] = true
						}
					})
			}
			run = 0
		}
		for _, e := range evs {
			switch e.Kind {
			case ActSearch, ActLookup:
				run++
				last = e
			case ActEdit, ActAnswer:
				flush()
			}
		}
		flush()
	}

	servedIn := map[string]bool{}
	for _, r := range in.Rules {
		if semantic.CanonicalRuleEvent(r.Event) == semantic.RuleEventServed {
			servedIn[r.Session+"\x00"+r.Rule] = true
		}
	}
	for _, r := range in.Rules {
		switch semantic.CanonicalRuleEvent(r.Event) {
		case semantic.RuleEventBlocked:
			hit(r.File, "", fmt.Sprintf("%s: edit denied by rule %s", r.At, r.Rule), func(a *frictionAcc) {
				a.h.Counts.Blocked++
				a.blocks[r.Rule]++
			})
		case semantic.RuleEventStillServed:
			served := servedIn[r.Session+"\x00"+r.Rule]
			note := fmt.Sprintf("%s: rule %s still topped this file's served set on a repeat edit", r.At, r.Rule)
			if served {
				note += " after it was served"
			}
			hit(r.File, "", note, func(a *frictionAcc) {
				a.h.Counts.Blocked++
				if served {
					a.served[r.Rule]++
				}
			})
		}
	}
	for _, u := range in.Unproven {
		hit(u.File, u.Symbol, fmt.Sprintf("row %s claimed but not proven", u.Row), func(a *frictionAcc) { a.h.Counts.ClaimedUnproven++ })
	}

	out := FrictionMap{Events: len(in.Events), RuleEvents: len(in.Rules)}
	for kind, dst := range map[string]*[]FrictionHotspot{HotspotFile: &out.Files, HotspotSymbol: &out.Symbols, HotspotZone: &out.Zones} {
		list := []FrictionHotspot{}
		for _, a := range accs[kind] {
			h := a.h
			h.Total = h.Counts.total()
			if h.Total == 0 {
				continue
			}
			h.Blocks = ruleCounts(a.blocks)
			h.ServedViolations = ruleCounts(a.served)
			h.LoopEnds = sortedKeys(a.loopEnds)
			if len(h.LoopEnds) == 0 {
				h.LoopEnds = nil
			}
			list = append(list, h)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Total != list[j].Total {
				return list[i].Total > list[j].Total
			}
			return list[i].Key < list[j].Key
		})
		*dst = list
	}
	return out
}

func ruleCounts(m map[string]int) []RuleCount {
	if len(m) == 0 {
		return nil
	}
	out := make([]RuleCount, 0, len(m))
	for r, n := range m {
		out = append(out, RuleCount{Rule: r, N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

func sortedKeysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── remedies (WI-7) ────────────────────────────────────────────────────────

// RemedyProposed is the only status a remedy has: fathom proposes, a
// person or an agent decides (WI-X4).
const RemedyProposed = "proposed"

// The pattern table (design §3 U3).
const (
	PatternRepeatedBlocks = "repeated blocks on one rule"
	PatternRediscovery    = "rediscovery of one symbol"
	PatternServedIgnored  = "violations of a served rule"
	PatternStaleDoc       = "a doc span flagged possibly stale near the hotspot"
	PatternLoopNoRefs     = "search loops ending in a symbol with no references"
)

// StaleDocSpan is a doc span G20's drift flags possibly stale, and the
// file or zone it sits near.
type StaleDocSpan struct {
	Doc  string
	Span string
	Near string
}

// RemedyInputs are the stored facts the pattern table reads: rule texts,
// reference counts of loop-end symbols (a symbol absent from the map was
// not counted, so no remedy is inferred for it), and stale doc spans.
type RemedyInputs struct {
	RuleText  map[string]string
	RefCounts map[string]int
	StaleDocs []StaleDocSpan
}

// Remedy is one proposal with its evidence.
type Remedy struct {
	TargetKind string   `json:"target_kind"`
	Target     string   `json:"target"`
	Pattern    string   `json:"pattern"`
	Proposal   string   `json:"proposal"`
	Evidence   []string `json:"evidence"`
	Status     string   `json:"status"`
}

// frictionRemedies computes a remedy for every hotspot a pattern matches.
// It reads m and in and writes nothing.
func frictionRemedies(m FrictionMap, in RemedyInputs) []Remedy {
	var out []Remedy
	add := func(h FrictionHotspot, pattern, proposal string, extra ...string) {
		ev := append(append([]string(nil), extra...), h.Evidence...)
		out = append(out, Remedy{TargetKind: h.Kind, Target: h.Key, Pattern: pattern, Proposal: proposal, Evidence: ev, Status: RemedyProposed})
	}
	var all []FrictionHotspot
	all = append(append(append(all, m.Files...), m.Symbols...), m.Zones...)
	for _, h := range all {
		if h.Kind == HotspotFile {
			for _, b := range h.Blocks {
				if b.N < 2 {
					continue
				}
				text := "rule " + b.Rule + ": " + in.RuleText[b.Rule]
				if in.RuleText[b.Rule] == "" {
					text = "rule " + b.Rule + ": its text was not read for this view"
				}
				add(h, PatternRepeatedBlocks, fmt.Sprintf("rule %s blocked %d edits here: it may be unclear — review its text against the blocked edits", b.Rule, b.N), text)
			}
			for _, v := range h.ServedViolations {
				add(h, PatternServedIgnored, fmt.Sprintf("rule %s was served and still topped this file's served set on a repeat edit %d time(s): move it to a hook (a compiled guard with enforce = true)", v.Rule, v.N))
			}
		}
		if h.Kind == HotspotSymbol {
			if h.Counts.Rediscovery >= 2 {
				add(h, PatternRediscovery, fmt.Sprintf("%s was re-found %d times after a pointer to it existed: add an anchor or a discovery pointer", h.Key, h.Counts.Rediscovery))
			}
			for _, sym := range h.LoopEnds {
				if n, ok := in.RefCounts[sym]; ok && n == 0 && sym == h.Key {
					add(h, PatternLoopNoRefs, fmt.Sprintf("search loops ended at %s, which has no references (unexported or dynamically dispatched): add an anchor or a governing note", sym))
				}
			}
		}
		for _, d := range in.StaleDocs {
			if d.Near == h.Key && h.Kind != HotspotSymbol {
				add(h, PatternStaleDoc, fmt.Sprintf("%s %s is flagged possibly stale near this hotspot: refresh the doc", d.Doc, d.Span), "stale span: "+d.Doc+" "+d.Span)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}
