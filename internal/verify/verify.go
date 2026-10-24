// Package verify computes an evidence-backed "definition of done" for plan tasks.
//
// It is the read-only verification layer of fathom: given a Plan (a human-edited
// list of tasks, each with an acceptance contract) and an Evidence view over the
// code index, VerifyPlan returns a per-task verdict where "done" is a
// deterministic predicate over governed code evidence — not a box someone ticked.
// An agent can claim a task is complete; verify can disagree, with citations.
//
// Honesty boundary (load-bearing): this layer is STATIC — it never executes a
// test or a command. `test_reaches` means a Test* function statically reaches
// the symbol via the call graph; it does NOT mean the test passes, asserts
// anything, or even runs. `tier == verified` means "parsed from real code," not
// "works." Whether the code actually works at runtime (`runtime_pass`) is read
// from attested evidence only: the newest attested run of the row's certifying
// tests in test_runs (TC-29, runtime.go), informational, never changing a
// row's status (I8 as amended by ADR-067). verify never emits "passing" from a
// static check.
//
// VerifyPlan is pure and deterministic: a function of (plan, evidence) only — no
// clock, no I/O, no randomness. Tasks and criteria come out in a fixed order.
package verify

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/scoring"
)

// Plan is the parsed form of a human-edited plan file. verify reads it; it never
// owns, stores, or mutates it.
type Plan struct {
	Service string
	Tasks   []Task

	// Baseline is the plan-level "Baseline: <sha>" header line (ADR-044 §4):
	// the base commit every row in the plan is proved against, written by
	// `fathom refactor plan` and read by `fathom refactor status` and
	// `fathom conserve`. "" when the plan carries no header line — most
	// plans, which pin no baseline.
	Baseline string

	// Proof is the plan-level "Proof: <value>" header line (ADR-049,
	// docs/design/spec-interop.md §4), like Baseline:. The only value
	// verifyTask acts on is "certified" (compared case-insensitively):
	// once set, every symbol: row must also carry certified_by:, or is
	// refused certificate-absent — through the same EvaluateCertifiedBy
	// path an ordinary certified_by: row already goes through, with no
	// skip_test escape. "" (unset) is today's tiers, unchanged.
	Proof string

	// DeletedRows is set externally by AnnotateSelfDeclared — NEVER by
	// ParseMarkdownPlan (there is no per-plan annotation for it): the
	// sorted IDs of rows present in the base plan and absent from this
	// one (I45's "rows deleted since base"). nil when no comparison was
	// made.
	DeletedRows []string

	// UnparsedRows are the lines carrying an author-id annotation (`{id:`)
	// that ParseMarkdownPlan did not read as a row under that id, in line
	// order: LintPlan names each as an error. nil when every one parsed.
	UnparsedRows []UnparsedRow
}

