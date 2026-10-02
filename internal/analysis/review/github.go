// VB-22 (docs/plans/verbs.md; design docs/vision/29-verbs-not-lookups.md §3
// wave F; ADR-035 decision 4/R11): one deterministic Markdown PR comment for
// a review.Verdict — the same composed analysis review_diff and `fathom
// review` already produce, rendered for a human on GitHub instead of an
// agent. renderGitHubReview is pure: every dependency (the Verdict, the
// resolved changed-symbol/caller detail, the touched plan rows, the test
// gap, the index commit) is a caller-resolved argument, per this package's
// own "Run is a pure function… does no I/O" convention (engine.go) — the CLI
// (cmd/fathom/review.go) does the git/store/plan-file I/O and hands this
// function a fully-resolved GitHubReviewInput. NO model prose anywhere on
// this path: every byte here is a fixed Go template over already-computed
// data, so identical input always renders identical Markdown (the
// determinism contract, internal/AGENTS.md).
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// GitHubReviewMarker is the fixed marker line every rendered comment opens
// with, so a GitHub Action can find its own prior comment (by substring
// match on this line) and update it in place instead of piling up a new
// comment per push (VB-23).
const GitHubReviewMarker = "<!-- fathom-review -->"

// CallerRef is one resolved caller of a changed symbol, labeled with the
// SAME resolution-tier vocabulary find_callers/blast_radius speak
// (core.EdgeConfidence — "resolved" | "heuristic", core.EdgeConfidenceOf):
// the one accessor for that classification, never re-derived here.
type CallerRef struct {
	Symbol   string
	File     string
	Line     int
	Citation string
	Tier     core.EdgeConfidence
	// Depth is the reverse-call-graph distance from the changed symbol this
	// caller was reached at: 1 for a direct caller, 2 for a caller of a
	// caller, etc. Zero value (0) from a caller built before this field
	// existed reads as depth 1 (ChangedSymbolReviews' pre-VB-24-recall-loop
	// behavior was always direct callers only).
	Depth int
}

// ChangedSymbolReview is one symbol defined in a changed file, with its
// resolved direct callers and the call-site-completeness figures a human
// reviewer needs: how many places call it at all (CallSitesTotal), and how
// many of those call sites live in a file this SAME diff also touched
// (CallSitesUpdated) — evidence the update reached its call sites, not just
// its own definition.
type ChangedSymbolReview struct {
	Symbol           string
	File             string
	Line             int
	Citation         string
	Callers          []CallerRef
	CallSitesTotal   int
	CallSitesUpdated int
}

// TouchedPlanRow is one docs/plans/*.md row whose {symbol, package} contract
// anchors a symbol this diff changed (resolved by the CLI via
// internal/verify.ParseMarkdownPlan over the service's plan files — I/O this
// leaf package never performs itself).
type TouchedPlanRow struct {
	Plan   string // repo-relative plan file path
	RowID  string
	Symbol string
}

// GitHubReviewInput bundles every already-resolved dependency
// renderGitHubReview needs — the same "caller does the I/O" discipline
// Input follows for Run.
type GitHubReviewInput struct {
	Verdict         Verdict
	TestGap         impact.TestGapResult
	IndexCommit     string // the index's watermark commit; "" when unknown
	DiffRange       string // human-readable diff range, e.g. "origin/main..HEAD"
	ChangedSymbols  []ChangedSymbolReview
	TouchedPlanRows []TouchedPlanRow
}

// changedSymbolCallerDepth bounds ChangedSymbolReviews' reverse-call-graph
// walk (VB-24 recall-loop mechanism (a), docs/bench/readings/
// review-recall-loop-2026-09-18.md): a direct caller (depth 1) alone missed
// real fix targets the blast-radius/test-gap dimensions already compute
// unbounded (impact.DiffImpactWithTypedEdges has no depth cap) but the
// GitHub-rendered "Changed symbols" section never surfaced past depth 1.
// Bounded at 2 — the same default blast_radius/find_callers settled on
// (internal/mcp/tools.go, internal/mcp/extra_tools.go: "DEFAULTS changed in
// 0.9: … depth 2") — for a maintainer-readable comment: unbounded would let
// one heavily-called symbol's transitive fan-in swamp the rendered PR
// comment.
const changedSymbolCallerDepth = 2

