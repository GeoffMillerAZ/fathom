// Package review composes fathom's existing analyzers into ONE cited verdict
// for a set of changed files — the review-verb tracer bullet (C1-1,
// docs/plans/review-verb.md; vision 11 §3.C): diff impact (blast radius,
// typed-edge aware) + obligations/findings scoped to the changed files +
// architecture conformance (project rule pack) + spec coverage, instead of
// the 8-step manual sequence docs/use-cases/change-review.md previously
// documented tool-by-tool.
//
// Run is a pure function: every dimension operates over data the caller
// already resolved (indexed symbols, typed edges, obligations already
// looked up per file, arch rules already resolved via
// internal/projectrules) — this package does no I/O and calls no clock
// (repo convention: "inject clocks, never call time.Now() in query/
// analysis logic"). The MCP tool (internal/mcp) and the CLI mirror
// (cmd/fathom) do the I/O and hand Run a fully-resolved Input.
//
// Honesty rule (C4-5's audit, binding on this row too): a dimension that
// cannot run — no rules configured, no claims compiled, the backend does
// not support it — reports Checked:false plus a Reason, NEVER a
// clean-shaped empty pass. See each run* function's doc comment for its own
// checked/unchecked split.
package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// Dimension names — the fixed, deterministic order Verdict.Dimensions is
// always built in.
const (
	DimImpact       = "impact"
	DimObligations  = "obligations"
	DimArch         = "arch_conformance"
	DimSpecCoverage = "spec_coverage"
	// DimCertifications (vision 33 / ADR-043 stage 1, TC-10) is exported
	// like DimTestCoverage's own dimCoverageName is NOT, because there is
	// no separate constructor-function identifier competing for this
	// name — runCertifications is unexported like every other run*
	// dimension function.
	DimCertifications = "certifications"
)

// Finding is one cited item within a Dimension. Every finding that survives
// into a Verdict carries a citation and a severity — "one cited verdict
// structure" per dimension (C1-1).
type Finding struct {
	// Kind is dimension-specific: "impact", "obligation" (or a served
	// claim's own Kind, e.g. "finding"), "arch_violation", "spec_uncovered".
	Kind string
	// Symbol is the affected symbol's canonical key, when applicable.
	Symbol string
	// File is the repo-relative path the finding is about, when known.
	File string
	// Line is the 1-based line the finding is anchored to; 0 when unknown
	// (Citation then carries a bare file path with no line range).
	Line int
	// Message is the human-readable finding text.
	Message string
	// Citation is the evidence citation: core.CiteRef's "path#Lstart-Lend"
	// form when Line is known, a bare repo-relative path otherwise, or ""
	// when neither is available.
	Citation string
	// Severity is "info" | "warn" | "error".
	Severity string

	// ClaimID and Status are set by AttachClaims / ApplyClaimStatus (C2-1,
	// the findings bridge) — both are zero-value until that pass runs.
	// Status is "" for a normally-served finding, or a non-served
	// core.ClaimStatus string ("refuted", "superseded") when a prior
	// review's persisted verdict on this exact finding overrides it.
	ClaimID string
	Status  string

	// Class (docs/design/findings-as-evidence.md §3.2, §6 item 2) is the
	// closed-vocabulary failure class this finding belongs to — assigned
	// DETERMINISTICALLY by the dimension that produced it (runArch always
	// emits FindingClassConformance; runSpecCoverage always emits
	// FindingClassTestGap), never by an LLM in this bridge. "" for
	// dimensions that never persist (impact, obligations — see
	// persistableDim in claims.go), since Class only matters for a
	// persisted claim.
	Class core.FindingClass

	// Test/Strength/State are certifications-dimension-only detail
	// (vision 33 / ADR-043 stage 1, TC-10): the certifying test's bare
	// name, its declared strength, and the certifications layer's own
	// state fact (live | flaky | broken | unproven | unreachable) — set
	// only on a Kind=="certification" finding, "" for every other
	// dimension's findings.
	Test     string
	Strength string
	State    string
}

