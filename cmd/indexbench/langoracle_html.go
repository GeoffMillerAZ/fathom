package main

// langoracle_html.go — HTML's L2 in W1's pair unit (design §1.8; wave lf5,
// item html; K19, committed with the holdout before any reading).
//
// W1 makes HTML's L2 its same-document ID references, and the pair is keyed
// by the document: (file, the module symbol "<module>", the id). An ID is
// resolved against the element's tree (the HTML standard: the first element
// in tree order within the referring element's tree whose ID is the value),
// never against the referring element's ancestors, so no enclosing element
// is the reference's owner; the referring element itself is seldom a
// definition (a label, an input with no id). The parse5 oracle writes every
// pair from "<module>" (calibration/oracles/parse5), and the html dialect gives
// every reference to the file's module symbol (calls.owner "module").
//
// Two scope rules follow, both html-only, so no other dialect's reading moves:
//
//   - htmlOwnCallers: HTML's pairs are made by the html dialect's own kinds.
//     An injected language's definitions (<script> JavaScript, <style> CSS,
//     design §3.6) make that language's calls, its own L2, never HTML's; they
//     leave fathom's side and are counted (html_injected_call_pairs).
//   - callExclusions: a pair whose callee the oracle file excludes (W1: an id
//     two or more elements of the document carry names no one element) leaves
//     both sides, counted per side (call_pairs_excluded_truth,
//     call_pairs_excluded_fathom).
//
// The dialect's name_match is "identity": an id is compared as written,
// case-sensitively and whole (an id may hold ".", ":" or "/", which the bare
// rule would cut).

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func init() {
	// The html dialect's src/href imports resolve as ingest resolves them
	// (fusion.ResolveFileEdges under "html"), so its file edges are scored
	// against the lychee oracle's imports (LB-F57).
	fileResolveLang["html"] = "html"
}

// callExclusion is one callee an oracle file takes out of a file's L2 pairs,
// with its reason (W1: "duplicate-id").
type callExclusion struct {
	Path   string `json:"path"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// Counts the html scope rules write.
const (
	cntHTMLInjectedCallPairs  = "html_injected_call_pairs"
	cntCallPairsExcludedTruth = "call_pairs_excluded_truth"
	cntCallPairsExcludedMine  = "call_pairs_excluded_fathom"
)

// htmlOwnCallers is nodes less the definitions of a language injected into an
// html file, when m is the html dialect; any other dialect's nodes are
// returned as they are. counts takes the pairs the dropped nodes made.
func htmlOwnCallers(m langbench.Manifest, nodes []core.SymbolNode, files map[string]bool, counts map[string]int) []core.SymbolNode {
	if own, ok := ownCallerRules[m.ID]; ok {
		return own(m, nodes, files, counts) // wave lf7: a component host's pairs are its module symbol's (langoracle_vue.go)
	}
	if m.ID != "html" {
		return nodes
	}
	own := make([]core.SymbolNode, 0, len(nodes))
	var foreign []core.SymbolNode
	for _, n := range nodes {
		if _, _, ok := m.CanonicalForSource(string(n.Kind)); ok {
			own = append(own, n)
		} else {
			foreign = append(foreign, n)
		}
	}
	if len(foreign) > 0 {
		counts[cntHTMLInjectedCallPairs] += len(m.CallPairs(foreign, files))
	}
	return own
}

// applyCallExclusions drops from truth and found every pair whose file and
// callee an exclusion names, counting each side's drops; it returns the
// number of exclusions on in-scope files.
func applyCallExclusions(m langbench.Manifest, ex []callExclusion, inScope map[string]bool, truth, found map[langbench.CallPair]bool, counts map[string]int) int {
	if len(ex) == 0 {
		return 0
	}
	type key struct{ path, callee string }
	out := map[key]bool{}
	for _, e := range ex {
		if inScope[e.Path] {
			out[key{e.Path, m.MatchName(e.To)}] = true
		}
	}
	for p := range truth {
		if out[key{p.Path, p.Callee}] {
			delete(truth, p)
			counts[cntCallPairsExcludedTruth]++
		}
	}
	for p := range found {
		if out[key{p.Path, p.Callee}] {
			delete(found, p)
			counts[cntCallPairsExcludedMine]++
		}
	}
	return len(out)
}

// noteHTMLPairUnit is the disclosure design §1.8 requires of an html reading
// whose L2 is W1's (K19, LB-F441).
const noteHTMLPairUnit = "L2 is W1's same-document ID references (design §1.8): one pair per (document, id), keyed by the file's module symbol <module>, graded under §1.2's call_recall and call_precision thresholds, no other; an injected language's calls (<script>, <style>) are its own L2 and are left out (html_injected_call_pairs), and a reference to an id two elements of its document carry leaves both sides (call_pairs_excluded_truth, call_pairs_excluded_fathom). L3 counts the links between the measured .html and .htm files; a link to a script, stylesheet or image is out of scope (import_oracle_out_of_scope)"

// toolOracleDisclosures is a tool-oracle file's own disclosures, each named
// for the subject, for the reading's notes, and for an html reading graded by
// an oracle's calls, the pair unit design §1.8 names.
func toolOracleDisclosures(m langbench.Manifest, name string, tf toolOracleFile) []string {
	out := make([]string, 0, len(tf.Disclose)+1)
	for _, d := range tf.Disclose {
		out = append(out, fmt.Sprintf("%s: %s", name, d))
	}
	if m.ID == "html" && len(tf.Calls) > 0 {
		out = append(out, noteHTMLPairUnit)
	}
	return out
}
