package semantic

// rawfallback.go — wave O1 (ADR-037): the product defect the benches found
// is that `obligations` serves NOTHING USEFUL when the compiled layer's
// coverage for the edited path's own scope is incomplete. vs-pilot-5's
// second reading: the obligations tool never served the predicted rule in
// 0 of 4 trials built to test it, because the compiled-rule layer hit
// budget_stopped before reaching the nested guide that mattered.
// guards-pilot-6's second reading: obligations never surfaced the
// task-specific governing rule on 6 of 8 tasks' wired trials — served only
// a fixed generic boilerplate subset of internal/CLAUDE.md, capped before
// the invariant appeared. Meanwhile grep finds the rule in the nearest
// CLAUDE.md in one call.
//
// RawFallbackRules is the fix: when a governing document's section was
// never turned into a compiled claim (no prior claim at its Subject, or a
// digest/prompt-version mismatch — the same freshness test LiveLayerCoverage
// already applies), the section is still a REAL, human-authored rule sitting
// on disk. Rank the layer's own UNCOMPILED sections by the exact same
// governing-scope proximity ranker rankByGoverningScope (VB-8) already
// applies to compiled claims, and serve the nearest ones — flagged
// Tier:"raw" and carrying a Why note — instead of silence or a root-tier
// rule that happens to have compiled.
//
// Deliberately small and deterministic: no LLM call on this path, a fixed
// RawFallbackReserve budget independent of the compiled MaxObligations cap
// (a raw row only ever ADDS to a compiled answer, never displaces one), and
// the same section-splitting/digest machinery LiveLayerCoverage already
// uses — no second corpus-scanning mechanism to keep in sync.
import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/adr"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// RawFallbackRules ranks corpus's sections that have NO fresh compiled
// claim on layerDef (no prior claim at the section's Subject at all, or a
// digest/prompt-version mismatch — allDigestsMatch, the identical freshness
// test LiveLayerCoverage uses) by proximity to file, via rankByGoverningScope
// — the same VB-8 ranker obligationsCore applies to compiled claims — and
// renders up to max of the nearest as raw Obligation rows: Tier "raw",
// Layer layerDef.Name, Src the section's own citation, Why the caller-
// supplied coverage-defect note. Deterministic: same claims/corpus/file,
// same rows, every call.
//
// extraTokens (wave O2, item 1) is an OPTIONAL list of symbol/import names
// the caller has already resolved from the index for file — folded into
// the candidates' relevance ranking at the same specificity as the file's
// own stem (symbolRelevanceTokens). RawFallbackRules stays a pure function
// of its inputs; it never queries a store itself. Pass nil when the caller
// has no such names (or does not want to resolve them) — relevance ranking
// still runs on the path-derived tokens alone.
//
// Returns (nil, nil) when file is empty, max<=0, the corpus is empty, or no
// candidate section's implied scope (impliedGuideScope) reaches file at
// all — an honest "nothing to add here", never a fabricated row. Returns a
// non-nil error only when layerDef's own prompt identity cannot be
// resolved (promptForLayer) — a load-time defect, not a coverage gap; the
// caller (pkg/client.Client.RawFallbackObligations) treats this as a
// serve-time enrichment failure and degrades to no raw rows, exactly like
// every other optional enrichment in this package.
//
// only restricts the rows to the sections whose Subject it names (nil means
// every uncompiled section). The obligations serve path passes it to put
// back the CURRENT text of a section whose compiled claim it withheld as
// stale (I38), without dragging in every other uncompiled section of the
// same document.
func RawFallbackRules(claims []core.SemanticClaim, layerDef LayerDef, corpus []core.SourceFile, file string, max int, why string, extraTokens []string, only map[string]bool, stats ...*RawFallbackStats) ([]Obligation, error) {
	if file == "" || max <= 0 || len(corpus) == 0 {
		return nil, nil
	}
	tokens := pathRelevanceTokens(file)
	tokens = append(tokens, symbolRelevanceTokens(extraTokens)...)
	_, promptVersion, err := promptForLayer(layerDef)
	if err != nil {
		return nil, err
	}

	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range claims {
		if cl.Layer != layerDef.Name {
			continue
		}
		if cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted {
			continue
		}
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}

	type candidate struct {
		sec      Section
		gs       governingScope
		rel      float64
		strong   bool
		text     string
		standing string
	}
	var cands []candidate
	refinder := rawStatsRefinder(stats)
	for _, ds := range refinder.corpusSections(corpus) {
		_, decisions := refinder.parse(ds.doc.Path, ds.doc.Content)

		for _, sec := range ds.sections {
			if only != nil && !only[sec.Subject] {
				continue
			}
			if prior, seen := bySubject[sec.Subject]; seen && allDigestsMatch(prior, sec.Digest, promptVersion) {
				continue // a fresh compiled claim already answers this exact section
			}

			scope := impliedGuideScope(sec.Path)
			// classifyGlob (rankByGoverningScope's own worker) documents
			// that it only classifies PROXIMITY for a glob already known to
			// match — it defensively falls back to root tier for a glob
			// shape it cannot place, rather than reporting no-match. The
			// real match gate is filter.Match, the exact one obligationsCore
			// (fileScopeMatch) already runs on a compiled claim's scope —
			// run it here too, or a nested guide that governs a WHOLLY
			// UNRELATED subtree would default straight to a false root-tier
			// "match".
			if !filter.New(scope).Match(file) {
				continue
			}
			gs := rankByGoverningScope(scope, file)
			if gs.tier < 0 {
				continue // defensive: rankByGoverningScope found no tier despite the match above
			}
			if !rawSectionInForce(sec, decisions) {
				countInForceWithheld(stats)
				continue
			}
			countCheckedPlanRows(sec, stats)
			text := rawRuleText(sec)
			if text == "" {
				countRawWithheld(stats)
				continue
			}
			rel, strong := relevanceStrength(sec.Text, tokens)
			cands = append(cands, candidate{sec: sec, gs: gs, rel: rel, strong: strong, text: text, standing: rawSectionStanding(sec, decisions)})
		}
	}
	if len(cands) == 0 {
		return nil, nil
	}

	// Nearest tier first (VB-8's own proximity order: exact_file < package <
	// parent_dir < root, deeper ancestor wins a parent_dir tie) — EXCEPT
	// (wave O3, item 1; wave O4, item 1; ADR-037 addendum) a farther-tier
	// section that STRONGLY names the file (its package path, stem, or a
	// caller-resolved symbol/behaviour token) leapfrogs a nearer-tier
	// section that is not itself strong — an ancestor-directory-only hit no
	// longer blocks this, only a hit at packageWeight or above does; see
	// crossTierCloser's own doc comment for the exact, deliberately narrow
	// rule. WITHIN a tie on that comparison (wave O2, vs-pilot-6 second
	// reading §5), the lexical relevance score decides NEXT — a nested
	// guide's several same-tier sections no longer fall straight to
	// document order; the one that actually names the file wins before
	// compileorder.go's own document-type discipline (a nested/root GUIDE's
	// own section order is never defeated by byte count; a root-OTHER
	// document's smaller sections sort first) gets a say, so a truncated
	// raw answer still reads like a deliberate, on-topic pick, not merely
	// an arbitrary or alphabetically-early one.
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.gs.tier != b.gs.tier || a.gs.depth != b.gs.depth {
			return crossTierCloser(a.gs, b.gs, a.strong, b.strong)
		}
		if a.rel != b.rel {
			return a.rel > b.rel
		}
		ta, tb := docCompileTier(a.sec.Path), docCompileTier(b.sec.Path)
		if ta != tb {
			return ta < tb
		}
		if ta == docTierRootOther {
			if la, lb := len(a.sec.Text), len(b.sec.Text); la != lb {
				return la < lb
			}
		}
		if a.sec.Path != b.sec.Path {
			return a.sec.Path < b.sec.Path
		}
		return a.sec.StartLine < b.sec.StartLine
	})
	if len(cands) > max {
		cands = cands[:max]
	}

	out := make([]Obligation, 0, len(cands))
	for _, c := range cands {
		start, end := citeWindow(c.sec, tokens)
		out = append(out, Obligation{
			Rule:    c.text,
			Subject: c.sec.Subject,
			InForce: c.standing,
			Src:     fmt.Sprintf("%s#L%d-L%d", c.sec.Path, start, end),
			Tier:    "raw",
			Layer:   layerDef.Name,
			Why:     why,
		})
	}
	return out, nil
}