// Dimension is one axis of the review.
type Dimension struct {
	Name    string
	Checked bool
	// Reason explains why Checked is false; always empty when Checked.
	Reason   string
	Findings []Finding
	// Count is the number of ACTIVE findings — Findings minus any labeled
	// with a non-served Status (ApplyClaimStatus). Equal to len(Findings)
	// until the claims bridge runs.
	Count int

	// Tests and Files are impact-dimension-only detail: the full affected
	// test/file sets impact.Result computes (change-review workflows need
	// "which tests to run" and "which files are touched" directly, not
	// reconstructed from Findings). Empty for every other dimension.
	Tests []string
	Files []string

	// Coverage is coverage-dimension-only detail (RC-41): one entry per
	// changed symbol DimTestCoverage found profile data for — {symbol,
	// layers, percent, uncovered}. Empty for every other dimension.
	Coverage []CoverageDetail
}

// Verdict is the ONE cited verdict review_diff returns.
type Verdict struct {
	Service      string
	ChangedFiles []string
	Dimensions   []Dimension
	Summary      string
}

// ObligationRule is one governing rule already resolved for a changed file
// — the flattened shape of semantic.ObligationsFor's output. review is a
// leaf analysis package and does not import internal/semantic (which must
// stay free of any internal/analysis dependency); the caller maps
// semantic.Obligation → ObligationRule.
type ObligationRule struct {
	File string
	Rule string
	// Src is the citation "path#Lstart-Lend", when the source claim has one.
	Src string
	// Tier is the claim's evidence tier (core.EvidenceTier string value).
	Tier string
	// Kind labels a claim served from a non-obligation layer ("finding",
	// …); "" for the obligations layer itself (mirrors semantic.Obligation.Kind).
	Kind string
	// Status is "" (fresh) | "stale" | "contradicted" (mirrors semantic.Obligation.Status).
	Status string
}

// Input bundles every already-resolved dependency Run needs. Every I/O
// (ListAllSymbols, TypedEdges, ObligationsFor per file, ServiceMeta +
// internal/projectrules.ResolveRules, Sources) happens in the caller.
type Input struct {
	Service      string
	ChangedFiles []string
	// Concern is an optional caller-supplied token; currently descriptive
	// only (carried for forward compatibility — no dimension filters on it
	// yet).
	Concern string

	// Base is the resolved base commit sha a --since/since caller compared
	// ChangedFiles against, and Root is the service's indexed corpus root
	// (ADR-044 §1.6, CV-13) — both filled by cmd/fathom's --since glue and
	// internal/mcp's reviewDiffWithBase. "" (Base) means no base ref was
	// given: DimIntegrity (a later row) then reports checked:false, reason
	// "needs a base ref", exactly as review_diff already behaves today. Run
	// itself does not read either field yet; they exist so a base-aware
	// dimension can be added without another Input-shape change.
	Base string
	Root string

	// impact
	Nodes      []core.SymbolNode
	TypedEdges []core.Edge

	// obligations — served claims scoped to the changed files.
	ObligationsSupported bool
	ObligationsByFile    map[string][]ObligationRule

	// arch_conformance — rules already resolved by the caller via
	// internal/projectrules.ResolveRules (inline | project | none).
	ArchRules         []archconformance.Rule
	ArchRuleSource    string
	ArchDenyByDefault bool
	ArchChecked       bool
	ArchReason        string

	// spec_coverage
	ChangedSources []core.SourceFile // source content of ONLY the changed files

	// coverage (RC-41) — caller-parsed, per-layer Go cover profiles,
	// already joined to symbols via
	// internal/analysis/coverage.JoinLayeredSymbols (ResolveCoverage does
	// the join). CoverageChecked is false whenever the caller resolved no
	// profile at all — DimTestCoverage reports checked:false, reason "no
	// coverage profile supplied" in that case, never a clean-shaped empty
	// pass. "By layer" is a caller-declared label on each profile
	// (coverage.LabeledProfile), never guessed from per-test data.
	CoverageChecked bool
	CoverageLayered []coverage.SymbolLayerCoverage

	// certifications (vision 33 / ADR-043 stage 1, TC-10). Supported is
	// the backend-capability gate (a live claims-serving backend, same
	// meaning as ObligationsSupported); Declared is false when the
	// certifications LAYER ITSELF is empty for this service (nothing
	// compiled anywhere, not merely nothing touching the diff) — design
	// doc §4's "checked: false, reason: 'no certifications declared' when
	// the layer is empty". ByFile maps a changed file to every compiled
	// certification claim (internal/certify.Records, flattened by the
	// caller — review does not import internal/certify, the same reason
	// ObligationRule mirrors semantic.Obligation instead of importing
	// internal/semantic) whose own Scope includes that file.
	CertificationsSupported bool
	CertificationsDeclared  bool
	CertificationsByFile    map[string][]CertificationRecord
}

