package langbench

import "strings"

// documentScopedLocal reports whether sym is a scip-dart local symbol. SCIP
// scopes a `local N` symbol to its own document, and scip-dart numbers them per
// document from 0, so the same `local 0` in two documents is two symbols. The
// first document to define a private member as `local 0` is not where another
// document's `local 0` lives, and an edge drawn to it is not a reference
// (LB-F245: 576 of the 772 file edges the oracle held and fathom did not were
// such collisions).
func documentScopedLocal(tool, sym string) bool {
	return tool == "scip-dart" && strings.HasPrefix(sym, "local ")
}
