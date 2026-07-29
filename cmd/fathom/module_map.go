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

	// 2. Watermark drift vs. the current corpus (moduleWatermark, not the
	// raw whole-directory connector.Watermark — see its doc comment: a
	// module's own manifest/eval/pack artifacts sharing root with the
	// corpus must never count as corpus drift).
	current, wErr := moduleWatermark(ctx, root, m)
	switch {
	case wErr != nil:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("could not recompute a watermark for %s to compare against the manifest's %q: %v", root, m.Watermark, wErr),
		})
	case current != m.Watermark:
		report.Gaps = append(report.Gaps, Gap{
			Code: GapWatermarkDrift,
			Line: fmt.Sprintf("manifest watermark %q does not match the corpus's current watermark %q — the corpus has changed since this manifest was compiled", m.Watermark, current),
		})
	}

	// 3. Store-dependent gaps: claim density and anchor health. Absent
	// --store is an explicit, honest note — never a fake zero.
	if storePath == "" {
		report.Gaps = append(report.Gaps, Gap{
			Code: GapStoreNotProvided,
			Line: "no --store given — claim-density and anchor-health checks skipped (not compiled yet)",
		})
		return report, nil
	}

	st, err := store.Open(storePath)
	if err != nil {
		return MapReport{}, fmt.Errorf("open store %q: %w", storePath, err)
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
		return report, nil
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

	// Anchor health: legacy-empty AnchorText, and citations that no longer
	// hash-match the corpus at root (drifted).
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
	for _, l := range links {
		if l.Rel != core.RelCites {
			continue
		}
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

	return report, nil
}
