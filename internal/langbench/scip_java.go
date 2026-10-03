package langbench

import (
	"strings"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// isSCIPJava reports whether sym is one scip-java wrote (its symbols open
// with the SemanticDB scheme; scip-java's Kotlin and Scala readings share it
// and share the `<init>` rule below).
func isSCIPJava(sym string) bool { return strings.HasPrefix(sym, "semanticdb ") }

// javaName is a scip-java symbol's descriptor leaf in the name fathom's
// dialect gives the same definition (any other producer's leaf is returned as
// it is): a constructor (`Foo#`<init>`().`) is its class, Foo,
// which is also how new Foo(...) is written as a call. owner is the
// descriptor text before the leaf ("…/Foo#").
func javaName(sym, leaf, owner string) string {
	if !isSCIPJava(sym) || leaf != "<init>" {
		return leaf
	}
	o := strings.TrimSuffix(owner, "#")
	return o[strings.LastIndexAny(o, "/#.:] `")+1:]
}

// CntScipJavaLocalMethods counts scip-java `local N` definitions of an
// anonymous or local class's method mapped to the extractor's same-named
// definition inside them (O458): callers of the call truth, never counted
// among the index's definitions (def_mapping is unchanged).
const CntScipJavaLocalMethods = "scip_java_local_methods_mapped"

// javaLocalMethods are a scip-java document's local methods and
// constructors (an anonymous or local class's members, `local N`), by
// symbol, named by their display name (O458). Any other producer has none.
func javaLocalMethods(tool string, d *scippb.Document) map[string]string {
	if tool != "scip-java" {
		return nil
	}
	var out map[string]string
	for _, si := range d.GetSymbols() {
		if !strings.HasPrefix(si.GetSymbol(), "local ") || si.GetDisplayName() == "" {
			continue
		}
		switch si.GetKind() {
		case scippb.SymbolInformation_Method, scippb.SymbolInformation_Constructor:
			if out == nil {
				out = map[string]string{}
			}
			out[si.GetSymbol()] = si.GetDisplayName()
		}
	}
	return out
}

// javaLocalMethodName is the name of a scip-java local definition that can
// be a caller of the call truth: a local method (javaLocalMethods) with an
// enclosing_range whose name is what its one-line range spans; "" otherwise.
func javaLocalMethodName(locals map[string]string, o *scippb.Occurrence, r [4]int32) string {
	name := locals[o.GetSymbol()]
	if name == "" || len(o.GetEnclosingRange()) == 0 || r[0] != r[2] || int(r[3]-r[1]) != len(name) {
		return ""
	}
	return name
}
