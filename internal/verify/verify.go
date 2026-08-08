// Package verify computes an evidence-backed "definition of done" for plan tasks.
//
// It is the read-only verification layer of fathom: given a Plan (a human-edited
// list of tasks, each with an acceptance contract) and an Evidence view over the
// code index, VerifyPlan returns a per-task verdict where "done" is a
// deterministic predicate over governed code evidence — not a box someone ticked.
// An agent can claim a task is complete; verify can disagree, with citations.
//
// Honesty boundary (load-bearing): this layer is STATIC. `test_reaches` means a
// Test* function statically reaches the symbol via the call graph; it does NOT
// mean the test passes, asserts anything, or even runs. `tier == verified` means
// "parsed from real code," not "works." Whether the code actually works at
// runtime (`runtime_pass`) is reported as Unknown until external CI/coverage
// evidence is ingested as its own evidence tier. verify never emits "passing"
// from a static check.
//
// VerifyPlan is pure and deterministic: a function of (plan, evidence) only — no
// clock, no I/O, no randomness. Tasks and criteria come out in a fixed order.
package verify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Plan is the parsed form of a human-edited plan file. verify reads it; it never
// owns, stores, or mutates it.
type Plan struct {
	Service string
	Tasks   []Task
}

// Task is one unit of planned work with an acceptance contract.
type Task struct {
	ID         string
	Title      string
	SpecIDs    []string // declared spec IDs this task implements (e.g. "[US-042]")
	Expect     Expect
	Claimed    bool // true when checkbox is [x] or [X] — the agent claims this is done
	Skipped    bool // true when checkbox is [-] — exempt from gating
	InProgress bool // true when checkbox is [~] — not done, not gating (except --require-all)

	// Disposition refines WHY a [-] task is skipped: "refused" | "deferred".
	// It is the one authored state evidence cannot derive (ADR-018); LintPlan
	// enforces its conventions (only on [-], requires Reason).
	Disposition string
	// Reason is the free-text justification for a disposition (or any skip).
	Reason string
	// Trigger is the condition under which a deferred task revives. Recorded
	// and surfaced only — never machine-evaluated in v0 (ADR-018).
	Trigger string

	// AnnotationSuspect holds the raw brace block when the line carried
	// something annotation-shaped that parsed to zero keys (O4): the row has
	// silently lost its contract and its author-owned id, and LintPlan is
	// loud about it. Empty on healthy rows.
	AnnotationSuspect string
}

// Expect is a task's acceptance contract. The zero value is the STRICTEST
// contract (a test must reach the symbol, any contradiction fails it, the symbol
// must be at the verified tier) so that an under-specified task cannot pass by
// omission. Loosen explicitly.
type Expect struct {
	Symbol             string            // claimed symbol that must exist (e.g. "ValidateToken")
	Package            string            // optional: repo-relative directory Symbol's file must live under (disambiguates bare names)
	ReachableFrom      string            // optional: an entrypoint that must reach Symbol
	SkipTest           bool              // false (default) → a Test* must statically reach Symbol
	SkipReason         string            // justification for SkipTest (the "skip:" key; LintPlan warns when absent)
	AllowContradiction bool              // false (default) → any contradiction on Symbol fails the task
	MinTier            core.EvidenceTier // "" (default) → treated as TierVerified
}

// CriterionStatus is the outcome of a single acceptance criterion.
type CriterionStatus string

const (
	Pass    CriterionStatus = "pass"
	Fail    CriterionStatus = "fail"
	Unknown CriterionStatus = "unknown" // not statically computable (e.g. runtime_pass)
	NA      CriterionStatus = "n/a"     // not applicable (e.g. tier when symbol is absent)
)

// Criterion is one checked clause of a task's contract, with a citation.
type Criterion struct {
	Name   string // symbol_present | tier | test_reaches | edge_wired | no_contradiction | spec_covered | runtime_pass
	Status CriterionStatus
	Detail string // evidence: file:line, tier, reaching test, etc.
}

// TaskVerdict is the rolled-up verdict for one task.
type TaskVerdict struct {
	TaskID     string
	Title      string
	Status     CriterionStatus // Pass | Fail | Unknown (overall, static)
	Label      string          // verified | claimed-but-absent | present-but-untested | edge-not-wired | contradicted | likely-unconfirmed | below-required-tier | spec-uncovered | no-contract
	Criteria   []Criterion
	Claimed    bool // propagated from Task.Claimed ([x])
	Skipped    bool // propagated from Task.Skipped ([-])
	InProgress bool // propagated from Task.InProgress ([~])
}

