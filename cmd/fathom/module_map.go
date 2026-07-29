package main

// module_map.go — `fathom module map`: DETERMINISTIC gaps only (law 11,
// docs/vision/13-expert-suite.md §8: "the advisor recommends; the gate
// decides" — there is no agentic advisor in Phase J at all, so every gap
// this verb reports is a plain, reproducible computation over the manifest,
// the corpus, and (when given) a compiled store). Every gap carries a
// stable machine Code and a human Line; a store-dependent gap that cannot
// be computed because no --store was given is reported as an explicit
// "not compiled yet" note — NEVER a fake zero (docs/vision/13-expert-
// suite.md §4 step 3).

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Gap codes — stable, machine-checkable identifiers (docs/vision/13-expert-
// suite.md §4 step 3's exact vocabulary, scoped to what Phase J actually
// computes: zero-match selectors, watermark drift, claim density, and
// anchor health. "Spec IDs cited but never defined" and "contradiction
// scan" are the vision doc's own STUDIO-scope items, deferred — see
// docs/plans/expert-suite-j.md's design section for the explicit scoping
// note).
const (
	GapZeroMatchSelector = "zero_match_selector"
	GapWatermarkDrift    = "watermark_drift"
	GapStoreNotProvided  = "store_not_provided"
	GapLayerNotCompiled  = "layer_not_compiled"
	GapZeroClaimDensity  = "zero_claim_density"
	GapLegacyEmptyAnchor = "legacy_empty_anchor"
	GapAnchorDrifted     = "anchor_drifted"
	// GapNoCitations fires when a layer HAS compiled claims but ZERO of
	// their links are "cites" relations (Phase J critic finding 10): a
	// hand-built or foreign store can carry claims with no citation links
	// at all, and the anchor loop below simply never iterates over
	// anything — silently reporting "0 broken anchors" in that case would
	// be a vacuous pass, not an honest one. Packaging (`module pack`)
	// invites exactly this: a store from anywhere, not only ones this
	// phase's own `module compile` produced.
	GapNoCitations = "no_citations"
)

// Gap is one deterministic finding `module map` reports.
type Gap struct {
	Code     string `json:"code"`
	Line     string `json:"line"`
	Selector string `json:"selector,omitempty"`
	ClaimID  string `json:"claim_id,omitempty"`
}

// MapReport is `module map`'s full output.
type MapReport struct {
	Manifest string `json:"manifest"`
	Root     string `json:"root"`
	Gaps     []Gap  `json:"gaps"`
}

func cmdModuleMap(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module map", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	root := fs.String("root", "", "corpus root (default: module.cue's own directory)")
	storePath := fs.String("store", "", "local store path — enables claim-density and anchor-health gaps")
	service := fs.String("service", "", "service ID in --store (default: manifest name)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "error: --manifest is required")
		fs.Usage()
		return 1
	}
	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	corpusRoot := moduleServiceRoot(*manifestPath, *root)
	svc := *service
	if svc == "" {
		svc = m.Name
	}

	report, err := computeMap(context.Background(), corpusRoot, *storePath, svc, m)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOut {
		jsonPrint(stdout, report)
		if len(report.Gaps) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "manifest: %s\nroot:     %s\n\n", report.Manifest, report.Root)
	if len(report.Gaps) == 0 {
		fmt.Fprintln(stdout, "no deterministic gaps found")
		return 0
	}
	for _, g := range report.Gaps {
		fmt.Fprintf(stdout, "[%s] %s\n", g.Code, g.Line)
	}
	return 1
}

func computeMap(ctx context.Context, root, storePath, service string, m modspec.Manifest) (MapReport, error) {
	report := MapReport{Manifest: m.Name, Root: root}

	// 1. Zero-match corpus selectors (reuses `sources`' own resolution so
	// the two verbs can never disagree about what "matches").
	inv, err := resolveSourceInventory(ctx, root, m)
	if err != nil {
		return MapReport{}, err
	}
	for _, sel := range inv.ZeroMatch {
		report.Gaps = append(report.Gaps, Gap{
			Code:     GapZeroMatchSelector,
			Line:     fmt.Sprintf("corpus selector %q matches zero files under %s", sel, root),
			Selector: sel,
		})
	}

	// The corpus's CURRENT watermark, in the two styles that matter here
	// (Phase J critic finding 3). moduleWatermark (corpus-scoped) is what
	// `module init` embeds into module.cue's own watermark field — scoped
	// specifically to avoid module.cue's own bytes invalidating themselves
	// (the fix for the self-reference bug this phase's cold run originally
	// caught). connector.Watermark (whole-directory) is what
	// pkg/client.CompileLayerWithEscape's OWN, unmodified ServiceWatermark
	// call actually records into compile_runs — frozen, shared machinery
	// this phase does not fork (docs/plans/expert-suite-j.md's design
	// section). These two are DIFFERENT ALGORITHMS measuring different
	// scopes of the same directory; they are never expected to produce the
	// same number for a module whose root holds anything besides its own
	// declared corpus (module.cue itself, .fathom/layers/, eval fixtures —
	// which is the common case). Comparing them to EACH OTHER, or comparing
	// manifest.Watermark directly against the store's recorded
	// whole-directory watermark, would therefore be comparing incomparable
	// values — not a freshness check at all. Instead, each side of the
	// freshness question is checked against ITS OWN algorithm's current
	// value, below.
	currentModuleWM, moduleWMErr := moduleWatermark(ctx, root, m)
	currentWholeWM, wholeWMErr := connector.NewLocalConnector().Watermark(ctx, core.ServiceRef{ID: m.Name, Path: root})

	// 2. Store-dependent gaps: claim density, the full watermark
	// cross-check, and anchor health. Absent --store, or a layer with no
	// compiled claims yet, only the store-INDEPENDENT two-way check
	// (manifest vs. current corpus) is possible — reported honestly as
	// such, never silently upgraded to the stronger three-way proof.
	if storePath == "" {
		report.Gaps = append(report.Gaps, Gap{
			Code: GapStoreNotProvided,
			Line: "no --store given — claim-density and anchor-health checks skipped (not compiled yet)",
		})
		appendTwoWayWatermarkGap(&report, m, currentModuleWM, moduleWMErr)
		return report, nil
	}

	// Phase J critic finding 5: MountReadOnly, never store.Open — Open runs
	// MkdirAll + CREATE TABLE, which would stamp fathom's own schema onto a
	// third-party artifact that merely happens to not exist yet, or add a
	// -wal/-shm sidecar beside a pristine copy. The handle is closed before
	// every return path below.
	st, err := store.MountReadOnly(storePath, store.LevelL0)
	if err != nil {
		return MapReport{}, fmt.Errorf("mount store %q read-only: %w", storePath, err)
	}
	defer st.Close()

	claims, err := st.ClaimsForService(ctx, service, m.Name)
	if err != nil {
		return MapReport{}, fmt.Errorf("load claims for (%s, %s): %w", service, m.Name, err)
	}
	if len(claims) == 0 {
		report.Gaps = append(report.Gaps, Gap{
			Code: GapLayerNotCompiled,
			Line: fmt.Sprintf("no claims compiled for layer %q on service %q yet (not compiled yet)", m.Name, service),
		})
		appendTwoWayWatermarkGap(&report, m, currentModuleWM, moduleWMErr)
		return report, nil
	}

	// The watermark-laundering fix (finding 3): two INDEPENDENT checks,
	// together covering all three quantities (manifest, store, corpus):
	//
	//   (a) COMPILE FRESHNESS — does not read manifest.Watermark at all, so
	//       hand-editing module.cue cannot launder past it: the store's own
	//       immutable compile_runs record must still match the corpus's
	//       CURRENT watermark, checked in WHICHEVER style actually produced
	//       it. A real (non-mock) compile records the whole-directory style
	//       (pkg/client.CompileLayerWithEscape's frozen, unmodified
	//       ServiceWatermark call — this phase does not fork it); the mock
	//       lane records the module-scoped style (scopedWatermarkConnector,
	//       module_compileeval.go — deliberately, so a mock-compiled
	//       module's freshness check is immune to sibling-file noise like
	//       editing module.cue's own eval_evidence afterward). storeWM is
	//       compared against BOTH current recomputations and passes if it
	//       matches EITHER — never both required, since a given compile run
	//       only ever used one algorithm. A mismatch against both means the
	//       corpus changed since the last REAL compile, regardless of what
	//       module.cue currently claims about itself.
	//   (b) MANIFEST HONESTY — module.cue's OWN declared watermark
	//       (module-scoped, `module init`'s convention) must match the
	//       corpus's CURRENT module-scoped watermark. A mismatch means the
	//       manifest's freshness claim about ITSELF no longer holds (hand-
	//       edited to a stale/fabricated value, or the declared corpus
	//       changed since module.cue was last written).
	//
	// A module that passes BOTH is fresh in every sense that matters.
	runs, runsErr := st.CompileRuns(ctx, service, m.Name)
	switch {
	case runsErr != nil:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("claims exist but compile_runs could not be read to cross-check the watermark: %v", runsErr),
		})
	case len(runs) == 0:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: "claims are compiled but no compile_runs record exists to cross-check the watermark against (an unusually old or hand-built store?)",
		})
	default:
		storeWM := runs[0].CorpusWatermark // CompileRuns is newest-first
		switch {
		case moduleWMErr != nil && wholeWMErr != nil:
			report.Gaps = append(report.Gaps, Gap{
				Code: GapWatermarkDrift,
				Line: fmt.Sprintf("could not recompute the corpus's current watermark (either style) to confirm the last compile (%q) is still fresh: %v / %v", storeWM, moduleWMErr, wholeWMErr),
			})
		case storeWM != currentModuleWM && storeWM != currentWholeWM:
			report.Gaps = append(report.Gaps, Gap{
				Code: GapWatermarkDrift,
				Line: fmt.Sprintf("the corpus has changed since the last compile: compile_runs recorded watermark %q, the corpus's current watermark is now %q (module-scoped) / %q (whole-directory)", storeWM, currentModuleWM, currentWholeWM),
			})
		}
	}
	switch {
	case moduleWMErr != nil:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("could not recompute the corpus's current module-scoped watermark to confirm the manifest's declared watermark %q still holds: %v", m.Watermark, moduleWMErr),
		})
	case m.Watermark != currentModuleWM:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("manifest declares watermark %q but the corpus's current module-scoped watermark is %q — module.cue's own freshness claim about itself no longer holds (hand-editing this field is not the same as recompiling)", m.Watermark, currentModuleWM),
		})
	}

	// Zero claim density per declared corpus selector: does at least one
	// claim's Scope glob match this selector's own glob text, or overlap
	// with at least one file the selector matches? A cheap, deterministic
	// proxy: a selector has zero density when no claim's Scope list
	// contains that EXACT selector string. Claims compiled by `module
	// compile` always carry Scope globs drawn from the SAME corpus
	// (verifyDraft rejects a scope matching zero indexed files), so an
	// exact-string match is the honest, non-inferential check this phase
	// commits to (law 11 — no semantic "is this related" judgment).
	scopedSelectors := map[string]bool{}
	for _, cl := range claims {
		for _, sc := range cl.Scope {
			scopedSelectors[sc] = true
		}
	}
	selectors := append([]string(nil), m.Corpus.Include...)
	sort.Strings(selectors)
	for _, sel := range selectors {
		if !scopedSelectors[sel] {
			report.Gaps = append(report.Gaps, Gap{
				Code:     GapZeroClaimDensity,
				Line:     fmt.Sprintf("corpus selector %q has zero claim density: no compiled claim scopes exactly to it", sel),
				Selector: sel,
			})
		}
	}

	// Anchor health: legacy-empty AnchorText, citations that no longer
	// hash-match the corpus at root (drifted), and the vacuous case (finding
	// 10) — claims present but zero citation links to check at all.
	ids := make([]string, 0, len(claims))
	for _, cl := range claims {
		ids = append(ids, cl.ClaimID)
	}
	links, err := st.ClaimLinks(ctx, service, ids)
	if err != nil {
		return MapReport{}, fmt.Errorf("load claim links: %w", err)
	}
	conn := connector.NewLocalConnector()
	// Fetch resolves a repo-relative ref against the root recorded by the
	// MOST RECENT Discover call on THIS connector instance (LocalConnector's
	// documented contract) — a Fetch with no preceding Discover on the same
	// instance errors "no root recorded for service", which the loop below
	// would otherwise (wrongly) report as GapAnchorDrifted "no longer
	// exists". Discover once, up front, to populate it.
	if _, derr := conn.Discover(ctx, core.ServiceRef{ID: m.Name, Path: root}); derr != nil {
		return MapReport{}, fmt.Errorf("discover %s for anchor-health check: %w", root, derr)
	}
	fileCache := map[string][]byte{}
	sort.Slice(links, func(i, j int) bool { return links[i].ClaimID < links[j].ClaimID })
	citesSeen := 0
	for _, l := range links {
		if l.Rel != core.RelCites {
			continue
		}
		citesSeen++
		if l.AnchorText == "" {
			report.Gaps = append(report.Gaps, Gap{
				Code:    GapLegacyEmptyAnchor,
				Line:    fmt.Sprintf("claim %s cites %q with no stored anchor text (compiled before anchor-text persistence)", l.ClaimID, l.TargetRef),
				ClaimID: l.ClaimID,
			})
			continue
		}
		path, start, end, ok := core.ParseCiteRef(l.TargetRef)
		if !ok {
			continue
		}
		content, cached := fileCache[path]
		if !cached {
			data, ferr := conn.Fetch(ctx, core.SourceRef{Service: m.Name, Path: path})
			if ferr != nil {
				report.Gaps = append(report.Gaps, Gap{
					Code:    GapAnchorDrifted,
					Line:    fmt.Sprintf("claim %s cites %s, which no longer exists under %s", l.ClaimID, l.TargetRef, root),
					ClaimID: l.ClaimID,
				})
				fileCache[path] = nil
				continue
			}
			content = data
			fileCache[path] = data
		}
		if content == nil {
			continue // already reported as missing above
		}
		hash, ok := core.HashLines(content, start, end)
		if !ok || hash != l.TargetHash {
			report.Gaps = append(report.Gaps, Gap{
				Code:    GapAnchorDrifted,
				Line:    fmt.Sprintf("claim %s's citation %s no longer hash-matches the corpus at %s", l.ClaimID, l.TargetRef, root),
				ClaimID: l.ClaimID,
			})
		}
	}
	if citesSeen == 0 {
		report.Gaps = append(report.Gaps, Gap{
			Code: GapNoCitations,
			Line: fmt.Sprintf("%d claim(s) compiled but ZERO citation links found — anchors cannot be verified (a hand-built or foreign store?)", len(claims)),
		})
	}

	return report, nil
}

// appendTwoWayWatermarkGap is the store-independent fallback: manifest vs.
// the corpus's current watermark only, used when no --store is given or
// the layer has no compiled claims yet (there is no store-recorded
// watermark to cross-check against in either case).
func appendTwoWayWatermarkGap(report *MapReport, m modspec.Manifest, current string, err error) {
	switch {
	case err != nil:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("could not recompute a watermark for %s to compare against the manifest's %q: %v", report.Root, m.Watermark, err),
		})
	case current != m.Watermark:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("manifest watermark %q does not match the corpus's current watermark %q (no compiled store available to cross-check the compile-time record against)", m.Watermark, current),
		})
	}
}
