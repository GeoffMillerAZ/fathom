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