// Summary holds report-level counts.
type Summary struct {
	Total   int
	Pass    int
	Fail    int
	Unknown int
}

// Report is the full verification result for a plan.
type Report struct {
	Service string
	Tasks   []TaskVerdict
	Summary Summary
}

// GateResult is the outcome of the push/CI blocking gate.
type GateResult struct {
	Pass     bool
	Failures []string // deterministic: "{id} {title}: {first-failing-criterion-detail}"
}

// Gate decides whether the push/CI gate passes for the given report.
// A task that is Claimed (checkbox [x]) and not Skipped must verify; if its
// overall Status is Fail, or if strict is true and Status is Unknown, Gate
// records a failure. Failures are in report order (sorted by task ID).
func Gate(rep Report, strict bool) GateResult {
	res := GateResult{Pass: true}
	for _, tv := range rep.Tasks {
		if !tv.Claimed || tv.Skipped {
			continue
		}
		if tv.Status == Fail || (strict && tv.Status == Unknown) {
			res.Pass = false
			res.Failures = append(res.Failures, fmt.Sprintf("%s %s: %s",
				tv.TaskID, tv.Title, gateFailDetail(tv, strict)))
		}
	}
	return res
}

// GateRequireAll is the wave-advance oracle (ADR-018 §2): normal Gate
// semantics PLUS every task must be either claimed (and verifying) or
// explicitly skipped. Open and in-progress tasks fail the gate — a wave may
// only advance when nothing is left undecided. Failures stay in report order:
// claimed-task failures first (from Gate), then incompleteness failures.
func GateRequireAll(rep Report, strict bool) GateResult {
	res := Gate(rep, strict)
	for _, tv := range rep.Tasks {
		if tv.Claimed || tv.Skipped {
			continue
		}
		state := "open"
		if tv.InProgress {
			state = "in-progress"
		}
		res.Pass = false
		res.Failures = append(res.Failures, fmt.Sprintf(
			"%s %s: not complete (%s) — require-all demands claimed-and-verified or skipped",
			tv.TaskID, tv.Title, state,
		))
	}
	return res
}

// gateFailDetail returns the most informative detail string for a failed gate
// check: the first failing criterion, then the first unknown (strict), then
// the task label as a last resort.
func gateFailDetail(tv TaskVerdict, strict bool) string {
	for _, c := range tv.Criteria {
		if c.Status == Fail {
			return c.Detail
		}
	}
	if strict {
		for _, c := range tv.Criteria {
			if c.Status == Unknown {
				return c.Detail
			}
		}
	}
	return tv.Label
}

// Evidence is the narrow view of the code index that verify needs. The MCP tool
// and the CLI implement it over the live store/call-graph; tests implement a
// fake. Implementations must return deterministically ordered results.
type Evidence interface {
	// FindSymbol returns symbol nodes matching name (exact / bare / qualified).
	FindSymbol(name string) []core.SymbolNode
	// TestReaches returns the canonical keys of Test* functions that statically
	// reach symbol via the call graph (empty if none).
	TestReaches(symbol string) []string
	// Reachable reports whether `from` reaches `to` in the call graph.
	Reachable(from, to string) bool
	// Contradictions returns all detected contradictions for the service.
	Contradictions() []core.Contradiction
	// SpecImplementors returns the symbols implementing specID (nil if uncovered).
	SpecImplementors(specID string) []string
}

// ReachDetail describes one Test* symbol that statically reaches a target,
// and the MODALITY of that reach (ADR-021 §3): whether the path crossed a
// typed interface-dispatch edge, and if so how many implementers were in
// scope. Mirrors internal/analysis/callgraph.ReachDetail — verify does not
// import internal/analysis (it depends on internal/core and agentic-kit's
// scoring foundation only), so DetailedEvidence implementations translate.
type ReachDetail struct {
	Key          string
	ViaDispatch  bool
	Implementers int // count of dispatch edges at the crossed caller; 0 when !ViaDispatch
}

// DetailedEvidence is an optional capability: an Evidence implementation
// that can additionally report the modality of a test_reaches path — did it
// cross a typed EdgeIfaceDispatch edge, and if so how many implementers
// were in scope (ADR-021 §3, dissolving the O9 ambiguity-refusal residue).
// VerifyPlan type-asserts for it and falls back to plain TestReaches when
// absent, so existing Evidence fakes — and any real Evidence backed by a
// store with no typed edges yet — keep working unchanged.
type DetailedEvidence interface {
	Evidence
	// TestReachesDetail returns TestReaches's canonical keys PLUS the
	// dispatch modality for each — same emptiness/ordering contract as
	// TestReaches.
	TestReachesDetail(symbol string) []ReachDetail
}