// CertificationRecord is one compiled certifications-layer claim, as the
// certifications dimension needs it (vision 33 / ADR-043 stage 1, TC-10).
// Callers build it from internal/certify.Record (certify.Records over
// ClaimsForService(service, certify.Layer)) — review stays a leaf
// internal/analysis package with no internal/certify dependency, mirrored
// exactly like ObligationRule flattens semantic.Obligation.
type CertificationRecord struct {
	// Subject is the target's "<kind>:<id>" identity (design doc §3).
	Subject  string
	Test     string
	Strength string
	// State is the certifications layer's own state fact: live | flaky |
	// broken | unproven | unreachable.
	State string
	Tier  string
	Rule  string
	// Status mirrors core.ClaimStatus ("" == fresh) — a stale/contradicted
	// certification is served WITH its status label, never silently
	// dropped (the same served-with-label discipline runObligations
	// applies).
	Status string
	// Scope is the claim's own file scope (the test file plus the files
	// of the symbols the test reaches that the target names) — what the
	// caller used to place this record into CertificationsByFile in the
	// first place; carried through so a Finding can cite it.
	Scope []string
}

// Run composes every dimension into one Verdict. Deterministic: identical
// Input always produces an identical Verdict (every slice is sorted).
func Run(in Input) Verdict {
	changed := append([]string(nil), in.ChangedFiles...)
	sort.Strings(changed)

	dims := []Dimension{
		runImpact(in),
		runObligations(in),
		runArch(in),
		runSpecCoverage(in),
		DimTestCoverage(in),
		runCertifications(in),
	}

	v := Verdict{
		Service:      in.Service,
		ChangedFiles: changed,
		Dimensions:   dims,
	}
	v.Summary = summarize(v)
	return v
}

func summarize(v Verdict) string {
	checked := 0
	var unchecked []string
	active := 0
	for _, d := range v.Dimensions {
		if d.Checked {
			checked++
			active += d.Count
		} else {
			unchecked = append(unchecked, d.Name)
		}
	}
	s := fmt.Sprintf("service %s: %d changed file(s); %d/%d dimensions checked; %d active finding(s)",
		v.Service, len(v.ChangedFiles), checked, len(v.Dimensions), active)
	if len(unchecked) > 0 {
		s += fmt.Sprintf("; unchecked: %s", strings.Join(unchecked, ", "))
	}
	return s
}

// ─── impact ───────────────────────────────────────────────────────────────

// runImpact reports the blast radius of the changed files: every symbol
// (beyond the changed files themselves) that transitively depends on them,
// plus the affected test set. Checked is false only when the service has no
// indexed symbols at all (never ingested) — a real empty result over an
// indexed, unaffected service is an honest clean pass, not a config gap.
func runImpact(in Input) Dimension {
	d := Dimension{Name: DimImpact}
	if len(in.Nodes) == 0 {
		d.Reason = "no symbols indexed for this service — ingest it before reviewing"
		return d
	}
	d.Checked = true

	result := impact.DiffImpactWithTypedEdges(in.Nodes, in.ChangedFiles, in.TypedEdges)
	d.Tests = append([]string(nil), result.Tests...)
	d.Files = append([]string(nil), result.Files...)

	changedSet := toSet(in.ChangedFiles)
	fileOf := make(map[string]string, len(in.Nodes))
	lineOf := make(map[string]int, len(in.Nodes))
	for _, n := range in.Nodes {
		fileOf[n.Key()] = n.FilePath
		lineOf[n.Key()] = n.Line
	}

	for _, sym := range result.Symbols {
		f := fileOf[sym]
		if changedSet[f] {
			continue // report downstream impact only, not the changed files themselves
		}
		line := lineOf[sym]
		d.Findings = append(d.Findings, Finding{
			Kind:     DimImpact,
			Symbol:   sym,
			File:     f,
			Line:     line,
			Message:  "transitively depends on changed code",
			Citation: citeOf(f, line),
			Severity: "info",
		})
	}
	sort.Slice(d.Findings, func(i, j int) bool { return d.Findings[i].Symbol < d.Findings[j].Symbol })
	d.Count = len(d.Findings)
	return d
}

// ─── obligations ──────────────────────────────────────────────────────────