// ChangedSymbolReviews resolves VB-22's per-changed-symbol caller detail:
// every symbol defined in a changed file, its callers out to
// changedSymbolCallerDepth hops (any tier — callgraph.CallersDetail walks
// both the syntax and the typed graph, core.EdgeConfidenceOf classifies
// each; a depth-2 caller is a caller of a direct caller, its own Depth
// field says which), and the call-site-completeness counts (DIRECT callers
// only — CallSitesTotal/CallSitesUpdated answer "how many places call this
// AT ALL", which a depth-2 indirect caller does not). Pure and
// deterministic: symbols and their callers are sorted (depth ascending,
// resolved tier before heuristic — the SAME typed-first ordering the
// resolution-mix summary field already renders fields in, mcp/
// resolution.go's `{typed, heuristic}` — then symbol name).
//
// Exported for the CLI to call while assembling GitHubReviewInput; kept in
// this package (not cmd/fathom) because it is pure analysis over
// []core.SymbolNode / []core.Edge, the same shape every other review
// dimension already computes from.
func ChangedSymbolReviews(nodes []core.SymbolNode, typed []core.Edge, changedFiles []string) []ChangedSymbolReview {
	changed := toSet(changedFiles)
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}

	g := callgraph.BuildBoth(nodes, typed)

	var keys []string
	for _, n := range nodes {
		if changed[n.FilePath] {
			keys = append(keys, n.Key())
		}
	}
	sort.Strings(keys)

	out := make([]ChangedSymbolReview, 0, len(keys))
	for _, k := range keys {
		n := byKey[k]
		csr := ChangedSymbolReview{
			Symbol:   k,
			File:     n.FilePath,
			Line:     n.Line,
			Citation: citeOf(n.FilePath, n.Line),
		}

		// DependentsDetail: callers and a type's consumers (references to
		// a Go type, which CallersDetail leaves out since wave lg), each
		// tiered by its own kind (a reference is heuristic).
		direct := callgraph.DependentsDetail(g, k)
		csr.CallSitesTotal = len(direct)
		seen := map[string]bool{k: true}
		type frontierItem struct {
			key   string
			depth int
		}
		var frontier []frontierItem
		for _, d := range direct {
			if seen[d.Key] {
				continue
			}
			seen[d.Key] = true
			cn := byKey[d.Key]
			if changed[cn.FilePath] {
				csr.CallSitesUpdated++
			}
			csr.Callers = append(csr.Callers, CallerRef{
				Symbol:   d.Key,
				File:     cn.FilePath,
				Line:     cn.Line,
				Citation: citeOf(cn.FilePath, cn.Line),
				Tier:     core.EdgeConfidenceOf(core.Edge{Kind: d.Kind}),
				Depth:    1,
			})
			frontier = append(frontier, frontierItem{d.Key, 1})
		}
		for i := 0; i < len(frontier); i++ {
			cur := frontier[i]
			if cur.depth >= changedSymbolCallerDepth {
				continue
			}
			for _, d := range callgraph.DependentsDetail(g, cur.key) {
				if seen[d.Key] {
					continue
				}
				seen[d.Key] = true
				cn := byKey[d.Key]
				csr.Callers = append(csr.Callers, CallerRef{
					Symbol:   d.Key,
					File:     cn.FilePath,
					Line:     cn.Line,
					Citation: citeOf(cn.FilePath, cn.Line),
					Tier:     core.EdgeConfidenceOf(core.Edge{Kind: d.Kind}),
					Depth:    cur.depth + 1,
				})
				frontier = append(frontier, frontierItem{d.Key, cur.depth + 1})
			}
		}

		sort.Slice(csr.Callers, func(i, j int) bool {
			a, b := csr.Callers[i], csr.Callers[j]
			if a.Depth != b.Depth {
				return a.Depth < b.Depth
			}
			if a.Tier != b.Tier {
				return a.Tier == core.EdgeConfidenceResolved // resolved (typed) sorts before heuristic
			}
			return a.Symbol < b.Symbol
		})
		out = append(out, csr)
	}
	return out
}

// claimShortID is VB-22's stable per-line id: a short hash of
// dimension+symbol+file, so a later re-render of the SAME finding (same
// dimension, same symbol, same file) reproduces the identical id and a
// consumer (a GitHub Action, a person) can diff one comment revision
// against the next. Deliberately narrower than review's persisted
// core.ClaimID (claims.go — which also hashes the finding's message so a
// changed rule text mints a new claim): this id exists only to let a
// comment-diff track "the same line", not to gate claim persistence.
func claimShortID(dimension, symbol, file string) string {
	h := sha256.Sum256([]byte(dimension + "\x00" + symbol + "\x00" + file))
	return hex.EncodeToString(h[:])[:12]
}

