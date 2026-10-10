package main

// langoracle_css.go — design 1.8 W2 (LB-F460, K19): how CSS's L2 is graded
// against lightningcss (calibration/oracles/lightningcss, tier A for what it
// attests).
//
// CSS has no calls. Its name references are a var() read of a custom property
// and an animation naming an @keyframes rule, and the dialect records each as a
// (owner, name) pair. Neither binds to one definition by itself: which
// `--brand` a var() reads is decided by the cascade, per element of a document
// the stylesheet does not contain, and lightningcss, a parser, attests no
// cascade. What a stylesheet does fix is the set of sheets that join it: an
// @import brings the imported sheet's rules into the importer's cascade, so a
// name the file or any sheet of its @import closure defines is a name the
// file's own rules can read. A name defined nowhere in that closure (a theme
// sheet the HTML loads beside it, a custom property an embedding page sets, a
// keyword the shorthand reads as a name) has no truth either side could be
// graded on.
//
// The rule: of the oracle's pairs and fathom's, a pair is graded only when its
// name is a custom property or an @keyframes rule the oracle defines in the
// pair's file or in a file that file's @import closure reaches (the oracle's
// own imports, transitively); every other pair is out of both, counted
// (css_w2_oracle_outside_closure, css_w2_fathom_outside_closure) and disclosed
// in the reading's notes. Owners and names are compared whole (name_match
// identity), the owner of a rule's declarations being the rule's first selector
// on both sides. Scoped to the css lang tag and to a tool oracle's calls; no
// other language, oracle or family changes.

import (
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const (
	cntCSSOracleOutsideClosure = "css_w2_oracle_outside_closure"
	cntCSSFathomOutsideClosure = "css_w2_fathom_outside_closure"
)

// cssClosureNames is, for each file of the oracle, the custom property and
// @keyframes names defined in it or in a file its @import closure reaches.
func cssClosureNames(m langbench.Manifest, tf *toolOracleFile) map[string]map[string]bool {
	defined := map[string]map[string]bool{}
	for _, d := range tf.Definitions {
		c, ok := m.OracleCanonical("tool", d.Kind, false)
		if !ok || (c != "variable" && c != "rule") {
			continue
		}
		if defined[d.Path] == nil {
			defined[d.Path] = map[string]bool{}
		}
		defined[d.Path][m.MatchName(d.Name)] = true
	}
	edges := map[string][]string{}
	for _, im := range tf.Imports {
		if im.ToPath != "" && im.ToPath != im.Path {
			edges[im.Path] = append(edges[im.Path], im.ToPath)
		}
	}
	files := map[string]bool{}
	for _, c := range tf.Calls {
		files[c.Path] = true
	}
	out := make(map[string]map[string]bool, len(files))
	for f := range files {
		names := map[string]bool{}
		seen := map[string]bool{f: true}
		queue := []string{f}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for n := range defined[cur] {
				names[n] = true
			}
			for _, next := range edges[cur] {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		out[f] = names
	}
	return out
}

// cssNameScope applies W2 to a CSS subject's call truth and fathom's pairs,
// counting what it leaves out into counts.
func cssNameScope(m langbench.Manifest, tf *toolOracleFile, truth, found map[langbench.CallPair]bool, counts map[string]int) (map[langbench.CallPair]bool, map[langbench.CallPair]bool) {
	names := cssClosureNames(m, tf)
	keep := func(pairs map[langbench.CallPair]bool, key string) map[langbench.CallPair]bool {
		out := make(map[langbench.CallPair]bool, len(pairs))
		for p := range pairs {
			if names[p.Path][p.Callee] {
				out[p] = true
			} else {
				counts[key]++
			}
		}
		return out
	}
	return keep(truth, cntCSSOracleOutsideClosure), keep(found, cntCSSFathomOutsideClosure)
}

// cssNameScopeNotes discloses W2 for one subject (design 1.8): what the scope
// left out of both sides.
func cssNameScopeNotes(subject string, counts map[string]int) []string {
	o, f := counts[cntCSSOracleOutsideClosure], counts[cntCSSFathomOutsideClosure]
	notes := []string{"W2 (design 1.8): lightningcss attests no cascade, so a var() or animation-name pair is graded as a name reference: kept when its name is a custom property or @keyframes rule defined in the file or its @import closure, every other pair out of both sides, counted"}
	if o+f > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d oracle and %d fathom name pairs name nothing defined in their file's @import closure (W2), out of both", subject, o, f))
	}
	sort.Strings(notes)
	return notes
}