// VerifyPlan evaluates every task in plan against ev and returns a sorted,
// deterministic report.
func VerifyPlan(plan Plan, ev Evidence) Report {
	// Index contradictions once, by their (bare) symbol name.
	contraBySymbol := map[string][]core.Contradiction{}
	for _, c := range ev.Contradictions() {
		contraBySymbol[c.Symbol] = append(contraBySymbol[c.Symbol], c)
	}

	verdicts := make([]TaskVerdict, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		verdicts = append(verdicts, verifyTask(t, ev, contraBySymbol))
	}
	// Deterministic order by task ID.
	sort.Slice(verdicts, func(i, j int) bool { return verdicts[i].TaskID < verdicts[j].TaskID })

	sum := Summary{Total: len(verdicts)}
	for _, v := range verdicts {
		switch v.Status {
		case Pass:
			sum.Pass++
		case Fail:
			sum.Fail++
		default:
			sum.Unknown++
		}
	}
	return Report{Service: plan.Service, Tasks: verdicts, Summary: sum}
}

func verifyTask(t Task, ev Evidence, contraBySymbol map[string][]core.Contradiction) TaskVerdict {
	e := t.Expect
	v := TaskVerdict{TaskID: t.ID, Title: t.Title, Status: Pass, Label: "verified", Claimed: t.Claimed, Skipped: t.Skipped, InProgress: t.InProgress}

	// fail downgrades the overall status and records the most salient label
	// (first failure wins, since criteria are appended in priority order).
	fail := func(name, detail, label string) {
		v.Criteria = append(v.Criteria, Criterion{name, Fail, detail})
		if v.Status != Fail {
			v.Status = Fail
			v.Label = label
		}
	}
	pass := func(name, detail string) { v.Criteria = append(v.Criteria, Criterion{name, Pass, detail}) }
	na := func(name, detail string) { v.Criteria = append(v.Criteria, Criterion{name, NA, detail}) }

	checked := false

	// --- symbol presence + tier + test_reaches ---
	// querySym is the identifier used for reachability queries. It stays the
	// claimed (possibly bare) name unless a `package:` filter narrows the
	// matches to exactly one canonical key — then that key is used, so a bare
	// name that is ambiguous across packages can still verify. A filter that
	// leaves multiple distinct keys keeps the bare name: ambiguity resolves to
	// nothing downstream, never first-match-wins.
	querySym := e.Symbol
	var matches []core.SymbolNode
	if e.Symbol != "" {
		checked = true
		matches = ev.FindSymbol(e.Symbol)
		if e.Package != "" {
			matches = underDir(matches, e.Package)
			if keys := distinctKeys(matches); len(keys) == 1 {
				querySym = keys[0]
			}
		}
		if len(matches) == 0 {
			detail := fmt.Sprintf("%q not found in the index", e.Symbol)
			if e.Package != "" {
				detail = fmt.Sprintf("%q not found under %q in the index", e.Symbol, e.Package)
			}
			fail("symbol_present", detail, "claimed-but-absent")
		} else {
			pass("symbol_present", fmt.Sprintf("%s at %s:%d", e.Symbol, matches[0].FilePath, matches[0].Line))
		}

		// tier
		if len(matches) == 0 {
			na("tier", "symbol absent")
		} else {
			minTier := e.MinTier
			if minTier == "" {
				minTier = core.TierVerified
			}
			best := bestTier(matches)
			if scoring.TierRank(string(best)) >= scoring.TierRank(string(minTier)) {
				pass("tier", fmt.Sprintf("%s (>= required %s)", best, minTier))
			} else {
				label := "below-required-tier"
				if best == core.TierPreScan {
					label = "likely-unconfirmed"
				}
				fail("tier", fmt.Sprintf("%s (< required %s)", best, minTier), label)
			}
		}

		// test_reaches (static). When ev additionally implements
		// DetailedEvidence (ADR-021 §3), a path crossing a typed
		// interface-dispatch edge counts as reached — strictly better
		// evidence than the old blanket refusal for an ambiguous method
		// name — and is labeled distinctly in the pass detail.
		if !e.SkipTest {
			if len(matches) == 0 {
				na("test_reaches", "symbol absent")
			} else if de, ok := ev.(DetailedEvidence); ok {
				if details := de.TestReachesDetail(querySym); len(details) > 0 {
					pass("test_reaches", testReachesDetail(details))
				} else {
					fail("test_reaches", "no Test* statically reaches it", "present-but-untested")
				}
			} else if tests := ev.TestReaches(querySym); len(tests) > 0 {
				pass("test_reaches", fmt.Sprintf("%d test(s) statically reach it, e.g. %s", len(tests), tests[0]))
			} else {
				fail("test_reaches", "no Test* statically reaches it", "present-but-untested")
			}
		}

		// no_contradiction
		if !e.AllowContradiction {
			if cs := contradictionsFor(e.Symbol, contraBySymbol); len(cs) > 0 {
				fail("no_contradiction",
					fmt.Sprintf("conflicting evidence: %s vs %s", cs[0].A.SourceRef, cs[0].B.SourceRef),
					"contradicted")
			} else {
				pass("no_contradiction", "no conflicting evidence")
			}
		}
	}

	// --- edge wiring (requires both endpoints) ---
	if e.ReachableFrom != "" && e.Symbol != "" {
		checked = true
		if ev.Reachable(e.ReachableFrom, querySym) {
			pass("edge_wired", fmt.Sprintf("%s reaches %s", e.ReachableFrom, querySym))
		} else {
			fail("edge_wired", fmt.Sprintf("%s does not reach %s", e.ReachableFrom, querySym), "edge-not-wired")
		}
	}

	// --- spec coverage ---
	if len(t.SpecIDs) > 0 {
		checked = true
		specs := append([]string(nil), t.SpecIDs...)
		sort.Strings(specs)
		var uncovered []string
		for _, s := range specs {
			if len(ev.SpecImplementors(s)) == 0 {
				uncovered = append(uncovered, s)
			}
		}
		if len(uncovered) == 0 {
			pass("spec_covered", fmt.Sprintf("all %d spec(s) have implementing symbols", len(specs)))
		} else {
			fail("spec_covered", "uncovered: "+strings.Join(uncovered, ", "), "spec-uncovered")
		}
	}

	// --- runtime: never asserted by a static check ---
	v.Criteria = append(v.Criteria, Criterion{
		Name:   "runtime_pass",
		Status: Unknown,
		Detail: "no CI/coverage evidence ingested — static checks cannot prove the test passes",
	})

	if !checked {
		// No acceptance contract to evaluate — don't pretend it's verified.
		v.Status = Unknown
		v.Label = "no-contract"
	}
	return v
}

