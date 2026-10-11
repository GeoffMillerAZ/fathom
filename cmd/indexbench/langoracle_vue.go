package main

// langoracle_vue.go — Vue single-file components' L1 to L3 under W4 (design
// §1.8 "W4 for Vue and Svelte"; wave lf7, item vuesvelte, K19), measured
// against @vue/compiler-sfc (calibration/oracles/vue-compiler-sfc, tier A).
// The Svelte reading shares these rules (langoracle_svelte.go).
//
//   - L1: the script blocks' top-level declarations, scored against the
//     definitions the file hosts (F2): the vue manifest names the TypeScript
//     and JavaScript kinds, so langbench.ScoreL1 needs no rule of its own.
//   - L2: one pair per (file, <module>, component). componentOwnCallers keeps
//     the module symbol's pairs alone (html's htmlOwnCallers precedent): an
//     injected script's or style's definitions make that language's own L2,
//     left out and counted (<id>_injected_call_pairs); the oracle's
//     call_exclusions take out the names the scripts call (F2 gives a
//     script's top-level calls to the module symbol), counted.
//   - L3: the file resolver serves the dialect (fileResolveLang), so its
//     imports are scored against the oracle's import statements and its file
//     edges against their (file, file) pairs (LB-F40, LB-F57).

import (
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func init() {
	fileResolveLang["vue"] = "vue"
	ownCallerRules["vue"] = componentOwnCallers
}

// ownCallerRules is the dialects whose L2 pairs are made by a subset of the
// nodes the harness extracts (htmlOwnCallers consults it first).
var ownCallerRules = map[string]func(m langbench.Manifest, nodes []core.SymbolNode, files map[string]bool, counts map[string]int) []core.SymbolNode{}

// componentOwnCallers is nodes less every definition but the component's
// module symbol (the manifest's canonical "module" kind): a component host's
// pairs are its template's, the module symbol's. counts takes the pairs the
// dropped nodes made under "<id>_injected_call_pairs".
func componentOwnCallers(m langbench.Manifest, nodes []core.SymbolNode, files map[string]bool, counts map[string]int) []core.SymbolNode {
	module := map[string]bool{}
	for _, k := range m.Kinds {
		if k.Canonical == "module" {
			module[k.SourceKind] = true
		}
	}
	own := make([]core.SymbolNode, 0, len(nodes))
	var foreign []core.SymbolNode
	for _, n := range nodes {
		if n.IsModuleSymbol() && module[string(n.Kind)] {
			own = append(own, n)
		} else {
			foreign = append(foreign, n)
		}
	}
	if len(foreign) > 0 {
		counts[m.ID+"_injected_call_pairs"] += len(m.CallPairs(foreign, files))
	}
	return own
}