// RootRawFallbackRules is RawFallbackRules' root-scope sibling (O1452): the
// obligations call with no file asks what governs the repository, and while
// the repository root's own guide (AGENTS.md, CLAUDE.md) is uncompiled or
// stale the compiled layer can only answer with other documents' old rules.
// This serves up to max of that guide's sections whose compiled claim is
// missing or no longer matches the section's text, as Tier "raw" rows with
// the root as their implied scope, in document order (path, then line).
// Only the root-level instruction files qualify: a nested guide governs its
// own subtree, not the root, and ADR.md and docs/design/*.md are not guides.
// Deterministic; returns (nil, nil) when nothing qualifies. A non-nil error
// only when layerDef's prompt identity cannot be resolved.
func RootRawFallbackRules(claims []core.SemanticClaim, layerDef LayerDef, corpus []core.SourceFile, max int, why string, stats ...*RawFallbackStats) ([]Obligation, error) {
	if max <= 0 || len(corpus) == 0 {
		return nil, nil
	}
	_, promptVersion, err := promptForLayer(layerDef)
	if err != nil {
		return nil, err
	}
	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range claims {
		if cl.Layer != layerDef.Name || cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted {
			continue
		}
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}
	var cands []Section
	refinder := rawStatsRefinder(stats)
	for _, ds := range refinder.corpusSections(corpus) {
		_, decisions := refinder.parse(ds.doc.Path, ds.doc.Content)
		for _, sec := range ds.sections {
			if !isRootGuidePath(sec.Path) {
				continue
			}
			if prior, seen := bySubject[sec.Subject]; seen && allDigestsMatch(prior, sec.Digest, promptVersion) {
				continue
			}
			if !rawSectionInForce(sec, decisions) {
				countInForceWithheld(stats)
				continue
			}
			if rawRuleText(sec) == "" {
				countRawWithheld(stats)
				continue
			}
			cands = append(cands, sec)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Path != cands[j].Path {
			return cands[i].Path < cands[j].Path
		}
		return cands[i].StartLine < cands[j].StartLine
	})
	if len(cands) > max {
		cands = cands[:max]
	}
	out := make([]Obligation, 0, len(cands))
	for _, sec := range cands {
		out = append(out, Obligation{
			Rule:    rawRuleText(sec),
			Subject: sec.Subject,
			Src:     fmt.Sprintf("%s#L%d-L%d", sec.Path, sec.StartLine, sec.EndLine),
			Tier:    "raw",
			Layer:   layerDef.Name,
			Why:     why,
		})
	}
	return out, nil
}

