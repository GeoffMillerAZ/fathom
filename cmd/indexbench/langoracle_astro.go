package main

// langoracle_astro.go — Astro's reading under design 1.8's Astro paragraph
// (K19; wave lf7, item angularastro, LB-F546).
//
// An Astro file is one component hosting TypeScript (its frontmatter and its
// JavaScript scripts) and CSS (its styles), F2. The reading scores the astro
// dialect's own extractor under calibration/oracles/astro-compiler/
// astro.scoring.json, the dialect's kinds plus the TypeScript kinds a hosted
// range keeps, against astro_oracle.mjs's tier-A file (@astrojs/compiler's
// parse for the frontmatter and the template's component nodes, TypeScript's
// parser for what a range declares, calls and imports). Two rules here, both
// astro-only, so no other dialect's reading moves:
//
//   - astroOwnCallers: a <style>'s CSS definitions make CSS's pairs (W2's
//     references, CSS's own L2), which leave Astro's L2, counted
//     (astro_injected_css_pairs).
//   - the dialect's relative imports resolve as ingest resolves them
//     (fusion.ResolveFileEdges under "astro", a file-resolve language as an
//     injection host, LB-F545), so its file edges are scored against the
//     oracle's imports (LB-F57) between the measured .astro files.

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// astroDialect is the reading's dialect id.
const astroDialect = "astro"

// cntAstroInjectedCSSPairs counts the pairs a style's CSS made in an Astro
// file, left out of Astro's L2.
const cntAstroInjectedCSSPairs = "astro_injected_css_pairs"

func init() {
	fileResolveLang[astroDialect] = astroDialect
}

var (
	cssKindsOnce sync.Once
	cssKinds     map[string]bool
)

// styleSourceKinds is every source kind a style dialect an Astro file injects
// (css, scss) declares.
func styleSourceKinds() map[string]bool {
	cssKindsOnce.Do(func() {
		cssKinds = map[string]bool{}
		ds, err := extractor.LoadDialects()
		if err != nil {
			return
		}
		for _, d := range ds {
			if d.ID != "css" && d.ID != "scss" {
				continue
			}
			for _, k := range d.Kinds {
				cssKinds[k.SourceKind] = true
			}
		}
	})
	return cssKinds
}

// astroOwnCallers is nodes less the definitions an injected style made, when
// m is the astro dialect; any other dialect's nodes are returned as they are.
// counts takes the pairs the dropped nodes made.
func astroOwnCallers(m langbench.Manifest, nodes []core.SymbolNode, files map[string]bool, counts map[string]int) []core.SymbolNode {
	if m.ID != astroDialect {
		return nodes
	}
	css := styleSourceKinds()
	own := make([]core.SymbolNode, 0, len(nodes))
	var style []core.SymbolNode
	for _, n := range nodes {
		if css[string(n.Kind)] {
			style = append(style, n)
		} else {
			own = append(own, n)
		}
	}
	if len(style) > 0 {
		counts[cntAstroInjectedCSSPairs] += len(m.CallPairs(style, files))
	}
	return own
}

// ownCallers is the nodes a dialect's L2 pairs are made by: html's own kinds
// (htmlOwnCallers, wave lf5), astro's less its styles' (astroOwnCallers),
// every node for any other dialect.
func ownCallers(m langbench.Manifest, nodes []core.SymbolNode, files map[string]bool, counts map[string]int) []core.SymbolNode {
	if m.ID == astroDialect {
		return astroOwnCallers(m, nodes, files, counts)
	}
	return htmlOwnCallers(m, nodes, files, counts)
}