// renderGitHubReview composes GitHubReviewInput into ONE deterministic
// Markdown PR comment (VB-22): a header naming the index commit and the
// diff range, changed symbols with their resolved callers and each
// caller's resolution tier, call-site completeness, governing rules (the
// obligations dimension's rules-layer findings — Kind "" / "obligation",
// never the opt-in claims-layer bindings VB-9 added), a verify line per
// touched plan row, affected tests and the test gap, and a coverage block
// when a profile was supplied. Every finding line carries claimShortID's
// stable id. Opens with GitHubReviewMarker so a GitHub Action can find and
// update its own prior comment in place (VB-23).
func renderGitHubReview(in GitHubReviewInput) string {
	var b strings.Builder

	b.WriteString(GitHubReviewMarker)
	b.WriteString("\n## fathom review\n\n")

	indexCommit := in.IndexCommit
	if indexCommit == "" {
		indexCommit = "unknown"
	}
	diffRange := in.DiffRange
	if diffRange == "" {
		diffRange = "unknown"
	}
	fmt.Fprintf(&b, "Index commit `%s` · diff range `%s`\n\n", indexCommit, diffRange)

	b.WriteString("### Changed symbols\n\n")
	if len(in.ChangedSymbols) == 0 {
		b.WriteString("_no indexed symbols in the changed files._\n\n")
	}
	for _, cs := range in.ChangedSymbols {
		id := claimShortID("changed_symbol", cs.Symbol, cs.File)
		fmt.Fprintf(&b, "- `%s` (%s) — called from %d place(s), %d updated in this diff — claim `%s`\n",
			cs.Symbol, orDash(cs.Citation), cs.CallSitesTotal, cs.CallSitesUpdated, id)
		if len(cs.Callers) == 0 {
			b.WriteString("  - no indexed callers\n")
			continue
		}
		for _, c := range cs.Callers {
			depth := c.Depth
			if depth == 0 {
				depth = 1
			}
			fmt.Fprintf(&b, "  - `%s` (%s) [%s, depth %d]\n", c.Symbol, orDash(c.Citation), c.Tier, depth)
		}
	}
	b.WriteString("\n")

	b.WriteString("### Governing rules\n\n")
	rules := governingRuleFindings(in.Verdict)
	if len(rules) == 0 {
		b.WriteString("_none._\n\n")
	}
	for _, f := range rules {
		id := claimShortID(DimObligations, f.Symbol, f.File)
		fmt.Fprintf(&b, "- %s (%s) — claim `%s`\n", f.Message, orDash(f.Citation), id)
	}
	b.WriteString("\n")

	b.WriteString("### Verify\n\n")
	if len(in.TouchedPlanRows) == 0 {
		b.WriteString("_no plan row anchors a symbol in this diff._\n\n")
	}
	for _, r := range in.TouchedPlanRows {
		fmt.Fprintf(&b, "- plan row `%s` in `%s` anchors `%s` — run `fathom verify --service %s --plan %s`\n",
			r.RowID, r.Plan, r.Symbol, in.Verdict.Service, r.Plan)
	}
	b.WriteString("\n")

	b.WriteString("### Affected tests\n\n")
	if len(in.TestGap.AffectedTests) == 0 {
		b.WriteString("_none._\n\n")
	}
	for _, t := range in.TestGap.AffectedTests {
		fmt.Fprintf(&b, "- `%s` (%s) reaches %s\n", t.Test, orDash(t.Citation), strings.Join(t.Reaches, ", "))
	}
	b.WriteString("\n")

	b.WriteString("### Test gap\n\n")
	if len(in.TestGap.Gap) == 0 {
		b.WriteString("_none._\n\n")
	}
	for _, g := range in.TestGap.Gap {
		fmt.Fprintf(&b, "- `%s` (%s): no test reaches it\n", g.Symbol, orDash(g.Citation))
	}
	b.WriteString("\n")

	if cov := coverageDimension(in.Verdict); cov != nil && cov.Checked {
		b.WriteString("### Coverage\n\n")
		if len(cov.Coverage) == 0 {
			b.WriteString("_no changed symbol has profile data._\n\n")
		}
		for _, c := range cov.Coverage {
			id := claimShortID(dimCoverageName, c.Symbol, c.File)
			fmt.Fprintf(&b, "- `%s` (%s): %.1f%% covered, layers [%s], uncovered_at [%s] — claim `%s`\n",
				c.Symbol, orDash(citeOf(c.File, c.Line)), c.Percent,
				strings.Join(c.Layers, ", "), strings.Join(c.UncoveredAt, ", "), id)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderGitHubReview is renderGitHubReview's exported entrypoint for
// callers outside this package (cmd/fathom's `fathom review --format
// github`, VB-22/VB-23) — the anchored function itself
// (docs/plans/verbs.md's `symbol: renderGitHubReview` contract) stays
// unexported so its name matches the plan row exactly; this is a thin,
// zero-logic pass-through.
func RenderGitHubReview(in GitHubReviewInput) string {
	return renderGitHubReview(in)
}

// governingRuleFindings returns the obligations dimension's RULES-layer
// findings only (Kind "" or "obligation" — the human-authored governing
// rules VB-9 defaults to), never the opt-in claims-layer bindings (Kind
// "finding", a served plan-row contract) or any other served-claim kind:
// "governing rules … (obligations, layer: rules)" per this row's own text.
func governingRuleFindings(v Verdict) []Finding {
	for _, d := range v.Dimensions {
		if d.Name != DimObligations {
			continue
		}
		var out []Finding
		for _, f := range d.Findings {
			if f.Kind == "" || f.Kind == "obligation" {
				out = append(out, f)
			}
		}
		return out
	}
	return nil
}

// coverageDimension returns the "coverage" dimension from v, or nil if
// somehow absent (Run always includes it, but a caller could hand-build a
// Verdict without it in a test).
func coverageDimension(v Verdict) *Dimension {
	for i := range v.Dimensions {
		if v.Dimensions[i].Name == dimCoverageName {
			return &v.Dimensions[i]
		}
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