// UncompiledRootGuide reports whether cov names a repository-root guide
// among its uncompiled scopes (O1452's trigger for the root raw fallback).
func UncompiledRootGuide(cov LayerCoverage) bool {
	for _, doc := range cov.UncompiledScopes {
		if isRootGuidePath(doc) {
			return true
		}
	}
	return false
}

// rawSnippetWindowLines bounds how many lines a long raw section's citation
// narrows to around its first lexical-relevance hit (wave O2, item 4): the
// MCP obligations tool's snippet renderer (internal/mcp/obligationsnippet.go,
// BP-20) truncates a citation's TEXT from its FIRST byte once it exceeds the
// 240-byte snippet cap — so a raw row whose Src always spanned the whole
// section showed the section's OPENING lines, never the bullet a reader
// actually wanted (H1-T01's "Adding things" section: the opening lines are
// about the Extractor recipe already in this repo's own guide, but a longer
// governing section elsewhere could easily open on something else).
// Narrowing Src itself — not the rendering path — fixes this for every
// caller of a raw row's citation, not just the one MCP snippet mechanism.
const rawSnippetWindowLines = 8

// citeWindow narrows a long raw section's own line range to the lines
// around the FIRST line that scores a relevance hit against tokens,
// leaving a short section's Src untouched — StartLine/EndLine already ARE
// its most relevant lines when there is nothing worth narrowing. Falls
// back to the section's own full range when no line scores a hit at all
// (an honest "nothing to center on", never a fabricated window).
func citeWindow(sec Section, tokens []weightedToken) (start, end int) {
	if sec.EndLine-sec.StartLine < rawSnippetWindowLines || len(tokens) == 0 {
		return sec.StartLine, sec.EndLine
	}
	lines := strings.Split(sec.Text, "\n")
	hit := -1
	for i, ln := range lines {
		if relevanceScore(ln, tokens) > 0 {
			hit = i
			break
		}
	}
	if hit < 0 {
		return sec.StartLine, sec.EndLine
	}
	half := rawSnippetWindowLines / 2
	start = sec.StartLine + hit - half
	if start < sec.StartLine {
		start = sec.StartLine
	}
	end = start + rawSnippetWindowLines
	if end > sec.EndLine {
		end = sec.EndLine
	}
	return start, end
}

// impliedGuideScope is a raw candidate section's scope for governing-scope
// ranking purposes, mirroring scopeFloorDir's own nested/root distinction
// (BP-32): a NESTED CLAUDE.md/AGENTS.md floors its section to its own
// directory subtree, exactly as the compiler floors a claim compiled from
// it; every other governing document in the layer's own corpus (the repo
// root's own CLAUDE.md/AGENTS.md, ADR.md, docs/plans/README.md, a
// project-local guard) reaches the whole repo, same as an unanchored
// compiled scope glob would.
func impliedGuideScope(docPath string) []string {
	if dir := scopeFloorDir(docPath); dir != "" {
		return []string{dir + "/**"}
	}
	return []string{"**"}
}