// runObligations reports every governing claim (obligations layer, and any
// other compiled layer served through the same channel — findings included)
// scoped to the changed files. Checked is false only when the backend does
// not serve semantic layers at all (a transport-level capability gap); an
// empty result from a capable backend is a real "no obligations govern this
// diff" clean state, matching the shipped obligations MCP tool's own
// contract (zero-output-on-clean).
func runObligations(in Input) Dimension {
	d := Dimension{Name: DimObligations}
	if !in.ObligationsSupported {
		d.Reason = "backend does not serve semantic layers"
		return d
	}
	d.Checked = true

	for _, f := range in.ChangedFiles {
		for _, r := range in.ObligationsByFile[f] {
			sev := "info"
			if r.Status == "stale" || r.Status == "contradicted" {
				sev = "warn"
			}
			kind := "obligation"
			if r.Kind != "" {
				kind = r.Kind
			}
			d.Findings = append(d.Findings, Finding{
				Kind:     kind,
				File:     f,
				Message:  r.Rule,
				Citation: r.Src,
				Severity: sev,
				// Served-with-label discipline (internal/semantic/serve.go):
				// a stale/contradicted served claim carries its status label
				// through to the verdict, never mapped away to severity only.
				Status: r.Status,
			})
		}
	}
	sort.Slice(d.Findings, func(i, j int) bool {
		if d.Findings[i].File != d.Findings[j].File {
			return d.Findings[i].File < d.Findings[j].File
		}
		return d.Findings[i].Message < d.Findings[j].Message
	})
	d.Count = len(d.Findings)
	return d
}

// ─── certifications ───────────────────────────────────────────────────────

// runCertifications reports the certifications layer's own compiled facts
// scoped to the diff (vision 33 / ADR-043 stage 1, TC-10, design doc §4):
// per changed file, the certificates (test, strength, state) whose Scope
// includes it, plus one finding per target (subject) touching the diff
// that has NO live certificate among those records. Checked is false —
// with a reason — when the backend doesn't serve semantic layers at all
// (mirrors runObligations), or when the certifications layer is
// genuinely empty for this service (mirrors DimTestCoverage's own "no
// coverage profile supplied": a capability the corpus never populated
// reports checked:false, never a clean-shaped empty pass).
func runCertifications(in Input) Dimension {
	d := Dimension{Name: DimCertifications}
	if !in.CertificationsSupported {
		d.Reason = "backend does not serve semantic layers"
		return d
	}
	if !in.CertificationsDeclared {
		d.Reason = "no certifications declared"
		return d
	}
	d.Checked = true

	liveSubjects := map[string]bool{}
	seenSubjects := map[string]string{} // subject -> first Rule text seen, for the gap finding's message

	for _, f := range in.ChangedFiles {
		for _, r := range in.CertificationsByFile[f] {
			sev := "info"
			if r.State != "live" {
				sev = "warn"
			}
			d.Findings = append(d.Findings, Finding{
				Kind: "certification", File: f, Message: r.Rule,
				Citation: citeOf(f, 0), Severity: sev,
				Test: r.Test, Strength: r.Strength, State: r.State,
				Status: r.Status,
			})
			seenSubjects[r.Subject] = r.Rule
			if r.State == "live" {
				liveSubjects[r.Subject] = true
			}
		}
	}

	var gapSubjects []string
	for subject := range seenSubjects {
		if !liveSubjects[subject] {
			gapSubjects = append(gapSubjects, subject)
		}
	}
	sort.Strings(gapSubjects)
	for _, subject := range gapSubjects {
		d.Findings = append(d.Findings, Finding{
			Kind:     "certification_gap",
			Message:  fmt.Sprintf("%s has no live certificate", subject),
			Severity: "warn",
			Class:    core.FindingClassTestGap,
		})
	}

	sort.Slice(d.Findings, func(i, j int) bool {
		if d.Findings[i].File != d.Findings[j].File {
			return d.Findings[i].File < d.Findings[j].File
		}
		if d.Findings[i].Kind != d.Findings[j].Kind {
			return d.Findings[i].Kind < d.Findings[j].Kind
		}
		return d.Findings[i].Message < d.Findings[j].Message
	})
	d.Count = len(d.Findings)
	return d
}

// ─── arch_conformance ─────────────────────────────────────────────────────