// UnparsedRow is one `{id:` line ParseMarkdownPlan could not read as a row.
type UnparsedRow struct {
	Line   int    // 1-based line of the row's checkbox, or of the stray line
	ReadAs string // the positional id the row was read under; "" when no row was
	Reason string // why the annotation was not read
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

	// Ctx carries discovery-pack pointers (symbol IDs, file paths, concern
	// IDs, thread IDs) parsed from a space-separated ctx: value. Non-gating
	// by construction: VerifyPlan never reads it. A pack renderer resolves
	// the pointers against the live index at read time.
	Ctx []string
	// CtxMalformed is true when the row carried an explicit but empty ctx:
	// value. LintPlan warns — never errors; ctx stays non-gating either way.
	CtxMalformed bool

	// Needs lists the ids (another row's id:) this row depends on — the
	// eligibility DAG. Non-gating in verify; LintPlan errors on an unknown
	// id, a self-reference, or a cycle. A target of the form
	// "<sibling>.md#<id>" names a row in another plan (PC-10): LintPlan
	// checks its shape only; resolveCrossPlanNeed reads the sibling.
	Needs []string

	// Source is the plan file (base name) a row was read from, set only by
	// loaders that merge several plan files into one Plan (langbench's
	// LoadBreadthPlan); the Markdown parser leaves it empty.
	Source string `json:"-"`

	// Line is the 1-based line of the checkbox row in the plan file — the
	// location a SARIF result points at (PC-21). 0 when the plan did not
	// come from Markdown.
	Line int
	// EndLine is the 1-based last line the parser read for this row: Line
	// for a one-line row, the last indented continuation line for a
	// wrapped one (G2-5). A writer that edits the row's annotation edits
	// lines Line..EndLine. 0 when the plan did not come from Markdown.
	EndLine int
	// UnreadAnnotation holds an annotation block found on an UNINDENTED
	// line directly under a row that carries none (G2-5): Markdown may read
	// that line as the row's lazy continuation, the parser does not, and
	// LintPlan names it so the contract is never lost silently.
	UnreadAnnotation string

	// Oracle names a non-index acceptance contract. "lexical" (ADR-030):
	// OraclePath is a repo-relative file that must exist and be non-empty
	// in the working tree verify reads plans from, and OraclePattern (a Go
	// regexp, line-anchored) must match it at least once. "ledger"
	// (ADR-034 decision 2, RC-34): OraclePath names a committed JSONL bench
	// ledger; OracleRunID/OracleTaskID/OracleArm key one row in it (a
	// task/arm-less row — the home bench's one-row-per-run shape — is
	// matched by OracleRunID alone), and OracleMinTrials (default 1, when
	// set) is the trial floor the row must meet. A task/arm-less run_id
	// that matches more than one home row (the same run scored under two
	// registered criteria) is ambiguous and refused unless OracleCriterion
	// names the one row's evaluation.criterion_id to pick. A matched row
	// whose own evaluation.verdict is fail/indeterminate is refused too,
	// unless OracleAcceptVerdict is "any". One contract per row: a row
	// carrying both a symbol and an oracle fails verification and is
	// refused by LintPlan.
	Oracle              string
	OraclePath          string
	OraclePattern       string
	OracleRunID         string
	OracleTaskID        string
	OracleArm           string
	OracleMinTrials     string
	OracleCriterion     string
	OracleAcceptVerdict string

	// SelfSkipped is set externally by AnnotateSelfSkip — NEVER by
	// ParseMarkdownPlan, there is no `{self_skipped: …}` annotation key.
	// O56 (docs/dogfood-observations.md / .fathom/observations.md): a
	// worker told its row was present-but-untested added `skip_test: true`
	// to its OWN row "to make the automated verifier pass cleanly." An
	// agent may not grade its own row: when a git-based comparison finds
	// this row's skip_test/skip was ADDED in the same edit (working tree
	// vs HEAD) or the same commit (HEAD vs its first parent) that flipped
	// its box to [x], AnnotateSelfSkip sets this true and verifyTask fails
	// the row outright — self-skipped — regardless of what the skip:
	// reason says. A skip that predates the claim (already present at the
	// comparison point) leaves this false and the row stands.
	SelfSkipped bool

	// SelfLibrary is SelfSkipped's twin for `library: true` (ADR-018
	// amendment 2026-09-28), set by the same comparisons (AnnotateSelfSkip,
	// AnnotateSelfDeclared) and never by ParseMarkdownPlan: a claimed row
	// whose library waiver was added in the edit/commit (or range) that
	// claims it gets no waiver — an agent may not grade its own row.
	SelfLibrary bool

	// SelfDeclared is set externally by AnnotateSelfDeclared — NEVER by
	// ParseMarkdownPlan. The sorted list of declaration keys (I45:
	// "skip", "disposition", "retires", "surface_change", "max") this row
	// gained relative to a base version of the plan — a declaration added
	// INSIDE the change range excuses nothing. nil when no comparison was
	// made, or the row gained none of them.
	SelfDeclared []string

	// --- ADR-044 (vision 34) refactor-conservation grammar ---
	//
	// Gone is the raw value of a `gone: <form>` row (ADR-044 §3.1): "import
	// <importpath>", "module <path>", `pattern "<regex>"` (optionally
	// followed by a `paths:` annotation, read separately), or "callers
	// <Symbol>" (with `package:` naming its package, already parsed into
	// Expect.Package). Evaluated by internal/verify.EvaluateGone (a later
	// row); this Task only carries the text.
	Gone string
	// Tests refines a `gone: callers` row: "allowed" permits nonzero TEST
	// referents to the retired symbol (design §3.1); any other value (most
	// commonly "", unset) requires zero test referents too, same as zero
	// non-test referents.
	Tests string
	// Retires lists capability-inventory keys (design §2.1) this row
	// declares INTENTIONALLY REMOVED — a `retires: <key>[, <key>...]`
	// value, repeatable via the same comma-separated convention Needs and
	// CertifiedBy use (quote a multi-key value). Each key is either an
	// exact inventory key or a "<prefix>*" wildcard. This is the
	// declaration internal/conserve.Declarations reads to call a loss
	// "prior" rather than "self" or "none".
	Retires []string
	// SurfaceChange lists capability-inventory keys this row declares
	// INTENTIONALLY CHANGED (not removed) — same shape and matching rule
	// as Retires.
	SurfaceChange []string
	// GonePaths is a `gone: pattern` row's optional `paths:` annotation
	// (design §3.1's "with optional paths:", CV-27/CV-28,
	// EvaluateGone/evaluateGonePattern): repeatable repo-relative paths
	// (an exact file, or a "dir/" prefix matching every file under it) —
	// same comma-separated convention Retires/SurfaceChange use — scoping
	// the row's regexp scan. Empty scans the whole corpus. Refused
	// (goneContractError) on any gone: form other than pattern.
	GonePaths []string
	// Max/Min/Eq bound a row's contract's match count: a lexical oracle's
	// pattern (CV-29, lexicalBoundsVerdict) or a `gone:` ratchet's caller
	// count (design §3.1/§4, a later row). nil means unset — a lexical row
	// with no bound keeps its original "at least one match" contract; a
	// `gone:` row with no Max is not a ratchet. Eq is refused (lint AND
	// verify) combined with either Max or Min — one bound shape per row.
	Max, Min, Eq *int
	// BoundsInvalid names the max/min/eq annotation keys whose raw value
	// did not parse as an integer (e.g. `max: nope`) — the row's contract
	// is malformed exactly the way an unparsable min_trials is (O4: loud,
	// never a silently-ignored bound). LintPlan reports each one.
	BoundsInvalid []string

	// --- spec interop (vision 38, ADR-049, docs/design/spec-interop.md §4) ---
	//
	// Weight is the row's share of a weighted progress rollup (`weight:`):
	// a positive integer, 1 when the row carries no weight: key. Never
	// gating — verifyTask does not read it. ParseMarkdownPlan always sets
	// it; a Task built by hand with Weight 0 means "unset", i.e. 1.
	Weight int
	// WeightInvalid is the quoted raw `weight:` value when it was not a
	// positive integer (zero, negative, fractional, a word, empty) —
	// LintPlan refuses it and Weight falls back to 1. "" on healthy rows.
	WeightInvalid string
	// Accept is the row's acceptance text (`accept:`, EARS-shaped where
	// the author likes). Never gating by itself: a row whose only
	// annotation is accept: still verifies no-contract. Shown in the
	// converge report; proven only through certified_by:.
	Accept string

	// DerivedTests are test names inferred from a foreign task's text.
	// Unlike Expect.CertifiedBy they add a certificate requirement without
	// replacing test_reaches. Synthesis carries them as derived_tests for
	// daemon verification; explicit certified_by rows ignore this field.
	DerivedTests []string
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

	// CertifiedBy names the test(s) a `certified_by: <Test>` annotation
	// declares (vision 33 / ADR-043 stage 1, TC-9, design doc §4) —
	// repeatable via a comma-separated value, same convention as Needs.
	// When non-empty, it REPLACES the ordinary test_reaches requirement
	// with the certifications layer's own, stronger proof: only a LIVE
	// certificate from one of these tests satisfies the row (see
	// EvaluateCertifiedBy). Empty means "no certified_by contract" — the
	// ordinary test_reaches/skip_test path applies unchanged.
	CertifiedBy []string
	// CertifyStrength is the optional `strength: <level>` floor every
	// named test's certificate must meet or exceed (design doc §1's
	// closed, ordered ladder). "" defaults to the ladder's own default
	// (extractor.DefaultCertifyStrength, "asserts").
	CertifyStrength string

	// Library is `library: true` (ADR-018 amendment 2026-09-28): the row's
	// deliverable is the API itself, so a symbol no product entry point
	// reaches is not library-only. Never free: it needs the row's
	// `reason:` naming why the API is the deliverable, and it may not be
	// added by the edit or commit that claims the row (Task.SelfLibrary).
	Library bool
}