// testReachesDetail renders test_reaches's pass detail from a DetailedEvidence
// result, surfacing the dispatch modality distinctly when the reaching path
// used one (ADR-021 §3) — a reader sees "reached via interface dispatch" at
// a glance instead of it reading identically to a direct static reach.
// details is assumed non-empty and sorted by Key (callgraph.TestImpactDetail's
// contract); the first dispatch-reaching entry is reported when one exists,
// otherwise the first entry overall.
func testReachesDetail(details []ReachDetail) string {
	for _, d := range details {
		if d.ViaDispatch {
			return fmt.Sprintf("%d test(s) statically reach it, e.g. %s (reached via interface dispatch, %d implementer(s))",
				len(details), d.Key, d.Implementers)
		}
	}
	return fmt.Sprintf("%d test(s) statically reach it, e.g. %s", len(details), details[0].Key)
}

// underDir returns the nodes whose FilePath lies under the repo-relative
// directory dir. Index paths use forward slashes; matching is a pure string
// prefix (subdirectories included), preserving VerifyPlan's purity.
func underDir(nodes []core.SymbolNode, dir string) []core.SymbolNode {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var out []core.SymbolNode
	for _, n := range nodes {
		if strings.HasPrefix(n.FilePath, prefix) {
			out = append(out, n)
		}
	}
	return out
}

// distinctKeys returns the distinct canonical keys of nodes, in first-seen
// order (duplicate observations of the same symbol collapse to one key).
func distinctKeys(nodes []core.SymbolNode) []string {
	seen := make(map[string]struct{}, len(nodes))
	var keys []string
	for _, n := range nodes {
		k := n.Key()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

// bestTier returns the highest-ranked evidence tier among the matched nodes.
func bestTier(nodes []core.SymbolNode) core.EvidenceTier {
	best := nodes[0].Tier
	for _, n := range nodes[1:] {
		if scoring.TierRank(string(n.Tier)) > scoring.TierRank(string(best)) {
			best = n.Tier
		}
	}
	return best
}

// contradictionsFor returns contradictions whose symbol matches name by exact or
// bare-name suffix (contradiction symbols are typically bare).
func contradictionsFor(name string, idx map[string][]core.Contradiction) []core.Contradiction {
	if cs, ok := idx[name]; ok {
		return cs
	}
	bare := name
	if i := strings.LastIndex(bare, "."); i >= 0 {
		bare = bare[i+1:]
	}
	return idx[bare]
}