// runArch reports every architecture-layering violation across the
// service's current call edges (whole-service scope: a layering rule is a
// service-wide invariant, and internal/analysis/archconformance.Violation
// does not carry a FromFile a caller could use to narrow it precisely to
// the diff — a disclosed scope choice, not a silent one). Checked and
// Reason are pass-through from the caller's project-rules resolution
// (internal/projectrules.ResolveRules): checked is false exactly when
// neither inline nor project rules were available (C4-2/C4-3's honesty
// contract, reused verbatim here).
func runArch(in Input) Dimension {
	d := Dimension{Name: DimArch, Checked: in.ArchChecked, Reason: in.ArchReason}
	if !in.ArchChecked {
		return d
	}

	edges := archconformance.EdgesFromNodes(in.Nodes)
	fileByFrom := make(map[string]string, len(edges))
	for _, e := range edges {
		if _, ok := fileByFrom[e.FromSymbol]; !ok {
			fileByFrom[e.FromSymbol] = e.FromFile
		}
	}
	// Normalized-key → node index: recovers the violating symbol's real
	// file AND line so the finding's citation is a parseable, hashable
	// "path#Lstart-Lend" — a bare-path citation must never reach a persisted
	// claim (it would read as anchor-dead to semantic.Sweep's ParseCiteRef
	// and flip the claim contradicted on the very next sweep).
	keyIdx := archconformance.KeyIndex(in.Nodes)

	var opts []archconformance.Option
	if in.ArchDenyByDefault {
		opts = append(opts, archconformance.DenyByDefault())
	}
	violations := archconformance.Check(edges, in.ArchRules, opts...)
	for _, v := range violations {
		f := fileByFrom[v.From]
		line := 0
		if n, ok := keyIdx[v.From]; ok {
			f = n.FilePath
			line = n.Line
		}
		d.Findings = append(d.Findings, Finding{
			Kind:     "arch_violation",
			Symbol:   v.From,
			File:     f,
			Line:     line,
			Message:  fmt.Sprintf("%s → %s: %s", v.From, v.To, v.Reason),
			Citation: citeOf(f, line),
			Severity: "error",
			// A layering rule broken is exactly core.FindingClassConformance's
			// own definition (docs/design/findings-as-evidence.md §3.2).
			Class: core.FindingClassConformance,
		})
	}
	// archconformance.Check already sorts by (From, To); Findings preserves
	// that order (stable, deterministic).
	d.Count = len(d.Findings)
	return d
}

// ─── spec_coverage ────────────────────────────────────────────────────────

// runSpecCoverage extracts spec-ID references touched by the changed files
// (from their source content and from indexed symbols defined in them), and
// reports any that have no covering symbol ANYWHERE in the service (codeRefs
// is built from the full node set, so coverage is not artificially narrowed
// to the diff). Checked is always true: there is no operator-omittable
// config here (sdd.ExtractRefs' default pattern set is a fixed, always-on
// scan, the same reasoning C4-5's audit applied to the existing spec_refs/
// traceability tools) — zero spec IDs referenced in the diff is an honest
// "nothing to check", not a capability gap.
func runSpecCoverage(in Input) Dimension {
	d := Dimension{Name: DimSpecCoverage, Checked: true}

	changedSet := toSet(in.ChangedFiles)

	// specIDs referenced BY the diff, with the first file/line each was
	// found at (for the finding's citation).
	type loc struct {
		file string
		line int
	}
	locOf := map[string]loc{}
	addRefs := func(refs []string, file string, line int) {
		for _, id := range refs {
			if _, ok := locOf[id]; !ok {
				locOf[id] = loc{file: file, line: line}
			}
		}
	}
	for _, sf := range in.ChangedSources {
		if !changedSet[sf.Path] {
			continue
		}
		content := string(sf.Content)
		for _, id := range sdd.ExtractRefs(content) {
			// Resolve the ref back to its first occurrence line so the
			// finding's citation is parseable/hashable (line 0 would mean a
			// bare-path citation, which AttachClaims refuses to persist a
			// cites link for). A normalized ref whose raw text can't be found
			// (e.g. "doc N" forms) stays line 0 — honest, and safe: no cites
			// link is written for it.
			addRefs([]string{id}, sf.Path, lineOfRef(content, id))
		}
	}
	for _, n := range in.Nodes {
		if !changedSet[n.FilePath] {
			continue
		}
		addRefs(sdd.ExtractRefs(n.Signature+" "+n.Name), n.FilePath, n.Line)
	}

	specIDs := make([]string, 0, len(locOf))
	for id := range locOf {
		specIDs = append(specIDs, id)
	}
	sort.Strings(specIDs)
	if len(specIDs) == 0 {
		return d
	}

	// codeRefs: whole-service, so coverage isn't narrowed to the diff.
	codeRefs := make(map[string][]string, len(in.Nodes))
	for _, n := range in.Nodes {
		if refs := sdd.ExtractRefs(n.Signature + " " + n.Name); len(refs) > 0 {
			codeRefs[n.Key()] = refs
		}
	}

	matrix := sdd.BuildMatrix(specIDs, codeRefs)
	for _, id := range matrix.Uncovered(specIDs) {
		l := locOf[id]
		d.Findings = append(d.Findings, Finding{
			Kind:     "spec_uncovered",
			File:     l.file,
			Line:     l.line,
			Message:  fmt.Sprintf("spec %s is referenced in this diff but has no covering symbol", id),
			Citation: citeOf(l.file, l.line),
			Severity: "warn",
			// A referenced spec with no covering symbol is a verification
			// gap — core.FindingClassTestGap (docs/design/findings-as-
			// evidence.md §3.2).
			Class: core.FindingClassTestGap,
		})
	}
	sort.Slice(d.Findings, func(i, j int) bool { return d.Findings[i].Message < d.Findings[j].Message })
	d.Count = len(d.Findings)
	return d
}