// RawFallbackStats is request-local accounting, never retained in the store.
type RawFallbackStats struct {
	Withheld        int
	InForceWithheld int
	Refinder        *RawRuleRefinder // shared only within this request, keyed by path and content hash
	Skipped         string           // the first selected citation that could not be read
}
type rawFallbackStatsKey struct{}

// WithRawFallbackStats lets the tool collect extraction counts from its backend
// without changing the existing raw-fallback interface.
func WithRawFallbackStats(ctx context.Context) (context.Context, *RawFallbackStats) {
	stats := &RawFallbackStats{}
	return context.WithValue(ctx, rawFallbackStatsKey{}, stats), stats
}

func RawFallbackStatsFromContext(ctx context.Context) *RawFallbackStats {
	stats, _ := ctx.Value(rawFallbackStatsKey{}).(*RawFallbackStats)
	return stats
}

func countRawWithheld(stats []*RawFallbackStats) {
	if len(stats) > 0 && stats[0] != nil {
		stats[0].Withheld++
	}
}

// RawRuleRefinder splits each live file/content identity once per call.
// It is request-local; it never persists corpus data across requests.
type RawRuleRefinder struct {
	sections  map[rawContentKey][]Section
	decisions map[rawContentKey][]adr.Section
}
type rawContentKey struct {
	path string
	hash [32]byte
}

func NewRawRuleRefinder() *RawRuleRefinder {
	return &RawRuleRefinder{sections: map[rawContentKey][]Section{}, decisions: map[rawContentKey][]adr.Section{}}
}

// RefindRawRule preserves caller-resolved relevance tokens at the live heading.
func RefindRawRule(row Obligation, data []byte, file string, extraTokens []string, refinders ...*RawRuleRefinder) (Obligation, bool) {
	var refinder *RawRuleRefinder
	if len(refinders) > 0 {
		refinder = refinders[0]
	}
	if refinder == nil {
		refinder = NewRawRuleRefinder()
	}
	return refinder.Refind(row, data, file, extraTokens)
}

func (r *RawRuleRefinder) Refind(row Obligation, data []byte, file string, extraTokens []string) (Obligation, bool) {
	p, _, _, ok := parseCite(row.Src)
	if !ok {
		return row, false
	}
	sections, decisions := r.parse(p, data)
	for _, sec := range sections {
		if sec.Subject != row.Subject {
			continue
		}
		standing := rawSectionStanding(sec, decisions)
		row.InForce = standing
		if inactiveStanding(standing) {
			return row, false
		}
		text := rawRuleText(sec)
		if text == "" {
			return row, false
		}
		tokens := append(pathRelevanceTokens(file), symbolRelevanceTokens(extraTokens)...)
		start, end := citeWindow(sec, tokens)
		row.Rule = text
		row.InForce = standing
		row.Src = fmt.Sprintf("%s#L%d-L%d", p, start, end)
		return row, true
	}
	return row, false
}

func decisionsForSection(p string, data []byte) []adr.Section {
	if isDecisionLog(p) {
		return adr.ParseSections(data)
	}
	return nil
}
func countInForceWithheld(stats []*RawFallbackStats) {
	if len(stats) > 0 && stats[0] != nil {
		stats[0].InForceWithheld++
	}
}

func countCheckedPlanRows(sec Section, stats []*RawFallbackStats) {
	if !strings.HasPrefix(sec.Path, "docs/plans/") {
		return
	}
	for _, line := range strings.Split(sec.Text, "\n") {
		if checkedPlanRow.MatchString(strings.TrimSpace(line)) {
			countInForceWithheld(stats)
		}
	}
}

// parse is shared by selection and live refinding, so unchanged live bytes
// reuse the exact section split that ranking already examined.
func (r *RawRuleRefinder) parse(p string, data []byte) ([]Section, []adr.Section) {
	key := rawContentKey{p, sha256.Sum256(data)}
	sections, found := r.sections[key]
	if !found {
		sections = SplitMarkdownSections(p, data)
		r.sections[key] = sections
		r.decisions[key] = decisionsForSection(p, data)
	}
	return sections, r.decisions[key]
}
func (r *RawRuleRefinder) corpusSections(corpus []core.SourceFile) []docSections {
	docs := append([]core.SourceFile(nil), corpus...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	all := make([]docSections, 0, len(docs))
	for _, doc := range docs {
		sections, _ := r.parse(doc.Path, doc.Content)
		all = append(all, docSections{doc: doc, sections: sections})
	}
	return dedupeTwinGuideSections(all)
}
func rawStatsRefinder(stats []*RawFallbackStats) *RawRuleRefinder {
	if len(stats) > 0 && stats[0] != nil {
		if stats[0].Refinder == nil {
			stats[0].Refinder = NewRawRuleRefinder()
		}
		return stats[0].Refinder
	}
	return NewRawRuleRefinder()
}