// IsZero reports whether e carries no contract at all — the struct-
// equality check `e == Expect{}` used to answer this before CertifiedBy
// (a slice, incomparable) joined the struct; callers outside this package
// (cmd/fathom's gen-plan-cue) use this instead of comparing to a literal
// zero value.
func (e Expect) IsZero() bool {
	return e.Symbol == "" && e.Package == "" && e.ReachableFrom == "" &&
		!e.SkipTest && e.SkipReason == "" && !e.AllowContradiction && e.MinTier == "" &&
		len(e.CertifiedBy) == 0 && e.CertifyStrength == "" && !e.Library
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
	Name   string // symbol_present | tier | test_reaches | edge_wired | wired | library | no_contradiction | spec_covered | runtime_pass
	Status CriterionStatus
	Detail string // evidence: file:line, tier, reaching test, etc.
}

// TaskVerdict is the rolled-up verdict for one task.
type TaskVerdict struct {
	TaskID     string
	Title      string
	Status     CriterionStatus // Pass | Fail | Unknown (overall, static)
	Label      string          // verified | claimed-but-absent | present-but-untested | edge-not-wired | library-only | contradicted | likely-unconfirmed | below-required-tier | spec-uncovered | no-contract
	Criteria   []Criterion
	Claimed    bool // propagated from Task.Claimed ([x])
	Skipped    bool // propagated from Task.Skipped ([-])
	InProgress bool // propagated from Task.InProgress ([~])
	Line       int  // propagated from Task.Line (0 when unknown)
	// Weight is propagated from Task.Weight (`weight:`, ADR-049): the row's
	// share of a weighted rollup, never gating. 0 means unset, i.e. 1.
	Weight int
	// EvidenceFile / EvidenceLine are the resolved location of the task's
	// claimed symbol when symbol_present passed — the SAME location
	// verifyTask's own symbol_present Pass detail names in prose ("%s at
	// %s:%d"), carried here as typed fields so a caller that needs the
	// location (OverlayMutation) never re-parses that free text. Both are
	// zero when the task carries no symbol or symbol_present did not pass.
	EvidenceFile string
	EvidenceLine int
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
		if c.Status == Fail && !informational(c.Name) {
			return c.Detail
		}
	}
	if strict {
		for _, c := range tv.Criteria {
			if c.Status == Unknown && !informational(c.Name) {
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
	// Reachable reports whether `from` reaches `to` in the call graph over
	// edges of resolution tier typed only (RF-5, I57): a reach that rests
	// on a heuristic edge is no reach (HeuristicReachEvidence names it).
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

// Certification is one compiled certifications-layer claim (vision 33 /
// ADR-043 stage 1, internal/certify.CompileCertifications), reduced to
// what EvaluateCertifiedBy needs to judge a `certified_by: <Test>`
// contract. verify does not import internal/certify (a leaf package
// depending only on internal/core and agentic-kit's scoring foundation) —
// CertificationEvidence implementations translate, the same pattern
// DetailedEvidence's ReachDetail already uses for internal/analysis/
// callgraph.
type Certification struct {
	// Test is the certifying test's bare name (the same form TestReaches'
	// own results use).
	Test string
	// Strength is the certificate's declared strength, on the closed
	// ladder design doc §1 names (smoke < asserts < golden < property <
	// conformance < mutation-killed).
	Strength string
	// State is the certifications layer's own state fact: "live" |
	// "flaky" | "broken" | "unproven" | "unreachable" (internal/certify.
	// State's values, mirrored here as plain strings for the same
	// no-cross-package-dependency reason as Strength).
	State string
	// Scope is the claim's own Scope — the test file plus the files of
	// the symbols the test reaches that the target names (design doc
	// §3). A certified_by row is satisfied by a test whose certificate
	// Scope includes the row's claimed symbol's FILE.
	Scope []string
}

// certifyStrengthRank mirrors internal/extractor.CertifyStrengthRank's
// ordering (design doc §1's closed ladder) without importing that
// package — verify stays a leaf dependency. The two lists MUST be kept in
// sync; TestEvaluateCertifiedBy_EveryVerdict pins the values this side
// depends on.
var certifyStrengthRank = map[string]int{
	"smoke": 0, "asserts": 1, "golden": 2, "property": 3, "conformance": 4, "mutation-killed": 5,
}

// defaultCertifyStrength mirrors internal/extractor.DefaultCertifyStrength.
const defaultCertifyStrength = "asserts"

// EvaluateCertifiedBy is the certified_by: <Test> contract's own criterion
// (vision 33 / ADR-043 stage 1, TC-9, design doc §4): symbolFile is the
// row's claimed symbol's defining file (verifyTask already resolves this
// for symbol_present); certifiedBy is the row's certified_by test name
// list; minStrength is the row's optional strength: floor ("" defaults to
// the ladder's own default); byTest maps a certifying test's bare name to
// every certification claim provenance-attributed to it (typically every
// compiled certifications-layer claim whose Meta.Test equals that name —
// CertificationEvidence.CertificationsFor's own contract).
//
// Only a LIVE certificate from one of the named tests, at or above
// minStrength, whose Scope includes symbolFile, proves the row
// ("certified"). Every other outcome fails, labeled by the MOST
// INFORMATIVE reason found across every named test's every certificate —
// certificate-broken (a certificate exists, scoped to this file, and its
// last attested run failed) > certificate-unproven (exists, scoped, never
// run) > certificate-unreachable (exists, scoped, but the test does not
// reach anything the target names) > certificate-absent (nothing
// certifies this file from any named test at all) — so a row naming
// three tests where one actually ran and failed is never silently
// reported as if none of them existed. The second return is the failing
// label (""  on Pass), the same (Criterion, label) shape evalLexicalOracle/
// ledgerOracle already use.
func EvaluateCertifiedBy(symbolFile string, certifiedBy []string, minStrength string, byTest map[string][]Certification) (Criterion, string) {
	if minStrength == "" {
		minStrength = defaultCertifyStrength
	}
	wantRank := certifyStrengthRank[minStrength]

	rank := func(state string) int {
		switch state {
		case "broken":
			return 3
		case "unproven":
			return 2
		case "unreachable":
			return 1
		default: // "absent" / unrecognized
			return 0
		}
	}
	best := "absent"
	var bestDetail string
	anyNamed := false
	for _, test := range certifiedBy {
		certs, ok := byTest[test]
		if !ok {
			continue
		}
		anyNamed = true
		for _, c := range certs {
			if !scopeContains(c.Scope, symbolFile) {
				continue
			}
			if certifyStrengthRank[c.Strength] < wantRank {
				continue
			}
			if c.State == "live" {
				return Criterion{Name: "certified_by", Status: Pass, Detail: fmt.Sprintf(
					"%s certifies this file (strength %s, live)", c.Test, c.Strength)}, ""
			}
			state := c.State
			if state == "" {
				state = "absent"
			}
			if rank(state) > rank(best) {
				best = state
				bestDetail = fmt.Sprintf("%s's certificate for this file is %s (strength %s)", c.Test, c.State, c.Strength)
			}
		}
	}
	if !anyNamed || bestDetail == "" {
		names := strings.Join(certifiedBy, ", ")
		return Criterion{Name: "certified_by", Status: Fail, Detail: fmt.Sprintf(
			"no certificate from %s covers this file", names)}, "certificate-absent"
	}
	return Criterion{Name: "certified_by", Status: Fail, Detail: bestDetail}, "certificate-" + best
}

// scopeContains reports whether file appears verbatim in scope — a
// certification claim's Scope is a flat list of repo-relative file paths
// (design doc §3: "the test file plus the files of the symbols the test
// reaches that the target names"), never a glob, so an exact match is the
// whole contract.
func scopeContains(scope []string, file string) bool {
	for _, s := range scope {
		if s == file {
			return true
		}
	}
	return false
}

// CertificationEvidence is an optional capability: an Evidence that can
// report the certifications layer's compiled state (vision 33 / ADR-043
// stage 1, TC-9). VerifyPlan type-asserts for it — an Evidence without it
// leaves every certified_by row Fail with "no certification evidence
// available", never a fabricated pass.
type CertificationEvidence interface {
	// CertificationsByTest returns, for the given test bare names, every
	// compiled certification claim provenance-attributed to each —
	// EvaluateCertifiedBy's own byTest map.
	CertificationsByTest(tests []string) map[string][]Certification
}

// ProductEvidence is an optional capability (ADR-018 amendment
// 2026-09-28, "done means proven AND wired"): an Evidence that can say
// which symbols a PRODUCT entry point reaches — every Go `package main`'s
// main outside testdata/, and the initialization (init functions and
// package-level variable initializers) of the packages those link
// (internal/analysis/callgraph.ProductEntryReach, over the same
// syntax+typed graph test_reaches walks, following its typed edges only
// since RF-5: HeuristicReachEvidence names a reach that rests on a
// heuristic edge). VerifyPlan type-asserts for it;
// an Evidence without it adds no `wired` criterion at all, so a row is
// judged exactly as before rather than refused for an instrument it was
// never given.
type ProductEvidence interface {
	// ProductEntryCount is the number of product entry points. 0 means
	// the service has no binary (a library): wiring does not apply.
	ProductEntryCount() int
	// ProductReachedBy names the product entry point that statically
	// reaches symbol (a canonical key or a name the call graph resolves)
	// over edges of resolution tier typed only (RF-5, I57).
	ProductReachedBy(symbol string) (entry string, ok bool)
}

// HeuristicEdge is one call edge of resolution tier heuristic a refused
// reach rested on (RF-5, ADR-067 D3, I57): the first edge, on a path from
// where the typed reach ends, that no typed pass confirmed. Mirrors
// internal/analysis/callgraph.HeuristicEdge; verify imports no analysis
// package, so HeuristicReachEvidence implementations translate. Kind is
// the edge's label ("" for a syntax-only call); Tier is its resolution
// tier as core.ResolutionTier.String renders it.
type HeuristicEdge struct {
	From, To string
	Kind     string
	Tier     string
}

// String renders the edge the way a refusal names it:
// "<from> -> <to> (tier heuristic, syntax-only)".
func (h HeuristicEdge) String() string {
	kind := h.Kind
	if kind == "" {
		kind = "syntax-only"
	}
	tier := h.Tier
	if tier == "" {
		tier = "heuristic"
	}
	return fmt.Sprintf("%s -> %s (tier %s, %s)", h.From, h.To, tier, kind)
}

// HeuristicReachEvidence is an optional capability (RF-5): Evidence.
// Reachable and ProductEvidence.ProductReachedBy answer over typed edges
// only, and an Evidence that also has the tier-blind walk names the
// heuristic edge a refused reach would have rested on, so the refusal says
// why. Read only after the typed answer refused; an Evidence without it
// gets the plain refusal.
type HeuristicReachEvidence interface {
	// HeuristicReach: from reaches to only through a heuristic edge, the
	// first such edge on the path.
	HeuristicReach(from, to string) (HeuristicEdge, bool)
	// ProductHeuristicReach: a product entry point reaches symbol only
	// through a heuristic edge — the entry and that edge.
	ProductHeuristicReach(symbol string) (entry string, edge HeuristicEdge, ok bool)
}

// ReachNameEvidence is an optional capability: an Evidence that says how a
// reach endpoint's name resolves in the graph Reachable walks, so a refused
// edge_wired tells a reachable_from that names nothing, or several symbols,
// from a missing typed path. Read only after the reach refused; an Evidence
// without it gets the plain refusal.
type ReachNameEvidence interface {
	// ResolveReachName returns the one key name resolves to. When it
	// resolves to none it returns "" and every key carrying the name,
	// sorted: two or more is an ambiguous name, none is a name the graph
	// does not hold.
	ResolveReachName(name string) (key string, candidates []string)
}

// maxReachCandidates caps the candidate keys an ambiguous reachable_from
// detail names.
const maxReachCandidates = 5

// unplacedReachFrom is edge_wired's detail when reachable_from resolves to
// no one key: cands are the keys carrying the name (ResolveReachName).
func unplacedReachFrom(name string, cands []string) string {
	if len(cands) < 2 {
		return fmt.Sprintf("reachable_from %s names no symbol in the call graph, so no reach was walked: check its spelling and that its file is indexed", name)
	}
	shown := cands
	more := ""
	if len(shown) > maxReachCandidates {
		shown = shown[:maxReachCandidates]
		more = fmt.Sprintf(", and %d more", len(cands)-maxReachCandidates)
	}
	return fmt.Sprintf("reachable_from %s is ambiguous: %d symbols carry that name (%s%s), and no reach is walked from an ambiguous name; write a method as Recv.Method, and rename a function until its name is unique",
		name, len(cands), strings.Join(shown, ", "), more)
}

// FileEvidence is an optional capability: an Evidence that can read the
// files of the repo the plan belongs to, for the lexical oracle (ADR-030).
// ReadRepoFile takes a repo-relative path; a missing file is an error.
// VerifyPlan type-asserts for it — an Evidence without it leaves every
// lexical row Unknown with a reason, never a fabricated pass.
type FileEvidence interface {
	ReadRepoFile(path string) ([]byte, error)
}

// VerifyPlan evaluates every task in plan against ev and returns a sorted,
// deterministic report.
func VerifyPlan(plan Plan, ev Evidence) Report {
	// Index contradictions once, by their (bare) symbol name.
	contraBySymbol := map[string][]core.Contradiction{}
	for _, c := range ev.Contradictions() {
		contraBySymbol[c.Symbol] = append(contraBySymbol[c.Symbol], c)
	}
	files, _ := ev.(FileEvidence)
	// ADR-049, design §4: "Proof: certified" is compared case-insensitively,
	// the same way the "Proof:" label itself is matched by parseProofHeader —
	// an author writing "Certified" in the header should not silently get
	// today's tiers back.
	proofCertified := strings.EqualFold(plan.Proof, "certified")

	verdicts := make([]TaskVerdict, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		verdicts = append(verdicts, verifyTask(t, ev, files, contraBySymbol, proofCertified))
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

// proofCertified is true when the plan's "Proof: certified" header
// (ADR-049, design §4) is set: every symbol: row must then also carry
// certified_by:, or is refused certificate-absent, with no skip_test
// escape.
func verifyTask(t Task, ev Evidence, files FileEvidence, contraBySymbol map[string][]core.Contradiction, proofCertified bool) TaskVerdict {
	e := t.Expect
	v := TaskVerdict{TaskID: t.ID, Title: t.Title, Status: Pass, Label: "verified", Claimed: t.Claimed, Skipped: t.Skipped, InProgress: t.InProgress, Line: t.Line, Weight: t.Weight}

	// fail downgrades the overall status and records the most salient label
	// (first failure wins, since criteria are appended in priority order).
	fail := func(name, detail, label string) {
		v.Criteria = append(v.Criteria, Criterion{name, Fail, detail})
		if v.Status != Fail {
			v.Status = Fail
			v.Label = label
		}
	}
	// graphFail is fail for a criterion read from the call graph: when the
	// evidence ran on the syntax graph because the typed edges could not be
	// read (O1162), a miss may be the fallback's, so the criterion reads
	// Unknown naming the cause, never Fail (I38, I44).
	graphFail := func(name, detail, label string) {
		cause := GraphFallbackCause(ev)
		if cause == "" {
			fail(name, detail, label)
			return
		}
		v.Criteria = append(v.Criteria, Criterion{Name: name, Status: Unknown, Detail: graphFallbackDetail(detail, cause)})
		if v.Status == Pass {
			v.Status, v.Label = Unknown, GraphFallbackLabel
		}
	}
	pass := func(name, detail string) { v.Criteria = append(v.Criteria, Criterion{name, Pass, detail}) }
	na := func(name, detail string) { v.Criteria = append(v.Criteria, Criterion{name, NA, detail}) }

	checked := false

	// --- non-code oracles: a row's contract is a file, not a symbol ---
	// Evaluated first so that a row carrying BOTH a symbol and an oracle is
	// labeled by the contract conflict, not by whatever the index says
	// about the symbol; the symbol path is skipped for it entirely.
	// "ledger" (ADR-034 decision 2) dispatches to ledgerOracle; every
	// other value (including "lexical" and an unrecognized name) goes to
	// evalLexicalOracle, which is itself the one that reports "unknown
	// oracle" for anything but "lexical".
	if t.Oracle != "" {
		checked = true
		evalOracle := evalLexicalOracle
		if t.Oracle == "ledger" {
			evalOracle = ledgerOracle
		}
		c, label := evalOracle(t, files)
		switch c.Status {
		case Pass:
			pass(c.Name, c.Detail)
		case Fail:
			fail(c.Name, c.Detail, label)
		default:
			v.Criteria = append(v.Criteria, c)
			if v.Status == Pass {
				v.Status = Unknown
				v.Label = label
			}
		}
		if e.Symbol != "" {
			v.Criteria = append(v.Criteria, Criterion{Name: "runtime_pass", Status: Unknown, Detail: "not evaluated: one contract per row"})
			return v
		}
	}

	// --- gone: rows (ADR-044 §3.1, CV-27/CV-28) ---
	// An absence contract, never combined with symbol:/oracle: (design
	// §3.1; goneContractError refuses the combination). Evaluated early,
	// like the oracle block above, so a row carrying gone AND symbol is
	// labeled by the contract conflict rather than by whatever the index
	// says about the symbol — the symbol path below is skipped for it
	// entirely. An Evidence with no GoneEvidence capability leaves the
	// row honestly Unknown (never a fabricated pass), the same pattern
	// FileEvidence/CertificationEvidence already use.
	if t.Gone != "" {
		checked = true
		var c Criterion
		var label string
		if ge, ok := ev.(GoneEvidence); ok {
			c, label = EvaluateGone(t, ge)
		} else {
			c, label = Criterion{Name: goneCriterion, Status: Unknown, Detail: "no gone evidence available on this evidence source"}, "gone-unevaluated"
		}
		switch c.Status {
		case Pass:
			pass(c.Name, c.Detail)
		case Fail:
			fail(c.Name, c.Detail, label)
		default:
			v.Criteria = append(v.Criteria, c)
			if v.Status == Pass {
				v.Status = Unknown
				v.Label = label
			}
		}
		if e.Symbol != "" {
			v.Criteria = append(v.Criteria, Criterion{Name: "runtime_pass", Status: Unknown, Detail: "not evaluated: one contract per row"})
			return v
		}
	}

	// --- symbol presence + tier + test_reaches ---
	// querySym is the identifier used for reachability queries. It stays the
	// claimed (possibly bare) name unless a `package:` filter narrows the
	// matches to exactly one canonical key — then that key is used, so a bare
	// name that is ambiguous across packages can still verify. A filter that
	// leaves multiple distinct keys keeps the bare name: ambiguity resolves to
	// nothing downstream, never first-match-wins.
	querySym := e.Symbol
	var matches []core.SymbolNode
	ambiguous := false // O1453: a bare anchor that more than one distinct definition matches
	// reaching is the tests test_reaches found: runtime_pass's certifying
	// tests when the row names none (runtimeTests).
	var reaching []string
	if e.Symbol != "" {
		checked = true
		raw := ev.FindSymbol(e.Symbol)
		matches = codeMatches(raw)
		if e.Package != "" {
			matches = underDir(matches, e.Package)
		}
		// A package filter that leaves one distinct key names the canonical
		// target. So does a bare name that collided with a doc heading and
		// narrowed to ONE code key once the doc chunks were dropped: handed
		// to the call graph as a bare name it would be ambiguous and every
		// reachability check would answer "nothing" (INC-21: h1's `{symbol:
		// Gate}` flipped between stores on exactly this). A bare name that
		// matched only code is passed through as written — the graph
		// resolves it by name, as it always has.
		if keys := distinctKeys(matches); len(keys) == 1 && (e.Package != "" || len(matches) < len(raw)) {
			querySym = keys[0]
		}
		if keys := distinctKeys(matches); e.Package == "" && len(keys) > 1 && bareAnchorUnproven(e, ev, proofCertified) {
			// O1453: a bare anchor two packages define names neither: the
			// row asks for `package:` instead of passing on the first match
			// and then failing reach as a missing test.
			ambiguous = true
			pkgs := make([]string, 0, len(keys))
			seen := map[string]bool{}
			for _, n := range matches {
				d := path.Dir(n.FilePath)
				if !seen[d] {
					seen[d] = true
					pkgs = append(pkgs, d)
				}
			}
			sort.Strings(pkgs)
			fail("symbol_present", fmt.Sprintf("ambiguous: %d definitions (%s); add package:", len(keys), strings.Join(pkgs, ", ")), "ambiguous-anchor")
		} else if len(matches) == 0 {
			detail := fmt.Sprintf("%q not found in the index", e.Symbol)
			if e.Package != "" {
				detail = fmt.Sprintf("%q not found under %q in the index", e.Symbol, e.Package)
			}
			fail("symbol_present", detail, "claimed-but-absent")
		} else {
			pass("symbol_present", fmt.Sprintf("%s at %s:%d", e.Symbol, matches[0].FilePath, matches[0].Line))
			v.EvidenceFile = matches[0].FilePath
			v.EvidenceLine = matches[0].Line
		}

		// tier
		if ambiguous {
			na("tier", "ambiguous anchor")
		} else if len(matches) == 0 {
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

		// test_reaches (static) — or, when the row claims skip_test, the
		// skip_test escape contract itself (O56). A skip_test exemption is
		// never free: it requires a `skip:` reason (a bare skip_test is
		// refused as skip-without-reason, same as an absent contract would
		// be) AND it may not be self-graded (a skip AnnotateSelfSkip found
		// was added in the same edit/commit that claims the row done is
		// refused as self-skipped, regardless of what the reason says —
		// the agent may not grade its own row). When ev additionally
		// implements DetailedEvidence (ADR-021 §3), a path crossing a typed
		// interface-dispatch edge counts as reached — strictly better
		// evidence than the old blanket refusal for an ambiguous method
		// name — and is labeled distinctly in the pass detail.
		switch {
		case proofCertified && len(e.CertifiedBy) == 0:
			// ADR-049, design §4: the plan-level header is the strongest
			// gate — a row cannot escape it via skip_test, and this case
			// is checked ahead of every skip_test case for exactly that
			// reason.
			fail("certified_by", "Proof: certified requires certified_by: on every symbol: row (docs/design/spec-interop.md §4)", "certificate-absent")
		case e.SkipTest && e.SkipReason == "":
			fail("skip_test", "skip_test: true with no skip: reason — bare skips are refused", "skip-without-reason")
		case e.SkipTest && t.SelfSkipped:
			fail("skip_test", fmt.Sprintf(
				"skip_test/skip was added in the same edit or commit that claims this row done — an agent may not grade its own row (self-skipped); skip: %q",
				e.SkipReason,
			), "self-skipped")
		case e.SkipTest:
			pass("skip_test", fmt.Sprintf("test-reach requirement waived: %s", e.SkipReason))
		case ambiguous:
			na("test_reaches", "ambiguous anchor")
		case len(matches) == 0:
			na("test_reaches", "symbol absent")
		case len(e.CertifiedBy) > 0:
			// certified_by (vision 33 / ADR-043 stage 1, TC-9, design doc
			// §4) REPLACES the ordinary test_reaches check with the
			// certifications layer's own stronger proof: only a LIVE
			// certificate from a named test satisfies the row. An
			// Evidence with no CertificationEvidence capability fails
			// honestly rather than silently falling back to the weaker
			// test_reaches contract.
			ce, ok := ev.(CertificationEvidence)
			if !ok {
				fail("certified_by", "no certification evidence available", "certificate-absent")
				break
			}
			byTest := ce.CertificationsByTest(e.CertifiedBy)
			crit, label := EvaluateCertifiedBy(matches[0].FilePath, e.CertifiedBy, e.CertifyStrength, byTest)
			if crit.Status == Pass {
				pass(crit.Name, crit.Detail)
			} else {
				fail(crit.Name, crit.Detail, label)
			}
		default:
			if de, ok := ev.(DetailedEvidence); ok {
				if details := de.TestReachesDetail(querySym); len(details) > 0 {
					pass("test_reaches", testReachesDetail(details))
					for _, d := range details {
						reaching = append(reaching, d.Key)
					}
				} else {
					graphFail("test_reaches", "no Test* statically reaches it", "present-but-untested")
				}
			} else if tests := ev.TestReaches(querySym); len(tests) > 0 {
				pass("test_reaches", fmt.Sprintf("%d test(s) statically reach it, e.g. %s", len(tests), tests[0]))
				reaching = tests
			} else {
				graphFail("test_reaches", "no Test* statically reaches it", "present-but-untested")
			}
		}

		// Foreign prose may only tighten a contract: keep the ordinary
		// test_reaches/skip_test verdict above AND require a live certificate
		// from a derived name. Explicit certified_by remains unchanged.
		if len(t.DerivedTests) > 0 && len(e.CertifiedBy) == 0 {
			switch {
			case ambiguous:
				na("derived_tests", "derived test certificates: ambiguous anchor")
			case len(matches) == 0:
				na("derived_tests", "derived test certificates: symbol absent")
			default:
				crit := Criterion{Name: "derived_tests", Status: Fail, Detail: "no certification evidence available"}
				label := "certificate-absent"
				if ce, ok := ev.(CertificationEvidence); ok {
					crit, label = EvaluateCertifiedBy(matches[0].FilePath, t.DerivedTests, e.CertifyStrength, ce.CertificationsByTest(t.DerivedTests))
				}
				detail := "derived test certificates: " + crit.Detail
				if crit.Status == Pass {
					pass("derived_tests", detail)
				} else {
					fail("derived_tests", detail, label)
				}
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
	if e.ReachableFrom != "" && e.Symbol != "" && ambiguous {
		checked = true
		na("edge_wired", "ambiguous anchor")
	} else if e.ReachableFrom != "" && e.Symbol != "" {
		checked = true
		if ev.Reachable(e.ReachableFrom, querySym) {
			pass("edge_wired", fmt.Sprintf("%s reaches %s", e.ReachableFrom, querySym))
		} else {
			detail := fmt.Sprintf("%s does not reach %s", e.ReachableFrom, querySym)
			unplaced := false
			if re, ok := ev.(ReachNameEvidence); ok {
				if key, cands := re.ResolveReachName(e.ReachableFrom); key == "" {
					detail, unplaced = unplacedReachFrom(e.ReachableFrom, cands), true
				}
			}
			if he, ok := ev.(HeuristicReachEvidence); ok && !unplaced {
				if edge, ok := he.HeuristicReach(e.ReachableFrom, querySym); ok {
					detail = fmt.Sprintf("%s reaches %s only through %s: verify passes on no heuristic edge (RF-5, I57)", e.ReachableFrom, querySym, edge)
				}
			}
			graphFail("edge_wired", detail, "edge-not-wired")
		}
	}

	// --- product wiring (ADR-018 amendment 2026-09-28) ---
	// Proven AND wired: a present Go symbol that a test reaches but no
	// product entry point does is library-only — Unknown, so the default
	// gate reports it and --strict (the pre-push gate) refuses it. It never
	// overrides an earlier failure's label.
	if pe, ok := ev.(ProductEvidence); ok && len(matches) > 0 && !ambiguous {
		c, label := evalWired(t, pe, querySym, matches)
		if cause := GraphFallbackCause(ev); cause != "" && c.Status == Unknown && !t.Expect.Library {
			c.Detail, label = graphFallbackDetail(c.Detail, cause), GraphFallbackLabel
		}
		v.Criteria = append(v.Criteria, c)
		if c.Status == Unknown && v.Status == Pass {
			v.Status = Unknown
			v.Label = label
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

	// --- runtime: read from attested runs, never executed (TC-29) ---
	// Informational (I8 and ADR-023 as amended by ADR-067): appended
	// without touching v.Status or v.Label, whatever the run says.
	tests, from := runtimeTests(e, reaching)
	v.Criteria = append(v.Criteria, evalRuntimePass(tests, from, ev))

	// --- self-declared (I45, ADR-044 §1.4) ---
	// Informational only — never sets `checked` and never on its own
	// changes v.Status. AnnotateSelfDeclared is the only source of
	// t.SelfDeclared (ParseMarkdownPlan never sets it); a caller with no
	// base plan to compare against (the ordinary `fathom verify`, no
	// --since) leaves it nil and this is a no-op. The one case that DOES
	// change Status — a self-declared skip — was already handled above by
	// the ordinary skip_test switch reading t.SelfSkipped directly; this
	// is the HTTP-mode daemon's own equivalent overlay
	// (OverlaySelfDeclared) for the report this function's caller (store
	// mode, or OverlayLocal's lexical re-evaluation) computes natively.
	if len(t.SelfDeclared) > 0 {
		v.Criteria = append(v.Criteria, Criterion{
			Name:   "self_declared",
			Status: NA,
			Detail: fmt.Sprintf("self-declared: %s", strings.Join(t.SelfDeclared, ", ")),
		})
	}

	if !checked {
		// No acceptance contract to evaluate — don't pretend it's verified.
		v.Status = Unknown
		v.Label = "no-contract"
	}
	return v
}

// evalWired is the `wired` criterion (ADR-018 amendment 2026-09-28): does a
// product entry point statically reach the row's symbol over typed edges
// (RF-5, I57: a reach resting on a heuristic edge is refused, the edge
// named when the evidence can name it)? matches are the
// row's resolved nodes. Wiring applies only to Go product functions and
// methods in a service that has a product: a type (call edges never reach
// one), a non-Go symbol, a test file or testdata fixture, or a service
// with no entry point at all reads n/a, named. An
// unreached symbol is library-only (Unknown) unless the row declares
// `library: true` with a `reason:`, and that declaration predates the
// claim — then the waiver is the criterion (`library`, Pass).
func evalWired(t Task, pe ProductEvidence, querySym string, matches []core.SymbolNode) (Criterion, string) {
	const name = "wired"
	// The functions and methods among the matches: a name like
	// CachedCallGraph.Graph also finds the Graph type, which says nothing
	// about whether the method is called.
	var callable []core.SymbolNode
	for _, m := range matches {
		if m.Kind == core.KindGoFunc || m.Kind == core.KindGoMethod {
			callable = append(callable, m)
		}
	}
	sym := matches[0]
	if len(callable) > 0 {
		sym = callable[0]
	}
	file := sym.FilePath
	switch {
	case !strings.HasSuffix(file, ".go"):
		return Criterion{Name: name, Status: NA, Detail: "wiring is computed for Go symbols only"}, ""
	case len(callable) == 0:
		return Criterion{Name: name, Status: NA, Detail: fmt.Sprintf("a %s, not a function: call edges never reach it, so wiring is computed for functions and methods only", sym.Kind)}, ""
	case strings.HasSuffix(file, "_test.go") || strings.HasPrefix(file, "testdata/") || strings.Contains(file, "/testdata/"):
		return Criterion{Name: name, Status: NA, Detail: fmt.Sprintf("test code (%s): wiring does not apply", file)}, ""
	case pe.ProductEntryCount() == 0:
		return Criterion{Name: name, Status: NA, Detail: "the service has no product entry point (no Go package main): a library, wiring does not apply"}, ""
	}
	if entry, ok := pe.ProductReachedBy(querySym); ok {
		return Criterion{Name: name, Status: Pass, Detail: fmt.Sprintf("product entry point %s reaches it", entry)}, ""
	}
	// A row whose symbol (under its package: filter) names more than one
	// function — the same method on two implementations of one interface —
	// is resolved by the call graph to nothing, exactly as test_reaches
	// is. Wiring is judged the way symbol_present is: the row is wired when
	// any function it names is, and the verdict names which. So is a row
	// whose name also finds a type (`symbol: Viz` finding the type Viz and
	// the method Corpus.Viz): the bare name resolves to the type, whose only
	// "reach" is a reference to it — an edge of tier heuristic (RF-5) —
	// so the function the row names is what is judged.
	keys := distinctKeys(callable)
	sort.Strings(keys)
	if len(keys) > 1 || len(keys) == 1 && keys[0] != querySym {
		for _, k := range keys {
			if entry, ok := pe.ProductReachedBy(k); ok {
				if len(keys) == 1 {
					return Criterion{Name: name, Status: Pass, Detail: fmt.Sprintf("product entry point %s reaches %s", entry, k)}, ""
				}
				return Criterion{Name: name, Status: Pass, Detail: fmt.Sprintf("product entry point %s reaches %s (one of %d functions this row names)", entry, k, len(keys))}, ""
			}
		}
	}
	unreached := "no product entry point reaches it (no main, and no init of a package a main links)"
	if he, ok := pe.(HeuristicReachEvidence); ok {
		// The functions the row names first: a same-named type's reach is a
		// reference, never the reason the row is refused.
		for _, k := range append(append([]string(nil), keys...), querySym) {
			if entry, edge, ok := he.ProductHeuristicReach(k); ok {
				unreached = fmt.Sprintf("product entry point %s reaches %s only through %s: verify passes on no heuristic edge (RF-5, I57)", entry, k, edge)
				break
			}
		}
	}
	switch {
	case t.Expect.Library && t.Reason != "" && !t.SelfLibrary:
		return Criterion{Name: "library", Status: Pass, Detail: fmt.Sprintf("%s; waived by library: true — %s", unreached, t.Reason)}, ""
	case t.Expect.Library && t.SelfLibrary:
		return Criterion{Name: name, Status: Unknown, Detail: unreached + "; library: true was added in the same edit or commit that claims this row done — an agent may not grade its own row, so it waives nothing"}, "library-only"
	case t.Expect.Library:
		return Criterion{Name: name, Status: Unknown, Detail: unreached + "; library: true with no reason: waives nothing — name why the API itself is the deliverable"}, "library-only"
	}
	return Criterion{Name: name, Status: Unknown, Detail: unreached + " (library-only): call it from the product, or declare library: true with a reason: when the API itself is the deliverable"}, "library-only"
}

// testReachesDetail renders test_reaches's pass detail from a DetailedEvidence
// result, surfacing the dispatch modality distinctly when the reaching path
// used one (ADR-021 §3) — a reader sees "reached via interface dispatch" at
// a glance instead of it reading identically to a direct static reach.
// details is assumed non-empty and sorted by Key (callgraph.TestImpactDetail's
// contract); the first dispatch-reaching entry is reported when one exists,
// otherwise the first entry overall.
// codeMatches drops DocChunk nodes from a `symbol:` lookup whenever at least
// one code node matched: a plan row's symbol contract names code, and a
// Markdown heading that happens to share the bare name (a reading titled
// "Gate" beside verify.Gate) is never its target. A doc-only match set is
// returned unchanged so a row that truly names nothing in code still fails
// as absent rather than silently changing meaning.
func codeMatches(nodes []core.SymbolNode) []core.SymbolNode {
	var code []core.SymbolNode
	for _, n := range nodes {
		if n.Kind != core.KindDocChunk {
			code = append(code, n)
		}
	}
	if len(code) == 0 {
		return nodes
	}
	return code
}

func testReachesDetail(details []ReachDetail) string {
	for _, d := range details {
		if d.ViaDispatch {
			return fmt.Sprintf("%d test(s) statically reach it, e.g. %s (reached via interface dispatch, %d implementer(s))",
				len(details), d.Key, d.Implementers)
		}
	}
	return fmt.Sprintf("%d test(s) statically reach it, e.g. %s", len(details), details[0].Key)
}

// underDir returns the nodes whose defining directory IS the repo-relative
// directory dir — a `package:` filter names one package, and a Go package
// is one directory. Index paths use forward slashes; the comparison is a
// pure string match on path.Dir(FilePath), preserving VerifyPlan's purity.
// It was a prefix match (subdirectories included) until PC-32: `package:
// internal/contextbench` then also matched internal/contextbench/publicloc,
// so a row naming the parent stayed ambiguous between two packages (the PB-3
// collision), and a filter could never single out a package that has
// sub-packages. The directory-qualified key (core.QualifiedPackage) is the
// same boundary: the filter resolves against the key's directory component.
func underDir(nodes []core.SymbolNode, dir string) []core.SymbolNode {
	dir = strings.TrimSuffix(dir, "/")
	var out []core.SymbolNode
	for _, n := range nodes {
		if path.Dir(n.FilePath) == dir {
			out = append(out, n)
		}
	}
	return out
}

// bareAnchorUnproven reports whether a bare anchor that several definitions
// match gets no reach proof from the call graph by its bare name (O1453):
// the row's test_reaches, or its edge_wired, would fail today. A bare name
// the graph does resolve (a row passing today) is left as it is, and so is
// a row whose contract is not the ordinary test_reaches check (skip_test,
// certified_by, a plan-level certified header).
func bareAnchorUnproven(e Expect, ev Evidence, proofCertified bool) bool {
	if e.SkipTest || len(e.CertifiedBy) > 0 || proofCertified {
		return false
	}
	reached := false
	if de, ok := ev.(DetailedEvidence); ok {
		reached = len(de.TestReachesDetail(e.Symbol)) > 0
	} else {
		reached = len(ev.TestReaches(e.Symbol)) > 0
	}
	if !reached {
		return true
	}
	return e.ReachableFrom != "" && !ev.Reachable(e.ReachableFrom, e.Symbol)
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