// ─── coverage ─────────────────────────────────────────────────────────────

// dimCoverageName is the "coverage" dimension's Name — a plain constant
// (not exported as DimCoverage alongside DimImpact/DimObligations/DimArch/
// DimSpecCoverage) because the exported identifier DimTestCoverage names
// the dimension's constructor FUNCTION instead (this row's own anchor),
// and the two cannot share a name.
const dimCoverageName = "coverage"

// CoverageDetail is coverage-dimension-only detail: one changed symbol's
// joined, per-layer test coverage, as DimTestCoverage renders it into
// Dimension.Coverage.
type CoverageDetail struct {
	Symbol string
	File   string
	Line   int
	// Layers is the sorted set of test layers that ACTUALLY COVER this
	// symbol (Covered>0 in that layer's own profile) — an assertion
	// grounded in which caller-labeled profile recorded the coverage
	// (coverage.LabeledProfile), never guessed from the symbol's own name
	// or file.
	Layers []string
	// UncoveredAt is the sorted set of layers that DID have profile data
	// for this symbol but recorded zero executions — distinct from a layer
	// with no data at all, which is simply absent from both Layers and
	// UncoveredAt.
	UncoveredAt []string
	// Percent/Uncovered are the UNION across every supplied layer
	// (coverage.SymbolLayerCoverage.Any): a statement counts as covered if
	// ANY layer covered it.
	Percent   float64
	Uncovered bool
}

func (c CoverageDetail) payload() map[string]any {
	return map[string]any{
		"symbol":       c.Symbol,
		"file":         c.File,
		"line":         c.Line,
		"layers":       nonNilStrings(c.Layers),
		"uncovered_at": nonNilStrings(c.UncoveredAt),
		"percent":      c.Percent,
		"uncovered":    c.Uncovered,
	}
}

// DimTestCoverage computes the "coverage" dimension (RC-41, extended for
// true per-layer attribution): per-CHANGED-symbol test coverage from
// caller-parsed, per-layer Go cover profiles (internal/analysis/coverage.
// ParseProfile + JoinLayeredSymbols, composed by ResolveCoverage), scoped
// to the diff exactly like every other dimension. Checked is false — with
// Reason "no coverage profile supplied" — whenever the caller resolved no
// profile at all (in.CoverageChecked): the SAME honesty discipline
// runObligations/runArch already apply — a capability the caller did not
// configure reports checked:false, never a clean-shaped empty pass. A
// changed symbol no supplied profile has data for (not
// statement-instrumented, or its package was outside every covered set) is
// silently omitted from Coverage, not reported as a fabricated 0%.
//
// "By layer" is a caller-declared LABEL on each profile
// (coverage.LabeledProfile — CLI: `--profile <layer>=<path>`, repeatable;
// MCP/review: `coverage_profiles: [{layer, path}]`), never guessed from
// per-test data: a standard `go test -coverprofile` file carries none. A
// symbol's Layers/UncoveredAt therefore report exactly which LABELED
// PROFILE recorded the coverage — real attribution when the caller ran one
// profile per layer, not an inference from the profile's own contents.
//
// The boundary this dimension keeps: fathom ASSERTS which changed symbols
// are statement-covered, by what percent, and by which layer(s) — it never
// judges whether that coverage is adequate, which tests SHOULD exist, or
// blocks a review on a coverage threshold. That judgment belongs to the
// review kit consuming this block, not to fathom.
//
// Exported (unlike the other run* dimension functions, runImpact/
// runObligations/runArch/runSpecCoverage) per the plan row's own anchor
// name.
func DimTestCoverage(in Input) Dimension {
	d := Dimension{Name: dimCoverageName}
	if !in.CoverageChecked {
		d.Reason = "no coverage profile supplied"
		return d
	}
	d.Checked = true

	changedSet := toSet(in.ChangedFiles)
	byKey := make(map[string]coverage.SymbolLayerCoverage, len(in.CoverageLayered))
	for _, sc := range in.CoverageLayered {
		byKey[sc.Symbol] = sc
	}

	for _, n := range in.Nodes {
		if !changedSet[n.FilePath] {
			continue
		}
		sc, ok := byKey[n.Key()]
		if !ok {
			continue // no profile data for this symbol — omitted, not fabricated as 0%
		}
		detail := CoverageDetail{
			Symbol:      sc.Symbol,
			File:        sc.File,
			Line:        sc.Line,
			Layers:      sc.CoveringLayers(),
			UncoveredAt: sc.UncoveredAt(),
			Percent:     sc.Any.Percent,
			Uncovered:   sc.Any.Covered == 0,
		}
		d.Coverage = append(d.Coverage, detail)
		if detail.Uncovered {
			d.Findings = append(d.Findings, Finding{
				Kind:     dimCoverageName,
				Symbol:   sc.Symbol,
				File:     sc.File,
				Line:     sc.Line,
				Message:  "changed symbol has no covering test in any supplied layer (0% statements covered)",
				Citation: citeOf(sc.File, sc.Line),
				Severity: "warn",
			})
		}
	}
	sort.Slice(d.Coverage, func(i, j int) bool {
		if d.Coverage[i].File != d.Coverage[j].File {
			return d.Coverage[i].File < d.Coverage[j].File
		}
		return d.Coverage[i].Line < d.Coverage[j].Line
	})
	sort.Slice(d.Findings, func(i, j int) bool { return d.Findings[i].Symbol < d.Findings[j].Symbol })
	d.Count = len(d.Findings)
	return d
}

// ResolveCoverage builds DimTestCoverage's Input.CoverageLayered from
// caller-labeled, parsed Go cover profiles: internal/analysis/coverage.
// JoinLayeredSymbols against nodes. Shared by the CLI (cmd/fathom review /
// tests coverage) and the review_diff MCP tool so "coverage_profile(s)
// supplied" resolves identically on every surface. Pure: does no I/O
// itself — callers read and coverage.ParseProfile each profile file first,
// labeling it by layer (an explicit `--profile <layer>=<path>` /
// `coverage_profiles[].layer`, or "unit" for an unlabeled bare path — see
// coverage.LabeledProfile's own doc comment).
func ResolveCoverage(profiles []coverage.LabeledProfile, nodes []core.SymbolNode) []coverage.SymbolLayerCoverage {
	return coverage.JoinLayeredSymbols(profiles, nodes)
}

// ─── shared helpers ───────────────────────────────────────────────────────

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// citeOf renders an evidence citation for (file, line): core.CiteRef's
// "path#Lstart-Lend" form when line is known (>=1), a bare path when it
// isn't, "" when even the file is unknown. A bare-path citation is a
// VERDICT-only convenience — AttachClaims never persists a cites link for
// one (claims.go).
func citeOf(file string, line int) string {
	if file == "" {
		return ""
	}
	if line < 1 {
		return file
	}
	return core.CiteRef(file, line, line)
}

// lineOfRef returns the 1-based line of the first occurrence of ref in
// content (line endings normalized), or 0 when ref's exact text does not
// appear (e.g. a ref the extractor normalized away from its raw form).
func lineOfRef(content, ref string) int {
	for i, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.Contains(line, ref) {
			return i + 1
		}
	}
	return 0
}
